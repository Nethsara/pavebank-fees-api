package billworkflow

import (
	"errors"
	"time"

	"encore.app/bill/money"
)

const TaskQueue = "bill-task-queue"

type Status string

const (
	StatusOpen   Status = "OPEN"
	StatusClosed Status = "CLOSED"
)

type ClosedReason string

const (
	ClosedManual    ClosedReason = "manual"
	ClosedPeriodEnd ClosedReason = "period_end"
)

var (
	ErrBillNotOpen          = errors.New("bill is not open")
	ErrCurrencyMismatch     = errors.New("line item currency does not match bill currency")
	ErrInvalidLineItem      = errors.New("invalid line item")
	ErrLineItemNotFound     = errors.New("line item not found")
	ErrIdempotencyKeyReused = errors.New("idempotency key already used with a different request")
)

const (
	UpdateAddLineItem  = "addLineItem"
	UpdateVoidLineItem = "voidLineItem"
	UpdateCloseBill    = "closeBill"
	QueryGetBill       = "getBill"
)

const (
	ErrTypeBillNotOpen          = "BillNotOpen"
	ErrTypeCurrencyMismatch     = "CurrencyMismatch"
	ErrTypeInvalidLineItem      = "InvalidLineItem"
	ErrTypeLineItemNotFound     = "LineItemNotFound"
	ErrTypeIdempotencyKeyReused = "IdempotencyKeyReused"
)

type LineItem struct {
	ID          string      `json:"id"`
	Description string      `json:"description"`
	Amount      money.Money `json:"amount"`
	AddedAt     time.Time   `json:"addedAt"`
	Voided      bool        `json:"voided"`
	VoidedAt    *time.Time  `json:"voidedAt,omitempty"`
}

type CreateBillInput struct {
	BillID    string         `json:"billId"`
	Currency  money.Currency `json:"currency"`
	Reference string         `json:"reference,omitempty"`
	PeriodEnd time.Time      `json:"periodEnd"`
}

type AddLineItemInput struct {
	IdempotencyKey string      `json:"idempotencyKey"`
	Description    string      `json:"description"`
	Amount         money.Money `json:"amount"`
}

type AddLineItemResult struct {
	LineItem     LineItem    `json:"lineItem"`
	RunningTotal money.Money `json:"runningTotal"`
}

type idempotentAddLineItem struct {
	Description string
	Amount      money.Money
	Result      AddLineItemResult
}

type VoidLineItemInput struct {
	LineItemID string `json:"lineItemId"`
}

type VoidLineItemResult struct {
	LineItem     LineItem    `json:"lineItem"`
	RunningTotal money.Money `json:"runningTotal"`
}

type CloseBillInput struct{}

type BillState struct {
	BillID       string         `json:"billId"`
	Currency     money.Currency `json:"currency"`
	Reference    string         `json:"reference,omitempty"`
	Status       Status         `json:"status"`
	Total        money.Money    `json:"total"`
	LineItems    []LineItem     `json:"lineItems"`
	PeriodEnd    time.Time      `json:"periodEnd"`
	CreatedAt    time.Time      `json:"createdAt"`
	ClosedAt     *time.Time     `json:"closedAt,omitempty"`
	ClosedReason ClosedReason   `json:"closedReason,omitempty"`
}