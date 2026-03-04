package api

import (
	"context"
	"iter"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/International-Combat-Archery-Alliance/donation-api/donations"
	"github.com/International-Combat-Archery-Alliance/payments"
	"github.com/Rhymond/go-money"
)

// MockPaymentQuerierWithPagination is a mock implementation of payments.PaymentQuerier that supports pagination
type MockPaymentQuerierWithPagination struct {
	Payments []payments.Payment
	Err      error
}

func (m *MockPaymentQuerierWithPagination) ListCharges(ctx context.Context, params payments.ChargeListParams) iter.Seq2[payments.Payment, error] {
	return func(yield func(payments.Payment, error) bool) {
		if m.Err != nil {
			yield(payments.Payment{}, m.Err)
			return
		}
		for _, payment := range m.Payments {
			if !yield(payment, nil) {
				return
			}
		}
	}
}

func (m *MockPaymentQuerierWithPagination) ListChargesPaginated(ctx context.Context, params payments.ChargeListPaginatedParams) (payments.ChargesPage, error) {
	if m.Err != nil {
		return payments.ChargesPage{}, m.Err
	}

	// Find starting index based on cursor
	startIdx := 0
	if params.Cursor != "" {
		for i, p := range m.Payments {
			if p.ID == params.Cursor {
				startIdx = i + 1
				break
			}
		}
	}

	// Calculate end index
	endIdx := startIdx + params.Limit
	if endIdx > len(m.Payments) {
		endIdx = len(m.Payments)
	}

	// Get page of payments
	pagePayments := m.Payments[startIdx:endIdx]

	// Determine if there are more results
	hasMore := endIdx < len(m.Payments)

	// Generate next cursor
	nextCursor := ""
	if hasMore && len(pagePayments) > 0 {
		nextCursor = pagePayments[len(pagePayments)-1].ID
	}

	return payments.ChargesPage{
		Payments:   pagePayments,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
}

func TestGetDonationsV1_Success(t *testing.T) {
	querier := &MockPaymentQuerierWithPagination{
		Payments: []payments.Payment{
			{
				ID:     "pay_1",
				Amount: money.New(5000, "USD"),
				Status: "succeeded",
				BillingDetails: &payments.BillingDetails{
					Email: "donor1@example.com",
					Name:  "John Doe",
					Address: &payments.Address{
						Country: "US",
						State:   "MA",
					},
				},
				Created:  time.Now(),
				Metadata: map[string]string{donations.MetadataKeyItemType: donations.MetadataValueDonation},
			},
			{
				ID:     "pay_2",
				Amount: money.New(10000, "USD"),
				Status: "succeeded",
				BillingDetails: &payments.BillingDetails{
					Email: "donor2@example.com",
					Name:  "Jane Smith",
					Address: &payments.Address{
						Country: "US",
						State:   "CA",
					},
				},
				Created:  time.Now(),
				Metadata: map[string]string{donations.MetadataKeyItemType: donations.MetadataValueDonation},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(nil, querier, nil, "https://example.com/return", logger, LOCAL)

	request := GetDonationsV1RequestObject{
		Params: GetDonationsV1Params{},
	}

	response, err := api.GetDonationsV1(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successResponse, ok := response.(GetDonationsV1200JSONResponse)
	if !ok {
		t.Fatalf("expected GetDonationsV1200JSONResponse, got %T", response)
	}

	if len(successResponse.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(successResponse.Items))
	}

	// Check first item
	if successResponse.Items[0].Id != "pay_1" {
		t.Errorf("expected first item ID pay_1, got %s", successResponse.Items[0].Id)
	}
	if successResponse.Items[0].Amount != 5000 {
		t.Errorf("expected first item amount 5000, got %d", successResponse.Items[0].Amount)
	}
	if successResponse.Items[0].Currency != "USD" {
		t.Errorf("expected first item currency USD, got %s", successResponse.Items[0].Currency)
	}
	if successResponse.Items[0].Status != "succeeded" {
		t.Errorf("expected first item status succeeded, got %s", successResponse.Items[0].Status)
	}

	// No next cursor since we got all results
	if successResponse.NextCursor != nil {
		t.Error("expected no next cursor")
	}
}

func TestGetDonationsV1_WithLimit(t *testing.T) {
	querier := &MockPaymentQuerierWithPagination{
		Payments: []payments.Payment{
			{ID: "pay_1", Amount: money.New(1000, "USD"), Status: "succeeded", Created: time.Now()},
			{ID: "pay_2", Amount: money.New(2000, "USD"), Status: "succeeded", Created: time.Now()},
			{ID: "pay_3", Amount: money.New(3000, "USD"), Status: "succeeded", Created: time.Now()},
			{ID: "pay_4", Amount: money.New(4000, "USD"), Status: "succeeded", Created: time.Now()},
			{ID: "pay_5", Amount: money.New(5000, "USD"), Status: "succeeded", Created: time.Now()},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(nil, querier, nil, "https://example.com/return", logger, LOCAL)

	limit := 2
	request := GetDonationsV1RequestObject{
		Params: GetDonationsV1Params{
			Limit: &limit,
		},
	}

	response, err := api.GetDonationsV1(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successResponse, ok := response.(GetDonationsV1200JSONResponse)
	if !ok {
		t.Fatalf("expected GetDonationsV1200JSONResponse, got %T", response)
	}

	if len(successResponse.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(successResponse.Items))
	}

	// Should have next cursor since there are more results
	if successResponse.NextCursor == nil {
		t.Error("expected next cursor")
	}
}

func TestGetDonationsV1_WithPagination(t *testing.T) {
	querier := &MockPaymentQuerierWithPagination{
		Payments: []payments.Payment{
			{ID: "pay_1", Amount: money.New(1000, "USD"), Status: "succeeded", Created: time.Now()},
			{ID: "pay_2", Amount: money.New(2000, "USD"), Status: "succeeded", Created: time.Now()},
			{ID: "pay_3", Amount: money.New(3000, "USD"), Status: "succeeded", Created: time.Now()},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(nil, querier, nil, "https://example.com/return", logger, LOCAL)

	// First page
	limit := 2
	request1 := GetDonationsV1RequestObject{
		Params: GetDonationsV1Params{
			Limit: &limit,
		},
	}

	response1, err := api.GetDonationsV1(context.Background(), request1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successResponse1, ok := response1.(GetDonationsV1200JSONResponse)
	if !ok {
		t.Fatalf("expected GetDonationsV1200JSONResponse, got %T", response1)
	}

	if len(successResponse1.Items) != 2 {
		t.Errorf("expected 2 items on first page, got %d", len(successResponse1.Items))
	}

	if successResponse1.NextCursor == nil {
		t.Fatal("expected next cursor for first page")
	}

	// Second page using cursor
	request2 := GetDonationsV1RequestObject{
		Params: GetDonationsV1Params{
			Limit:  &limit,
			Cursor: successResponse1.NextCursor,
		},
	}

	response2, err := api.GetDonationsV1(context.Background(), request2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successResponse2, ok := response2.(GetDonationsV1200JSONResponse)
	if !ok {
		t.Fatalf("expected GetDonationsV1200JSONResponse, got %T", response2)
	}

	if len(successResponse2.Items) != 1 {
		t.Errorf("expected 1 item on second page, got %d", len(successResponse2.Items))
	}

	if successResponse2.NextCursor != nil {
		t.Error("expected no next cursor on last page")
	}
}

func TestGetDonationsV1_WithDateFilter(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	querier := &MockPaymentQuerierWithPagination{
		Payments: []payments.Payment{
			{
				ID:      "pay_1",
				Amount:  money.New(5000, "USD"),
				Status:  "succeeded",
				Created: now,
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(nil, querier, nil, "https://example.com/return", logger, LOCAL)

	request := GetDonationsV1RequestObject{
		Params: GetDonationsV1Params{
			CreatedAfter:  &yesterday,
			CreatedBefore: &tomorrow,
		},
	}

	response, err := api.GetDonationsV1(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successResponse, ok := response.(GetDonationsV1200JSONResponse)
	if !ok {
		t.Fatalf("expected GetDonationsV1200JSONResponse, got %T", response)
	}

	if len(successResponse.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(successResponse.Items))
	}
}

func TestGetDonationsV1_NoPayments(t *testing.T) {
	querier := &MockPaymentQuerierWithPagination{
		Payments: []payments.Payment{},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(nil, querier, nil, "https://example.com/return", logger, LOCAL)

	request := GetDonationsV1RequestObject{
		Params: GetDonationsV1Params{},
	}

	response, err := api.GetDonationsV1(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successResponse, ok := response.(GetDonationsV1200JSONResponse)
	if !ok {
		t.Fatalf("expected GetDonationsV1200JSONResponse, got %T", response)
	}

	if len(successResponse.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(successResponse.Items))
	}

	if successResponse.NextCursor != nil {
		t.Error("expected no next cursor")
	}
}

func TestGetDonationsV1_LimitExceedsMax(t *testing.T) {
	querier := &MockPaymentQuerierWithPagination{
		Payments: []payments.Payment{
			{ID: "pay_1", Amount: money.New(1000, "USD"), Status: "succeeded", Created: time.Now()},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(nil, querier, nil, "https://example.com/return", logger, LOCAL)

	// Try to request more than max (100)
	limit := 200
	request := GetDonationsV1RequestObject{
		Params: GetDonationsV1Params{
			Limit: &limit,
		},
	}

	response, err := api.GetDonationsV1(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successResponse, ok := response.(GetDonationsV1200JSONResponse)
	if !ok {
		t.Fatalf("expected GetDonationsV1200JSONResponse, got %T", response)
	}

	// Should still return the item (limit capped at 100)
	if len(successResponse.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(successResponse.Items))
	}
}
