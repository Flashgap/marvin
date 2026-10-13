package middlewares_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"

	"github.com/Flashgap/marvin/internal/config"
	"github.com/Flashgap/marvin/internal/middlewares"
	"github.com/Flashgap/marvin/internal/middlewares/errorhandler"
	apperrors "github.com/Flashgap/marvin/internal/web/errors"
)

const (
	taskAudience       = "https://marvin.example.com"
	taskServiceAccount = "marvin-trigger@project.iam.gserviceaccount.com"
	googleKeyID        = "google-key"
	googleKeysURL      = "https://www.googleapis.com/oauth2/v3/certs"
	iapKeyID           = "iap-key"
	iapKeysURL         = "https://www.gstatic.com/iap/verify/public_key-jwk"
)

// googleKey and iapKey stand in for Google's and IAP's signing keys:
// fakeKeysClient serves their public halves as the key sets idtoken fetches.
var (
	googleKey = newRSAKey()
	iapKey    = newECKey()
)

// jwk is a public key as Google and IAP publish them, in a jwkSet.
type jwk struct {
	KeyID     string `json:"kid"`
	Algorithm string `json:"alg"`
	KeyType   string `json:"kty"`
	Use       string `json:"use"`
	N         string `json:"n,omitempty"`
	E         string `json:"e,omitempty"`
	Curve     string `json:"crv,omitempty"`
	X         string `json:"x,omitempty"`
	Y         string `json:"y,omitempty"`
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

func newECKey() *ecdsa.PrivateKey {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
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

// fakeKeysClient answers the validator's fetches of Google's and IAP's public
// keys with googleKey and iapKey, and any other request with a 404.
func fakeKeysClient() *http.Client {
	point, err := iapKey.PublicKey.Bytes() // 0x04 || X || Y
	Expect(err).ToNot(HaveOccurred())

	keySets := map[string][]byte{
		googleKeysURL: toJSON(jwkSet{Keys: []jwk{{
			KeyID:     googleKeyID,
			Algorithm: "RS256",
			KeyType:   "RSA",
			Use:       "sig",
			N:         b64(googleKey.N.Bytes()),
			E:         b64(big.NewInt(int64(googleKey.E)).Bytes()),
		}}}),
		iapKeysURL: toJSON(jwkSet{Keys: []jwk{{
			KeyID:     iapKeyID,
			Algorithm: "ES256",
			KeyType:   "EC",
			Use:       "sig",
			Curve:     "P-256",
			X:         b64(point[1:33]),
			Y:         b64(point[33:]),
		}}}),
	}

	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		keySet, ok := keySets[r.URL.String()]
		if !ok {
			return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody, Request: r}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(keySet)), Request: r}, nil
	})}
}

// signingInput is the part of a JWT its signature covers: its encoded header
// and claims.
func signingInput(header tokenHeader, claims tokenClaims) string {
	return b64(toJSON(header)) + "." + b64(toJSON(claims))
}

// signToken builds a JWT carrying claims, signed with key under kid.
func signToken(key *rsa.PrivateKey, kid string, claims tokenClaims) string {
	input := signingInput(tokenHeader{Algorithm: "RS256", KeyID: kid, Type: "JWT"}, claims)
	digest := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	Expect(err).ToNot(HaveOccurred())

	return input + "." + b64(signature)
}

// schedulerClaims are the claims of a token Cloud Scheduler gets from Google
// for taskServiceAccount.
func schedulerClaims() tokenClaims {
	now := time.Now()
	return tokenClaims{
		Issuer:          "https://accounts.google.com",
		Audience:        taskAudience,
		AuthorizedParty: "1234567890",
		Subject:         "1234567890",
		Email:           taskServiceAccount,
		EmailVerified:   true,
		IssuedAt:        now.Unix(),
		Expiry:          now.Add(time.Hour).Unix(),
	}
}

// schedulerToken is a token Cloud Scheduler gets from Google for
// taskServiceAccount.
func schedulerToken() string {
	return signToken(googleKey, googleKeyID, schedulerClaims())
}

