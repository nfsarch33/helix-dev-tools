package mem0search

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BaseURL != "http://127.0.0.1:18888" {
		t.Errorf("unexpected base URL: %s", cfg.BaseURL)
	}
	if cfg.AppID != "cursor-global-kb" {
		t.Errorf("unexpected app ID: %s", cfg.AppID)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("unexpected timeout: %v", cfg.Timeout)
	}
}

func TestCheckHealth_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()

	cfg := Config{BaseURL: srv.URL, Timeout: 5 * time.Second}
	status := CheckHealth(cfg)

	if !status.Healthy {
		t.Error("expected healthy")
	}
	if status.Latency <= 0 {
		t.Error("expected positive latency")
	}
}

func TestCheckHealth_Unreachable(t *testing.T) {
	cfg := Config{BaseURL: "http://127.0.0.1:1", Timeout: 1 * time.Second}
	status := CheckHealth(cfg)

	if status.Healthy {
		t.Error("expected unhealthy for unreachable host")
	}
	if status.Error == "" {
		t.Error("expected error message")
	}
}

func TestSearch_Success(t *testing.T) {
	results := []SearchResult{
		{ID: "mem-1", Memory: "test memory", Score: 0.95},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(results)
		}
	}))
	defer srv.Close()

	cfg := Config{BaseURL: srv.URL, APIKey: "test", AppID: "test", UserID: "test", Timeout: 5 * time.Second}
	got, err := Search(cfg, "test query", 3)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].ID != "mem-1" {
		t.Errorf("expected mem-1, got %s", got[0].ID)
	}
}

func TestSearch_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := Config{BaseURL: srv.URL, APIKey: "test", AppID: "test", UserID: "test", Timeout: 5 * time.Second}
	_, err := Search(cfg, "test", 3)

	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestDiagnoseSearchPipeline_HealthyButSearchBroken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusOK)
		case "/search":
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(`{"detail":"Upstream provider error."}`))
		}
	}))
	defer srv.Close()

	cfg := Config{BaseURL: srv.URL, APIKey: "test", AppID: "test", UserID: "test", Timeout: 5 * time.Second}
	diag := DiagnoseSearchPipeline(cfg)

	if diag["tunnel"] == "" {
		t.Error("expected tunnel status")
	}
}

func TestSearch_DefaultLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]SearchResult{})
	}))
	defer srv.Close()

	cfg := Config{BaseURL: srv.URL, APIKey: "test", AppID: "test", UserID: "test", Timeout: 5 * time.Second}
	_, err := Search(cfg, "test", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
