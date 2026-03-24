// Package stderror contains generic errors.
package stderror

import (
	"errors"
)

var (
	ErrInternal     = errors.New("internal error")
	ErrParsing      = errors.New("cannot parse data")
	ErrMissingParam = errors.New("missing param")
	ErrInvalidParam = errors.New("invalid param")
	ErrNotFound     = errors.New("not found")
	ErrNotAllowed   = errors.New("not allowed")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNonRetryable = errors.New("non retryable")
)
