package billworkflow

import (
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
	state := BillState{BillID: "b1", Currency: money.USD, Status: StatusOpen}
	err := validateAddLineItem(state, AddLineItemInput{
		Description: "API calls", Amount: amount(t, money.USD, "10.50"),
	})
	require.NoError(t, err)
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