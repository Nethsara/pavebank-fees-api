package billworkflow

import (
	"testing"
	"time"

	"encore.app/bill/money"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func amount(t *testing.T, c money.Currency, s string) money.Money {
	t.Helper()
	m, err := money.ParseDecimal(c, s)
	require.NoError(t, err)
	return m
}


func sendUpdate(env *testsuite.TestWorkflowEnvironment, name string, resultOut *interface{}, errOut *error, args ...interface{}) {
	cb := &testsuite.TestUpdateCallback{
		OnAccept: func() {},
		OnReject: func(e error) {
			if errOut != nil {
				*errOut = e
			}
		},
		OnComplete: func(r interface{}, e error) {
			if resultOut != nil {
				*resultOut = r
			}
			if errOut != nil {
				*errOut = e
			}
		},
	}
	env.UpdateWorkflow(name, "", cb, args...)
}

func TestAddLineItemsAndManualClose(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	periodEnd := time.Now().Add(24 * time.Hour)
	in := CreateBillInput{BillID: "bill-1", Currency: money.USD, PeriodEnd: periodEnd}

	var item1Err, item2Err, closeErr error
	env.RegisterDelayedCallback(func() {
		sendUpdate(env, UpdateAddLineItem, nil, &item1Err, AddLineItemInput{
			Description: "API calls", Amount: amount(t, money.USD, "10.50"),
		})
	}, time.Minute)

	env.RegisterDelayedCallback(func() {
		sendUpdate(env, UpdateAddLineItem, nil, &item2Err, AddLineItemInput{
			Description: "Storage", Amount: amount(t, money.USD, "4.49"),
		})
	}, 2*time.Minute)

	env.RegisterDelayedCallback(func() {
		sendUpdate(env, UpdateCloseBill, nil, &closeErr, CloseBillInput{})
	}, 3*time.Minute)

	env.ExecuteWorkflow(BillWorkflow, in)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.NoError(t, item1Err)
	require.NoError(t, item2Err)
	require.NoError(t, closeErr)

	var result BillState
	require.NoError(t, env.GetWorkflowResult(&result))

	require.Equal(t, StatusClosed, result.Status)
	require.Equal(t, ClosedManual, result.ClosedReason)
	require.Len(t, result.LineItems, 2)
	require.Equal(t, "14.99", result.Total.DecimalString())
}

func TestCurrencyMismatchRejected(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	periodEnd := time.Now().Add(24 * time.Hour)
	in := CreateBillInput{BillID: "bill-3", Currency: money.USD, PeriodEnd: periodEnd}

	var mismatchErr, closeErr error
	env.RegisterDelayedCallback(func() {
		sendUpdate(env, UpdateAddLineItem, nil, &mismatchErr, AddLineItemInput{
			Description: "wrong currency", Amount: amount(t, money.GEL, "1.00"),
		})
	}, time.Minute)

	env.RegisterDelayedCallback(func() {
		sendUpdate(env, UpdateCloseBill, nil, &closeErr, CloseBillInput{})
	}, 2*time.Minute)

	env.ExecuteWorkflow(BillWorkflow, in)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.NoError(t, closeErr)

	require.Error(t, mismatchErr)
	require.ErrorContains(t, mismatchErr, ErrCurrencyMismatch.Error())

	var result BillState
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Empty(t, result.LineItems, "the rejected line item must not have been added")
	require.Equal(t, "0.00", result.Total.DecimalString())
}

func TestIdempotentAddLineItem(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	periodEnd := time.Now().Add(24 * time.Hour)
	in := CreateBillInput{BillID: "bill-4", Currency: money.USD, PeriodEnd: periodEnd}

	const key = "retry-key-1"

	var firstErr, retryErr, closeErr error
	env.RegisterDelayedCallback(func() {
		sendUpdate(env, UpdateAddLineItem, nil, &firstErr, AddLineItemInput{
			IdempotencyKey: key, Description: "one-time fee", Amount: amount(t, money.USD, "5.00"),
		})
	}, time.Minute)

	
	env.RegisterDelayedCallback(func() {
		sendUpdate(env, UpdateAddLineItem, nil, &retryErr, AddLineItemInput{
			IdempotencyKey: key, Description: "one-time fee", Amount: amount(t, money.USD, "5.00"),
		})
	}, 2*time.Minute)

	env.RegisterDelayedCallback(func() {
		sendUpdate(env, UpdateCloseBill, nil, &closeErr, CloseBillInput{})
	}, 3*time.Minute)

	env.ExecuteWorkflow(BillWorkflow, in)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.NoError(t, firstErr)
	require.NoError(t, retryErr)
	require.NoError(t, closeErr)

	var result BillState
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Len(t, result.LineItems, 1)
	require.Equal(t, "5.00", result.Total.DecimalString())
}

func TestAutoCloseOnPeriodEnd(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	periodEnd := time.Now().Add(time.Hour)
	in := CreateBillInput{BillID: "bill-5", Currency: money.GEL, PeriodEnd: periodEnd}

	var addErr error
	env.RegisterDelayedCallback(func() {
		sendUpdate(env, UpdateAddLineItem, nil, &addErr, AddLineItemInput{
			Description: "usage fee", Amount: amount(t, money.GEL, "2.00"),
		})
	}, 30*time.Minute)

	env.ExecuteWorkflow(BillWorkflow, in)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.NoError(t, addErr)

	var result BillState
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, StatusClosed, result.Status)
	require.Equal(t, ClosedPeriodEnd, result.ClosedReason)
	require.Equal(t, "2.00", result.Total.DecimalString())
}

func TestGetBillQuery(t *testing.T) {
	env := (&testsuite.WorkflowTestSuite{}).NewTestWorkflowEnvironment()

	periodEnd := time.Now().Add(24 * time.Hour)
	in := CreateBillInput{BillID: "bill-6", Currency: money.USD, PeriodEnd: periodEnd}

	var addErr, closeErr error
	env.RegisterDelayedCallback(func() {
		sendUpdate(env, UpdateAddLineItem, nil, &addErr, AddLineItemInput{
			Description: "fee", Amount: amount(t, money.USD, "1.00"),
		})
	}, time.Minute)

	env.RegisterDelayedCallback(func() {
		encoded, err := env.QueryWorkflow(QueryGetBill)
		require.NoError(t, err)
		var state BillState
		require.NoError(t, encoded.Get(&state))
		require.Equal(t, StatusOpen, state.Status)
		require.Len(t, state.LineItems, 1)
	}, 2*time.Minute)

	env.RegisterDelayedCallback(func() {
		sendUpdate(env, UpdateCloseBill, nil, &closeErr, CloseBillInput{})
	}, 3*time.Minute)

	env.ExecuteWorkflow(BillWorkflow, in)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.NoError(t, addErr)
	require.NoError(t, closeErr)
}
