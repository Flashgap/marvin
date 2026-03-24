package errors

import (
	"fmt"
	"net/http"
)

// ErrorMessage represents an api error containing a code and an optional message
type ErrorMessage struct {
	HTTPCode int    `json:"-"`
	Code     int    `json:"code"`
	Message  string `json:"message"`
}

// DetailedErrorMessage represents a detailed api error.
type DetailedErrorMessage struct {
	*ErrorMessage
	Detail string `json:"detail"`
}

// NewErrorMessage returns a new error message with the associated code and message
func NewErrorMessage(httpCode int, code int, message string) *ErrorMessage {
	return &ErrorMessage{
		HTTPCode: httpCode,
		Code:     code,
		Message:  message,
	}
}

// Error returns a string version of the error to make it compliant with the interface
func (e *ErrorMessage) Error() string {
	return fmt.Sprintf("%s (%d)", e.Message, e.Code)
}

func (e *ErrorMessage) WithDetail(err error) *DetailedErrorMessage {
	return &DetailedErrorMessage{
		ErrorMessage: e,
		Detail:       err.Error(),
	}
}

// GenericJSONParseError is thrown when the body does not contain a json payload
var GenericJSONParseError = NewErrorMessage(
	http.StatusBadRequest,
	1001,
	"Invalid/Malformed json payload",
)

// GenericPayloadValidationError is thrown when a request payload fail the validation stage
var GenericPayloadValidationError = NewErrorMessage(
	http.StatusBadRequest,
	1002,
	"Payload validation failure",
)

// GenericNotFoundError is thrown when a requested resource cannot be found - in most cases it will be an account
var GenericNotFoundError = NewErrorMessage(
	http.StatusNotFound,
	1003,
	"Not found",
)

// GenericInternalServerError is thrown when a critical error happens and we cannot handle it
var GenericInternalServerError = NewErrorMessage(
	http.StatusInternalServerError,
	1004,
	"Internal server error",
)

// GenericForbiddenActionError is thrown when an action is not allowed at the request's time
var GenericForbiddenActionError = NewErrorMessage(
	http.StatusBadRequest,
	1007,
	"Action not permitted",
)

// GenericIgnoredRequestError is thrown when an action is ignored and the user must still receive a ok response
var GenericIgnoredRequestError = NewErrorMessage(
	http.StatusOK,
	1008,
	"OK",
)

// GenericNotImplementedError is thrown when an endpoint is not yet implemented
var GenericNotImplementedError = NewErrorMessage(
	http.StatusNotImplemented,
	1009,
	"Not Implemented",
)

// GenericMutexTimeoutError is thrown when a mutex acquisition times out
var GenericMutexTimeoutError = NewErrorMessage(
	http.StatusRequestTimeout,
	1011,
	"Mutex lock timeout",
)

// GenericUnknownLanguage is thrown when an unknown language is passed in a query
var GenericUnknownLanguage = NewErrorMessage(
	http.StatusBadRequest,
	1013,
	"Unknown language",
)

// GenericAllocateIDError is thrown when we fail to allocate new IDs with Datastore
var GenericAllocateIDError = NewErrorMessage(
	http.StatusInternalServerError,
	1014,
	"Failed to allocate IDs",
)

// GenericTooManyRequestsError is thrown when we fail to allocate new IDs with Datastore
var GenericTooManyRequestsError = NewErrorMessage(
	http.StatusTooManyRequests,
	1015,
	"Too many requests",
)

// GenericTaskDontRetryError is thrown on non-recoverable error.
var GenericTaskDontRetryError = NewErrorMessage(
	http.StatusOK,
	1016,
	"Not retryable error",
)

// AuthInvalidError is thrown when auth headers verification fails or isn't present
var AuthInvalidError = NewErrorMessage(
	http.StatusUnauthorized,
	2002,
	"Authentication failure",
)

// AuthForbiddenError is thrown when user is forbidden to use a route
var AuthForbiddenError = NewErrorMessage(
	http.StatusForbidden,
	2003,
	"Forbidden to use this path",
)

// AuthUserForbiddenError is thrown when a user tries to access a forbidden path
// Deprecated: Use AuthForbiddenError instead
var AuthUserForbiddenError = NewErrorMessage(
	http.StatusForbidden,
	2004,
	"Not a path for you",
)

// AuthAdminForbiddenError is thrown when an admin tries to access a path without the proper ACL
// Deprecated: Use AuthForbiddenError
var AuthAdminForbiddenError = NewErrorMessage(
	http.StatusForbidden,
	2005,
	"Insufficient rights",
)

// AuthMissingACL is thrown when we cannot retrieve a user's ACL
var AuthMissingACL = NewErrorMessage(
	http.StatusForbidden,
	2006,
	"Cannot get ACL",
)
