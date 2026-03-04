package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/International-Combat-Archery-Alliance/donation-api/donations"
	"github.com/International-Combat-Archery-Alliance/payments"
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
	api := NewAPI(mockManager, "https://example.com/return", logger, LOCAL)

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

	if successResponse.CheckoutUrl != expectedClientSecret {
		t.Errorf("expected checkout URL %s, got %s", expectedClientSecret, successResponse.CheckoutUrl)
	}
}

func TestPostDonationsV1_InvalidAmount(t *testing.T) {
	mockManager := &MockCheckoutManager{
		CreateCheckoutFunc: func(ctx context.Context, params payments.CheckoutParams) (payments.CheckoutInfo, error) {
			return payments.CheckoutInfo{}, donations.NewInvalidAmountError(50)
		},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	api := NewAPI(mockManager, "https://example.com/return", logger, LOCAL)

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

	if errorResponse.Error != "BAD_REQUEST" {
		t.Errorf("expected error code BAD_REQUEST, got %s", errorResponse.Error)
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
	api := NewAPI(mockManager, "https://example.com/return", logger, LOCAL)

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

	if errorResponse.Error != "INTERNAL_ERROR" {
		t.Errorf("expected error code INTERNAL_ERROR, got %s", errorResponse.Error)
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
	api := NewAPI(mockManager, "https://example.com/return", logger, LOCAL)

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

	if errorResponse.Error != "INTERNAL_ERROR" {
		t.Errorf("expected error code INTERNAL_ERROR, got %s", errorResponse.Error)
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
			api := NewAPI(mockManager, "https://example.com/return", logger, LOCAL)

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

			if successResponse.CheckoutUrl != "secret_"+currency {
				t.Errorf("expected checkout URL secret_%s, got %s", currency, successResponse.CheckoutUrl)
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
			api := NewAPI(mockManager, "https://example.com/return", logger, LOCAL)

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

			if successResponse.CheckoutUrl != "secret_amount" {
				t.Errorf("expected checkout URL secret_amount, got %s", successResponse.CheckoutUrl)
			}
		})
	}
}
