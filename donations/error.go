package donations

import (
	"fmt"
)

type ErrorReason string

const (
	REASON_INVALID_AMOUNT            ErrorReason = "INVALID_AMOUNT"
	REASON_FAILED_TO_CREATE_CHECKOUT ErrorReason = "FAILED_TO_CREATE_CHECKOUT"
	REASON_INVALID_DATE_RANGE        ErrorReason = "INVALID_DATE_RANGE"
)

type Error struct {
	Reason  ErrorReason
	Message string
	Cause   error
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s. Cause: %s", e.Reason, e.Message, e.Cause)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func newDonationError(reason ErrorReason, message string, cause error) *Error {
	return &Error{
		Reason:  reason,
		Message: message,
		Cause:   cause,
	}
}

func NewInvalidAmountError(amount int64) *Error {
	return newDonationError(REASON_INVALID_AMOUNT, fmt.Sprintf("Amount must be at least 100 (smallest currency unit), got: %d", amount), nil)
}

func NewFailedToCreateCheckoutError(message string, cause error) *Error {
	return newDonationError(REASON_FAILED_TO_CREATE_CHECKOUT, message, cause)
}

func NewInvalidDateRangeError(message string) *Error {
	return newDonationError(REASON_INVALID_DATE_RANGE, message, nil)
}
