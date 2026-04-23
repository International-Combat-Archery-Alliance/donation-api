package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"

	"github.com/International-Combat-Archery-Alliance/auth/token"
	"github.com/International-Combat-Archery-Alliance/donation-api/api"
	"github.com/International-Combat-Archery-Alliance/payments/stripe"
	"github.com/International-Combat-Archery-Alliance/telemetry"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

const (
	stripeWebhookSigningSecretSSMKey = "/stripeEndpointSecret"
	stripeAPISecretKeySSMKey         = "/stripeSecretKey"
	newRelicLicenseEnvVar            = "NEW_RELIC_LICENSE_KEY"
	newRelicLicenseSSMPath           = "/newrelic-license-key"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	env := getEnvironment()

	licenseKey, err := getNewRelicLicenseKey(ctx, env)
	if err != nil {
		logger.Error("failed to get New Relic license key", "error", err)
		os.Exit(1)
	}

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "otlp.nr-data.net:4317"
	}

	traceShutdown, flushTraces, err := telemetry.Init(ctx, telemetry.Options{
		ServiceName: "donation-api",
		Endpoint:    endpoint,
		APIKey:      licenseKey,
		Lambda:      telemetry.LambdaInfoFromEnv(),
	})
	if err != nil {
		logger.Error("failed to initialize telemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := traceShutdown(shutdownCtx); err != nil {
			logger.Error("failed to shutdown telemetry", "error", err)
		}
	}()

	// Start a root trace span for startup
	tracer := otel.Tracer("github.com/International-Combat-Archery-Alliance/donation-api/cmd")
	ctx, span := tracer.Start(ctx, "startup")

	instrumentedHTTPClient := telemetry.InstrumentedHTTPClient()

	var stripeClient *stripe.Client
	if err := telemetry.RunWithSpan(ctx, tracer, "init-stripe-client", func(ctx context.Context) error {
		var err error
		stripeClient, err = createStripeClient(ctx, env, instrumentedHTTPClient)
		return err
	}); err != nil {
		span.RecordError(err)
		logger.Error("Failed to create Stripe client", "error", err)
		os.Exit(1)
	}

	var signingKeys map[string]token.SigningKey
	var currentKeyID string
	if err := telemetry.RunWithSpan(ctx, tracer, "init-jwt-signing-keys", func(ctx context.Context) error {
		var err error
		signingKeys, currentKeyID, err = getJWTSigningKeys(ctx, env)
		return err
	}); err != nil {
		span.RecordError(err)
		logger.Error("failed to get JWT signing keys", "error", err)
		os.Exit(1)
	}

	tokenService := token.NewTokenService(
		signingKeys[currentKeyID],
		token.WithSigningKeys(signingKeys, currentKeyID),
	)

	returnURL := getReturnURL(env)

	donationAPI := api.NewAPI(stripeClient, stripeClient, tokenService, returnURL, logger, env, flushTraces)

	// End startup span after initialization completes
	span.End()

	host := getEnvOrDefault("HOST", "")
	port := getEnvOrDefault("PORT", "3003")

	logger.Info("Starting donation API server", "host", host, "port", port, "env", env)

	sigCtx, sigStop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer sigStop()

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- donationAPI.ListenAndServe(host, port)
	}()

	select {
	case <-sigCtx.Done():
		logger.Info("shutting down gracefully")
	case err := <-serverErrCh:
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

func loadAWSConfig(ctx context.Context, optFns ...func(*config.LoadOptions) error) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return aws.Config{}, err
	}
	telemetry.InstrumentAWSConfig(&cfg)
	return cfg, nil
}

func getReturnURL(env api.Environment) string {
	if env == api.LOCAL {
		return getEnvOrDefault("STRIPE_RETURN_URL", "http://localhost:5173/donation/success")
	}
	return getEnvOrDefault("STRIPE_RETURN_URL", "https://www.icaa.world/donation/success")
}

func createStripeClient(ctx context.Context, env api.Environment, httpClient *http.Client) (*stripe.Client, error) {
	if env == api.LOCAL {
		apiKey := getEnvOrDefault("STRIPE_SECRET_KEY", "")
		if apiKey == "" {
			return nil, fmt.Errorf("STRIPE_SECRET_KEY environment variable is required")
		}
		webhookSecret := getEnvOrDefault("STRIPE_ENDPOINT_SECRET", "")
		return stripe.NewClient(apiKey, webhookSecret, stripe.WithHTTPClient(httpClient)), nil
	}

	cfg, err := loadAWSConfig(ctx)
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
		stripe.WithHTTPClient(httpClient),
	), nil
}

// getNewRelicLicenseKey retrieves the New Relic license key from environment variable (local)
// or AWS Parameter Store (production)
func getNewRelicLicenseKey(ctx context.Context, env api.Environment) (string, error) {
	if env == api.LOCAL {
		return os.Getenv(newRelicLicenseEnvVar), nil
	}

	cfg, err := loadAWSConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("unable to load AWS SDK config: %w", err)
	}

	client := ssm.NewFromConfig(cfg)

	result, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(newRelicLicenseSSMPath),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get New Relic license key from Parameter Store: %w", err)
	}

	return *result.Parameter.Value, nil
}

// jwtSigningKeysData represents the JSON structure for signing keys
type jwtSigningKeysData struct {
	CurrentKey string            `json:"currentKey"`
	Keys       map[string]string `json:"keys"`
}

// getJWTSigningKeys retrieves the JWT signing keys from environment variable (local)
// or AWS Parameter Store (production)
func getJWTSigningKeys(ctx context.Context, env api.Environment) (map[string]token.SigningKey, string, error) {
	if env == api.LOCAL {
		// Local development: use environment variable
		key := os.Getenv("JWT_SIGNING_KEY")
		if key == "" {
			key = "local-development-signing-key-minimum-32-characters-long"
		}
		return map[string]token.SigningKey{
			"local": {ID: "local", Key: []byte(key)},
		}, "local", nil
	}

	// Production: retrieve from AWS Parameter Store
	cfg, err := loadAWSConfig(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("unable to load AWS SDK config: %w", err)
	}

	client := ssm.NewFromConfig(cfg)

	result, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String("/jwtSigningKeys"),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get JWT signing keys from Parameter Store: %w", err)
	}

	// Parse JSON response
	var data jwtSigningKeysData
	if err := json.Unmarshal([]byte(*result.Parameter.Value), &data); err != nil {
		return nil, "", fmt.Errorf("failed to parse JWT signing keys JSON: %w", err)
	}

	// Convert to map of SigningKey (keys are base64 encoded)
	signingKeys := make(map[string]token.SigningKey)
	for keyID, keyValue := range data.Keys {
		decodedKey, err := base64.StdEncoding.DecodeString(keyValue)
		if err != nil {
			return nil, "", fmt.Errorf("failed to decode base64 key %q: %w", keyID, err)
		}
		signingKeys[keyID] = token.SigningKey{
			ID:  keyID,
			Key: decodedKey,
		}
	}

	// Validate that current key exists
	if _, ok := signingKeys[data.CurrentKey]; !ok {
		return nil, "", fmt.Errorf("current key ID %q not found in keys", data.CurrentKey)
	}

	return signingKeys, data.CurrentKey, nil
}
