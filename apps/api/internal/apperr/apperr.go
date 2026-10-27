// Package apperr defines transport-agnostic domain errors. Handlers translate
// them to the JSON error envelope in one place.
package apperr

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrForbidden    = errors.New("forbidden")
	ErrConflict     = errors.New("conflict")
	ErrValidation   = errors.New("validation failed")
	ErrUnauthorized = errors.New("unauthorized")
	ErrRateLimited  = errors.New("rate limited")
	ErrClosed       = errors.New("closed")
)

type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string { return "validation failed" }
func (e *ValidationError) Unwrap() error { return ErrValidation }

func Invalid(field, message string) error {
	return &ValidationError{Fields: map[string]string{field: message}}
}

func InvalidFields(fields map[string]string) error {
	return &ValidationError{Fields: fields}
}
