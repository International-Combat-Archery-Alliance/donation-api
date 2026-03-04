package donations

import (
	"context"
	"errors"
	"iter"
	"testing"
	"time"

	"github.com/International-Combat-Archery-Alliance/payments"
	"github.com/Rhymond/go-money"
)

// MockPaymentQuerierForList is a mock implementation of payments.PaymentQuerier for testing ListDonations
type MockPaymentQuerierForList struct {
	Pages     []payments.ChargesPage
	PageIndex int
	Err       error
}

func (m *MockPaymentQuerierForList) ListCharges(ctx context.Context, params payments.ChargeListParams) iter.Seq2[payments.Payment, error] {
	return func(yield func(payments.Payment, error) bool) {
		if m.Err != nil {
			yield(payments.Payment{}, m.Err)
			return
		}
		// Return empty for this mock - we use ListChargesPaginated for list tests
	}
}

func (m *MockPaymentQuerierForList) ListChargesPaginated(ctx context.Context, params payments.ChargeListPaginatedParams) (payments.ChargesPage, error) {
	if m.Err != nil {
		return payments.ChargesPage{}, m.Err
	}

	if m.PageIndex < len(m.Pages) {
		page := m.Pages[m.PageIndex]
		m.PageIndex++
		return page, nil
	}

	return payments.ChargesPage{}, nil
}

func TestListDonations_Success(t *testing.T) {
	now := time.Now()
	mockQuerier := &MockPaymentQuerierForList{
		Pages: []payments.ChargesPage{
			{
				Payments: []payments.Payment{
					{
						ID:      "pay_1",
						Amount:  money.New(5000, "USD"),
						Status:  "succeeded",
						Created: now,
						BillingDetails: &payments.BillingDetails{
							Email: "donor@example.com",
							Name:  "John Doe",
						},
						Metadata: map[string]string{MetadataKeyItemType: MetadataValueDonation},
					},
					{
						ID:      "pay_2",
						Amount:  money.New(10000, "USD"),
						Status:  "succeeded",
						Created: now,
						BillingDetails: &payments.BillingDetails{
							Email: "donor2@example.com",
							Name:  "Jane Smith",
						},
						Metadata: map[string]string{MetadataKeyItemType: MetadataValueDonation},
					},
				},
				HasMore:    false,
				NextCursor: "",
			},
		},
	}

	params := ListParams{
		Limit: 20,
	}

	result, err := ListDonations(context.Background(), mockQuerier, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(result.Items))
	}

	// Check first item
	if result.Items[0].ID != "pay_1" {
		t.Errorf("expected ID pay_1, got %s", result.Items[0].ID)
	}
	if result.Items[0].Amount.Amount() != 5000 {
		t.Errorf("expected amount 5000, got %d", result.Items[0].Amount.Amount())
	}
	if result.Items[0].Amount.Currency().Code != "USD" {
		t.Errorf("expected currency USD, got %s", result.Items[0].Amount.Currency().Code)
	}
	if result.Items[0].Status != "succeeded" {
		t.Errorf("expected status succeeded, got %s", result.Items[0].Status)
	}
	if result.Items[0].DonorEmail != "donor@example.com" {
		t.Errorf("expected donor email donor@example.com, got %s", result.Items[0].DonorEmail)
	}

	// No next cursor
	if result.NextCursor != nil {
		t.Error("expected no next cursor")
	}
}

func TestListDonations_WithPagination(t *testing.T) {
	now := time.Now()
	nextCursor := "pay_2"
	mockQuerier := &MockPaymentQuerierForList{
		Pages: []payments.ChargesPage{
			{
				Payments: []payments.Payment{
					{
						ID:      "pay_1",
						Amount:  money.New(5000, "USD"),
						Status:  "succeeded",
						Created: now,
					},
					{
						ID:      "pay_2",
						Amount:  money.New(10000, "USD"),
						Status:  "succeeded",
						Created: now,
					},
				},
				HasMore:    true,
				NextCursor: nextCursor,
			},
		},
	}

	params := ListParams{
		Limit: 2,
	}

	result, err := ListDonations(context.Background(), mockQuerier, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(result.Items))
	}

	// Should have next cursor
	if result.NextCursor == nil {
		t.Fatal("expected next cursor")
	}
	if *result.NextCursor != nextCursor {
		t.Errorf("expected next cursor %s, got %s", nextCursor, *result.NextCursor)
	}
}

func TestListDonations_WithCursor(t *testing.T) {
	now := time.Now()
	cursor := "pay_1"
	mockQuerier := &MockPaymentQuerierForList{
		Pages: []payments.ChargesPage{
			{
				Payments: []payments.Payment{
					{
						ID:      "pay_2",
						Amount:  money.New(10000, "USD"),
						Status:  "succeeded",
						Created: now,
					},
				},
				HasMore:    false,
				NextCursor: "",
			},
		},
	}

	params := ListParams{
		Limit:  2,
		Cursor: &cursor,
	}

	result, err := ListDonations(context.Background(), mockQuerier, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(result.Items))
	}

	if result.Items[0].ID != "pay_2" {
		t.Errorf("expected ID pay_2, got %s", result.Items[0].ID)
	}
}

