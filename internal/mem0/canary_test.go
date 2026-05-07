package mem0

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCanary_OSSHitRateAtLeast95Pct(t *testing.T) {
	var served atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served.Add(1)
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/v1/memories/" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"id": "m-1", "memory": "test memory", "score": 0.95},
			},
		})
	}))
	defer ts.Close()

	c := &Canary{
		OSSEndpoint: ts.URL,
		APIKey:      "test-key",
		Queries:     []string{"cursor rules", "evoloop status", "mem0 health", "workspace doctor"},
		Timeout:     5 * time.Second,
	}

	result, err := c.Run(context.Background())
	if err != nil {
		t.Fatalf("Canary.Run: %v", err)
	}

	if result.Total != 4 {
		t.Errorf("total: got %d, want 4", result.Total)
	}
	if result.Hits != 4 {
		t.Errorf("hits: got %d, want 4", result.Hits)
	}
	if result.HitRate() < 0.95 {
		t.Errorf("hit rate: got %.2f, want >= 0.95", result.HitRate())
	}
	if served.Load() != 4 {
		t.Errorf("server requests: got %d, want 4", served.Load())
	}
}

func TestCanary_PartialFailureStillReportsRate(t *testing.T) {
	var reqCount atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := reqCount.Add(1)
		if n%3 == 0 {
			http.Error(w, "transient", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"id": "m-1", "memory": "ok", "score": 0.9},
			},
		})
	}))
	defer ts.Close()

	c := &Canary{
		OSSEndpoint: ts.URL,
		APIKey:      "test-key",
		Queries:     []string{"q1", "q2", "q3", "q4", "q5", "q6"},
		Timeout:     5 * time.Second,
	}

	result, err := c.Run(context.Background())
	if err != nil {
		t.Fatalf("Canary.Run: %v", err)
	}

	if result.Total != 6 {
		t.Errorf("total: got %d, want 6", result.Total)
	}
	if result.Hits != 4 {
		t.Errorf("hits: got %d, want 4 (2 failures at req 3,6)", result.Hits)
	}
	wantRate := 4.0 / 6.0
	if got := result.HitRate(); got < wantRate-0.01 || got > wantRate+0.01 {
		t.Errorf("hit rate: got %.4f, want ~%.4f", got, wantRate)
	}
}

func TestCanary_EmptyQueriesReturnsError(t *testing.T) {
	c := &Canary{
		OSSEndpoint: "http://localhost:0",
		APIKey:      "k",
		Timeout:     time.Second,
	}
	_, err := c.Run(context.Background())
	if err == nil {
		t.Fatal("expected error for empty queries, got nil")
	}
}

func TestCanary_MissingAPIKeyReturnsUnauth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	}))
	defer ts.Close()

	c := &Canary{
		OSSEndpoint: ts.URL,
		Queries:     []string{"test"},
		Timeout:     5 * time.Second,
	}

	result, err := c.Run(context.Background())
	if err != nil {
		t.Fatalf("Canary.Run: %v", err)
	}
	if result.Hits != 0 {
		t.Errorf("hits: got %d, want 0 (all unauth)", result.Hits)
	}
}
