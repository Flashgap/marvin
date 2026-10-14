// Package googleid verifies ID tokens signed by Google, such as the OIDC
// tokens Cloud Scheduler sends.
package googleid

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
)

// issuers are the two values Google uses for the iss claim of its ID tokens.
var issuers = []string{"https://accounts.google.com", "accounts.google.com"}

// Verifier verifies that ID tokens are Google's, minted for one service
// account with one audience.
type Verifier struct {
	validator      *idtoken.Validator
	audience       string
	serviceAccount string
}

// NewVerifier returns a Verifier accepting the tokens Google mints for
// serviceAccount with audience. It fetches Google's public keys with client.
func NewVerifier(ctx context.Context, client *http.Client, audience, serviceAccount string) (*Verifier, error) {
	validator, err := idtoken.NewValidator(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}
	return &Verifier{validator: validator, audience: audience, serviceAccount: serviceAccount}, nil
}

// Verify returns nil if token is a Google ID token minted for the Verifier's
// service account, with its audience. idtoken checks the signature against
// Google's public keys, the audience and the expiry; the issuer, email and
// email_verified are checked here. The email check is what pins the caller:
// any service account can get a Google-signed token for any audience.
//
// Verify rejects every token when the audience or the service account is
// empty.
func (v *Verifier) Verify(ctx context.Context, token string) error {
	if v.audience == "" || v.serviceAccount == "" {
		return errors.New("googleid: audience and service account must both be set")
	}

	payload, err := v.validator.Validate(ctx, token, v.audience)
	if err != nil {
		return err
	}

	if !slices.Contains(issuers, payload.Issuer) {
		return fmt.Errorf("googleid: unexpected issuer %q", payload.Issuer)
	}
	if email, _ := payload.Claims["email"].(string); email != v.serviceAccount {
		return fmt.Errorf("googleid: unexpected email %q", email)
	}
	if verified, _ := payload.Claims["email_verified"].(bool); !verified {
		return errors.New("googleid: email not verified")
	}

	return nil
}
