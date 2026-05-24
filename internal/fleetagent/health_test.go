package fleetagent

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthServer_Healthz(t *testing.T) {
	agent := New(Config{AgentID: "test-agent", PollInterval: time.Second}, nil, nil, nil, slog.Default())
	hs := NewHealthServer(agent)
	hs.IncrementPollCount()
	hs.IncrementPollCount()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	hs.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp healthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.AgentID != "test-agent" {
		t.Errorf("expected agent_id test-agent, got %s", resp.AgentID)
	}
	if resp.PollCount != 2 {
		t.Errorf("expected poll_count 2, got %d", resp.PollCount)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status ok, got %s", resp.Status)
	}
}
