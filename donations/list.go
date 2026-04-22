package donations

import (
	"context"
	"time"

	"github.com/International-Combat-Archery-Alliance/payments"
	"github.com/Rhymond/go-money"
	"go.opentelemetry.io/otel/codes"
)

// ListParams defines parameters for listing donations with pagination
type ListParams struct {
	Limit         int
	Cursor        *string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
}

// ListResult contains the paginated list of donations
type ListResult struct {
	Items      []DonationItem
	NextCursor *string
}

// DonationItem represents a single donation in the list
type DonationItem struct {
	ID             string
	Amount         *money.Money
	Status         string
	DonorEmail     string
	BillingDetails *payments.BillingDetails
	CreatedAt      time.Time
	Metadata       map[string]string
}

// ListDonations queries donations with server-side pagination support
func ListDonations(
	ctx context.Context,
	querier payments.PaymentQuerier,
	params ListParams,
) (*ListResult, error) {
	ctx, span := tracer.Start(ctx, "ListDonations")
	defer span.End()

	// Apply default and max limits
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Build cursor from pointer
	cursor := ""
	if params.Cursor != nil {
		cursor = *params.Cursor
	}

	paymentParams := payments.ChargeListPaginatedParams{
		CreatedAfter:   params.CreatedAfter,
		CreatedBefore:  params.CreatedBefore,
		MetadataFilter: map[string]string{MetadataKeyItemType: MetadataValueDonation},
		Limit:          limit,
		Cursor:         cursor,
	}

	page, err := querier.ListChargesPaginated(ctx, paymentParams)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, NewListDonationsError("Failed to list donations", err)
	}

	// Convert payments to donation items
	items := make([]DonationItem, len(page.Payments))
	for i, payment := range page.Payments {
		items[i] = DonationItem{
			ID:             payment.ID,
			Amount:         payment.Amount,
			Status:         payment.Status,
			DonorEmail:     getDonorEmail(payment),
			BillingDetails: payment.BillingDetails,
			CreatedAt:      payment.Created,
			Metadata:       payment.Metadata,
		}
	}

	result := &ListResult{
		Items: items,
	}

	// Set next cursor if there are more results
	if page.HasMore {
		result.NextCursor = &page.NextCursor
	}

	return result, nil
}

// getDonorEmail extracts the donor email from payment billing details
func getDonorEmail(payment payments.Payment) string {
	if payment.BillingDetails != nil && payment.BillingDetails.Email != "" {
		return payment.BillingDetails.Email
	}
	return ""
}

// REASON_LIST_DONATIONS_FAILED is the error reason for list donations failures
const REASON_LIST_DONATIONS_FAILED ErrorReason = "LIST_DONATIONS_FAILED"

// NewListDonationsError creates a new list donations error
func NewListDonationsError(message string, cause error) *Error {
	return &Error{
		Reason:  REASON_LIST_DONATIONS_FAILED,
		Message: message,
		Cause:   cause,
	}
}
