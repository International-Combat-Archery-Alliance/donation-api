package donations

import (
	"context"
	"time"

	"github.com/International-Combat-Archery-Alliance/donation-api/ptr"
	"github.com/International-Combat-Archery-Alliance/payments"
	"github.com/Rhymond/go-money"
)

// CreateDonationCheckout validates amount and creates a Stripe checkout for a donation
func CreateDonationCheckout(ctx context.Context, checkoutManager payments.CheckoutManager, returnURL string, amount int64, currency, donorEmail string) (string, error) {
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
	}

	checkoutInfo, err := checkoutManager.CreateCheckout(ctx, checkoutParams)
	if err != nil {
		return "", NewFailedToCreateCheckoutError("Failed to create Stripe checkout", err)
	}

	return checkoutInfo.ClientSecret, nil
}