func TestListDonations_DefaultLimit(t *testing.T) {
	now := time.Now()
	mockQuerier := &MockPaymentQuerierForList{
		Pages: []payments.ChargesPage{
			{
				Payments: []payments.Payment{
					{ID: "pay_1", Amount: money.New(1000, "USD"), Status: "succeeded", Created: now},
				},
				HasMore:    false,
				NextCursor: "",
			},
		},
	}

	// Pass 0 as limit - should default to 20
	params := ListParams{
		Limit: 0,
	}

	_, err := ListDonations(context.Background(), mockQuerier, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The mock doesn't validate the limit, but we can verify it doesn't error
}

func TestListDonations_MaxLimit(t *testing.T) {
	now := time.Now()
	mockQuerier := &MockPaymentQuerierForList{
		Pages: []payments.ChargesPage{
			{
				Payments: []payments.Payment{
					{ID: "pay_1", Amount: money.New(1000, "USD"), Status: "succeeded", Created: now},
				},
				HasMore:    false,
				NextCursor: "",
			},
		},
	}

	// Pass 200 as limit - should be capped at 100
	params := ListParams{
		Limit: 200,
	}

	_, err := ListDonations(context.Background(), mockQuerier, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The mock doesn't validate the limit, but we can verify it doesn't error
}

func TestListDonations_Error(t *testing.T) {
	mockQuerier := &MockPaymentQuerierForList{
		Err: errors.New("database connection failed"),
	}

	params := ListParams{
		Limit: 20,
	}

	_, err := ListDonations(context.Background(), mockQuerier, params)
	if err == nil {
		t.Fatal("expected error")
	}

	// Check that it's wrapped in our custom error type
	var donationErr *Error
	if !errors.As(err, &donationErr) {
		t.Error("expected error to be wrapped in donations.Error")
	}
	if donationErr.Reason != REASON_LIST_DONATIONS_FAILED {
		t.Errorf("expected reason LIST_DONATIONS_FAILED, got %s", donationErr.Reason)
	}
}

func TestListDonations_WithDateFilters(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	mockQuerier := &MockPaymentQuerierForList{
		Pages: []payments.ChargesPage{
			{
				Payments: []payments.Payment{
					{
						ID:      "pay_1",
						Amount:  money.New(5000, "USD"),
						Status:  "succeeded",
						Created: now,
					},
				},
				HasMore:    false,
				NextCursor: "",
			},
		},
	}

	params := ListParams{
		Limit:         20,
		CreatedAfter:  &yesterday,
		CreatedBefore: &tomorrow,
	}

	result, err := ListDonations(context.Background(), mockQuerier, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(result.Items))
	}
}

func TestListDonations_NoBillingDetails(t *testing.T) {
	now := time.Now()
	mockQuerier := &MockPaymentQuerierForList{
		Pages: []payments.ChargesPage{
			{
				Payments: []payments.Payment{
					{
						ID:             "pay_1",
						Amount:         money.New(5000, "USD"),
						Status:         "succeeded",
						Created:        now,
						BillingDetails: nil,
					},
				},
				HasMore:    false,
				NextCursor: "",
			},
		},
	}

	params := ListParams{
		Limit: 20,
	}

	result, err := ListDonations(context.Background(), mockQuerier, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(result.Items))
	}

	// Donor email should be empty
	if result.Items[0].DonorEmail != "" {
		t.Errorf("expected empty donor email, got %s", result.Items[0].DonorEmail)
	}
}

func TestGetDonorEmail_WithBillingDetails(t *testing.T) {
	payment := payments.Payment{
		BillingDetails: &payments.BillingDetails{
			Email: "donor@example.com",
		},
	}

	email := getDonorEmail(payment)
	if email != "donor@example.com" {
		t.Errorf("expected donor@example.com, got %s", email)
	}
}

func TestGetDonorEmail_WithoutBillingDetails(t *testing.T) {
	payment := payments.Payment{
		BillingDetails: nil,
	}

	email := getDonorEmail(payment)
	if email != "" {
		t.Errorf("expected empty email, got %s", email)
	}
}

func TestGetDonorEmail_WithEmptyEmail(t *testing.T) {
	payment := payments.Payment{
		BillingDetails: &payments.BillingDetails{
			Email: "",
		},
	}

	email := getDonorEmail(payment)
	if email != "" {
		t.Errorf("expected empty email, got %s", email)
	}
}

func TestNewListDonationsError(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewListDonationsError("Failed to list", cause)

	if err.Reason != REASON_LIST_DONATIONS_FAILED {
		t.Errorf("expected reason LIST_DONATIONS_FAILED, got %s", err.Reason)
	}
	if err.Message != "Failed to list" {
		t.Errorf("expected message 'Failed to list', got %s", err.Message)
	}
	if err.Cause != cause {
		t.Error("expected cause to be set")
	}

	// Test error message formatting
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("expected non-empty error message")
	}
}
