package console

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServer_Healthz(t *testing.T) {
	s := NewServer(nil)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Errorf("expected 'ok', got %q", w.Body.String())
	}
}

func TestServer_Dashboard_Empty(t *testing.T) {
	s := NewServer(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var data DashboardData
	if err := json.NewDecoder(w.Body).Decode(&data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if data.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestServer_Components_WithProbes(t *testing.T) {
	probes := []HealthProbe{
		func() HealthStatus { return HealthStatus{Name: "mem0", Healthy: true} },
		func() HealthStatus { return HealthStatus{Name: "vllm", Healthy: false, Detail: "offline"} },
	}
	s := NewServer(probes)
	req := httptest.NewRequest(http.MethodGet, "/api/components", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)

	var components []HealthStatus
	if err := json.NewDecoder(w.Body).Decode(&components); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(components) != 2 {
		t.Errorf("expected 2 components, got %d", len(components))
	}
	if components[0].Name != "mem0" || !components[0].Healthy {
		t.Errorf("unexpected first component: %+v", components[0])
	}
	if components[1].Name != "vllm" || components[1].Healthy {
		t.Errorf("unexpected second component: %+v", components[1])
	}
}

func TestServer_UnknownRoute_404(t *testing.T) {
	s := NewServer(nil)
	req := httptest.NewRequest(http.MethodGet, "/not-found", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
