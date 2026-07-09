package bill

import (
	"context"
	"errors"
	"time"

	"encore.app/bill/billworkflow"
	"encore.app/bill/money"
	"encore.dev/beta/errs"
	"github.com/google/uuid"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

// CreateBill starts a new bill/workflow for a fee period.
//
//encore:api public method=POST path=/bills
func (s *Service) CreateBill(ctx context.Context, req *CreateBillRequest) (*BillResponse, error) {
	currency := money.Currency(req.Currency)
	if !money.IsSupported(currency) {
		return nil, errs.B().Code(errs.InvalidArgument).Msgf("unsupported currency %q", req.Currency).Err()
	}
	if req.PeriodEnd.IsZero() {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("periodEnd is required").Err()
	}
	if !req.PeriodEnd.After(time.Now()) {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("periodEnd must be in the future").Err()
	}

	billID := req.BillID
	if billID == "" {
		billID = uuid.NewString()
	} else if !isValidBillID(billID) {
		return nil, errs.B().Code(errs.InvalidArgument).Msgf("invalid billId %q: must match %s", billID, billIDPattern).Err()
	}

	in := billworkflow.CreateBillInput{BillID: billID, Currency: currency, Reference: req.Reference, PeriodEnd: req.PeriodEnd}
	_, err := s.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                                       billWorkflowID(billID),
		TaskQueue:                                billworkflow.TaskQueue,
		WorkflowIDReusePolicy:                    enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		WorkflowIDConflictPolicy:                 enums.WORKFLOW_ID_CONFLICT_POLICY_FAIL,
		WorkflowExecutionErrorWhenAlreadyStarted: true,
	}, billworkflow.BillWorkflow, in)
	if err != nil {
		var alreadyStarted *serviceerror.WorkflowExecutionAlreadyStarted
		if errors.As(err, &alreadyStarted) {
			return s.resolveBillAlreadyExists(ctx, billID, req)
		}
		return nil, errs.B().Code(errs.Internal).Cause(err).Msg("failed to start bill").Err()
	}

	return s.GetBill(ctx, billID)
}

func (s *Service) resolveBillAlreadyExists(ctx context.Context, billID string, req *CreateBillRequest) (*BillResponse, error) {
	existing, err := s.GetBill(ctx, billID)
	if err != nil {
		return nil, errs.B().Code(errs.AlreadyExists).Msgf("bill %q already exists", billID).Err()
	}

	if !billMatchesCreateRequest(existing, req) {
		return nil, errs.B().Code(errs.AlreadyExists).
			Msgf("bill %q already exists with a different currency, periodEnd, or reference", billID).Err()
	}

	return existing, nil
}

func billMatchesCreateRequest(existing *BillResponse, req *CreateBillRequest) bool {
	return existing.Currency == req.Currency &&
		existing.PeriodEnd.Equal(req.PeriodEnd) &&
		existing.Reference == req.Reference
}

// GetBill returns the current state of a bill, including its line items and running total.
//
//encore:api public method=GET path=/bills/:billID
func (s *Service) GetBill(ctx context.Context, billID string) (*BillResponse, error) {
	encoded, err := s.client.QueryWorkflow(ctx, billWorkflowID(billID), "", billworkflow.QueryGetBill)
	if err != nil {
		return nil, s.mapBillWorkflowErr(ctx, billID, err)
	}
	var state billworkflow.BillState
	if err := encoded.Get(&state); err != nil {
		return nil, errs.B().Code(errs.Internal).Cause(err).Msg("failed to decode bill state").Err()
	}
	return billResponse(state), nil
}

