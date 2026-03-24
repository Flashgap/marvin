package errorhandler

import (
	"errors"
	"fmt"
	"slices"

	"github.com/gin-gonic/gin"

	apperrors "github.com/Flashgap/marvin/internal/web/errors"
	"github.com/Flashgap/marvin/pkg/logger"
	"github.com/Flashgap/marvin/pkg/option"
	"github.com/Flashgap/marvin/pkg/utils"
)

type errorHandlerOptions struct {
	includeDetail bool
	fallbackErr   *apperrors.ErrorMessage
}

type Option = option.Option[errorHandlerOptions]

// WithFallback is an option that enables the error handler to fallback to the provided error if no mapping is found.
// If not set, the error handler will not abort the request if no mapping is found.
func WithFallback(errMsg *apperrors.ErrorMessage) Option {
	return func(o *errorHandlerOptions) {
		o.fallbackErr = errMsg
	}
}

// Middleware is middleware that enables you to configure error handling from a centralised place via its fluent API.
// Warning: Don't forget to set WithFallback() if you want to fallback to a generic error if no mapping is found.
// You can chain several middleware to handle different error mappings.
func Middleware(errorMappings []*ErrorMapping, opts ...option.Option[errorHandlerOptions]) gin.HandlerFunc {
	o := option.New(opts)

	return func(c *gin.Context) {
		c.Next() // This middleware runs after the handler.

		if c.Writer.Written() { // This is to prevent rewriting the response.
			return // Continue, response already written.
		}

		if len(c.Errors) == 0 {
			return // Continue, no errors.
		}

		// We iterate handler errors in reverse order: In most of the case we should try to abort only
		// with one error. But in any case, the last one should be the most relevant.
		handlerErrors := slices.Clone(c.Errors)
		slices.Reverse(handlerErrors)

		var sourceErr error                     // This is the error that will be converted to an app error.
		var finalAppErr *apperrors.ErrorMessage // This is the final app error.
		var mapping *ErrorMapping               // This is the mapping that was used to find the final error.

		for _, ginErr := range handlerErrors {
			if errors.As(ginErr.Err, &finalAppErr) {
				sourceErr = ginErr.Err
				break // Error is already an app error
			}

			// Try to find if in mapping:
			for _, mapping = range errorMappings {
				if appErr := mapping.HasMapping(ginErr.Err); appErr != nil {
					sourceErr = ginErr.Err
					finalAppErr = appErr
					break // We found a mapping.
				}
			}
		}

		if finalAppErr == nil && o.fallbackErr == nil {
			return // Continue, no mapping found and no fallback error set.
		}

		if finalAppErr == nil {
			finalAppErr = o.fallbackErr
		}

		if mapping == nil || !mapping.DisableLog {
			if sourceErr != nil {
				logger.WithContext(c).
					WithPrefix(fmt.Sprintf("[%s]", utils.CleanFuncName(c.HandlerName()))).
					WithError(sourceErr).
					Error(sourceErr.Error())
			} else {
				logger.WithContext(c).
					WithPrefix(fmt.Sprintf("[%s]", utils.CleanFuncName(c.HandlerName()))).
					WithError(finalAppErr).
					Error(c.Errors.Last().Error())
			}
		}

		if o.includeDetail && sourceErr != nil {
			c.AbortWithStatusJSON(finalAppErr.HTTPCode, finalAppErr.WithDetail(sourceErr))
		} else {
			c.AbortWithStatusJSON(finalAppErr.HTTPCode, finalAppErr)
		}
	}
}
