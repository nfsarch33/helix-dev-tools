package autoresearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewEngramClientDefaults(t *testing.T) {
	t.Setenv("ENGRAM_URL", "")
	t.Setenv("ENGRAM_API_KEY", "")
	t.Setenv("ENGRAM_USER_ID", "")

	c := NewEngramClient()
	if c.BaseURL != "http://localhost:8281" {
		t.Errorf("BaseURL: got %q, want %q", c.BaseURL, "http://localhost:8281")
	}
	if c.UserID != "autoresearch-agent" {
		t.Errorf("UserID: got %q, want %q", c.UserID, "autoresearch-agent")
	}
}

func TestNewEngramClientFromEnv(t *testing.T) {
	t.Setenv("ENGRAM_URL", "http://custom:9999")
	t.Setenv("ENGRAM_API_KEY", "test-key")
	t.Setenv("ENGRAM_USER_ID", "custom-user")

	c := NewEngramClient()
	if c.BaseURL != "http://custom:9999" {
		t.Errorf("BaseURL: got %q", c.BaseURL)
	}
	if c.APIKey != "test-key" {
		t.Errorf("APIKey: got %q", c.APIKey)
	}
	if c.UserID != "custom-user" {
		t.Errorf("UserID: got %q", c.UserID)
	}
}

func TestStoreResearch(t *testing.T) {
	var received engramAddPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/memories/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"mem-123"}`))
	}))
	defer srv.Close()

	c := &EngramClient{
		BaseURL: srv.URL,
		UserID:  "test-user",
		APIKey:  "test-key",
		HTTP:    srv.Client(),
	}

	meta := map[string]string{"kind": "test"}
	err := c.StoreResearch(context.Background(), "test finding", meta)
	if err != nil {
		t.Fatalf("StoreResearch: %v", err)
	}
	if received.AppID != engramAppID {
		t.Errorf("app_id: got %q, want %q", received.AppID, engramAppID)
	}
	if received.UserID != "test-user" {
		t.Errorf("user_id: got %q", received.UserID)
	}
	if received.Text != "test finding" {
		t.Errorf("text: got %q", received.Text)
	}
}

func TestSearchResearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/memories/search/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		results := []EngramMemory{
			{ID: "m1", Memory: "finding 1", Score: 0.9},
			{ID: "m2", Memory: "finding 2", Score: 0.7},
		}
		json.NewEncoder(w).Encode(results)
	}))
	defer srv.Close()

	c := &EngramClient{
		BaseURL: srv.URL,
		UserID:  "test-user",
		HTTP:    srv.Client(),
	}

	results, err := c.SearchResearch(context.Background(), "error patterns", 5)
	if err != nil {
		t.Fatalf("SearchResearch: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Memory != "finding 1" {
		t.Errorf("first result memory: got %q", results[0].Memory)
	}
}

func TestSearchResearchDefaultLimit(t *testing.T) {
	var receivedLimit int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload engramSearchPayload
		json.NewDecoder(r.Body).Decode(&payload)
		receivedLimit = payload.Limit
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := &EngramClient{BaseURL: srv.URL, UserID: "u", HTTP: srv.Client()}
	c.SearchResearch(context.Background(), "test", 0)

	if receivedLimit != 10 {
		t.Errorf("default limit: got %d, want 10", receivedLimit)
	}
}

func TestStoreResearchAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := &EngramClient{BaseURL: srv.URL, UserID: "u", APIKey: "secret", HTTP: srv.Client()}
	c.StoreResearch(context.Background(), "test", nil)

	if gotAuth != "Token secret" {
		t.Errorf("auth header: got %q, want 'Token secret'", gotAuth)
	}
}

func TestStoreResearchNoAuthWhenEmpty(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := &EngramClient{BaseURL: srv.URL, UserID: "u", APIKey: "", HTTP: srv.Client()}
	c.StoreResearch(context.Background(), "test", nil)

	if gotAuth != "" {
		t.Errorf("auth header should be empty when no API key, got %q", gotAuth)
	}
}

func TestStoreResearchServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer srv.Close()

	c := &EngramClient{BaseURL: srv.URL, UserID: "u", HTTP: srv.Client()}
	err := c.StoreResearch(context.Background(), "test", nil)
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("TEST_ENV_VAR", "custom")
	if v := envOrDefault("TEST_ENV_VAR", "default"); v != "custom" {
		t.Errorf("got %q, want 'custom'", v)
	}
	t.Setenv("TEST_ENV_VAR", "")
	if v := envOrDefault("TEST_ENV_VAR", "default"); v != "default" {
		t.Errorf("got %q, want 'default'", v)
	}
}
