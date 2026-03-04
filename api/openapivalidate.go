package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/International-Combat-Archery-Alliance/middleware"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	nethttpMiddleware "github.com/oapi-codegen/nethttp-middleware"
)

type ValidationErrorCode string

const (
	InputValidationErrorCode ValidationErrorCode = "InputValidationError"
	InternalErrorCode        ValidationErrorCode = "InternalError"
)

type ValidationError struct {
	Message string              `json:"message"`
	Code    ValidationErrorCode `json:"code"`
}

func (a *API) openapiValidateMiddleware(swagger *openapi3.T) middleware.MiddlewareFunc {
	return nethttpMiddleware.OapiRequestValidatorWithOptions(swagger, &nethttpMiddleware.Options{
		ErrorHandlerWithOpts: func(ctx context.Context, err error, w http.ResponseWriter, r *http.Request, opts nethttpMiddleware.ErrorHandlerOpts) {
			logger := a.getLoggerOrBaseLogger(ctx)

			var e ValidationError

			var requestErr *openapi3filter.RequestError
			if errors.As(err, &requestErr) {
				e = ValidationError{
					Message: err.Error(),
					Code:    InputValidationErrorCode,
				}
			} else {
				e = ValidationError{
					Message: err.Error(),
					Code:    InternalErrorCode,
				}
			}
			jsonBody, err := json.Marshal(&e)
			if err != nil {
				logger.Error("failed to marshal input validation error resp", "error", err)
				jsonBody = []byte(`{"message": "input is invalid", "code": "InputValidationError"}`)
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(opts.StatusCode)
			w.Write(jsonBody)
		},
	})
}
