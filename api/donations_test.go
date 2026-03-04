package api

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/International-Combat-Archery-Alliance/donation-api/donations"
	"github.com/International-Combat-Archery-Alliance/payments"
	"github.com/Rhymond/go-money"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// MockCheckoutManager is a mock implementation of payments.CheckoutManager
type MockCheckoutManager struct {
	CreateCheckoutFunc  func(ctx context.Context, params payments.CheckoutParams) (payments.CheckoutInfo, error)
	ConfirmCheckoutFunc func(ctx context.Context, payload []byte, signature string) (map[string]string, error)
}

func (m *MockCheckoutManager) CreateCheckout(ctx context.Context, params payments.CheckoutParams) (payments.CheckoutInfo, error) {
	if m.CreateCheckoutFunc != nil {
		return m.CreateCheckoutFunc(ctx, params)
	}
	return payments.CheckoutInfo{}, nil
}

func (m *MockCheckoutManager) ConfirmCheckout(ctx context.Context, payload []byte, signature string) (map[string]string, error) {
	if m.ConfirmCheckoutFunc != nil {
		return m.ConfirmCheckoutFunc(ctx, payload, signature)
	}
	return nil, nil
}

func TestPostDonationsV1_Success(t *testing.T) {
	expectedClientSecret := "secret_test_123"
	mockManager := &MockCheckoutManager{
		CreateCheckoutFunc: func(ctx context.Context, params payments.CheckoutParams) (payments.CheckoutInfo, error) {
			return payments.CheckoutInfo{
				ClientSecret: expectedClientSecret,
			}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(mockManager, nil, nil, "https://example.com/return", logger, LOCAL)

	request := PostDonationsV1RequestObject{
		Body: &PostDonationsV1JSONRequestBody{
			Amount:     5000,
			Currency:   "USD",
			DonorEmail: openapi_types.Email("donor@example.com"),
		},
	}

	response, err := api.PostDonationsV1(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successResponse, ok := response.(PostDonationsV1200JSONResponse)
	if !ok {
		t.Fatalf("expected PostDonationsV1200JSONResponse, got %T", response)
	}

	if successResponse.ClientSecret != expectedClientSecret {
		t.Errorf("expected client secret %s, got %s", expectedClientSecret, successResponse.ClientSecret)
	}
}

func TestPostDonationsV1_InvalidAmount(t *testing.T) {
	mockManager := &MockCheckoutManager{
		CreateCheckoutFunc: func(ctx context.Context, params payments.CheckoutParams) (payments.CheckoutInfo, error) {
			return payments.CheckoutInfo{}, donations.NewInvalidAmountError(50)
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(mockManager, nil, nil, "https://example.com/return", logger, LOCAL)

	request := PostDonationsV1RequestObject{
		Body: &PostDonationsV1JSONRequestBody{
			Amount:     50,
			Currency:   "USD",
			DonorEmail: openapi_types.Email("donor@example.com"),
		},
	}

	response, err := api.PostDonationsV1(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	errorResponse, ok := response.(PostDonationsV1400JSONResponse)
	if !ok {
		t.Fatalf("expected PostDonationsV1400JSONResponse, got %T", response)
	}

	if errorResponse.Code != BadRequest {
		t.Errorf("expected error code BadRequest, got %s", errorResponse.Code)
	}

	if errorResponse.Message == "" {
		t.Error("expected non-empty error message")
	}
}

func TestPostDonationsV1_CheckoutManagerError(t *testing.T) {
	mockManager := &MockCheckoutManager{
		CreateCheckoutFunc: func(ctx context.Context, params payments.CheckoutParams) (payments.CheckoutInfo, error) {
			return payments.CheckoutInfo{}, donations.NewFailedToCreateCheckoutError("Stripe API error", errors.New("connection failed"))
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(mockManager, nil, nil, "https://example.com/return", logger, LOCAL)

	request := PostDonationsV1RequestObject{
		Body: &PostDonationsV1JSONRequestBody{
			Amount:     5000,
			Currency:   "USD",
			DonorEmail: openapi_types.Email("donor@example.com"),
		},
	}

	response, err := api.PostDonationsV1(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	errorResponse, ok := response.(PostDonationsV1500JSONResponse)
	if !ok {
		t.Fatalf("expected PostDonationsV1500JSONResponse, got %T", response)
	}

	if errorResponse.Code != InternalError {
		t.Errorf("expected error code InternalError, got %s", errorResponse.Code)
	}

	if errorResponse.Message != "Failed to create donation checkout" {
		t.Errorf("expected message 'Failed to create donation checkout', got %s", errorResponse.Message)
	}
}

func TestPostDonationsV1_GenericError(t *testing.T) {
	// Test with a generic error that's not a donations.Error
	mockManager := &MockCheckoutManager{
		CreateCheckoutFunc: func(ctx context.Context, params payments.CheckoutParams) (payments.CheckoutInfo, error) {
			return payments.CheckoutInfo{}, errors.New("unexpected generic error")
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(mockManager, nil, nil, "https://example.com/return", logger, LOCAL)

	request := PostDonationsV1RequestObject{
		Body: &PostDonationsV1JSONRequestBody{
			Amount:     5000,
			Currency:   "USD",
			DonorEmail: openapi_types.Email("donor@example.com"),
		},
	}

	response, err := api.PostDonationsV1(context.Background(), request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	errorResponse, ok := response.(PostDonationsV1500JSONResponse)
	if !ok {
		t.Fatalf("expected PostDonationsV1500JSONResponse, got %T", response)
	}

	if errorResponse.Code != InternalError {
		t.Errorf("expected error code InternalError, got %s", errorResponse.Code)
	}
}

func TestPostDonationsV1_DifferentCurrencies(t *testing.T) {
	currencies := []string{"USD", "EUR", "GBP"}

	for _, currency := range currencies {
		t.Run(currency, func(t *testing.T) {
			mockManager := &MockCheckoutManager{
				CreateCheckoutFunc: func(ctx context.Context, params payments.CheckoutParams) (payments.CheckoutInfo, error) {
					if params.Items[0].Price.Currency().Code != currency {
						t.Errorf("expected currency %s, got %s", currency, params.Items[0].Price.Currency().Code)
					}
					return payments.CheckoutInfo{ClientSecret: "secret_" + currency}, nil
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			api := NewAPI(mockManager, nil, nil, "https://example.com/return", logger, LOCAL)

			request := PostDonationsV1RequestObject{
				Body: &PostDonationsV1JSONRequestBody{
					Amount:     10000,
					Currency:   currency,
					DonorEmail: openapi_types.Email("donor@example.com"),
				},
			}

			response, err := api.PostDonationsV1(context.Background(), request)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			successResponse, ok := response.(PostDonationsV1200JSONResponse)
			if !ok {
				t.Fatalf("expected PostDonationsV1200JSONResponse, got %T", response)
			}

			if successResponse.ClientSecret != "secret_"+currency {
				t.Errorf("expected client secret secret_%s, got %s", currency, successResponse.ClientSecret)
			}
		})
	}
}

func TestPostDonationsV1_VaryingAmounts(t *testing.T) {
	amounts := []int{100, 500, 1000, 5000, 10000}

	for _, amount := range amounts {
		t.Run(fmt.Sprintf("amount_%d", amount), func(t *testing.T) {
			mockManager := &MockCheckoutManager{
				CreateCheckoutFunc: func(ctx context.Context, params payments.CheckoutParams) (payments.CheckoutInfo, error) {
					if params.Items[0].Price.Amount() != int64(amount) {
						t.Errorf("expected amount %d, got %d", amount, params.Items[0].Price.Amount())
					}
					return payments.CheckoutInfo{ClientSecret: "secret_amount"}, nil
				},
			}

			logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
			api := NewAPI(mockManager, nil, nil, "https://example.com/return", logger, LOCAL)

			request := PostDonationsV1RequestObject{
				Body: &PostDonationsV1JSONRequestBody{
					Amount:     amount,
					Currency:   "USD",
					DonorEmail: openapi_types.Email("donor@example.com"),
				},
			}

			response, err := api.PostDonationsV1(context.Background(), request)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			successResponse, ok := response.(PostDonationsV1200JSONResponse)
			if !ok {
				t.Fatalf("expected PostDonationsV1200JSONResponse, got %T", response)
			}

			if successResponse.ClientSecret != "secret_amount" {
				t.Errorf("expected client secret secret_amount, got %s", successResponse.ClientSecret)
			}
		})
	}
}

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
