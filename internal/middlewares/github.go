package middlewares

import (
	"bytes"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v63/github"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/internal/web"
	stderror "github.com/Flashgap/marvin/pkg/stderr"
)

func ValidateGithubWebhook(cfg config.Github, isDevEnv bool) gin.HandlerFunc {
	if isDevEnv {
		return func(c *gin.Context) {
			delivery := c.Request.Header.Get("X-GitHub-Delivery")
			ghCtx := &GithubContext{Delivery: delivery}
			SetGHContext(c, ghCtx)

			c.Next()
		}
	}

	return func(c *gin.Context) {
		payload, err := github.ValidatePayload(c.Request, []byte(cfg.GithubWebhookSecret))
		if err != nil {
			web.DefaultController.Error(c, fmt.Errorf("%w: ValidateGithubWebhook: %w", stderror.ErrUnauthorized, err))
			return
		}

		delivery := c.Request.Header.Get("X-GitHub-Delivery")
		ghCtx := &GithubContext{Delivery: delivery}
		SetGHContext(c, ghCtx)

		c.Request.Body = io.NopCloser(bytes.NewReader(payload))

		c.Next()
	}
}
