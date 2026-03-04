package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/International-Combat-Archery-Alliance/donation-api/donations"
)

func (a *API) GetDonationsV1PerState(ctx context.Context, request GetDonationsV1PerStateRequestObject) (GetDonationsV1PerStateResponseObject, error) {
	logger := a.getLoggerOrBaseLogger(ctx)

	// Validate date range if both are provided
	if request.Params.CreatedAfter != nil && request.Params.CreatedBefore != nil {
		if request.Params.CreatedBefore.Before(*request.Params.CreatedAfter) {
			return GetDonationsV1PerState400JSONResponse{
				Code:    InvalidDateRange,
				Message: "created_before cannot be before created_after",
			}, nil
		}
	}

	// Parse date parameters
	var createdAfter, createdBefore *time.Time
	if request.Params.CreatedAfter != nil {
		createdAfter = request.Params.CreatedAfter
	}
	if request.Params.CreatedBefore != nil {
		createdBefore = request.Params.CreatedBefore
	}

	// Query and aggregate donations
	aggregations, err := donations.AggregateDonationsByState(ctx, a.paymentQuerier, createdAfter, createdBefore)
	if err != nil {
		logger.Error("failed to aggregate donations", "error", err)
		if errors.Is(err, errors.New("created_before cannot be before created_after")) {
			return GetDonationsV1PerState400JSONResponse{
				Code:    InvalidDateRange,
				Message: err.Error(),
			}, nil
		}
		return GetDonationsV1PerState500JSONResponse{
			Code:    InternalError,
			Message: "Failed to aggregate donations",
		}, nil
	}

	// Convert to API response format
	items := make([]AggregationItem, len(aggregations))
	for i, agg := range aggregations {
		items[i] = AggregationItem{
			Country: agg.Country,
			State:   agg.State,
			Money: Money{
				Amount:   int(agg.Money.Amount()),
				Currency: agg.Money.Currency().Code,
			},
			Count: agg.Count,
		}
	}

	logger.Info("Admin fetched donation aggregations",
		"adminEmail", a.getUserEmail(ctx),
		"count", len(items),
		"createdAfter", fmt.Sprintf("%v", createdAfter),
		"createdBefore", fmt.Sprintf("%v", createdBefore),
	)

	return GetDonationsV1PerState200JSONResponse{
		Aggregations: items,
	}, nil
}
