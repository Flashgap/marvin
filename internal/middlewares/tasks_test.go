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
func newTaskRouter(cfg config.Tasks) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(errorhandler.Middleware(errorhandler.DefaultErrorMapping, errorhandler.WithFallback(apperrors.GenericInternalServerError)))
	r.POST("/task",
		middlewares.ValidateTaskSecret(cfg),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)
	return r
}

var _ = Describe("ValidateTaskSecret", func() {
	const secret = "test-secret"

	serve := func(cfg config.Tasks, authorization string) int {
		req := httptest.NewRequest(http.MethodPost, "/task", http.NoBody)
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		rec := httptest.NewRecorder()
		newTaskRouter(cfg).ServeHTTP(rec, req)
		return rec.Code
	}

	It("accepts the right bearer secret", func() {
		Expect(serve(config.Tasks{TasksSecret: secret}, "Bearer "+secret)).To(Equal(http.StatusOK))
	})

	It("rejects a wrong secret", func() {
		Expect(serve(config.Tasks{TasksSecret: secret}, "Bearer nope")).To(Equal(http.StatusUnauthorized))
	})

	It("rejects the secret without the Bearer scheme", func() {
		Expect(serve(config.Tasks{TasksSecret: secret}, secret)).To(Equal(http.StatusUnauthorized))
	})

	It("rejects a missing header", func() {
		Expect(serve(config.Tasks{TasksSecret: secret}, "")).To(Equal(http.StatusUnauthorized))
	})

	It("rejects every request when no secret is configured", func() {
		Expect(serve(config.Tasks{}, "Bearer ")).To(Equal(http.StatusUnauthorized))
	})
})
