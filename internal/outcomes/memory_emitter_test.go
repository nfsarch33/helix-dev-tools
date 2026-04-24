package outcomes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type mem0Capture struct {
	mu       sync.Mutex
	requests []map[string]interface{}
	failOnce bool
}

func newMem0TestServer(c *mem0Capture) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/v1/memories") {
			http.Error(w, "unknown path", http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		c.mu.Lock()
		c.requests = append(c.requests, payload)
		failNow := c.failOnce
		c.failOnce = false
		c.mu.Unlock()

		if failNow {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	}))
}

func TestMemoryEmitter_PublishesToMem0(t *testing.T) {
	c := &mem0Capture{}
	srv := newMem0TestServer(c)
	defer srv.Close()

	em := NewMemoryEmitter(MemoryEmitterConfig{
		APIKey:  "test-token",
		UserID:  "fleet-evoloop",
		AppID:   AppIDFleetOutcomes,
		BaseURL: srv.URL,
		Timeout: 2 * time.Second,
	})

	skill := true
	o := Outcome{
		Kind:      KindAgentOutcome,
		Actor:     ActorCursorHook,
		Machine:   "macbook",
		Event:     "guard-shell:allow",
		Detail:    "ls -la",
		LatencyMs: 17,
		SkillHit:  &skill,
		Sprint:    "v253",
	}
	if err := em.Emit(context.Background(), o); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(c.requests))
	}
	got := c.requests[0]

	if got["app_id"] != AppIDFleetOutcomes {
		t.Errorf("app_id = %v, want %s", got["app_id"], AppIDFleetOutcomes)
	}
	if got["user_id"] != "fleet-evoloop" {
		t.Errorf("user_id = %v, want fleet-evoloop", got["user_id"])
	}
	text, _ := got["text"].(string)
	if !strings.Contains(text, "guard-shell:allow") {
		t.Errorf("text missing event: %q", text)
	}
	meta, _ := got["metadata"].(map[string]interface{})
	if meta["kind"] != KindAgentOutcome {
		t.Errorf("metadata.kind = %v, want %s", meta["kind"], KindAgentOutcome)
	}
	if meta["actor"] != ActorCursorHook {
		t.Errorf("metadata.actor = %v, want %s", meta["actor"], ActorCursorHook)
	}
	if meta["sprint"] != "v253" {
		t.Errorf("metadata.sprint = %v, want v253", meta["sprint"])
	}
	if got["infer"] != false {
		t.Errorf("infer should be false (capsule, not summary), got %v", got["infer"])
	}
}

func TestMemoryEmitter_RetriesOnServerError(t *testing.T) {
	c := &mem0Capture{failOnce: true}
	srv := newMem0TestServer(c)
	defer srv.Close()

	em := NewMemoryEmitter(MemoryEmitterConfig{
		APIKey:     "test-token",
		UserID:     "fleet-evoloop",
		BaseURL:    srv.URL,
		Timeout:    2 * time.Second,
		MaxRetries: 2,
		RetryDelay: 1 * time.Millisecond,
	})

	o := newValidOutcome()
	if err := em.Emit(context.Background(), o); err != nil {
		t.Fatalf("Emit (with retry): %v", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.requests) != 2 {
		t.Errorf("expected 2 attempts (1 fail + 1 success), got %d", len(c.requests))
	}
}

func TestMemoryEmitter_ValidatesBeforeSend(t *testing.T) {
	c := &mem0Capture{}
	srv := newMem0TestServer(c)
	defer srv.Close()

	em := NewMemoryEmitter(MemoryEmitterConfig{
		APIKey:  "test-token",
		UserID:  "fleet-evoloop",
		BaseURL: srv.URL,
	})

	if err := em.Emit(context.Background(), Outcome{Kind: KindAgentOutcome}); err == nil {
		t.Errorf("expected validation error")
	}
	if len(c.requests) != 0 {
		t.Errorf("expected 0 HTTP calls, got %d", len(c.requests))
	}
}

func TestMemoryEmitter_DefaultsAppliedFromConfig(t *testing.T) {
	em := NewMemoryEmitter(MemoryEmitterConfig{
		APIKey: "test",
		UserID: "u",
	})
	if em.cfg.AppID != AppIDFleetOutcomes {
		t.Errorf("default AppID = %s, want %s", em.cfg.AppID, AppIDFleetOutcomes)
	}
	if em.cfg.BaseURL == "" {
		t.Errorf("BaseURL should default to mem0.ai")
	}
	if em.cfg.Timeout == 0 {
		t.Errorf("Timeout should default")
	}
}
