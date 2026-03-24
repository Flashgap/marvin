package logger

import (
	"net/http"
	"strings"
	"time"

	"github.com/Flashgap/logrus"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// parseXCloudTraceContext returns the traceID and spanID
// "X-Cloud-Trace-Context: TRACE_ID/SPAN_ID;o=TRACE_TRUE"
//
// `TRACE_ID` is a 32-character hexadecimal value representing a 128-bit
// number. It should be unique between your requests, unless you
// intentionally want to bundle the requests together. You can use UUIDs.
//
// `SPAN_ID` is the decimal representation of the (unsigned) span ID. It
// should be 0 for the first span in your trace. For subsequent requests,
// set SPAN_ID to the span ID of the parent request. See the description
// of TraceSpan (REST, RPC) for more information about nested traces.
//
// `TRACE_TRUE` must be 1 to trace this request. Specify 0 to not trace the
// request.
func parseXCloudTraceContext(t string) (string, string) {
	// handle "TRACE_ID/SPAN_ID" missing the ";o=1" part.
	parts := strings.Split(t, ";")[0]
	if parts == "" {
		return "", ""
	}
	ids := strings.Split(parts, "/")
	if len(ids) == 2 {
		return ids[0], ids[1]
	}
	return parts, ""
}

// LogContextMiddleware sets context keys from generic request information, and logs response statuses
func LogContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.FullPath() == "/health" {
			return
		}
		// Set all necessary context values for the request
		c.Set(logrus.FieldKeyFullPath, c.FullPath())
		c.Set(logrus.FieldKeyRequest, c.Request)
		c.Set(logrus.FieldKeyRemoteIP, c.ClientIP())
		traceID, spanID := parseXCloudTraceContext(c.GetHeader("X-Cloud-Trace-Context"))
		if traceID == "" {
			traceID = uuid.NewString()
		}
		c.Set(logrus.FieldKeyTrace, traceID)
		if spanID != "" {
			c.Set(logrus.FieldKeySpan, spanID)
		}
		c.Set(logrus.FieldKeyStartAt, time.Now().UTC())

		// Proceed with the handlers' code
		c.Next()

		// Catch the result and log the request info
		c.Set(logrus.FieldKeyResponseStatus, c.Writer.Status())
		c.Set(logrus.FieldKeyResponseSize, c.Writer.Size())
		if c.Writer.Status() >= http.StatusInternalServerError {
			WithContext(c).Critical(c.FullPath())
		} else if c.Writer.Status() >= http.StatusBadRequest {
			WithContext(c).Error(c.FullPath())
		} else {
			WithContext(c).Info(c.FullPath())
		}
	}
}
