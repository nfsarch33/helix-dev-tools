package embedhealth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing auth header")
		}
		vec := make([]float64, 1536)
		for i := range vec {
			vec[i] = float64(i) * 0.001
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"embedding": vec, "index": 0},
			},
		})
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, APIKey: "test-key", ExpectDims: 1536, Timeout: 5 * time.Second})
	status := c.Check(context.Background())

	if !status.Healthy {
		t.Fatalf("expected healthy, got error: %s", status.Error)
	}
	if status.Dims != 1536 {
		t.Errorf("expected 1536 dims, got %d", status.Dims)
	}
	if status.Latency == 0 {
		t.Error("latency should be non-zero")
	}
}

func TestCheckDimensionMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vec := make([]float64, 768)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"embedding": vec, "index": 0},
			},
		})
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, ExpectDims: 1536, Timeout: 5 * time.Second})
	status := c.Check(context.Background())

	if status.Healthy {
		t.Fatal("should fail on dimension mismatch")
	}
	if status.Dims != 768 {
		t.Errorf("expected 768, got %d", status.Dims)
	}
}

func TestCheckServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid key"}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Timeout: 5 * time.Second})
	status := c.Check(context.Background())

	if status.Healthy {
		t.Fatal("should not be healthy on 401")
	}
	if status.Error == "" {
		t.Error("error should describe the failure")
	}
}

func TestCheckTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Timeout: 50 * time.Millisecond})
	status := c.Check(context.Background())

	if status.Healthy {
		t.Fatal("should timeout")
	}
}

func TestCheckNoBaseURL(t *testing.T) {
	c := New(Config{})
	status := c.Check(context.Background())

	if status.Healthy {
		t.Fatal("should fail without base URL")
	}
	if status.Error != "no base URL configured" {
		t.Errorf("unexpected error: %s", status.Error)
	}
}

func TestCheckEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, Timeout: 5 * time.Second})
	status := c.Check(context.Background())

	if status.Healthy {
		t.Fatal("should fail on empty response")
	}
}

func TestIsHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vec := make([]float64, 1536)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"embedding": vec, "index": 0},
			},
		})
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, ExpectDims: 1536, Timeout: 5 * time.Second})
	if !c.IsHealthy(context.Background()) {
		t.Fatal("should be healthy")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Timeout != 10*time.Second {
		t.Errorf("default timeout: %v", cfg.Timeout)
	}
	if cfg.ExpectDims != 1536 {
		t.Errorf("default dims: %d", cfg.ExpectDims)
	}
}