// newTaskRouter wires the middleware in front of a 200-OK handler with the
// project's standard error handler so token failures surface as 401.
func newTaskRouter(cfg config.Tasks, isDevEnv bool) *gin.Engine {
	validator, err := idtoken.NewValidator(context.Background(), option.WithHTTPClient(fakeKeysClient()))
	Expect(err).ToNot(HaveOccurred())

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(errorhandler.Middleware(errorhandler.DefaultErrorMapping, errorhandler.WithFallback(apperrors.GenericInternalServerError)))
	r.POST("/task",
		middlewares.ValidateTaskToken(cfg, isDevEnv, validator),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)
	return r
}

// serveTask sends a task request, with authorization as its Authorization
// header when not empty, and returns the response status.
func serveTask(cfg config.Tasks, isDevEnv bool, authorization string) int {
	req := httptest.NewRequest(http.MethodPost, "/task", http.NoBody)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	newTaskRouter(cfg, isDevEnv).ServeHTTP(rec, req)
	return rec.Code
}

var _ = Describe("ValidateTaskToken", func() {
	configured := config.Tasks{TasksAudience: taskAudience, TasksServiceAccount: taskServiceAccount}

	It("accepts a token Google minted for the service account", func() {
		Expect(serveTask(configured, false, "Bearer "+schedulerToken())).To(Equal(http.StatusOK))
	})

	It("rejects a missing header or a token without the Bearer scheme", func() {
		Expect(serveTask(configured, false, "")).To(Equal(http.StatusUnauthorized))
		Expect(serveTask(configured, false, schedulerToken())).To(Equal(http.StatusUnauthorized))
	})

	It("rejects a token signed with another key", func() {
		forged := signToken(newRSAKey(), googleKeyID, schedulerClaims())
		Expect(serveTask(configured, false, "Bearer "+forged)).To(Equal(http.StatusUnauthorized))
	})

	It("rejects a token not signed with RS256", func() {
		// Without the RS256 pin, idtoken checks it against IAP's keys and panics on the signature's length.
		header := tokenHeader{Algorithm: "ES256", KeyID: iapKeyID, Type: "JWT"}
		token := signingInput(header, schedulerClaims()) + "." + b64([]byte{1})
		Expect(serveTask(configured, false, "Bearer "+token)).To(Equal(http.StatusUnauthorized))
	})

	DescribeTable("rejects a Google-signed token with",
		func(tamper func(*tokenClaims)) {
			claims := schedulerClaims()
			tamper(&claims)
			token := signToken(googleKey, googleKeyID, claims)
			Expect(serveTask(configured, false, "Bearer "+token)).To(Equal(http.StatusUnauthorized))
		},
		Entry("an expiry in the past", func(c *tokenClaims) { c.Expiry = time.Now().Add(-time.Minute).Unix() }),
		Entry("another audience", func(c *tokenClaims) { c.Audience = "https://other.example.com" }),
		Entry("another service account", func(c *tokenClaims) { c.Email = "attacker@other-project.iam.gserviceaccount.com" }),
		Entry("an unverified email", func(c *tokenClaims) { c.EmailVerified = false }),
		Entry("another issuer", func(c *tokenClaims) { c.Issuer = "https://cloud.google.com/iap" }),
	)

	It("rejects every request when the config is incomplete", func() {
		token := "Bearer " + schedulerToken()
		Expect(serveTask(config.Tasks{}, false, token)).To(Equal(http.StatusUnauthorized))
		Expect(serveTask(config.Tasks{TasksAudience: taskAudience}, false, token)).To(Equal(http.StatusUnauthorized))
		Expect(serveTask(config.Tasks{TasksServiceAccount: taskServiceAccount}, false, token)).To(Equal(http.StatusUnauthorized))
	})

	It("skips verification in dev", func() {
		Expect(serveTask(config.Tasks{}, true, "")).To(Equal(http.StatusOK))
		Expect(serveTask(configured, true, "")).To(Equal(http.StatusOK))
	})
})
