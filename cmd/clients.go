package main

import (
	"net/http"

	"github.com/International-Combat-Archery-Alliance/payments/stripe"
)

func makeStripeClient(secretKey, endpointSecret string, httpClient *http.Client) *stripe.Client {
	return stripe.NewClient(secretKey, endpointSecret, stripe.WithHTTPClient(httpClient))
}
