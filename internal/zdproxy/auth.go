package zdproxy

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
)

// localAuthHeader is the canonical header callers should set on every
// inbound request. The value is `Bearer <token>` matching the token written
// to the local-token file at startup.
const localAuthHeader = "X-Local-Auth"

// fallbackAuthHeader is accepted as a fallback so OpenAI-shape clients
// (Cursor's "API key" field, the OpenAI Go/Python SDK, etc.) can present
// the local proxy token unchanged. Because the listener is loopback-only
// and the bearer is a 32-byte process-scoped random token, accepting
// Authorization: Bearer <local-token> is no weaker than X-Local-Auth.
const fallbackAuthHeader = "Authorization"

// NewLocalToken returns a fresh URL-safe random token derived from
// cryptographically secure randomness. The token is 32 bytes of entropy
// rendered into a 43-char URL-safe base64 string (no padding).
func NewLocalToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// AuthMiddleware returns an http middleware that validates the local-auth
// header against the expected token using a constant-time comparison. The
// expected token MUST NOT appear in any error response.
//
// Two header names are accepted (in priority order):
//   - X-Local-Auth: Bearer <local-token>   (canonical)
//   - Authorization: Bearer <local-token>  (fallback for OpenAI-shape
//     clients such as Cursor; the listener is loopback-only so this is
//     no weaker than X-Local-Auth)
//
// When Authorization is consumed for local-auth, the middleware strips
// it from the request before passing it on so the upstream request the
// transport sends is not contaminated by the inbound bearer.
func AuthMiddleware(expected string) func(http.Handler) http.Handler {
	expectedBytes := []byte(expected)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			source, raw := readBearer(r)
			if raw == "" {
				http.Error(w, `{"error":{"type":"unauthorized","message":"missing X-Local-Auth or Authorization bearer"}}`, http.StatusUnauthorized)
				return
			}
			if subtle.ConstantTimeCompare([]byte(raw), expectedBytes) != 1 {
				http.Error(w, `{"error":{"type":"unauthorized","message":"invalid local auth bearer"}}`, http.StatusUnauthorized)
				return
			}
			if source == fallbackAuthHeader {
				// Strip the inbound Authorization so the transport
				// can inject the upstream gateway bearer cleanly.
				r.Header.Del(fallbackAuthHeader)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// readBearer extracts and trims a Bearer token from X-Local-Auth (preferred)
// or Authorization (fallback). Returns the source header name (so the
// caller can strip Authorization on a successful match) and the bare token.
func readBearer(r *http.Request) (source string, token string) {
	if v := strings.TrimSpace(r.Header.Get(localAuthHeader)); v != "" {
		return localAuthHeader, strings.TrimSpace(strings.TrimPrefix(v, "Bearer "))
	}
	if v := strings.TrimSpace(r.Header.Get(fallbackAuthHeader)); v != "" {
		return fallbackAuthHeader, strings.TrimSpace(strings.TrimPrefix(v, "Bearer "))
	}
	return "", ""
}
