package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	apperrors "github.com/Flashgap/marvin/internal/web/errors"
	stderror "github.com/Flashgap/marvin/pkg/stderr"
	"github.com/Flashgap/marvin/pkg/utils"
)

var (
	ErrTooLarge = errors.New("request body too large")
)

// DefaultController is a global controller that can be used in middlewares and tests
var DefaultController = &BaseController{}

// Controller represents common actions that should be implemented by controllers.
type Controller interface {
	// RouteEndpoints will add handlers to the `_ep` route group.
	// router is under authN/authZ.
	RouteEndpoints(router gin.IRouter)

	// RouteTasks will add handlers to the `_task` route group.
	// router is under authN/authZ.
	RouteTasks(router gin.IRouter)

	// RouteWebhooks will add handlers to the `_webhook` route group.
	// router is under authN/authZ.
	RouteWebhooks(router gin.IRouter)
}

// BaseController is a base struct for creating Gin based web controller.
type BaseController struct {
}

// Error is a shorthand to set Gin error if not nil and abort. Returns true if err != nil.
func (*BaseController) Error(c *gin.Context, err error) bool {
	// Catch nil is not nil apperrors.
	var errMsg *apperrors.ErrorMessage
	if errors.As(err, &errMsg) && errMsg == nil { // nolint:revive // intended
		return false
	}

	if err != nil {
		_ = c.Error(err)
		c.Abort()
		return true
	}

	return false
}

// Bind is a shorthand to bind a request. Returns false if error.
// binding variadic param could be a specific binding.Binding or will be defaulted to binding.JSON.
func (*BaseController) Bind(c *gin.Context, obj any, bindings ...binding.Binding) bool {
	if len(bindings) > 0 {
		if err := c.ShouldBindWith(obj, bindings[0]); err != nil {
			_ = c.Error(fmt.Errorf("%w: %w: %s", stderror.ErrParsing, err, bindings[0].Name()))
			return false
		}
	} else {
		if err := c.ShouldBindJSON(obj); err != nil {
			_ = c.Error(fmt.Errorf("%w: %w: json", stderror.ErrParsing, err))
			return false
		}
	}

	return true
}

// BindJSONProto is a shorthand to bind a JSON protobuf request or abort.
func (*BaseController) BindJSONProto(c *gin.Context, obj proto.Message) bool {
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		_ = c.Error(fmt.Errorf("%w: failed to read body: %w", stderror.ErrInvalidParam, err))
		return false
	}

	unmarshaller := protojson.UnmarshalOptions{
		AllowPartial: true, // Needed for Protobuf backward compatibility.
	}
	if err := unmarshaller.Unmarshal(data, obj); err != nil {
		_ = c.Error(fmt.Errorf("%w: failed to parse json: %w", stderror.ErrParsing, err))
		return false
	}

	return true
}

// BindProto is a shorthand to bind a protobuf request or abort.
func (*BaseController) BindProto(c *gin.Context, obj proto.Message) bool {
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		_ = c.Error(fmt.Errorf("%w: failed to read body: %w", stderror.ErrInvalidParam, err))
		return false
	}

	if err := proto.Unmarshal(data, obj); err != nil {
		_ = c.Error(fmt.Errorf("%w: failed to unmarshal proto: %w", stderror.ErrParsing, err))
		return false
	}

	return true
}

// BindMultipart is a shorthand to bind a multipart payload.
// Returns false if error.
func (b *BaseController) BindMultipart(c *gin.Context, obj any) bool {
	if err := c.ShouldBindWith(obj, binding.FormMultipart); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			_ = c.Error(fmt.Errorf("%w: Media too large: %w (limit=%d)", ErrTooLarge, err, maxBytesError.Limit))
			return false
		}

		if err := c.ShouldBindJSON(obj); err != nil {
			_ = c.Error(fmt.Errorf("%w: %w: multipart", stderror.ErrParsing, err))
			return false
		}
	}

	return true
}

// StartCursor returns the optional "start_cursor" query parameter.
func (b *BaseController) StartCursor(c *gin.Context) *string {
	return utils.NilIfZero(c.Query("start_cursor"))
}

// UserID is a shorthand to get current UserID.
// Returns true if param found.
func (b *BaseController) UserID(c *gin.Context, param *string) bool {
	return b.Param(c, UserIDParam, param)
}

// Param is a shorthand to get a URL parameter. Returns false if not found.
func (*BaseController) Param(c *gin.Context, name Param, param any) bool {
	val := c.Param(string(name))
	if val == "" {
		_ = c.Error(fmt.Errorf("%w: %s", stderror.ErrMissingParam, name))
		return false
	}

	switch v := param.(type) {
	case *string:
		*v = val
	case *int:
		if intVal, err := strconv.Atoi(val); err != nil {
			_ = c.Error(fmt.Errorf("%w: %s: cannot convert %q to an integer", stderror.ErrInvalidParam, name, val))
			return false
		} else {
			*v = intVal
		}
	case *int64:
		if intVal, err := strconv.ParseInt(val, 10, 64); err != nil {
			_ = c.Error(fmt.Errorf("%w: %s: cannot convert %q to an integer", stderror.ErrInvalidParam, name, val))
			return false
		} else {
			*v = intVal
		}
	case *bool:
		if boolVal, err := strconv.ParseBool(val); err != nil {
			_ = c.Error(fmt.Errorf("%w: %s: cannot convert %q to an boolean", stderror.ErrInvalidParam, name, val))
			return false
		} else {
			*v = boolVal
		}
	default:
		_ = c.Error(fmt.Errorf("%w: %s: invalid param type for %q", stderror.ErrInvalidParam, name, val))
		return false
	}

	return true
}
