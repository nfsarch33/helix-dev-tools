package zdproxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthMiddleware_RejectsMissingToken(t *testing.T) {
	mw := AuthMiddleware("expected-token")
	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/messages", strings.NewReader("{}"))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	if called {
		t.Fatal("downstream handler should not have been invoked")
	}
}

func TestAuthMiddleware_RejectsWrongToken(t *testing.T) {
	mw := AuthMiddleware("expected-token")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/messages", strings.NewReader("{}"))
	req.Header.Set("X-Local-Auth", "Bearer wrong-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_AcceptsCorrectToken(t *testing.T) {
	mw := AuthMiddleware("expected-token")
	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/messages", strings.NewReader("{}"))
	req.Header.Set("X-Local-Auth", "Bearer expected-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatal("downstream handler should have been invoked")
	}
}

func TestAuthMiddleware_DoesNotEchoExpectedTokenInError(t *testing.T) {
	mw := AuthMiddleware("super-secret-12345")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/messages", strings.NewReader("{}"))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if strings.Contains(rr.Body.String(), "super-secret-12345") {
		t.Fatalf("error body must not echo the expected token, got %q", rr.Body.String())
	}
}

// TestAuthMiddleware_AcceptsAuthorizationFallback proves OpenAI-shape
// clients (Cursor, OpenAI SDKs) can present the local proxy token in the
// Authorization header instead of X-Local-Auth — necessary because they
// don't expose a custom-header field. The middleware must also strip the
// inbound Authorization header before passing the request downstream so
// the upstream transport's bearer is the only one that ever leaves the
// loopback.
func TestAuthMiddleware_AcceptsAuthorizationFallback(t *testing.T) {
	mw := AuthMiddleware("expected-token")
	var saw http.Header
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		saw = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer expected-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 via Authorization fallback, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	if v := saw.Get("Authorization"); v != "" {
		t.Fatalf("expected Authorization stripped before downstream, got %q", v)
	}
}

func TestAuthMiddleware_AuthorizationFallback_RejectsWrongToken(t *testing.T) {
	mw := AuthMiddleware("expected-token")
	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer some-other-key")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	if called {
		t.Fatal("downstream handler should not have been invoked")
	}
}

// X-Local-Auth wins when both are present, so callers that explicitly
// chose the canonical header are not surprised by an Authorization in
// flight.
func TestAuthMiddleware_PrefersXLocalAuth(t *testing.T) {
	mw := AuthMiddleware("expected-token")
	var saw http.Header
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		saw = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader("{}"))
	req.Header.Set("X-Local-Auth", "Bearer expected-token")
	req.Header.Set("Authorization", "Bearer something-else")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	// We did NOT consume Authorization -- it must remain on the request
	// so a downstream that legitimately needs it (none exist today, but
	// the contract is "X-Local-Auth wins, untouched") sees it as-is.
	if v := saw.Get("Authorization"); v != "Bearer something-else" {
		t.Fatalf("expected Authorization preserved when X-Local-Auth wins, got %q", v)
	}
}

func TestNewLocalToken_HasMinimumEntropy(t *testing.T) {
	tok, err := NewLocalToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tok) < 32 {
		t.Fatalf("expected token length >= 32, got %d (%q)", len(tok), tok)
	}
	tok2, err := NewLocalToken()
	if err != nil {
		t.Fatalf("unexpected error on second token: %v", err)
	}
	if tok == tok2 {
		t.Fatalf("two consecutive tokens must differ; got %q twice", tok)
	}
}
