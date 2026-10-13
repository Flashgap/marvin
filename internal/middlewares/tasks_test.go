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
	"maps"
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
// googleKeysClient serves their public halves as the key sets idtoken fetches.
var (
	googleKey = newRSAKey()
	iapKey    = newECKey()
)

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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// googleKeysClient answers the validator's fetches of Google's and IAP's public
// keys with googleKey and iapKey, and any other request with a 404.
func googleKeysClient() *http.Client {
	googleKeys, err := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kid": googleKeyID,
		"alg": "RS256",
		"kty": "RSA",
		"use": "sig",
		"n":   b64(googleKey.N.Bytes()),
		"e":   b64(big.NewInt(int64(googleKey.E)).Bytes()),
	}}})
	Expect(err).ToNot(HaveOccurred())

	point, err := iapKey.PublicKey.Bytes() // 0x04 || X || Y
	Expect(err).ToNot(HaveOccurred())
	iapKeys, err := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kid": iapKeyID,
		"alg": "ES256",
		"kty": "EC",
		"crv": "P-256",
		"use": "sig",
		"x":   b64(point[1:33]),
		"y":   b64(point[33:]),
	}}})
	Expect(err).ToNot(HaveOccurred())

	keySets := map[string][]byte{googleKeysURL: googleKeys, iapKeysURL: iapKeys}
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		keys, ok := keySets[r.URL.String()]
		if !ok {
			return &http.Response{StatusCode: http.StatusNotFound, Body: http.NoBody, Request: r}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(keys)), Request: r}, nil
	})}
}

// unsignedToken builds the first two segments of a JWT: its header and claims.
func unsignedToken(header map[string]string, claims map[string]any) string {
	encodedHeader, err := json.Marshal(header)
	Expect(err).ToNot(HaveOccurred())
	payload, err := json.Marshal(claims)
	Expect(err).ToNot(HaveOccurred())

	return b64(encodedHeader) + "." + b64(payload)
}

// signToken builds a JWT carrying claims, signed with key under kid.
func signToken(key *rsa.PrivateKey, kid string, claims map[string]any) string {
	signed := unsignedToken(map[string]string{"alg": "RS256", "kid": kid, "typ": "JWT"}, claims)
	digest := sha256.Sum256([]byte(signed))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	Expect(err).ToNot(HaveOccurred())

	return signed + "." + b64(signature)
}

// schedulerClaims are the claims of a token Cloud Scheduler gets from Google
// for taskServiceAccount.
func schedulerClaims() map[string]any {
	now := time.Now()
	return map[string]any{
		"iss":            "https://accounts.google.com",
		"aud":            taskAudience,
		"azp":            "1234567890",
		"sub":            "1234567890",
		"email":          taskServiceAccount,
		"email_verified": true,
		"iat":            now.Unix(),
		"exp":            now.Add(time.Hour).Unix(),
	}
}

// newTaskRouter wires the middleware in front of a 200-OK handler with the
// project's standard error handler so token failures surface as 401.
func newTaskRouter(cfg config.Tasks, isDevEnv bool) *gin.Engine {
	validator, err := idtoken.NewValidator(context.Background(), option.WithHTTPClient(googleKeysClient()))
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

var _ = Describe("ValidateTaskToken", func() {
	cfg := config.Tasks{TasksAudience: taskAudience, TasksServiceAccount: taskServiceAccount}

	serve := func(cfg config.Tasks, isDevEnv bool, authorization string) int {
		req := httptest.NewRequest(http.MethodPost, "/task", http.NoBody)
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		rec := httptest.NewRecorder()
		newTaskRouter(cfg, isDevEnv).ServeHTTP(rec, req)
		return rec.Code
	}

	schedulerToken := func() string {
		return signToken(googleKey, googleKeyID, schedulerClaims())
	}

	It("accepts a token Google minted for the service account", func() {
		Expect(serve(cfg, false, "Bearer "+schedulerToken())).To(Equal(http.StatusOK))
	})

	It("rejects a missing header or a token without the Bearer scheme", func() {
		Expect(serve(cfg, false, "")).To(Equal(http.StatusUnauthorized))
		Expect(serve(cfg, false, schedulerToken())).To(Equal(http.StatusUnauthorized))
	})

	It("rejects a token signed with another key", func() {
		forged := signToken(newRSAKey(), googleKeyID, schedulerClaims())
		Expect(serve(cfg, false, "Bearer "+forged)).To(Equal(http.StatusUnauthorized))
	})

	It("rejects a token not signed with RS256", func() {
		// Without the RS256 pin, idtoken checks it against IAP's keys and panics on the signature's length.
		header := map[string]string{"alg": "ES256", "kid": iapKeyID, "typ": "JWT"}
		token := unsignedToken(header, schedulerClaims()) + "." + b64([]byte{1})
		Expect(serve(cfg, false, "Bearer "+token)).To(Equal(http.StatusUnauthorized))
	})

	DescribeTable("rejects a Google-signed token with",
		func(claim string, value any) {
			claims := schedulerClaims()
			maps.Copy(claims, map[string]any{claim: value})
			token := signToken(googleKey, googleKeyID, claims)
			Expect(serve(cfg, false, "Bearer "+token)).To(Equal(http.StatusUnauthorized))
		},
		Entry("an expiry in the past", "exp", time.Now().Add(-time.Minute).Unix()),
		Entry("another audience", "aud", "https://other.example.com"),
		Entry("another service account", "email", "attacker@other-project.iam.gserviceaccount.com"),
		Entry("an unverified email", "email_verified", false),
		Entry("another issuer", "iss", "https://cloud.google.com/iap"),
	)

	It("rejects every request when the config is incomplete", func() {
		token := "Bearer " + schedulerToken()
		Expect(serve(config.Tasks{}, false, token)).To(Equal(http.StatusUnauthorized))
		Expect(serve(config.Tasks{TasksAudience: taskAudience}, false, token)).To(Equal(http.StatusUnauthorized))
		Expect(serve(config.Tasks{TasksServiceAccount: taskServiceAccount}, false, token)).To(Equal(http.StatusUnauthorized))
	})

	It("skips verification in dev", func() {
		Expect(serve(config.Tasks{}, true, "")).To(Equal(http.StatusOK))
		Expect(serve(cfg, true, "")).To(Equal(http.StatusOK))
	})
})
