# Donation API Service

This repository contains the backend service for accepting donations to ICAA (International Combat Archery Alliance), built using Go and AWS SAM (Serverless Application Model). It integrates with Stripe to handle payment processing for donations.

## Code Structure

The project is organized into the following main directories:

-   `api/`: Contains the API definitions, handlers, and OpenAPI specifications. This is where the HTTP endpoints are defined and implemented.
-   `cmd/`: Holds the main application entry point.
-   `donations/`: Defines core business logic and error handling for donation processing.
-   `ptr/`: Utility functions for creating pointers to primitive types.
-   `spec/`: Contains the OpenAPI specification (`api.yaml`) for the service.

## Prerequisites

To run this project locally or deploy it, you'll need:

-   [Go](https://go.dev/) (version 1.25.1 or later)
-   [AWS SAM CLI](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/install-sam-cli.html)
-   [Docker](https://docs.docker.com/get-docker/) (for SAM local testing)
-   Stripe account and API credentials (for payment processing)

## Running Locally

To run the project locally, ensure you have the prerequisites installed and your Stripe API credentials ready.

1.  **Build the project:**
    ```bash
    make build
    ```
    This command generates the Go code from the OpenAPI specification and builds the application binary.

2.  **Run locally with Stripe credentials:**
    ```bash
    STRIPE_SECRET_KEY=sk_test_... STRIPE_ENDPOINT_SECRET=whsec_... go run ./cmd
    ```
    Or using SAM local:
    ```bash
    make local
    ```

    The API will be available at `http://localhost:3003` by default (or the port specified by the `PORT` environment variable).

3.  **Test the API:**
    You can test the donation endpoint with:
    ```bash
    curl -X POST http://localhost:3003/donations/v1 \
      -H "Content-Type: application/json" \
      -d '{"amount": 5000, "currency": "USD", "donorEmail": "test@example.com"}'
    ```

## Environment Variables

The following environment variables are used:

| Variable | Description | Required |
|----------|-------------|----------|
| `STRIPE_SECRET_KEY` | Stripe API secret key | Yes (local mode) |
| `STRIPE_ENDPOINT_SECRET` | Stripe webhook signing secret | Yes (local mode) |
| `AWS_SAM_LOCAL` | Set to `true` to enable local mode (uses env vars instead of SSM) | No (defaults to false) |
| `HOST` | Server host to bind to | No (defaults to empty string) |
| `PORT` | Server port to listen on | No (defaults to 3003) |
| `STRIPE_RETURN_URL` | URL to redirect after successful payment | No (defaults based on environment) |

## API Endpoints

The service exposes the following endpoints:

-   **POST /donations/v1** - Creates a new donation and returns a Stripe checkout session URL
    -   Request body: `{ "amount": number, "currency": string, "donorEmail": string }`
    -   Response: `{ "checkoutUrl": string }`
    -   Amount must be at least 100 (smallest currency unit, e.g., cents)

## Development Commands

```bash
# Build the application
make build

# Build with SAM
make build-sam

# Run tests
make test

# Run tests with verbose output
make test-verbose

# Format code
make fmt

# Vet code for issues
make vet

# Tidy Go module dependencies
make tidy

# Clean build artifacts
make clean
```

## Deployment

This service is deployed using AWS SAM. The infrastructure is defined in `template.yml`.

Key deployment details:
-   Runs as an AWS Lambda function with HTTP API Gateway
-   Uses AWS Systems Manager Parameter Store for Stripe credentials in production
-   Containerized deployment using the provided Dockerfile
-   Supports both x86_64 and arm64 architectures

To deploy:
```bash
sam build --parameter-overrides architecture=x86_64
sam deploy
```

## OpenAPI Specification

The API is defined using OpenAPI 3.0.0 in `spec/api.yaml`. The Go server code is generated from this specification using `oapi-codegen`. To regenerate the API code after modifying the spec:

```bash
cd api && go generate
```

## License

See [LICENSE](LICENSE) file for details.
