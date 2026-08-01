// Package apierror is the single error model for ruuma (docs/04 §2).
//
// Every error that reaches a client is an *Error carrying a stable code, a
// human-readable message and optional structured details. Driver errors,
// wrapped causes and stack traces are kept internally and never serialised
// (docs/12, A05: "errors never leak stack traces or driver messages").
package apierror

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is a stable, machine-readable error identifier. Clients switch on these;
// they never parse messages.
type Code string

const (
	// Generic
	CodeValidation         Code = "VALIDATION_FAILED"
	CodeUnauthenticated    Code = "UNAUTHENTICATED"
	CodeForbidden          Code = "FORBIDDEN"
	CodeNotFound           Code = "NOT_FOUND"
	CodeConflict           Code = "CONFLICT"
	CodeRateLimited        Code = "RATE_LIMITED"
	CodeInternal           Code = "INTERNAL"
	CodeIdempotencyMismatch Code = "IDEMPOTENCY_MISMATCH"

	// Store & scope (BR-2.1.x, BR-2.7.8)
	CodeStoreOutOfScope Code = "STORE_OUT_OF_SCOPE"
	CodeStoreInactive   Code = "STORE_INACTIVE"
	CodeModeUnsupported Code = "MODE_UNSUPPORTED"
	CodeModeDisabled    Code = "MODE_DISABLED"

	// Scheduling (BR-2.3.x)
	CodeDateNotBookable Code = "DATE_NOT_BOOKABLE"
	CodeSlotNotBookable Code = "SLOT_NOT_BOOKABLE"
	CodeSlotFull        Code = "SLOT_FULL"
	CodeSlotPast        Code = "SLOT_PAST"
	CodeSlotLeadTime    Code = "SLOT_LEAD_TIME"
	CodeSlotCutoff      Code = "SLOT_CUTOFF"
	CodeBlackout        Code = "BLACKOUT"

	// Catalogue (BR-2.2.x)
	CodeItemUnavailable Code = "ITEM_UNAVAILABLE"
	CodeOptionInvalid   Code = "OPTION_INVALID"

	// Money & promotions (BR-2.5.x)
	CodeTotalMismatch  Code = "TOTAL_MISMATCH"
	CodePromoInvalid   Code = "PROMO_INVALID"
	CodePromoExhausted Code = "PROMO_EXHAUSTED"

	// Orders & payments (BR-2.4.x, BR-2.6.x)
	CodeIllegalTransition       Code = "ILLEGAL_TRANSITION"
	CodeUnpaidLimitReached      Code = "UNPAID_LIMIT_REACHED"
	CodePaymentAlreadyVerified  Code = "PAYMENT_ALREADY_VERIFIED"
	CodeSelfVerificationForbidden Code = "SELF_VERIFICATION_FORBIDDEN"
	CodeRejectionReasonRequired Code = "REJECTION_REASON_REQUIRED"
	CodeProofRequired           Code = "PROOF_REQUIRED"

	// Identity (BR-2.7.x)
	CodePhoneVerificationRequired Code = "PHONE_VERIFICATION_REQUIRED"
	CodeAccountLocked             Code = "ACCOUNT_LOCKED"
	CodeInvalidCredentials        Code = "INVALID_CREDENTIALS"
)

// Error is an application error with an HTTP status and a stable code.
type Error struct {
	Code    Code
	Message string
	Status  int
	Details map[string]any

	cause error // never serialised
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.cause }

// WithDetails attaches structured details. Details are client-visible, so they
// must never carry PII or internal identifiers beyond resource ids.
func (e *Error) WithDetails(d map[string]any) *Error {
	c := *e
	c.Details = d
	return &c
}

// WithCause attaches an internal cause for logging. The cause is never rendered
// to a client.
func (e *Error) WithCause(err error) *Error {
	c := *e
	c.cause = err
	return &c
}

// WithMessage overrides the human-readable message.
func (e *Error) WithMessage(msg string) *Error {
	c := *e
	c.Message = msg
	return &c
}

func New(status int, code Code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

// Constructors for the statuses in docs/04 §2.

func BadRequest(code Code, msg string) *Error   { return New(http.StatusBadRequest, code, msg) }
func Unauthorized(msg string) *Error            { return New(http.StatusUnauthorized, CodeUnauthenticated, msg) }
func Forbidden(code Code, msg string) *Error    { return New(http.StatusForbidden, code, msg) }
func NotFound(msg string) *Error                { return New(http.StatusNotFound, CodeNotFound, msg) }
func Conflict(code Code, msg string) *Error     { return New(http.StatusConflict, code, msg) }
func Unprocessable(code Code, msg string) *Error { return New(http.StatusUnprocessableEntity, code, msg) }
func TooManyRequests(msg string) *Error         { return New(http.StatusTooManyRequests, CodeRateLimited, msg) }

// Internal wraps an unexpected error. The cause is logged; the client sees a
// generic message (docs/12, A05).
func Internal(err error) *Error {
	return &Error{
		Status:  http.StatusInternalServerError,
		Code:    CodeInternal,
		Message: "Something went wrong on our side.",
		cause:   err,
	}
}

// Validation reports a failed input validation with field details.
func Validation(msg string, fields map[string]any) *Error {
	return &Error{
		Status:  http.StatusBadRequest,
		Code:    CodeValidation,
		Message: msg,
		Details: fields,
	}
}

// From converts any error into an *Error, defaulting to Internal. It is the
// single funnel used by the HTTP error middleware.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var ae *Error
	if errors.As(err, &ae) {
		return ae
	}
	return Internal(err)
}

// Is reports whether err is an *Error with the given code.
func Is(err error, code Code) bool {
	var ae *Error
	return errors.As(err, &ae) && ae.Code == code
}
