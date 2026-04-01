//go:generate go tool oapi-codegen --config openapi-codegen-config.yaml ../spec/api.yaml
package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/International-Combat-Archery-Alliance/auth/token"
	"github.com/International-Combat-Archery-Alliance/middleware"
	"github.com/International-Combat-Archery-Alliance/payments"
)

type Environment int

const (
	LOCAL Environment = iota
	PROD
)

// API implements the StrictServerInterface
type API struct {
	checkoutManager payments.CheckoutManager
	paymentQuerier  payments.PaymentQuerier
	tokenService    *token.TokenService
	returnURL       string
	logger          *slog.Logger
	env             Environment
}

var _ StrictServerInterface = (*API)(nil)

func NewAPI(
	checkoutManager payments.CheckoutManager,
	paymentQuerier payments.PaymentQuerier,
	tokenService *token.TokenService,
	returnURL string,
	logger *slog.Logger,
	env Environment,
) *API {
	return &API{
		checkoutManager: checkoutManager,
		paymentQuerier:  paymentQuerier,
		tokenService:    tokenService,
		returnURL:       returnURL,
		logger:          logger,
		env:             env,
	}
}

func (a *API) ListenAndServe(host string, port string) error {
	swagger, err := GetSwagger()
	if err != nil {
		return fmt.Errorf("Error loading swagger spec: %w", err)
	}

	swagger.Servers = nil

	strictHandler := NewStrictHandler(a, []StrictMiddlewareFunc{})

	r := http.NewServeMux()

	HandlerFromMux(strictHandler, r)

	swaggerUIMiddleware, err := middleware.HostSwaggerUI("/donations", swagger)
	if err != nil {
		return fmt.Errorf("failed to create swagger ui middleware: %w", err)
	}

	middlewares := []middleware.MiddlewareFunc{
		// Executes from the bottom up
		a.openapiValidateMiddleware(swagger),
		a.corsMiddleware(),
		swaggerUIMiddleware,
		middleware.AccessLogging(a.logger),
	}

	if a.env == PROD {
		middlewares = append(middlewares, middleware.BaseNamePrefix(a.logger, "/donations"))
	}

	h := middleware.UseMiddlewares(r, middlewares...)

	s := &http.Server{
		Handler: h,
		Addr:    net.JoinHostPort(host, port),
	}

	return s.ListenAndServe()
}

func (a *API) getLoggerOrBaseLogger(ctx context.Context) *slog.Logger {
	logger, ok := middleware.GetLoggerFromCtx(ctx)
	if !ok {
		a.logger.Error("tried to get logger and it wasn't in the context")
		return a.logger
	}
	return logger
}

func (a *API) userIsAdmin(ctx context.Context) bool {
	if authToken, ok := middleware.GetJWTFromCtx(ctx); ok {
		return authToken.IsAdmin()
	}
	return false
}

func (a *API) getUserEmail(ctx context.Context) string {
	if authToken, ok := middleware.GetJWTFromCtx(ctx); ok {
		return authToken.UserEmail()
	}
	return ""
}
