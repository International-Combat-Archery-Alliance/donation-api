package api

import (
	"context"
	"iter"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/International-Combat-Archery-Alliance/payments"
	"github.com/Rhymond/go-money"
)

// MockPaymentQuerier is a mock implementation of payments.PaymentQuerier for admin tests
type MockPaymentQuerier struct {
	Payments []payments.Payment
	Err      error
}

func (m *MockPaymentQuerier) ListCharges(ctx context.Context, params payments.ChargeListParams) iter.Seq2[payments.Payment, error] {
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

func (m *MockPaymentQuerier) ListChargesPaginated(ctx context.Context, params payments.ChargeListPaginatedParams) (payments.ChargesPage, error) {
	// For tests, just return all payments as a single page
	return payments.ChargesPage{
		Payments:   m.Payments,
		HasMore:    false,
		NextCursor: "",
	}, m.Err
}

func TestGetDonationsV1PerState_Success(t *testing.T) {
	querier := &MockPaymentQuerier{
		Payments: []payments.Payment{
			{
				ID:     "pay_1",
				Amount: money.New(5000, "USD"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "US",
						State:   "MA",
					},
				},
			},
			{
				ID:     "pay_2",
				Amount: money.New(10000, "USD"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "US",
						State:   "MA",
					},
				},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(nil, querier, nil, "https://example.com/return", logger, LOCAL)

	request := GetDonationsV1PerStateRequestObject{
		Params: GetDonationsV1PerStateParams{},
	}

	response, err := api.GetDonationsV1PerState(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successResponse, ok := response.(GetDonationsV1PerState200JSONResponse)
	if !ok {
		t.Fatalf("expected GetDonationsV1PerState200JSONResponse, got %T", response)
	}

	if len(successResponse.Aggregations) != 1 {
		t.Errorf("expected 1 aggregation, got %d", len(successResponse.Aggregations))
	}

	agg := successResponse.Aggregations[0]
	if agg.Country != "US" {
		t.Errorf("expected country US, got %s", agg.Country)
	}
	if agg.State != "MA" {
		t.Errorf("expected state MA, got %s", agg.State)
	}
	if agg.Count != 2 {
		t.Errorf("expected count 2, got %d", agg.Count)
	}
	if agg.Money.Amount != 15000 {
		t.Errorf("expected amount 15000, got %d", agg.Money.Amount)
	}
	if agg.Money.Currency != "USD" {
		t.Errorf("expected currency USD, got %s", agg.Money.Currency)
	}
}

func TestGetDonationsV1PerState_InvalidDateRange(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(nil, &MockPaymentQuerier{}, nil, "https://example.com/return", logger, LOCAL)

	request := GetDonationsV1PerStateRequestObject{
		Params: GetDonationsV1PerStateParams{
			CreatedAfter:  &now,
			CreatedBefore: &yesterday,
		},
	}

	response, err := api.GetDonationsV1PerState(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	errorResponse, ok := response.(GetDonationsV1PerState400JSONResponse)
	if !ok {
		t.Fatalf("expected GetDonationsV1PerState400JSONResponse, got %T", response)
	}

	if errorResponse.Code != InvalidDateRange {
		t.Errorf("expected error code InvalidDateRange, got %s", errorResponse.Code)
	}
}

func TestGetDonationsV1PerState_WithDateRange(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	querier := &MockPaymentQuerier{
		Payments: []payments.Payment{
			{
				ID:     "pay_1",
				Amount: money.New(5000, "USD"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "US",
						State:   "MA",
					},
				},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(nil, querier, nil, "https://example.com/return", logger, LOCAL)

	request := GetDonationsV1PerStateRequestObject{
		Params: GetDonationsV1PerStateParams{
			CreatedAfter:  &yesterday,
			CreatedBefore: &tomorrow,
		},
	}

	response, err := api.GetDonationsV1PerState(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successResponse, ok := response.(GetDonationsV1PerState200JSONResponse)
	if !ok {
		t.Fatalf("expected GetDonationsV1PerState200JSONResponse, got %T", response)
	}

	if len(successResponse.Aggregations) != 1 {
		t.Errorf("expected 1 aggregation, got %d", len(successResponse.Aggregations))
	}
}

func TestGetDonationsV1PerState_OnlyCreatedAfter(t *testing.T) {
	yesterday := time.Now().Add(-24 * time.Hour)

	querier := &MockPaymentQuerier{
		Payments: []payments.Payment{
			{
				ID:     "pay_1",
				Amount: money.New(5000, "USD"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "US",
						State:   "MA",
					},
				},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(nil, querier, nil, "https://example.com/return", logger, LOCAL)

	request := GetDonationsV1PerStateRequestObject{
		Params: GetDonationsV1PerStateParams{
			CreatedAfter: &yesterday,
		},
	}

	response, err := api.GetDonationsV1PerState(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successResponse, ok := response.(GetDonationsV1PerState200JSONResponse)
	if !ok {
		t.Fatalf("expected GetDonationsV1PerState200JSONResponse, got %T", response)
	}

	if len(successResponse.Aggregations) != 1 {
		t.Errorf("expected 1 aggregation, got %d", len(successResponse.Aggregations))
	}
}

func TestGetDonationsV1PerState_OnlyCreatedBefore(t *testing.T) {
	tomorrow := time.Now().Add(24 * time.Hour)

	querier := &MockPaymentQuerier{
		Payments: []payments.Payment{
			{
				ID:     "pay_1",
				Amount: money.New(5000, "USD"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "US",
						State:   "MA",
					},
				},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(nil, querier, nil, "https://example.com/return", logger, LOCAL)

	request := GetDonationsV1PerStateRequestObject{
		Params: GetDonationsV1PerStateParams{
			CreatedBefore: &tomorrow,
		},
	}

	response, err := api.GetDonationsV1PerState(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successResponse, ok := response.(GetDonationsV1PerState200JSONResponse)
	if !ok {
		t.Fatalf("expected GetDonationsV1PerState200JSONResponse, got %T", response)
	}

	if len(successResponse.Aggregations) != 1 {
		t.Errorf("expected 1 aggregation, got %d", len(successResponse.Aggregations))
	}
}

func TestGetDonationsV1PerState_NoPayments(t *testing.T) {
	querier := &MockPaymentQuerier{
		Payments: []payments.Payment{},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(nil, querier, nil, "https://example.com/return", logger, LOCAL)

	request := GetDonationsV1PerStateRequestObject{
		Params: GetDonationsV1PerStateParams{},
	}

	response, err := api.GetDonationsV1PerState(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successResponse, ok := response.(GetDonationsV1PerState200JSONResponse)
	if !ok {
		t.Fatalf("expected GetDonationsV1PerState200JSONResponse, got %T", response)
	}

	if len(successResponse.Aggregations) != 0 {
		t.Errorf("expected 0 aggregations, got %d", len(successResponse.Aggregations))
	}
}

func TestGetDonationsV1PerState_MultipleStatesAndCurrencies(t *testing.T) {
	querier := &MockPaymentQuerier{
		Payments: []payments.Payment{
			{
				ID:     "pay_1",
				Amount: money.New(5000, "USD"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "US",
						State:   "MA",
					},
				},
			},
			{
				ID:     "pay_2",
				Amount: money.New(10000, "USD"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "US",
						State:   "CA",
					},
				},
			},
			{
				ID:     "pay_3",
				Amount: money.New(8000, "EUR"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "US",
						State:   "MA",
					},
				},
			},
			{
				ID:     "pay_4",
				Amount: money.New(3000, "CAD"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "CA",
						State:   "ON",
					},
				},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(nil, querier, nil, "https://example.com/return", logger, LOCAL)

	request := GetDonationsV1PerStateRequestObject{
		Params: GetDonationsV1PerStateParams{},
	}

	response, err := api.GetDonationsV1PerState(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successResponse, ok := response.(GetDonationsV1PerState200JSONResponse)
	if !ok {
		t.Fatalf("expected GetDonationsV1PerState200JSONResponse, got %T", response)
	}

	// Should have 4 aggregations: US:MA:USD, US:MA:EUR, US:CA:USD, CA:ON:CAD
	if len(successResponse.Aggregations) != 4 {
		t.Errorf("expected 4 aggregations, got %d", len(successResponse.Aggregations))
	}

	// Verify we have all expected combinations
	expectedCombos := map[string]bool{
		"US:MA:USD": false,
		"US:MA:EUR": false,
		"US:CA:USD": false,
		"CA:ON:CAD": false,
	}

	for _, agg := range successResponse.Aggregations {
		key := agg.Country + ":" + agg.State + ":" + agg.Money.Currency
		expectedCombos[key] = true
	}

	for combo, found := range expectedCombos {
		if !found {
			t.Errorf("expected aggregation for %s not found", combo)
		}
	}
}

func TestGetDonationsV1PerState_WithNoAddress(t *testing.T) {
	querier := &MockPaymentQuerier{
		Payments: []payments.Payment{
			{
				ID:             "pay_1",
				Amount:         money.New(5000, "USD"),
				BillingDetails: nil,
			},
			{
				ID:     "pay_2",
				Amount: money.New(3000, "USD"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "US",
						State:   "NY",
					},
				},
			},
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(nil, querier, nil, "https://example.com/return", logger, LOCAL)

	request := GetDonationsV1PerStateRequestObject{
		Params: GetDonationsV1PerStateParams{},
	}

	response, err := api.GetDonationsV1PerState(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successResponse, ok := response.(GetDonationsV1PerState200JSONResponse)
	if !ok {
		t.Fatalf("expected GetDonationsV1PerState200JSONResponse, got %T", response)
	}

	// Should have 2 aggregations: N/A:N/A and US:NY
	if len(successResponse.Aggregations) != 2 {
		t.Errorf("expected 2 aggregations, got %d", len(successResponse.Aggregations))
	}

	// Check for N/A aggregation
	var foundNA bool
	for _, agg := range successResponse.Aggregations {
		if agg.Country == "N/A" && agg.State == "N/A" {
			foundNA = true
			if agg.Count != 1 {
				t.Errorf("expected N/A count 1, got %d", agg.Count)
			}
		}
	}
	if !foundNA {
		t.Error("expected N/A:N/A aggregation for payment without address")
	}
}
