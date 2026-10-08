package middlewares_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/internal/middlewares"
	"github.com/Flashgap/marvin/internal/middlewares/errorhandler"
	apperrors "github.com/Flashgap/marvin/internal/web/errors"
)

// newTaskRouter wires the middleware in front of a 200-OK handler with the
// project's standard error handler so secret failures surface as 401.
func newTaskRouter(cfg config.Tasks, isDevEnv bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(errorhandler.Middleware(errorhandler.DefaultErrorMapping, errorhandler.WithFallback(apperrors.GenericInternalServerError)))
	r.POST("/task",
		middlewares.ValidateTaskSecret(cfg, isDevEnv),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)
	return r
}

var _ = Describe("ValidateTaskSecret", func() {
	const secret = "test-secret"

	serve := func(cfg config.Tasks, isDevEnv bool, authorization string) int {
		req := httptest.NewRequest(http.MethodPost, "/task", http.NoBody)
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		rec := httptest.NewRecorder()
		newTaskRouter(cfg, isDevEnv).ServeHTTP(rec, req)
		return rec.Code
	}

	It("accepts the right bearer secret", func() {
		Expect(serve(config.Tasks{TasksSecret: secret}, false, "Bearer "+secret)).To(Equal(http.StatusOK))
	})

	It("rejects a wrong secret", func() {
		Expect(serve(config.Tasks{TasksSecret: secret}, false, "Bearer nope")).To(Equal(http.StatusUnauthorized))
	})

	It("rejects the secret without the Bearer scheme", func() {
		Expect(serve(config.Tasks{TasksSecret: secret}, false, secret)).To(Equal(http.StatusUnauthorized))
	})

	It("rejects a missing header", func() {
		Expect(serve(config.Tasks{TasksSecret: secret}, false, "")).To(Equal(http.StatusUnauthorized))
	})

	It("checks the secret in dev when one is configured", func() {
		Expect(serve(config.Tasks{TasksSecret: secret}, true, "")).To(Equal(http.StatusUnauthorized))
	})

	It("bypasses the check in dev when no secret is configured", func() {
		Expect(serve(config.Tasks{}, true, "")).To(Equal(http.StatusOK))
	})

	It("rejects every request outside dev when no secret is configured", func() {
		Expect(serve(config.Tasks{}, false, "Bearer ")).To(Equal(http.StatusUnauthorized))
	})
})
