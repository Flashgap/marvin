package middlewares_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/internal/middlewares"
	"github.com/Flashgap/marvin/internal/middlewares/errorhandler"
	apperrors "github.com/Flashgap/marvin/internal/web/errors"
)

func TestMiddlewares(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Middlewares suite")
}

func sign(secret, timestamp, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:" + timestamp + ":" + body))
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

// newRouter wires the middleware in front of a 200-OK handler with the
// project's standard error handler so signature failures surface as 401.
func newRouter(cfg config.Slack, isDevEnv bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(errorhandler.Middleware(errorhandler.DefaultErrorMapping, errorhandler.WithFallback(apperrors.GenericInternalServerError)))
	r.POST("/slack/lock",
		middlewares.ValidateSlackWebhook(cfg, isDevEnv),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)
	return r
}

var _ = Describe("ValidateSlackWebhook", func() {
	const secret = "test-secret"
	body := "token=x&user_id=Uabc&text=<@Uxyz|alice>"

	freshTS := func() string { return strconv.FormatInt(time.Now().Unix(), 10) }

	It("accepts a valid signature", func() {
		ts := freshTS()
		req := httptest.NewRequest(http.MethodPost, "/slack/lock", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Slack-Request-Timestamp", ts)
		req.Header.Set("X-Slack-Signature", sign(secret, ts, body))

		rec := httptest.NewRecorder()
		newRouter(config.Slack{SlackSigningSecret: secret}, false).ServeHTTP(rec, req)
		Expect(rec.Code).To(Equal(http.StatusOK))
	})

	It("rejects a tampered signature", func() {
		ts := freshTS()
		req := httptest.NewRequest(http.MethodPost, "/slack/lock", strings.NewReader(body))
		req.Header.Set("X-Slack-Request-Timestamp", ts)
		req.Header.Set("X-Slack-Signature", "v0=deadbeef")

		rec := httptest.NewRecorder()
		newRouter(config.Slack{SlackSigningSecret: secret}, false).ServeHTTP(rec, req)
		Expect(rec.Code).To(Equal(http.StatusUnauthorized))
	})

	It("rejects a stale timestamp", func() {
		oldTS := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
		req := httptest.NewRequest(http.MethodPost, "/slack/lock", strings.NewReader(body))
		req.Header.Set("X-Slack-Request-Timestamp", oldTS)
		req.Header.Set("X-Slack-Signature", sign(secret, oldTS, body))

		rec := httptest.NewRecorder()
		newRouter(config.Slack{SlackSigningSecret: secret}, false).ServeHTTP(rec, req)
		Expect(rec.Code).To(Equal(http.StatusUnauthorized))
	})

	It("rejects missing headers", func() {
		req := httptest.NewRequest(http.MethodPost, "/slack/lock", strings.NewReader(body))
		rec := httptest.NewRecorder()
		newRouter(config.Slack{SlackSigningSecret: secret}, false).ServeHTTP(rec, req)
		Expect(rec.Code).To(Equal(http.StatusUnauthorized))
	})

	It("bypasses verification in dev when no secret is configured", func() {
		req := httptest.NewRequest(http.MethodPost, "/slack/lock", strings.NewReader(body))
		rec := httptest.NewRecorder()
		newRouter(config.Slack{}, true).ServeHTTP(rec, req)
		Expect(rec.Code).To(Equal(http.StatusOK))
	})
})
