package middlewares

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMiddlewares(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Middlewares suite")
}

func sign(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v0:" + timestamp + ":"))
	mac.Write(body)
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

var _ = Describe("verifySlackSignature", func() {
	const secret = "test-secret"
	body := []byte("token=x&user_id=Uabc&text=<@Uxyz|alice>")
	now := time.Unix(1_700_000_000, 0)
	ts := strconv.FormatInt(now.Unix(), 10)

	It("accepts a valid signature within the freshness window", func() {
		sig := sign(secret, ts, body)
		Expect(verifySlackSignature(secret, ts, sig, body, now)).To(Succeed())
	})

	It("rejects a tampered signature", func() {
		Expect(verifySlackSignature(secret, ts, "v0=deadbeef", body, now)).To(MatchError(ContainSubstring("signature mismatch")))
	})

	It("rejects a stale timestamp", func() {
		oldTs := strconv.FormatInt(now.Unix()-int64((10 * time.Minute).Seconds()), 10)
		sig := sign(secret, oldTs, body)
		Expect(verifySlackSignature(secret, oldTs, sig, body, now)).To(MatchError(ContainSubstring("stale")))
	})

	It("rejects a timestamp far in the future", func() {
		futureTs := strconv.FormatInt(now.Unix()+int64((10 * time.Minute).Seconds()), 10)
		sig := sign(secret, futureTs, body)
		Expect(verifySlackSignature(secret, futureTs, sig, body, now)).To(MatchError(ContainSubstring("stale")))
	})

	It("rejects missing headers", func() {
		Expect(verifySlackSignature(secret, "", "v0=x", body, now)).To(MatchError(ContainSubstring("missing signature headers")))
		Expect(verifySlackSignature(secret, ts, "", body, now)).To(MatchError(ContainSubstring("missing signature headers")))
	})

	It("rejects when secret is unset", func() {
		Expect(verifySlackSignature("", ts, "v0=x", body, now)).To(MatchError(ContainSubstring("missing signing secret")))
	})
})
