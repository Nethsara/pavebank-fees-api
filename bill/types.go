package bill

import (
	"time"

	"encore.app/bill/billworkflow"
	"encore.app/bill/money"
)

type CreateBillRequest struct {
	BillID    string    `json:"billId,omitempty"`
	Currency  string    `json:"currency"`
	Reference string 	`json:"reference,omitempty"`
	PeriodEnd time.Time `json:"periodEnd"`
}

type AddLineItemRequest struct {
	Description    string `json:"description"`
	Currency       string `json:"currency"`
	Amount         string `json:"amount"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
}

type MoneyResponse struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}

type LineItemResponse struct {
	ID          string        `json:"id"`
	Description string        `json:"description"`
	Amount      MoneyResponse `json:"amount"`
	AddedAt     time.Time     `json:"addedAt"`
	Voided      bool          `json:"voided"`
	VoidedAt    *time.Time    `json:"voidedAt,omitempty"`
}

type BillResponse struct {
	BillID       string             `json:"billId"`
	Currency     string             `json:"currency"`
	Reference    string             `json:"reference,omitempty"`
	Status       string             `json:"status"`
	Total        MoneyResponse      `json:"total"`
	LineItems    []LineItemResponse `json:"lineItems"`
	PeriodEnd    time.Time          `json:"periodEnd"`
	CreatedAt    time.Time          `json:"createdAt"`
	ClosedAt     *time.Time         `json:"closedAt,omitempty"`
	ClosedReason string             `json:"closedReason,omitempty"`
}

type AddLineItemResponse struct {
	LineItem     LineItemResponse `json:"lineItem"`
	RunningTotal MoneyResponse    `json:"runningTotal"`
}

type VoidLineItemResponse struct {
	LineItem     LineItemResponse `json:"lineItem"`
	RunningTotal MoneyResponse    `json:"runningTotal"`
}

func moneyResponse(m money.Money) MoneyResponse {
	return MoneyResponse{Currency: string(m.Currency), Amount: m.DecimalString()}
}

func lineItemResponse(li billworkflow.LineItem) LineItemResponse {
	return LineItemResponse{
		ID:          li.ID,
		Description: li.Description,
		Amount:      moneyResponse(li.Amount),
		AddedAt:     li.AddedAt,
		Voided:      li.Voided,
		VoidedAt:    li.VoidedAt,
	}
}

func billResponse(s billworkflow.BillState) *BillResponse {
	items := make([]LineItemResponse, len(s.LineItems))
	for i, li := range s.LineItems {
		items[i] = lineItemResponse(li)
	}
	return &BillResponse{
		BillID:       s.BillID,
		Currency:     string(s.Currency),
		Reference:    s.Reference,
		Status:       string(s.Status),
		Total:        moneyResponse(s.Total),
		LineItems:    items,
		PeriodEnd:    s.PeriodEnd,
		CreatedAt:    s.CreatedAt,
		ClosedAt:     s.ClosedAt,
		ClosedReason: string(s.ClosedReason),
	}
}