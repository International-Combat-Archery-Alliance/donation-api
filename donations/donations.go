package donations

import (
	"context"
	"time"

	"github.com/International-Combat-Archery-Alliance/donation-api/ptr"
	"github.com/International-Combat-Archery-Alliance/payments"
	"github.com/Rhymond/go-money"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

var tracer = otel.Tracer("github.com/International-Combat-Archery-Alliance/donation-api/donations")

// Metadata constants for donation identification
const (
	MetadataKeyItemType   = "item_type"
	MetadataValueDonation = "donation"
)

// CreateDonationCheckout validates amount and creates a Stripe checkout for a donation
func CreateDonationCheckout(ctx context.Context, checkoutManager payments.CheckoutManager, returnURL string, amount int64, currency, donorEmail string) (string, error) {
	ctx, span := tracer.Start(ctx, "CreateDonationCheckout")
	defer span.End()

	// Validate amount
	if amount < 100 {
		return "", NewInvalidAmountError(amount)
	}

	// Create Stripe checkout session
	checkoutParams := payments.CheckoutParams{
		SessionAliveDuration: ptr.Duration(30 * time.Minute),
		ReturnURL:            returnURL,
		Items: []payments.Item{{
			Name:     "Donation to ICAA",
			Quantity: 1,
			Price:    money.New(amount, currency),
		}},
		CustomerEmail: &donorEmail,
		ReceiptEmail:  &donorEmail,
		Metadata: map[string]string{
			MetadataKeyItemType: MetadataValueDonation,
		},
	}

	checkoutInfo, err := checkoutManager.CreateCheckout(ctx, checkoutParams)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", NewFailedToCreateCheckoutError("Failed to create Stripe checkout", err)
	}

	return checkoutInfo.ClientSecret, nil
}
