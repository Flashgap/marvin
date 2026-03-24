// Package testenv provides helpers for HTTP integration testing.
package testenv

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/onsi/gomega"
)

// HTTPEnv bootstraps and provides useful tools for testing HTTP endpoints.
type HTTPEnv struct {
	Handler http.Handler
}

// NewHTTPEnv returns an HTTPEnv with the provided handler.
func NewHTTPEnv(handler http.Handler) *HTTPEnv {
	return &HTTPEnv{Handler: handler}
}

// ServeHTTPRequest serves a simple HTTP request and asserts the expected status code.
func (e *HTTPEnv) ServeHTTPRequest(
	method string,
	path string,
	headers map[string]any,
	payload any,
	expectedStatus int,
) *httptest.ResponseRecorder {
	var body bytes.Buffer
	if payload != nil {
		switch v := payload.(type) {
		case string:
			body.WriteString(v)
		case []byte:
			body.Write(v)
		default:
			_ = json.NewEncoder(&body).Encode(payload)
		}
	}

	req, err := http.NewRequest(method, path, &body)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	for k, v := range headers {
		req.Header.Set(k, v.(string))
	}

	rec := httptest.NewRecorder()
	e.Handler.ServeHTTP(rec, req)
	gomega.Expect(rec.Code).To(gomega.Equal(expectedStatus),
		"unexpected status for %s %s: body=%s", method, path, rec.Body.String())

	return rec
}
