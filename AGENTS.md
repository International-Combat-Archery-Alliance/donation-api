# AGENTS.md - Coding Guidelines for donation-api

## Build & Run Commands

```bash
# Build the application
cd api && go generate && go build -o bootstrap ./cmd

# Run all tests
make test

# Run tests with verbose output
make test-verbose

# Build with SAM (for deployment)
make build-sam

# Run SAM local (requires Stripe env vars)
make local

# Format code
make fmt

# Vet code for issues
make vet

# Tidy dependencies
make tidy

# Clean build artifacts
make clean
```

## Code Style Guidelines

### Imports
- Group imports: stdlib first, then internal packages, then external dependencies
- Use blank line between groups
- Example:
  ```go
  import (
      "context"
      "fmt"
      "log/slog"
      
      "github.com/International-Combat-Archery-Alliance/donation-api/donations"
      "github.com/International-Combat-Archery-Alliance/payments"
      
      "github.com/aws/aws-sdk-go-v2/config"
  )
  ```

### Naming Conventions
- **Constants**: UPPER_SNAKE_CASE (e.g., `REASON_INVALID_AMOUNT`)
- **Types**: PascalCase (e.g., `CheckoutManager`, `ErrorReason`)
- **Functions**: PascalCase for exported, camelCase for unexported
- **Variables**: camelCase (e.g., `checkoutURL`, `stripeClient`)
- **Receivers**: Use first letter of type name (e.g., `a *API`, `e *Error`)
- **Interfaces**: End with "-er" suffix (e.g., `CheckoutManager`)
- **Error reasons**: PascalCase with REASON_ prefix

### Error Handling
- Use custom error types with `Reason`, `Message`, and `Cause` fields
- Implement `Error()` and `Unwrap()` methods
- Wrap errors with context using `fmt.Errorf("...: %w", err)`
- Use `errors.As()` for error type checking
- Example:
  ```go
  return nil, fmt.Errorf("failed to create Stripe client: %w", err)
  ```

### Logging
- Use `log/slog` with JSON handler
- Use structured logging: `logger.Info("msg", "key", value)`
- Extract logger from context when available via middleware

### Types
- Prefer concrete types over interfaces unless necessary
- Use pointer receivers for mutating methods
- Use value receivers for read-only methods
- Define enums using `iota` with type constants

### Code Generation
- OpenAPI specs are in `spec/api.yaml`
- Generated code goes in `api/gen.go`
- Run `go generate` in the `api/` directory after spec changes
- Config file: `api/openapi-codegen-config.yaml`

### API Design
- Use strict server interface from oapi-codegen
- Implement `StrictServerInterface` for handlers
- Use middleware pattern from `github.com/International-Combat-Archery-Alliance/middleware`

### Testing
- No tests exist yet - add tests in `*_test.go` files alongside source
- Use standard Go testing package
- Run single test: `go test ./package -run TestFunctionName`

## Project Structure

```
.
├── cmd/                    # Main application entry point
├── api/                    # HTTP handlers and generated API code
│   ├── api.go             # Server setup and middleware
│   ├── donations.go       # Donation endpoint handlers
│   ├── gen.go             # Generated from OpenAPI spec
│   └── openapi-codegen-config.yaml
├── donations/             # Business logic for donations
├── ptr/                   # Helper functions for pointer values
├── spec/api.yaml          # OpenAPI specification
├── template.yml           # AWS SAM template
├── Dockerfile             # Container build
├── Makefile               # Build automation
└── README.md              # Project documentation
```

## Environment Setup

Required environment variables for local development:
- `STRIPE_SECRET_KEY` - Stripe API secret key
- `STRIPE_ENDPOINT_SECRET` - Stripe webhook signing secret
- `AWS_SAM_LOCAL=true` - Enables local mode (uses env vars instead of SSM)
- Optional: `HOST`, `PORT`, `STRIPE_RETURN_URL`
