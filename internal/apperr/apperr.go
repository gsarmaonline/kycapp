package apperr

import "errors"

var (
	ErrNotFound            = errors.New("not_found")
	ErrConflict            = errors.New("conflict")
	ErrValidation          = errors.New("validation_error")
	ErrIdempotencyConflict = errors.New("idempotency_conflict")
	ErrUnauthorized        = errors.New("unauthorized")
	ErrRateLimited         = errors.New("rate_limited")
)

// Error carries an API error code and message.
type Error struct {
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func (e *Error) Unwrap() error { return e.Err }

func Validation(msg string) error {
	return &Error{Code: "validation_error", Message: msg, Err: ErrValidation}
}

func NotFound(msg string) error {
	return &Error{Code: "not_found", Message: msg, Err: ErrNotFound}
}

func Conflict(msg string) error {
	return &Error{Code: "conflict", Message: msg, Err: ErrConflict}
}

func IdempotencyConflict(msg string) error {
	return &Error{Code: "idempotency_conflict", Message: msg, Err: ErrIdempotencyConflict}
}

func Unauthorized(msg string) error {
	return &Error{Code: "unauthorized", Message: msg, Err: ErrUnauthorized}
}

func RateLimited(msg string) error {
	return &Error{Code: "rate_limited", Message: msg, Err: ErrRateLimited}
}
