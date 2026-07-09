package billworkflow

import (
	"errors"
	"fmt"

	"encore.app/bill/money"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func toApplicationError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrBillNotOpen):
		return temporal.NewApplicationError(err.Error(), ErrTypeBillNotOpen)
	case errors.Is(err, ErrCurrencyMismatch):
		return temporal.NewApplicationError(err.Error(), ErrTypeCurrencyMismatch)
	case errors.Is(err, ErrInvalidLineItem):
		return temporal.NewApplicationError(err.Error(), ErrTypeInvalidLineItem)
	case errors.Is(err, ErrLineItemNotFound):
		return temporal.NewApplicationError(err.Error(), ErrTypeLineItemNotFound)
	case errors.Is(err, ErrIdempotencyKeyReused):
		return temporal.NewApplicationError(err.Error(), ErrTypeIdempotencyKeyReused)
	default:
		return err
	}
}

func validateAddLineItem(state BillState, in AddLineItemInput) error {
	if state.Status != StatusOpen {
		return fmt.Errorf("%w: bill %s", ErrBillNotOpen, state.BillID)
	}
	if in.Description == "" {
		return fmt.Errorf("%w: description is required", ErrInvalidLineItem)
	}
	if in.Amount.Currency != state.Currency {
		return fmt.Errorf("%w: bill is %s, line item is %s", ErrCurrencyMismatch, state.Currency, in.Amount.Currency)
	}
	if in.Amount.MinorUnits <= 0 {
		return fmt.Errorf("%w: amount must be positive", ErrInvalidLineItem)
	}
	if _, err := state.Total.Add(in.Amount); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidLineItem, err)
	}
	return nil
}

func validateCloseBill(state BillState) error {
	if state.Status != StatusOpen {
		return fmt.Errorf("%w: bill %s is already closed", ErrBillNotOpen, state.BillID)
	}
	return nil
}

func validateVoidLineItem(state BillState, in VoidLineItemInput) error {
	if state.Status != StatusOpen {
		return fmt.Errorf("%w: bill %s", ErrBillNotOpen, state.BillID)
	}
	for _, li := range state.LineItems {
		if li.ID == in.LineItemID {
			return nil
		}
	}
	return fmt.Errorf("%w: %q on bill %s", ErrLineItemNotFound, in.LineItemID, state.BillID)
}

func recomputeTotal(currency money.Currency, items []LineItem) (money.Money, error) {
	total := money.Money{Currency: currency, MinorUnits: 0}
	for _, li := range items {
		if li.Voided {
			continue
		}
		var err error
		total, err = total.Add(li.Amount)
		if err != nil {
			return money.Money{}, err
		}
	}
	return total, nil
}

func BillWorkflow(ctx workflow.Context, in CreateBillInput) (BillState, error) {
	state := BillState{
		BillID:    in.BillID,
		Currency:  in.Currency,
		Reference: in.Reference,
		Status:    StatusOpen,
		Total:     money.Money{Currency: in.Currency, MinorUnits: 0},
		LineItems: []LineItem{},
		PeriodEnd: in.PeriodEnd,
		CreatedAt: workflow.Now(ctx),
	}

	seen := make(map[string]idempotentAddLineItem)
	closeRequested := false

	err := workflow.SetUpdateHandlerWithOptions(ctx, UpdateAddLineItem,
		func(ctx workflow.Context, in AddLineItemInput) (AddLineItemResult, error) {
			if in.IdempotencyKey != "" {
				if cached, ok := seen[in.IdempotencyKey]; ok {
					if cached.Description != in.Description || cached.Amount != in.Amount {
						err := fmt.Errorf("%w: %q", ErrIdempotencyKeyReused, in.IdempotencyKey)
						return AddLineItemResult{}, toApplicationError(err)
					}
					return cached.Result, nil
				}
			}

			item := LineItem{
				ID:          fmt.Sprintf("li-%d", len(state.LineItems)+1),
				Description: in.Description,
				Amount:      in.Amount,
				AddedAt:     workflow.Now(ctx),
			}

			newTotal, err := state.Total.Add(in.Amount)
			if err != nil {
				return AddLineItemResult{}, err
			}

			state.Total = newTotal
			state.LineItems = append(state.LineItems, item)

			result := AddLineItemResult{LineItem: item, RunningTotal: state.Total}
			if in.IdempotencyKey != "" {
				seen[in.IdempotencyKey] = idempotentAddLineItem{
					Description: in.Description,
					Amount:      in.Amount,
					Result:      result,
				}
			}
			return result, nil
		},
		workflow.UpdateHandlerOptions{
			Validator: func(ctx workflow.Context, in AddLineItemInput) error {
				return toApplicationError(validateAddLineItem(state, in))
			},
		},
	)
	if err != nil {
		return BillState{}, err
	}

	err = workflow.SetUpdateHandlerWithOptions(ctx, UpdateVoidLineItem,
		func(ctx workflow.Context, in VoidLineItemInput) (VoidLineItemResult, error) {
			for i := range state.LineItems {
				li := &state.LineItems[i]
				if li.ID != in.LineItemID {
					continue
				}
				if li.Voided {
					return VoidLineItemResult{LineItem: *li, RunningTotal: state.Total}, nil
				}

				li.Voided = true
				now := workflow.Now(ctx)
				li.VoidedAt = &now

				newTotal, err := recomputeTotal(state.Currency, state.LineItems)
				if err != nil {
					return VoidLineItemResult{}, err
				}
				state.Total = newTotal

				return VoidLineItemResult{LineItem: *li, RunningTotal: state.Total}, nil
			}
			return VoidLineItemResult{}, toApplicationError(fmt.Errorf("%w: %q", ErrLineItemNotFound, in.LineItemID))
		},
		workflow.UpdateHandlerOptions{
			Validator: func(ctx workflow.Context, in VoidLineItemInput) error {
				return toApplicationError(validateVoidLineItem(state, in))
			},
		},
	)
	if err != nil {
		return BillState{}, err
	}

	err = workflow.SetUpdateHandlerWithOptions(ctx, UpdateCloseBill,
		func(ctx workflow.Context, _ CloseBillInput) (BillState, error) {
			closeRequested = true
			state.Status = StatusClosed
			state.ClosedReason = ClosedManual
			now := workflow.Now(ctx)
			state.ClosedAt = &now
			return state, nil
		},
		workflow.UpdateHandlerOptions{
			Validator: func(ctx workflow.Context, _ CloseBillInput) error {
				return toApplicationError(validateCloseBill(state))
			},
		},
	)
	if err != nil {
		return BillState{}, err
	}

	err = workflow.SetQueryHandler(ctx, QueryGetBill, func() (BillState, error) {
		return state, nil
	})
	if err != nil {
		return BillState{}, err
	}

	deadline := in.PeriodEnd.Sub(workflow.Now(ctx))
	closedInTime, err := workflow.AwaitWithTimeout(ctx, deadline, func() bool { return closeRequested })
	if err != nil {
		return BillState{}, err
	}
	if !closedInTime {
		state.Status = StatusClosed
		state.ClosedReason = ClosedPeriodEnd
		now := workflow.Now(ctx)
		state.ClosedAt = &now
	}

	err = workflow.Await(ctx, func() bool { return workflow.AllHandlersFinished(ctx) })
	if err != nil {
		return BillState{}, err
	}

	return state, nil
}
