package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/International-Combat-Archery-Alliance/auth/google"
	"github.com/International-Combat-Archery-Alliance/donation-api/api"
	"github.com/International-Combat-Archery-Alliance/payments/stripe"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

const (
	stripeWebhookSigningSecretSSMKey = "/stripeEndpointSecret"
	stripeAPISecretKeySSMKey         = "/stripeSecretKey"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	env := getEnvironment()

	stripeClient, err := createStripeClient(ctx, env)
	if err != nil {
		logger.Error("Failed to create Stripe client", "error", err)
		os.Exit(1)
	}

	googleAuthValidator, err := google.NewValidator(ctx)
	if err != nil {
		logger.Error("failed to create google auth validator", "error", err)
		os.Exit(1)
	}

	returnURL := getReturnURL(env)

	donationAPI := api.NewAPI(stripeClient, stripeClient, googleAuthValidator, returnURL, logger, env)

	host := getEnvOrDefault("HOST", "")
	port := getEnvOrDefault("PORT", "3003")

	logger.Info("Starting donation API server", "host", host, "port", port, "env", env)

	err = donationAPI.ListenAndServe(host, port)
	if err != nil {
		logger.Error("Server error", "error", err)
		os.Exit(1)
	}
}

func getEnvironment() api.Environment {
	if isLocal() {
		return api.LOCAL
	}
	return api.PROD
}

func isLocal() bool {
	return getEnvOrDefault("AWS_SAM_LOCAL", "false") == "true"
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getReturnURL(env api.Environment) string {
	if env == api.LOCAL {
		return getEnvOrDefault("STRIPE_RETURN_URL", "http://localhost:5173/donation/success")
	}
	return getEnvOrDefault("STRIPE_RETURN_URL", "https://www.icaa.world/donation/success")
}

func createStripeClient(ctx context.Context, env api.Environment) (*stripe.Client, error) {
	if env == api.LOCAL {
		apiKey := getEnvOrDefault("STRIPE_SECRET_KEY", "")
		if apiKey == "" {
			return nil, fmt.Errorf("STRIPE_SECRET_KEY environment variable is required")
		}
		webhookSecret := getEnvOrDefault("STRIPE_ENDPOINT_SECRET", "")
		return stripe.NewClient(apiKey, webhookSecret), nil
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	ssmClient := ssm.NewFromConfig(cfg)

	apiKeyParam, err := ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(stripeAPISecretKeySSMKey),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get Stripe API key from SSM: %w", err)
	}

	webhookSecretParam, err := ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(stripeWebhookSigningSecretSSMKey),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get Stripe webhook secret from SSM: %w", err)
	}

	return stripe.NewClient(
		*apiKeyParam.Parameter.Value,
		*webhookSecretParam.Parameter.Value,
	), nil
}
