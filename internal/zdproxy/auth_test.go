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
