package donations

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/International-Combat-Archery-Alliance/payments"
	"github.com/Rhymond/go-money"
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

func TestCreateDonationCheckout_InvalidAmount(t *testing.T) {
	mockManager := &MockCheckoutManager{}

	testCases := []struct {
		name   string
		amount int64
	}{
		{
			name:   "amount too low",
			amount: 50,
		},
		{
			name:   "amount zero",
			amount: 0,
		},
		{
			name:   "negative amount",
			amount: -100,
		},
		{
			name:   "amount just below minimum",
			amount: 99,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CreateDonationCheckout(
				context.Background(),
				mockManager,
				"https://example.com/return",
				tc.amount,
				"USD",
				"donor@example.com",
			)

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var donationErr *Error
			if !errors.As(err, &donationErr) {
				t.Fatalf("expected *donations.Error, got %T", err)
			}

			if donationErr.Reason != REASON_INVALID_AMOUNT {
				t.Errorf("expected reason %s, got %s", REASON_INVALID_AMOUNT, donationErr.Reason)
			}
		})
	}
}

func TestCreateDonationCheckout_ValidAmount(t *testing.T) {
	expectedClientSecret := "secret_test_123"
	mockManager := &MockCheckoutManager{
		CreateCheckoutFunc: func(ctx context.Context, params payments.CheckoutParams) (payments.CheckoutInfo, error) {
			// Verify params are correctly set
			if params.ReturnURL != "https://example.com/return" {
				t.Errorf("expected return URL %s, got %s", "https://example.com/return", params.ReturnURL)
			}
			if params.SessionAliveDuration == nil || *params.SessionAliveDuration != 30*time.Minute {
				t.Errorf("expected session alive duration %v, got %v", 30*time.Minute, params.SessionAliveDuration)
			}
			if params.CustomerEmail == nil || *params.CustomerEmail != "donor@example.com" {
				t.Errorf("expected customer email %s, got %v", "donor@example.com", params.CustomerEmail)
			}
			if len(params.Items) != 1 {
				t.Fatalf("expected 1 item, got %d", len(params.Items))
			}
			item := params.Items[0]
			if item.Name != "Donation to ICAA" {
				t.Errorf("expected item name %s, got %s", "Donation to ICAA", item.Name)
			}
			if item.Quantity != 1 {
				t.Errorf("expected quantity %d, got %d", 1, item.Quantity)
			}
			expectedPrice := money.New(1000, "USD")
			if item.Price.Amount() != expectedPrice.Amount() || item.Price.Currency() != expectedPrice.Currency() {
				t.Errorf("expected price %v, got %v", expectedPrice, item.Price)
			}

			return payments.CheckoutInfo{
				ClientSecret: expectedClientSecret,
			}, nil
		},
	}

	clientSecret, err := CreateDonationCheckout(
		context.Background(),
		mockManager,
		"https://example.com/return",
		1000,
		"USD",
		"donor@example.com",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if clientSecret != expectedClientSecret {
		t.Errorf("expected client secret %s, got %s", expectedClientSecret, clientSecret)
	}
}

func TestCreateDonationCheckout_CheckoutManagerError(t *testing.T) {
	expectedErr := errors.New("stripe connection failed")
	mockManager := &MockCheckoutManager{
		CreateCheckoutFunc: func(ctx context.Context, params payments.CheckoutParams) (payments.CheckoutInfo, error) {
			return payments.CheckoutInfo{}, expectedErr
		},
	}

	_, err := CreateDonationCheckout(
		context.Background(),
		mockManager,
		"https://example.com/return",
		1000,
		"EUR",
		"donor@example.com",
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var donationErr *Error
	if !errors.As(err, &donationErr) {
		t.Fatalf("expected *donations.Error, got %T", err)
	}

	if donationErr.Reason != REASON_FAILED_TO_CREATE_CHECKOUT {
		t.Errorf("expected reason %s, got %s", REASON_FAILED_TO_CREATE_CHECKOUT, donationErr.Reason)
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error to wrap %v, but it doesn't", expectedErr)
	}
}

func TestCreateDonationCheckout_DifferentCurrencies(t *testing.T) {
	currencies := []string{"USD", "EUR", "GBP", "CAD", "AUD"}

	for _, currency := range currencies {
		t.Run(currency, func(t *testing.T) {
			mockManager := &MockCheckoutManager{
				CreateCheckoutFunc: func(ctx context.Context, params payments.CheckoutParams) (payments.CheckoutInfo, error) {
					if len(params.Items) != 1 {
						t.Fatalf("expected 1 item, got %d", len(params.Items))
					}
					if params.Items[0].Price.Currency().Code != currency {
						t.Errorf("expected currency %s, got %s", currency, params.Items[0].Price.Currency().Code)
					}
					return payments.CheckoutInfo{ClientSecret: "secret_" + currency}, nil
				},
			}

			_, err := CreateDonationCheckout(
				context.Background(),
				mockManager,
				"https://example.com/return",
				5000,
				currency,
				"donor@example.com",
			)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCreateDonationCheckout_ContextCancellation(t *testing.T) {
	mockManager := &MockCheckoutManager{
		CreateCheckoutFunc: func(ctx context.Context, params payments.CheckoutParams) (payments.CheckoutInfo, error) {
			select {
			case <-ctx.Done():
				return payments.CheckoutInfo{}, ctx.Err()
			default:
				return payments.CheckoutInfo{ClientSecret: "secret"}, nil
			}
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := CreateDonationCheckout(
		ctx,
		mockManager,
		"https://example.com/return",
		1000,
		"USD",
		"donor@example.com",
	)

	if err == nil {
		t.Fatal("expected error due to cancelled context")
	}
}

func TestCreateDonationCheckout_ExactMinimumAmount(t *testing.T) {
	mockManager := &MockCheckoutManager{
		CreateCheckoutFunc: func(ctx context.Context, params payments.CheckoutParams) (payments.CheckoutInfo, error) {
			return payments.CheckoutInfo{ClientSecret: "secret_min"}, nil
		},
	}

	clientSecret, err := CreateDonationCheckout(
		context.Background(),
		mockManager,
		"https://example.com/return",
		100, // Exact minimum
		"USD",
		"donor@example.com",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if clientSecret != "secret_min" {
		t.Errorf("expected client secret %s, got %s", "secret_min", clientSecret)
	}
}
