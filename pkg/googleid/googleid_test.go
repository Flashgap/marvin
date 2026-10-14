package googleid_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/Flashgap/marvin/pkg/googleid"
)

func TestGoogleID(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "googleid suite")
}

const (
	audience       = "https://marvin.example.com"
	serviceAccount = "marvin-trigger@project.iam.gserviceaccount.com"
	googleKeyID    = "google-key"
	googleKeysURL  = "https://www.googleapis.com/oauth2/v3/certs"
)

// googleKey stands in for Google's signing key: fakeKeysTransport serves its
// public half as the key set idtoken fetches.
var googleKey = newRSAKey()

// jwk is an RSA public key as Google publishes them, in a jwkSet.
type jwk struct {
	KeyID     string `json:"kid"`
	Algorithm string `json:"alg"`
	KeyType   string `json:"kty"`
	Use       string `json:"use"`
	N         string `json:"n"`
	E         string `json:"e"`
}

type jwkSet struct {
	Keys []jwk `json:"keys"`
}

// tokenHeader and tokenClaims are the header and claims of a Google ID token.
type tokenHeader struct {
	Algorithm string `json:"alg"`
	KeyID     string `json:"kid"`
	Type      string `json:"typ"`
}

type tokenClaims struct {
	Issuer          string `json:"iss"`
	Audience        string `json:"aud"`
	AuthorizedParty string `json:"azp"`
	Subject         string `json:"sub"`
	Email           string `json:"email"`
	EmailVerified   bool   `json:"email_verified"`
	IssuedAt        int64  `json:"iat"`
	Expiry          int64  `json:"exp"`
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newRSAKey() *rsa.PrivateKey {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return key
}

func b64(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func toJSON(v any) []byte {
	b, err := json.Marshal(v)
	Expect(err).ToNot(HaveOccurred())
	return b
}

// fakeKeysTransport answers the validator's fetch of Google's public keys with
// googleKey, and any other request with a 404.
func fakeKeysTransport() http.RoundTripper {
	keySet := toJSON(jwkSet{Keys: []jwk{{
		KeyID:     googleKeyID,
		Algorithm: "RS256",
		KeyType:   "RSA",
		Use:       "sig",
		N:         b64(googleKey.N.Bytes()),
		E:         b64(big.NewInt(int64(googleKey.E)).Bytes()),
	}}})

	return roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != googleKeysURL {
			return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody, Request: r}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(keySet)), Request: r}, nil
	})
}

// signToken builds a JWT carrying claims, signed with key under kid.
func signToken(key *rsa.PrivateKey, kid string, claims tokenClaims) string {
	header := tokenHeader{Algorithm: "RS256", KeyID: kid, Type: "JWT"}
	input := b64(toJSON(header)) + "." + b64(toJSON(claims))
	digest := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	Expect(err).ToNot(HaveOccurred())

	return input + "." + b64(signature)
}

// schedulerClaims are the claims of a token Cloud Scheduler gets from Google
// for serviceAccount.
func schedulerClaims() tokenClaims {
	now := time.Now()
	return tokenClaims{
		Issuer:          "https://accounts.google.com",
		Audience:        audience,
		AuthorizedParty: "1234567890",
		Subject:         "1234567890",
		Email:           serviceAccount,
		EmailVerified:   true,
		IssuedAt:        now.Unix(),
		Expiry:          now.Add(time.Hour).Unix(),
	}
}

var _ = Describe("Verifier", func() {
	// verify checks token with a Verifier for aud and account.
	verify := func(ctx SpecContext, aud, account, token string) error {
		verifier, err := googleid.NewVerifier(ctx, fakeKeysTransport(), aud, account)
		Expect(err).ToNot(HaveOccurred())
		return verifier.Verify(ctx, token)
	}

	It("accepts a token Google minted for the service account", func(ctx SpecContext) {
		token := signToken(googleKey, googleKeyID, schedulerClaims())
		Expect(verify(ctx, audience, serviceAccount, token)).To(Succeed())
	})

	It("fetches Google's public keys with a deadline", func(ctx SpecContext) {
		var hasDeadline bool
		keys := fakeKeysTransport()
		transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
			_, hasDeadline = r.Context().Deadline()
			return keys.RoundTrip(r)
		})
		verifier, err := googleid.NewVerifier(ctx, transport, audience, serviceAccount)
		Expect(err).ToNot(HaveOccurred())

		// Verify gets a context without a deadline: only the Verifier's own
		// client can set one on the key fetch.
		token := signToken(googleKey, googleKeyID, schedulerClaims())
		Expect(verifier.Verify(context.Background(), token)).To(Succeed())
		Expect(hasDeadline).To(BeTrue())
	})

	It("rejects a token signed with another key", func(ctx SpecContext) {
		token := signToken(newRSAKey(), googleKeyID, schedulerClaims())
		Expect(verify(ctx, audience, serviceAccount, token)).ToNot(Succeed())
	})

	DescribeTable("rejects a Google-signed token with",
		func(ctx SpecContext, tamper func(*tokenClaims)) {
			claims := schedulerClaims()
			tamper(&claims)
			token := signToken(googleKey, googleKeyID, claims)
			Expect(verify(ctx, audience, serviceAccount, token)).ToNot(Succeed())
		},
		Entry("an expiry in the past", func(c *tokenClaims) { c.Expiry = time.Now().Add(-time.Minute).Unix() }),
		Entry("another audience", func(c *tokenClaims) { c.Audience = "https://other.example.com" }),
		Entry("another issuer", func(c *tokenClaims) { c.Issuer = "https://cloud.google.com/iap" }),
		Entry("another service account", func(c *tokenClaims) { c.Email = "attacker@other-project.iam.gserviceaccount.com" }),
		Entry("an unverified email", func(c *tokenClaims) { c.EmailVerified = false }),
	)

	It("rejects every token when the audience or the service account is empty", func(ctx SpecContext) {
		token := signToken(googleKey, googleKeyID, schedulerClaims())
		Expect(verify(ctx, "", "", token)).ToNot(Succeed())
		Expect(verify(ctx, audience, "", token)).ToNot(Succeed())
		Expect(verify(ctx, "", serviceAccount, token)).ToNot(Succeed())
	})
})
