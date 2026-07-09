package billworkflow

import (
	"math"
	"testing"

	"encore.app/bill/money"
	"github.com/stretchr/testify/require"
)

func TestValidateAddLineItem_RejectsClosedBill(t *testing.T) {
	state := BillState{BillID: "b1", Currency: money.USD, Status: StatusClosed}
	err := validateAddLineItem(state, AddLineItemInput{
		Description: "late fee", Amount: amount(t, money.USD, "1.00"),
	})
	require.ErrorIs(t, err, ErrBillNotOpen)
}

func TestValidateAddLineItem_RejectsCurrencyMismatch(t *testing.T) {
	state := BillState{BillID: "b1", Currency: money.USD, Status: StatusOpen}
	err := validateAddLineItem(state, AddLineItemInput{
		Description: "wrong currency", Amount: amount(t, money.GEL, "1.00"),
	})
	require.ErrorIs(t, err, ErrCurrencyMismatch)
}

func TestValidateAddLineItem_RejectsEmptyDescription(t *testing.T) {
	state := BillState{BillID: "b1", Currency: money.USD, Status: StatusOpen}
	err := validateAddLineItem(state, AddLineItemInput{
		Description: "", Amount: amount(t, money.USD, "1.00"),
	})
	require.ErrorIs(t, err, ErrInvalidLineItem)
}

func TestValidateAddLineItem_RejectsNonPositiveAmount(t *testing.T) {
	state := BillState{BillID: "b1", Currency: money.USD, Status: StatusOpen}
	err := validateAddLineItem(state, AddLineItemInput{
		Description: "zero fee", Amount: money.Money{Currency: money.USD, MinorUnits: 0},
	})
	require.ErrorIs(t, err, ErrInvalidLineItem)
}

func TestValidateAddLineItem_AcceptsValidInput(t *testing.T) {
	state := BillState{
		BillID: "b1", Currency: money.USD, Status: StatusOpen,
		Total: money.Money{Currency: money.USD, MinorUnits: 0},
	}
	err := validateAddLineItem(state, AddLineItemInput{
		Description: "API calls", Amount: amount(t, money.USD, "10.50"),
	})
	require.NoError(t, err)
}

func TestValidateAddLineItem_RejectsOverflow(t *testing.T) {
	state := BillState{
		BillID: "b1", Currency: money.USD, Status: StatusOpen,
		Total: money.Money{Currency: money.USD, MinorUnits: math.MaxInt64},
	}
	err := validateAddLineItem(state, AddLineItemInput{
		Description: "tips it over", Amount: money.Money{Currency: money.USD, MinorUnits: 1},
	})
	require.ErrorIs(t, err, ErrInvalidLineItem)
}

func TestValidateCloseBill_RejectsAlreadyClosed(t *testing.T) {
	state := BillState{BillID: "b1", Currency: money.USD, Status: StatusClosed}
	err := validateCloseBill(state)
	require.ErrorIs(t, err, ErrBillNotOpen)
}

func TestValidateCloseBill_AcceptsOpenBill(t *testing.T) {
	state := BillState{BillID: "b1", Currency: money.USD, Status: StatusOpen}
	require.NoError(t, validateCloseBill(state))
}

func TestValidateVoidLineItem_RejectsClosedBill(t *testing.T) {
	state := BillState{
		BillID: "b1", Currency: money.USD, Status: StatusClosed,
		LineItems: []LineItem{{ID: "li-1", Amount: amount(t, money.USD, "1.00")}},
	}
	err := validateVoidLineItem(state, VoidLineItemInput{LineItemID: "li-1"})
	require.ErrorIs(t, err, ErrBillNotOpen)
}

func TestValidateVoidLineItem_RejectsUnknownLineItem(t *testing.T) {
	state := BillState{BillID: "b1", Currency: money.USD, Status: StatusOpen}
	err := validateVoidLineItem(state, VoidLineItemInput{LineItemID: "li-999"})
	require.ErrorIs(t, err, ErrLineItemNotFound)
}

func TestValidateVoidLineItem_AcceptsExistingLineItem(t *testing.T) {
	state := BillState{
		BillID: "b1", Currency: money.USD, Status: StatusOpen,
		LineItems: []LineItem{{ID: "li-1", Amount: amount(t, money.USD, "1.00")}},
	}
	require.NoError(t, validateVoidLineItem(state, VoidLineItemInput{LineItemID: "li-1"}))
}

func TestValidateVoidLineItem_AcceptsAlreadyVoidedLineItem(t *testing.T) {
	state := BillState{
		BillID: "b1", Currency: money.USD, Status: StatusOpen,
		LineItems: []LineItem{{ID: "li-1", Amount: amount(t, money.USD, "1.00"), Voided: true}},
	}
	require.NoError(t, validateVoidLineItem(state, VoidLineItemInput{LineItemID: "li-1"}))
}
