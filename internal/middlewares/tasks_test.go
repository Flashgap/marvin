package middlewares_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Flashgap/marvin/internal/middlewares"
	"github.com/Flashgap/marvin/internal/middlewares/errorhandler"
	apperrors "github.com/Flashgap/marvin/internal/web/errors"
)

var _ = Describe("ValidateTaskToken", func() {
	var verified []string // tokens verify was called with

	BeforeEach(func() {
		verified = nil
	})

	// verifyReturning records the tokens it's called with, then returns err.
	verifyReturning := func(err error) func(context.Context, string) error {
		return func(_ context.Context, token string) error {
			verified = append(verified, token)
			return err
		}
	}

	// serve sends a task request through the middleware, with authorization as
	// its Authorization header when not empty, and returns the response status.
	serve := func(isDevEnv bool, verify func(context.Context, string) error, authorization string) int {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.Use(errorhandler.Middleware(errorhandler.DefaultErrorMapping, errorhandler.WithFallback(apperrors.GenericInternalServerError)))
		r.POST("/task",
			middlewares.ValidateTaskToken(isDevEnv, verify),
			func(c *gin.Context) { c.Status(http.StatusOK) },
		)

		req := httptest.NewRequest(http.MethodPost, "/task", http.NoBody)
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}

	It("lets the request through when verify accepts the token", func() {
		Expect(serve(false, verifyReturning(nil), "Bearer some.id.token")).To(Equal(http.StatusOK))
		Expect(verified).To(Equal([]string{"some.id.token"}))
	})

	It("rejects the request when verify rejects the token", func() {
		Expect(serve(false, verifyReturning(errors.New("invalid")), "Bearer some.id.token")).To(Equal(http.StatusUnauthorized))
	})

	It("rejects a missing header or a token without the Bearer scheme, without verifying it", func() {
		Expect(serve(false, verifyReturning(nil), "")).To(Equal(http.StatusUnauthorized))
		Expect(serve(false, verifyReturning(nil), "some.id.token")).To(Equal(http.StatusUnauthorized))
		Expect(verified).To(BeEmpty())
	})

	It("skips verification in dev", func() {
		Expect(serve(true, verifyReturning(errors.New("invalid")), "")).To(Equal(http.StatusOK))
		Expect(verified).To(BeEmpty())
	})
})
