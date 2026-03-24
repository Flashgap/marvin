package errorhandler

import (
	"errors"

	apperrors "github.com/Flashgap/marvin/internal/web/errors"
	stderror "github.com/Flashgap/marvin/pkg/stderr"
)

type ErrorMapping struct {
	FromErrors   []error
	ErrorMessage *apperrors.ErrorMessage
	DisableLog   bool // Disable logging for this error.
}

func (r *ErrorMapping) HasMapping(err error) *apperrors.ErrorMessage {
	for _, fromErr := range r.FromErrors {
		if errors.Is(err, fromErr) {
			return r.ErrorMessage
		}
	}

	return nil
}

// ToHTTPError specifies the apperrors error returned to a caller when the error is handled.
func (r *ErrorMapping) ToHTTPError(errMsg *apperrors.ErrorMessage) *ErrorMapping {
	r.ErrorMessage = errMsg
	return r
}

// DisableLogging specifies if the error should be logged.
func (r *ErrorMapping) DisableLogging() *ErrorMapping {
	r.DisableLog = true
	return r
}

// Map enables you to map errors to a given response status code or response body.
func Map(err ...error) *ErrorMapping {
	return &ErrorMapping{
		FromErrors: err,
	}
}

// DefaultErrorMapping is the default error mapping for all endpoints.
var DefaultErrorMapping = []*ErrorMapping{
	Map(stderror.ErrInternal).ToHTTPError(apperrors.GenericInternalServerError),
	Map(stderror.ErrParsing).ToHTTPError(apperrors.GenericJSONParseError),
	Map(stderror.ErrMissingParam, stderror.ErrInvalidParam).ToHTTPError(apperrors.GenericPayloadValidationError),
	Map(stderror.ErrInvalidParam).ToHTTPError(apperrors.GenericPayloadValidationError),
	Map(stderror.ErrNotFound).ToHTTPError(apperrors.GenericNotFoundError).DisableLogging(),
	Map(stderror.ErrNotAllowed).ToHTTPError(apperrors.GenericForbiddenActionError).DisableLogging(),
	Map(stderror.ErrUnauthorized).ToHTTPError(apperrors.AuthInvalidError).DisableLogging(),
	Map(stderror.ErrForbidden).ToHTTPError(apperrors.AuthForbiddenError).DisableLogging(),
}

// TasksErrorMapping is the default error mapping for tasks.
var TasksErrorMapping = []*ErrorMapping{
	Map(stderror.ErrNotFound).ToHTTPError(apperrors.GenericTaskDontRetryError).DisableLogging(),
	Map(stderror.ErrNonRetryable).ToHTTPError(apperrors.GenericTaskDontRetryError).DisableLogging(),
}

// WebhookErrorMapping is the default error mapping for webhooks.
var WebhookErrorMapping = TasksErrorMapping
