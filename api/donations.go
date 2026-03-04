package api

import (
	"context"
	"errors"
	"time"

	"github.com/International-Combat-Archery-Alliance/donation-api/donations"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

func (a *API) GetDonationsV1(ctx context.Context, request GetDonationsV1RequestObject) (GetDonationsV1ResponseObject, error) {
	logger := a.getLoggerOrBaseLogger(ctx)

	// Apply default limit if not provided
	limit := 20
	if request.Params.Limit != nil {
		limit = *request.Params.Limit
		if limit > 100 {
			limit = 100
		}
	}

	// Build query params
	listParams := donations.ListParams{
		Limit:  limit,
		Cursor: request.Params.Cursor,
	}

	if request.Params.CreatedAfter != nil {
		listParams.CreatedAfter = request.Params.CreatedAfter
	}
	if request.Params.CreatedBefore != nil {
		listParams.CreatedBefore = request.Params.CreatedBefore
	}

	// Query donations with pagination
	result, err := donations.ListDonations(ctx, a.paymentQuerier, listParams)
	if err != nil {
		logger.Error("Failed to list donations", "error", err)
		return GetDonationsV1500JSONResponse{
			Code:    InternalError,
			Message: "Failed to list donations",
		}, nil
	}

	// Build response items
	items := make([]DonationItem, len(result.Items))
	for i, item := range result.Items {
		items[i] = DonationItem{
			Id:        item.ID,
			Amount:    int(item.Amount.Amount()),
			Currency:  item.Amount.Currency().Code,
			Status:    item.Status,
			CreatedAt: item.CreatedAt,
		}

		// Add donor email if available
		if item.DonorEmail != "" {
			email := openapi_types.Email(item.DonorEmail)
			items[i].DonorEmail = &email
		}

		// Add metadata if available
		if len(item.Metadata) > 0 {
			items[i].Metadata = &item.Metadata
		}

		// Add billing details if available
		if item.BillingDetails != nil {
			bd := BillingDetails{
				Name: &item.BillingDetails.Name,
			}
			if item.BillingDetails.Email != "" {
				email := openapi_types.Email(item.BillingDetails.Email)
				bd.Email = &email
			}
			if item.BillingDetails.Address != nil {
				addr := item.BillingDetails.Address
				bd.Address = &Address{
					City:       &addr.City,
					Country:    &addr.Country,
					Line1:      &addr.Line1,
					Line2:      &addr.Line2,
					PostalCode: &addr.PostalCode,
					State:      &addr.State,
				}
			}
			items[i].BillingDetails = &bd
		}
	}

	return GetDonationsV1200JSONResponse{
		Items:      items,
		NextCursor: result.NextCursor,
	}, nil
}

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
		var donationErr *donations.Error
		if errors.As(err, &donationErr) && donationErr.Reason == donations.REASON_INVALID_DATE_RANGE {
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

	return GetDonationsV1PerState200JSONResponse{
		Aggregations: items,
	}, nil
}

func (a *API) PostDonationsV1(ctx context.Context, request PostDonationsV1RequestObject) (PostDonationsV1ResponseObject, error) {
	logger := a.getLoggerOrBaseLogger(ctx)

	body := request.Body

	checkoutURL, err := donations.CreateDonationCheckout(
		ctx,
		a.checkoutManager,
		a.returnURL,
		int64(body.Amount),
		body.Currency,
		string(body.DonorEmail),
	)

	if err != nil {
		var donationErr *donations.Error
		if errors.As(err, &donationErr) {
			switch donationErr.Reason {
			case donations.REASON_INVALID_AMOUNT:
				logger.Warn("Invalid donation request", "error", err)
				return PostDonationsV1400JSONResponse{
					Code:    BadRequest,
					Message: donationErr.Message,
				}, nil
			}
		}

		logger.Error("Failed to create donation checkout", "error", err)
		return PostDonationsV1500JSONResponse{
			Code:    InternalError,
			Message: "Failed to create donation checkout",
		}, nil
	}

	return PostDonationsV1200JSONResponse{
		ClientSecret: checkoutURL,
	}, nil
}
