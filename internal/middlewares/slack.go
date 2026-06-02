package middlewares

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/internal/web"
	stderror "github.com/Flashgap/marvin/pkg/stderr"
)

// slackMaxRequestAge bounds how far a request's timestamp may be from now
// before the signature is considered replayed. Slack's recommendation.
const slackMaxRequestAge = 5 * time.Minute

// ValidateSlackWebhook verifies inbound slash-command requests with the
// X-Slack-Signature HMAC scheme. The middleware reads the request body and
// restores it for downstream handlers so they can re-bind the form payload.
//
// In dev (IsDevEnv) when SlackSigningSecret is empty, signature verification
// is skipped — same escape hatch as ValidateGithubWebhook so local dev doesn't
// need a real Slack app.
func ValidateSlackWebhook(cfg config.Slack, isDevEnv bool) gin.HandlerFunc {
	bypass := isDevEnv && cfg.SlackSigningSecret == ""

	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			web.DefaultController.Error(c, fmt.Errorf("%w: ValidateSlackWebhook read body: %w", stderror.ErrUnauthorized, err))
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		if bypass {
			c.Next()
			return
		}

		if err := verifySlackSignature(cfg.SlackSigningSecret, c.Request.Header.Get("X-Slack-Request-Timestamp"), c.Request.Header.Get("X-Slack-Signature"), body, time.Now()); err != nil {
			web.DefaultController.Error(c, fmt.Errorf("%w: ValidateSlackWebhook: %w", stderror.ErrUnauthorized, err))
			return
		}

		c.Next()
	}
}

func verifySlackSignature(secret, timestamp, signature string, body []byte, now time.Time) error {
	if secret == "" {
		return fmt.Errorf("missing signing secret")
	}
	if timestamp == "" || signature == "" {
		return fmt.Errorf("missing signature headers")
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}
	if delta := now.Unix() - ts; delta > int64(slackMaxRequestAge.Seconds()) || delta < -int64(slackMaxRequestAge.Seconds()) {
		return fmt.Errorf("stale request timestamp")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	// Slack's signing base string: v0:<timestamp>:<raw body>.
	mac.Write([]byte("v0:"))
	mac.Write([]byte(timestamp))
	mac.Write([]byte(":"))
	mac.Write(body)
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}
