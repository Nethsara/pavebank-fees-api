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
	ErrBillNotOpen      = errors.New("bill is not open")
	ErrCurrencyMismatch = errors.New("line item currency does not match bill currency")
	ErrInvalidLineItem  = errors.New("invalid line item")
)

const (
	UpdateAddLineItem = "addLineItem"
	UpdateCloseBill   = "closeBill"
	QueryGetBill      = "getBill"
)

const (
	ErrTypeBillNotOpen      = "BillNotOpen"
	ErrTypeCurrencyMismatch = "CurrencyMismatch"
	ErrTypeInvalidLineItem  = "InvalidLineItem"
)

type LineItem struct {
	ID          string      `json:"id"`
	Description string      `json:"description"`
	Amount      money.Money `json:"amount"`
	AddedAt     time.Time   `json:"addedAt"`
}

type CreateBillInput struct {
	BillID    string         `json:"billId"`
	Currency  money.Currency `json:"currency"`
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

type CloseBillInput struct{}

type BillState struct {
	BillID       string         `json:"billId"`
	Currency     money.Currency `json:"currency"`
	Status       Status         `json:"status"`
	Total        money.Money    `json:"total"`
	LineItems    []LineItem     `json:"lineItems"`
	PeriodEnd    time.Time      `json:"periodEnd"`
	CreatedAt    time.Time      `json:"createdAt"`
	ClosedAt     *time.Time     `json:"closedAt,omitempty"`
	ClosedReason ClosedReason   `json:"closedReason,omitempty"`
}