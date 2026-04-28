package zdproxy

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
)

// localAuthHeader is the case-insensitive header callers must set on every
// inbound request. The value is `Bearer <token>` matching the token written to
// the local-token file at startup.
const localAuthHeader = "X-Local-Auth"

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
func AuthMiddleware(expected string) func(http.Handler) http.Handler {
	expectedBytes := []byte(expected)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := strings.TrimSpace(r.Header.Get(localAuthHeader))
			if got == "" {
				http.Error(w, `{"error":{"type":"unauthorized","message":"missing X-Local-Auth header"}}`, http.StatusUnauthorized)
				return
			}
			got = strings.TrimSpace(strings.TrimPrefix(got, "Bearer "))
			if subtle.ConstantTimeCompare([]byte(got), expectedBytes) != 1 {
				http.Error(w, `{"error":{"type":"unauthorized","message":"invalid X-Local-Auth header"}}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
