package middlewares

import (
	"context"
	"fmt"

	"github.com/Flashgap/logrus"
	"github.com/gin-gonic/gin"

	"github.com/Flashgap/marvin/pkg/logger"
)

const (
	// ghContextKey is the name of the key inside Gin context
	ghContextKey = "github"
)

// GithubContext wraps GitHub information.
type GithubContext struct {
	// X-GitHub-Delivery header
	Delivery string
	// Repository name
	Repository string
	// Identifier (usually check run ID or PR number)
	Identifier int
}

// SetGHContext sets a GithubContext in the gin context.
func SetGHContext(c *gin.Context, ghCtx *GithubContext) {
	c.Set(ghContextKey, ghCtx)
}

// GetGHContext gets an GithubContext from a context.
func GetGHContext(ctx context.Context) *GithubContext {
	if val, ok := ctx.Value(ghContextKey).(*GithubContext); ok {
		return val
	}

	return nil
}

// EnrichGHContext adds basic information to an existing GHContext.
func EnrichGHContext(ctx context.Context, repo string, id int) {
	if ghCtx := GetGHContext(ctx); ghCtx != nil {
		ghCtx.Repository = repo
		ghCtx.Identifier = id
	}
}

// AmendGHContextIdentifier changes the identifier field of the GitHub context.
// Useful when we resolve the PR Number from a check run.
func AmendGHContextIdentifier(ctx context.Context, id int) {
	if ghCtx := GetGHContext(ctx); ghCtx != nil {
		ghCtx.Identifier = id
	}
}

// LoggerFromGHContext returns a logrus entry with prefixes set to the values in the GHContext.
func LoggerFromGHContext(ctx context.Context, prefix string) *logrus.Entry {
	if ghCtx := GetGHContext(ctx); ghCtx != nil {
		return logger.WithContext(ctx).WithPrefix(
			fmt.Sprintf("%s[%s][%s][#%d]", ghCtx.Delivery, prefix, ghCtx.Repository, ghCtx.Identifier),
		)
	}

	return logger.WithContext(ctx).WithPrefix("[" + prefix + "]")
}
