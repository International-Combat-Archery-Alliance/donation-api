package api

import (
	"context"
	"errors"

	"github.com/International-Combat-Archery-Alliance/donation-api/donations"
)

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
