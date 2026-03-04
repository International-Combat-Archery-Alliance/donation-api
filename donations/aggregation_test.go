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

// MockPaymentQuerier is a mock implementation of payments.PaymentQuerier
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

func TestAggregateDonationsByState_Success(t *testing.T) {
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
			{
				ID:     "pay_3",
				Amount: money.New(3000, "USD"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "US",
						State:   "CA",
					},
				},
			},
		},
	}

	result, err := AggregateDonationsByState(context.Background(), querier, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 aggregations, got %d", len(result))
	}

	// Find MA aggregation
	var maAgg, caAgg *StateAggregation
	for i := range result {
		if result[i].State == "MA" {
			maAgg = &result[i]
		}
		if result[i].State == "CA" {
			caAgg = &result[i]
		}
	}

	if maAgg == nil {
		t.Fatal("expected MA aggregation")
	}
	if maAgg.Count != 2 {
		t.Errorf("expected MA count 2, got %d", maAgg.Count)
	}
	if maAgg.Money.Amount() != 15000 {
		t.Errorf("expected MA amount 15000, got %d", maAgg.Money.Amount())
	}

	if caAgg == nil {
		t.Fatal("expected CA aggregation")
	}
	if caAgg.Count != 1 {
		t.Errorf("expected CA count 1, got %d", caAgg.Count)
	}
	if caAgg.Money.Amount() != 3000 {
		t.Errorf("expected CA amount 3000, got %d", caAgg.Money.Amount())
	}
}

func TestAggregateDonationsByState_MultipleCurrencies(t *testing.T) {
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
				Amount: money.New(8000, "EUR"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "US",
						State:   "MA",
					},
				},
			},
		},
	}

	result, err := AggregateDonationsByState(context.Background(), querier, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 aggregations (one per currency), got %d", len(result))
	}
}

func TestAggregateDonationsByState_NoAddress(t *testing.T) {
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
					Address: nil,
				},
			},
			{
				ID:     "pay_3",
				Amount: money.New(2000, "USD"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "US",
						State:   "NY",
					},
				},
			},
		},
	}

	result, err := AggregateDonationsByState(context.Background(), querier, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 aggregations: N/A:N/A and US:NY
	if len(result) != 2 {
		t.Errorf("expected 2 aggregations, got %d", len(result))
	}

	// Check for N/A aggregation
	var naAgg, nyAgg *StateAggregation
	for i := range result {
		if result[i].Country == "N/A" && result[i].State == "N/A" {
			naAgg = &result[i]
		}
		if result[i].State == "NY" {
			nyAgg = &result[i]
		}
	}

	if naAgg == nil {
		t.Fatal("expected N/A:N/A aggregation for payments without address")
	}
	if naAgg.Count != 2 {
		t.Errorf("expected N/A count 2, got %d", naAgg.Count)
	}
	if naAgg.Money.Amount() != 8000 {
		t.Errorf("expected N/A amount 8000, got %d", naAgg.Money.Amount())
	}

	if nyAgg == nil {
		t.Fatal("expected NY aggregation")
	}
	if nyAgg.Count != 1 {
		t.Errorf("expected NY count 1, got %d", nyAgg.Count)
	}
}

func TestAggregateDonationsByState_EmptyCountryOrState(t *testing.T) {
	querier := &MockPaymentQuerier{
		Payments: []payments.Payment{
			{
				ID:     "pay_1",
				Amount: money.New(5000, "USD"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "",
						State:   "MA",
					},
				},
			},
			{
				ID:     "pay_2",
				Amount: money.New(3000, "USD"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "CA",
						State:   "",
					},
				},
			},
		},
	}

	result, err := AggregateDonationsByState(context.Background(), querier, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 aggregations, got %d", len(result))
	}

	// Check that empty country/state becomes "N/A"
	for _, agg := range result {
		if agg.Country == "" {
			t.Error("expected empty country to be replaced with N/A")
		}
		if agg.State == "" {
			t.Error("expected empty state to be replaced with N/A")
		}
	}
}

func TestAggregateDonationsByState_WithDateRange(t *testing.T) {
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

	result, err := AggregateDonationsByState(context.Background(), querier, &yesterday, &tomorrow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("expected 1 aggregation, got %d", len(result))
	}
}

func TestAggregateDonationsByState_InvalidDateRange(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)

	querier := &MockPaymentQuerier{}

	_, err := AggregateDonationsByState(context.Background(), querier, &now, &yesterday)
	if err == nil {
		t.Fatal("expected error for invalid date range")
	}

	var donationErr *Error
	if !errors.As(err, &donationErr) {
		t.Error("expected error to be of type donations.Error")
	}
	if donationErr.Reason != REASON_INVALID_DATE_RANGE {
		t.Errorf("expected reason INVALID_DATE_RANGE, got %s", donationErr.Reason)
	}
}

func TestAggregateDonationsByState_ListChargesError(t *testing.T) {
	querier := &MockPaymentQuerier{
		Err: errors.New("database connection failed"),
	}

	_, err := AggregateDonationsByState(context.Background(), querier, nil, nil)
	if err == nil {
		t.Fatal("expected error when ListCharges fails")
	}
}

func TestAggregateDonationsByState_NoPayments(t *testing.T) {
	querier := &MockPaymentQuerier{
		Payments: []payments.Payment{},
	}

	result, err := AggregateDonationsByState(context.Background(), querier, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 aggregations for empty payments, got %d", len(result))
	}
}

func TestAggregateDonationsByState_International(t *testing.T) {
	querier := &MockPaymentQuerier{
		Payments: []payments.Payment{
			{
				ID:     "pay_1",
				Amount: money.New(5000, "USD"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "CA",
						State:   "ON",
					},
				},
			},
			{
				ID:     "pay_2",
				Amount: money.New(10000, "CAD"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "CA",
						State:   "BC",
					},
				},
			},
			{
				ID:     "pay_3",
				Amount: money.New(8000, "GBP"),
				BillingDetails: &payments.BillingDetails{
					Address: &payments.Address{
						Country: "GB",
						State:   "ENG",
					},
				},
			},
		},
	}

	result, err := AggregateDonationsByState(context.Background(), querier, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 aggregations (CA:ON, CA:BC, GB:ENG), got %d", len(result))
	}

	// Verify currencies are correct
	for _, agg := range result {
		switch agg.State {
		case "ON":
			if agg.Money.Currency().Code != "USD" {
				t.Errorf("expected ON to have USD, got %s", agg.Money.Currency().Code)
			}
		case "BC":
			if agg.Money.Currency().Code != "CAD" {
				t.Errorf("expected BC to have CAD, got %s", agg.Money.Currency().Code)
			}
		case "ENG":
			if agg.Money.Currency().Code != "GBP" {
				t.Errorf("expected ENG to have GBP, got %s", agg.Money.Currency().Code)
			}
		}
	}
}