// AddLineItem appends a line item to an open bill. Rejected if the bill is closed.
//
//encore:api public method=POST path=/bills/:billID/line-items
func (s *Service) AddLineItem(ctx context.Context, billID string, req *AddLineItemRequest) (*AddLineItemResponse, error) {
	if req.Description == "" {
		return nil, errs.B().Code(errs.InvalidArgument).Msg("description is required").Err()
	}
	currency := money.Currency(req.Currency)
	amount, err := money.ParseDecimal(currency, req.Amount)
	if err != nil {
		return nil, errs.B().Code(errs.InvalidArgument).Cause(err).Msg("invalid amount").Err()
	}

	in := billworkflow.AddLineItemInput{
		IdempotencyKey: req.IdempotencyKey,
		Description:    req.Description,
		Amount:         amount,
	}

	handle, err := s.client.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		WorkflowID:   billWorkflowID(billID),
		UpdateName:   billworkflow.UpdateAddLineItem,
		WaitForStage: client.WorkflowUpdateStageCompleted,
		Args:         []interface{}{in},
	})
	if err != nil {
		return nil, s.mapBillWorkflowErr(ctx, billID, err)
	}

	var result billworkflow.AddLineItemResult
	if err := handle.Get(ctx, &result); err != nil {
		return nil, s.mapBillWorkflowErr(ctx, billID, err)
	}

	return &AddLineItemResponse{
		LineItem:     lineItemResponse(result.LineItem),
		RunningTotal: moneyResponse(result.RunningTotal),
	}, nil
}

// VoidLineItem soft-deletes a line item on an open bill and recomputes the total.
//
//encore:api public method=POST path=/bills/:billID/line-items/:lineItemID/void
func (s *Service) VoidLineItem(ctx context.Context, billID, lineItemID string) (*VoidLineItemResponse, error) {
	handle, err := s.client.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		WorkflowID:   billWorkflowID(billID),
		UpdateName:   billworkflow.UpdateVoidLineItem,
		WaitForStage: client.WorkflowUpdateStageCompleted,
		Args:         []interface{}{billworkflow.VoidLineItemInput{LineItemID: lineItemID}},
	})
	if err != nil {
		return nil, s.mapBillWorkflowErr(ctx, billID, err)
	}

	var result billworkflow.VoidLineItemResult
	if err := handle.Get(ctx, &result); err != nil {
		return nil, s.mapBillWorkflowErr(ctx, billID, err)
	}

	return &VoidLineItemResponse{
		LineItem:     lineItemResponse(result.LineItem),
		RunningTotal: moneyResponse(result.RunningTotal),
	}, nil
}

// CloseBill closes an open bill, finalizing the total and the set of line items charged.
//
//encore:api public method=POST path=/bills/:billID/close
func (s *Service) CloseBill(ctx context.Context, billID string) (*BillResponse, error) {
	handle, err := s.client.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		WorkflowID:   billWorkflowID(billID),
		UpdateName:   billworkflow.UpdateCloseBill,
		WaitForStage: client.WorkflowUpdateStageCompleted,
		Args:         []interface{}{billworkflow.CloseBillInput{}},
	})
	if err != nil {
		return nil, s.mapBillWorkflowErr(ctx, billID, err)
	}

	var state billworkflow.BillState
	if err := handle.Get(ctx, &state); err != nil {
		return nil, s.mapBillWorkflowErr(ctx, billID, err)
	}
	return billResponse(state), nil
}

func (s *Service) mapBillWorkflowErr(ctx context.Context, billID string, err error) error {
	var appErr *temporal.ApplicationError
	if errors.As(err, &appErr) {
		switch appErr.Type() {
		case billworkflow.ErrTypeBillNotOpen:
			return errs.B().Code(errs.Aborted).Msg(appErr.Message()).Err()
		case billworkflow.ErrTypeCurrencyMismatch, billworkflow.ErrTypeInvalidLineItem:
			return errs.B().Code(errs.InvalidArgument).Msg(appErr.Message()).Err()
		case billworkflow.ErrTypeLineItemNotFound:
			return errs.B().Code(errs.NotFound).Msg(appErr.Message()).Err()
		case billworkflow.ErrTypeIdempotencyKeyReused:
			return errs.B().Code(errs.Aborted).Msg(appErr.Message()).Err()
		default:
			return errs.B().Code(errs.Internal).Cause(appErr).Err()
		}
	}

	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		if _, queryErr := s.client.QueryWorkflow(ctx, billWorkflowID(billID), "", billworkflow.QueryGetBill); queryErr == nil {
			return errs.B().Code(errs.Aborted).Msgf("bill %q is already closed", billID).Err()
		}
		return errs.B().Code(errs.NotFound).Msgf("bill %q not found", billID).Err()
	}

	return errs.B().Code(errs.Unavailable).Cause(err).Msg("failed to reach bill workflow").Err()
}
