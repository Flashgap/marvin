package middlewares

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/api/idtoken"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/internal/web"
	stderror "github.com/Flashgap/marvin/pkg/stderr"
)

// signingAlgorithm is the only algorithm Google signs ID tokens with: its
// discovery document lists id_token_signing_alg_values_supported: ["RS256"].
// It's checked before idtoken sees the token, since idtoken checks ES256
// tokens against IAP's keys and panics on a short ES256 signature. Widening it
// is a security decision, not a compatibility fix.
const signingAlgorithm = "RS256"

// googleIssuers are the two values Google uses for the iss claim of its ID tokens.
var googleIssuers = []string{"https://accounts.google.com", "accounts.google.com"}

// jwtHeader is the part of a JWT header read before the token is verified.
type jwtHeader struct {
	Algorithm string `json:"alg"`
}

// idTokenClaims are the claims of a Google ID token that idtoken doesn't check.
type idTokenClaims struct {
	Issuer        string `json:"iss"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

// ValidateTaskToken verifies that inbound task requests (e.g. from Cloud
// Scheduler) carry "Authorization: Bearer <token>", where token is a Google
// OIDC token minted for cfg.TasksServiceAccount with cfg.TasksAudience as
// audience.
//
// Marvin checks the token itself; it doesn't rely on anything in front of it,
// which must pass the Authorization header through unchanged. validator checks
// the signature against Google's public keys, the audience and the expiry; the
// algorithm, issuer, email and email_verified are checked around it. The email
// check is what pins the caller: any service account can get a Google-signed
// token for our audience.
//
// In dev (IsDevEnv), verification is skipped, same escape hatch as
// ValidateGithubWebhook. Everywhere else it always runs, and a missing field
// rejects every request.
func ValidateTaskToken(cfg config.Tasks, isDevEnv bool, validator *idtoken.Validator) gin.HandlerFunc {
	if isDevEnv {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		if err := verifyTaskToken(c.Request.Context(), c.GetHeader("Authorization"), cfg, validator); err != nil {
			web.DefaultController.Error(c, fmt.Errorf("%w: ValidateTaskToken: %w", stderror.ErrUnauthorized, err))
			return
		}

		c.Next()
	}
}

func verifyTaskToken(ctx context.Context, authorization string, cfg config.Tasks, validator *idtoken.Validator) error {
	if !cfg.TasksConfigured() {
		return errors.New("MARVIN_TASKS_AUDIENCE and MARVIN_TASKS_SERVICE_ACCOUNT must both be set")
	}

	token, ok := strings.CutPrefix(authorization, "Bearer ")
	if !ok {
		return errors.New("missing bearer token")
	}

	segments := strings.Split(token, ".")
	if len(segments) != 3 {
		return fmt.Errorf("malformed token: %d segments", len(segments))
	}

	var header jwtHeader
	if err := decodeSegment(segments[0], &header); err != nil {
		return fmt.Errorf("decode token header: %w", err)
	}
	if header.Algorithm != signingAlgorithm {
		return fmt.Errorf("unexpected alg %q", header.Algorithm)
	}

	// idtoken returns the claims as a map: the ones it doesn't check are decoded
	// from the same segment into idTokenClaims instead.
	if _, err := validator.Validate(ctx, token, cfg.TasksAudience); err != nil {
		return err
	}

	var claims idTokenClaims
	if err := decodeSegment(segments[1], &claims); err != nil {
		return fmt.Errorf("decode token claims: %w", err)
	}
	if !slices.Contains(googleIssuers, claims.Issuer) {
		return fmt.Errorf("unexpected issuer %q", claims.Issuer)
	}
	if claims.Email != cfg.TasksServiceAccount {
		return fmt.Errorf("unexpected email %q", claims.Email)
	}
	if !claims.EmailVerified {
		return errors.New("email not verified")
	}

	return nil
}

// decodeSegment decodes a JWT segment, base64url-encoded JSON, into v. It
// decodes it with encoding/json like idtoken does, so both read the same
// values, duplicate or differently cased keys included.
func decodeSegment(segment string, v any) error {
	decoded, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return err
	}
	return json.Unmarshal(decoded, v)
}
