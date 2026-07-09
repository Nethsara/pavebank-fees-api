package bill

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"encore.app/bill/billworkflow"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
)

func newTestService() *Service {
	return &Service{}
}

func TestMapBillWorkflowErr_BillNotOpen(t *testing.T) {
	s := newTestService()
	appErr := temporal.NewApplicationError("bill is not open: b1", billworkflow.ErrTypeBillNotOpen)

	err := s.mapBillWorkflowErr(context.Background(), "b1", appErr)

	require.Error(t, err)
	require.Contains(t, err.Error(), "aborted")
}

func TestMapBillWorkflowErr_CurrencyMismatch(t *testing.T) {
	s := newTestService()
	appErr := temporal.NewApplicationError("currency mismatch", billworkflow.ErrTypeCurrencyMismatch)

	err := s.mapBillWorkflowErr(context.Background(), "b1", appErr)

	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid_argument")
}

func TestMapBillWorkflowErr_InvalidLineItem(t *testing.T) {
	s := newTestService()
	appErr := temporal.NewApplicationError("invalid line item", billworkflow.ErrTypeInvalidLineItem)

	err := s.mapBillWorkflowErr(context.Background(), "b1", appErr)

	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid_argument")
}

func TestMapBillWorkflowErr_LineItemNotFound(t *testing.T) {
	s := newTestService()
	appErr := temporal.NewApplicationError("line item not found", billworkflow.ErrTypeLineItemNotFound)

	err := s.mapBillWorkflowErr(context.Background(), "b1", appErr)

	require.Error(t, err)
	require.Contains(t, err.Error(), "not_found")
}

func TestMapBillWorkflowErr_IdempotencyKeyReused(t *testing.T) {
	s := newTestService()
	appErr := temporal.NewApplicationError("idempotency key already used", billworkflow.ErrTypeIdempotencyKeyReused)

	err := s.mapBillWorkflowErr(context.Background(), "b1", appErr)

	require.Error(t, err)
	require.Contains(t, err.Error(), "aborted")
}

func TestMapBillWorkflowErr_UnknownApplicationErrorTypeIsInternal(t *testing.T) {
	s := newTestService()
	appErr := temporal.NewApplicationError("boom", "SomeUnmappedType")

	err := s.mapBillWorkflowErr(context.Background(), "b1", appErr)

	require.Error(t, err)
	require.Contains(t, err.Error(), "internal")
}

func TestMapBillWorkflowErr_UnrecognizedErrorIsUnavailable(t *testing.T) {
	s := newTestService()

	err := s.mapBillWorkflowErr(context.Background(), "b1", errors.New("connection refused"))

	require.Error(t, err)
	require.Contains(t, err.Error(), "unavailable")
}

func TestIsValidBillID(t *testing.T) {
	valid := []string{"acme-2026-07", "a", "A_b.c-1", "550e8400-e29b-41d4-a716-446655440000"}
	for _, id := range valid {
		require.True(t, isValidBillID(id), "expected %q to be valid", id)
	}

	invalid := []string{"", "has a space", "has/a/slash", "tab\ttab", "emoji😀", strings.Repeat("a", 129)}
	for _, id := range invalid {
		require.False(t, isValidBillID(id), "expected %q to be invalid", id)
	}
}

func TestBillMatchesCreateRequest_IdenticalDataMatches(t *testing.T) {
	periodEnd := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	existing := &BillResponse{Currency: "USD", PeriodEnd: periodEnd, Reference: "customer-42"}
	req := &CreateBillRequest{Currency: "USD", PeriodEnd: periodEnd, Reference: "customer-42"}

	require.True(t, billMatchesCreateRequest(existing, req))
}

func TestBillMatchesCreateRequest_EquivalentTimeInDifferentZoneMatches(t *testing.T) {
	// A retry that re-serializes the same instant in a different offset must still match; this
	// is why the comparison uses time.Time.Equal, not ==.
	periodEnd := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	existing := &BillResponse{Currency: "USD", PeriodEnd: periodEnd, Reference: "customer-42"}
	req := &CreateBillRequest{Currency: "USD", PeriodEnd: periodEnd.In(time.FixedZone("UTC-5", -5*60*60)), Reference: "customer-42"}

	require.True(t, billMatchesCreateRequest(existing, req))
}

func TestBillMatchesCreateRequest_DifferentCurrencyMismatches(t *testing.T) {
	periodEnd := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	existing := &BillResponse{Currency: "USD", PeriodEnd: periodEnd, Reference: "customer-42"}
	req := &CreateBillRequest{Currency: "GEL", PeriodEnd: periodEnd, Reference: "customer-42"}

	require.False(t, billMatchesCreateRequest(existing, req))
}

func TestBillMatchesCreateRequest_DifferentPeriodEndMismatches(t *testing.T) {
	existing := &BillResponse{Currency: "USD", PeriodEnd: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), Reference: "customer-42"}
	req := &CreateBillRequest{Currency: "USD", PeriodEnd: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Reference: "customer-42"}

	require.False(t, billMatchesCreateRequest(existing, req))
}

func TestBillMatchesCreateRequest_DifferentReferenceMismatches(t *testing.T) {
	periodEnd := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	existing := &BillResponse{Currency: "USD", PeriodEnd: periodEnd, Reference: "customer-42"}
	req := &CreateBillRequest{Currency: "USD", PeriodEnd: periodEnd, Reference: "customer-99"}

	require.False(t, billMatchesCreateRequest(existing, req))
}
