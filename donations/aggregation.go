package donations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/International-Combat-Archery-Alliance/payments"
	"github.com/Rhymond/go-money"
)

// StateAggregation represents aggregated donation data for a specific location and currency
type StateAggregation struct {
	Country string
	State   string
	Money   *money.Money
	Count   int
}

// AggregateDonationsByState queries payments with donation metadata and aggregates by country, state, and currency.
// Returns a slice of StateAggregation. Only includes aggregations with data (excludes empty results).
func AggregateDonationsByState(
	ctx context.Context,
	querier payments.PaymentQuerier,
	createdAfter, createdBefore *time.Time,
) ([]StateAggregation, error) {
	// Validate date range if both are provided
	if createdAfter != nil && createdBefore != nil && createdBefore.Before(*createdAfter) {
		return nil, errors.New("created_before cannot be before created_after")
	}

	params := payments.ChargeListParams{
		CreatedAfter:   createdAfter,
		CreatedBefore:  createdBefore,
		MetadataFilter: map[string]string{"item_type": "donation"},
	}

	// Aggregation key: "country:state:currency"
	aggregations := make(map[string]*aggregationData)

	for payment, err := range querier.ListCharges(ctx, params) {
		if err != nil {
			return nil, fmt.Errorf("failed to list charges: %w", err)
		}

		// Extract location information from billing details
		country, state := extractLocation(payment)

		currencyCode := payment.Amount.Currency().Code
		key := fmt.Sprintf("%s:%s:%s", country, state, currencyCode)

		if _, exists := aggregations[key]; !exists {
			aggregations[key] = &aggregationData{
				country:  country,
				state:    state,
				currency: currencyCode,
				amount:   0,
				count:    0,
			}
		}

		aggregations[key].amount += payment.Amount.Amount()
		aggregations[key].count++
	}

	// Convert map to slice
	result := make([]StateAggregation, 0, len(aggregations))
	for _, data := range aggregations {
		result = append(result, StateAggregation{
			Country: data.country,
			State:   data.state,
			Money:   money.New(data.amount, data.currency),
			Count:   data.count,
		})
	}

	return result, nil
}

type aggregationData struct {
	country  string
	state    string
	currency string
	amount   int64
	count    int
}

// extractLocation extracts country and state from payment billing details.
// Returns "N/A" for country and/or state when the information is not available.
func extractLocation(payment payments.Payment) (country, state string) {
	if payment.BillingDetails == nil || payment.BillingDetails.Address == nil {
		return "N/A", "N/A"
	}

	country = payment.BillingDetails.Address.Country
	if country == "" {
		country = "N/A"
	}

	state = payment.BillingDetails.Address.State
	if state == "" {
		state = "N/A"
	}

	return country, state
}
