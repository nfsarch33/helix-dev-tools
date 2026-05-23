package coordination

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewClient_DefaultBaseURL(t *testing.T) {
	c := NewClient("key", "user", "")
	if c.BaseURL != "https://api.mem0.ai" {
		t.Errorf("BaseURL = %q, want https://api.mem0.ai", c.BaseURL)
	}
}

func TestNewClient_CustomBaseURL(t *testing.T) {
	c := NewClient("key", "user", "http://localhost:8080/")
	if c.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL = %q, want trailing slash trimmed", c.BaseURL)
	}
}

func TestAddSignal(t *testing.T) {
	var received mem0AddPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/memories/") {
			t.Errorf("path = %s, want /v1/memories/", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Token test-key" {
			t.Errorf("Authorization = %q, want 'Token test-key'", auth)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"mem-123"}`))
	}))
	defer srv.Close()

	c := NewClient("test-key", "test-user", srv.URL)
	s := Signal{
		Type:      SignalTaskDispatch,
		Machine:   "wsl",
		TargetFor: "macbook",
		Message:   "Review the PR",
		Priority:  "high",
		Sprint:    "154",
	}

	err := c.AddSignal(context.Background(), s)
	if err != nil {
		t.Fatalf("AddSignal() error = %v", err)
	}

	if received.UserID != "test-user" {
		t.Errorf("user_id = %q, want test-user", received.UserID)
	}
	if received.AppID != AppID {
		t.Errorf("app_id = %q, want %s", received.AppID, AppID)
	}
	if !strings.Contains(received.Text, "Task for macbook") {
		t.Errorf("text = %q, missing 'Task for macbook'", received.Text)
	}
	if received.Metadata["type"] != "task-dispatch" {
		t.Errorf("metadata.type = %q, want task-dispatch", received.Metadata["type"])
	}
	if received.Infer {
		t.Error("infer should be false")
	}
}

func TestAddSignal_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer srv.Close()

	c := NewClient("key", "user", srv.URL)
	err := c.AddSignal(context.Background(), Signal{Type: SignalBlocker, Machine: "wsl", Message: "test"})
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want to contain '500'", err.Error())
	}
}

func TestSearchSignals(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v1/memories/search/") {
			t.Errorf("path = %s, want /v1/memories/search/", r.URL.Path)
		}

		var payload mem0SearchPayload
		json.NewDecoder(r.Body).Decode(&payload)
		if payload.Query != "tasks for wsl" {
			t.Errorf("query = %q, want 'tasks for wsl'", payload.Query)
		}

		results := []mem0SearchResult{
			{
				ID:     "mem-1",
				Memory: "Task for wsl: Review the PR",
				Score:  0.95,
				Metadata: map[string]interface{}{
					"type":       "task-dispatch",
					"machine":    "macbook",
					"target_for": "wsl",
					"priority":   "high",
				},
			},
			{
				ID:     "mem-2",
				Memory: "wsl working on: fuzz targets",
				Score:  0.80,
				Metadata: map[string]interface{}{
					"type":    "active-state",
					"machine": "wsl",
				},
			},
		}
		json.NewEncoder(w).Encode(results)
	}))
	defer srv.Close()

	c := NewClient("key", "user", srv.URL)
	signals, err := c.SearchSignals(context.Background(), "tasks for wsl", 10)
	if err != nil {
		t.Fatalf("SearchSignals() error = %v", err)
	}
	if len(signals) != 2 {
		t.Fatalf("got %d signals, want 2", len(signals))
	}

	s0 := signals[0]
	if s0.Type != SignalTaskDispatch {
		t.Errorf("signals[0].Type = %q, want task-dispatch", s0.Type)
	}
	if s0.TargetFor != "wsl" {
		t.Errorf("signals[0].TargetFor = %q, want wsl", s0.TargetFor)
	}
	if s0.Priority != "high" {
		t.Errorf("signals[0].Priority = %q, want high", s0.Priority)
	}
}

func TestParseListResults_EmptyResultsObject(t *testing.T) {
	signals, total, err := parseListResults([]byte(`{"results":[],"total":0}`))
	if err != nil {
		t.Fatalf("parseListResults: %v", err)
	}
	if len(signals) != 0 || total != 0 {
		t.Fatalf("got len=%d total=%d", len(signals), total)
	}
}

func TestListSignals_EmptyPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := mem0ListResult{Results: []mem0SearchResult{}, Total: 0}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient("key", "user", srv.URL)
	signals, err := c.ListSignals(context.Background())
	if err != nil {
		t.Fatalf("ListSignals() error = %v", err)
	}
	if len(signals) != 0 {
		t.Fatalf("got %d signals, want 0", len(signals))
	}
}

func TestListSignals(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v2/memories/") {
			t.Errorf("path = %s, want /v2/memories/", r.URL.Path)
		}

		resp := mem0ListResult{
			Results: []mem0SearchResult{
				{
					ID:       "mem-1",
					Memory:   "wsl working on: tests",
					Metadata: map[string]interface{}{"type": "active-state", "machine": "wsl"},
				},
			},
			Total: 1,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient("key", "user", srv.URL)
	signals, err := c.ListSignals(context.Background())
	if err != nil {
		t.Fatalf("ListSignals() error = %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("got %d signals, want 1", len(signals))
	}
	if signals[0].Type != SignalActiveState {
		t.Errorf("type = %q, want active-state", signals[0].Type)
	}
}

func TestDeleteSignal(t *testing.T) {
	var deletedID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		parts := strings.Split(r.URL.Path, "/")
		for i, p := range parts {
			if p == "memories" && i+1 < len(parts) {
				deletedID = parts[i+1]
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("key", "user", srv.URL)
	err := c.DeleteSignal(context.Background(), "mem-42")
	if err != nil {
		t.Fatalf("DeleteSignal() error = %v", err)
	}
	if deletedID != "mem-42" {
		t.Errorf("deleted ID = %q, want mem-42", deletedID)
	}
}

func TestCleanStaleSignals(t *testing.T) {
	callCount := 0
	var deletedIDs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			parts := strings.Split(r.URL.Path, "/")
			for i, p := range parts {
				if p == "memories" && i+1 < len(parts) {
					deletedIDs = append(deletedIDs, parts[i+1])
				}
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		callCount++
		resp := mem0ListResult{
			Results: []mem0SearchResult{
				{
					ID:       "stale-1",
					Memory:   "old state",
					Metadata: map[string]interface{}{"type": "active-state", "machine": "wsl"},
				},
				{
					ID:       "completed-1",
					Memory:   "done item",
					Metadata: map[string]interface{}{"type": "completed", "machine": "wsl"},
				},
				{
					ID:       "fresh-1",
					Memory:   "current task",
					Metadata: map[string]interface{}{"type": "task-dispatch", "machine": "wsl", "target_for": "macbook"},
				},
			},
			Total: 3,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient("key", "user", srv.URL)
	// CreatedAt is zero for all → only "completed" type gets cleaned
	deleted, err := c.CleanStaleSignals(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatalf("CleanStaleSignals() error = %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1 (only the completed signal)", deleted)
	}
	if len(deletedIDs) != 1 || deletedIDs[0] != "completed-1" {
		t.Errorf("deletedIDs = %v, want [completed-1]", deletedIDs)
	}
}

func TestFilterForMachine(t *testing.T) {
	signals := []Signal{
		{Type: SignalActiveState, Machine: "wsl", Message: "state"},
		{Type: SignalTaskDispatch, Machine: "macbook", TargetFor: "wsl", Message: "for wsl"},
		{Type: SignalTaskDispatch, Machine: "wsl", TargetFor: "macbook", Message: "for macbook"},
	}

	wslSignals := FilterForMachine(signals, "wsl")
	if len(wslSignals) != 2 {
		t.Fatalf("FilterForMachine(wsl) = %d, want 2 (untargeted + targeted to wsl)", len(wslSignals))
	}

	macSignals := FilterForMachine(signals, "macbook")
	if len(macSignals) != 2 {
		t.Fatalf("FilterForMachine(macbook) = %d, want 2 (untargeted + targeted to macbook)", len(macSignals))
	}
}

func TestFilterPendingTasks(t *testing.T) {
	signals := []Signal{
		{Type: SignalActiveState, Machine: "wsl", Message: "not a task"},
		{Type: SignalTaskDispatch, Machine: "macbook", TargetFor: "wsl", Message: "task for wsl"},
		{Type: SignalTaskDispatch, Machine: "wsl", TargetFor: "macbook", Message: "task for macbook"},
		{Type: SignalBlocker, Machine: "wsl", Message: "blocker"},
	}

	wslTasks := FilterPendingTasks(signals, "wsl")
	if len(wslTasks) != 1 {
		t.Fatalf("FilterPendingTasks(wsl) = %d, want 1", len(wslTasks))
	}
	if wslTasks[0].Message != "task for wsl" {
		t.Errorf("task message = %q", wslTasks[0].Message)
	}
}

func TestResolveCredentials_FromEnv(t *testing.T) {
	t.Setenv("MEM0_API_KEY", "env-key")
	t.Setenv("MEM0_DEFAULT_USER_ID", "env-user")

	key, user, err := ResolveCredentials("/nonexistent/path")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if key != "env-key" || user != "env-user" {
		t.Errorf("got key=%q user=%q, want env-key/env-user", key, user)
	}
}

func TestResolveCredentials_FromMCPConfig(t *testing.T) {
	t.Setenv("MEM0_API_KEY", "")
	t.Setenv("MEM0_DEFAULT_USER_ID", "")

	mcpJSON := `{
		"mcpServers": {
			"mem0": {
				"env": {
					"MEM0_API_KEY": "config-key",
					"MEM0_DEFAULT_USER_ID": "config-user"
				}
			}
		}
	}`
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	os.WriteFile(path, []byte(mcpJSON), 0o644)

	key, user, err := ResolveCredentials(path)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if key != "config-key" || user != "config-user" {
		t.Errorf("got key=%q user=%q, want config-key/config-user", key, user)
	}
}

func TestResolveCredentials_EnvRef(t *testing.T) {
	t.Setenv("MEM0_API_KEY", "")
	t.Setenv("MEM0_DEFAULT_USER_ID", "")
	t.Setenv("MY_MEM0_KEY", "resolved-key")
	t.Setenv("MY_MEM0_USER", "resolved-user")

	mcpJSON := `{
		"mcpServers": {
			"mem0": {
				"env": {
					"MEM0_API_KEY": "$MY_MEM0_KEY",
					"MEM0_DEFAULT_USER_ID": "$MY_MEM0_USER"
				}
			}
		}
	}`
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")
	os.WriteFile(path, []byte(mcpJSON), 0o644)

	key, user, err := ResolveCredentials(path)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if key != "resolved-key" || user != "resolved-user" {
		t.Errorf("got key=%q user=%q, want resolved-key/resolved-user", key, user)
	}
}

func TestResolveCredentials_MissingConfig(t *testing.T) {
	t.Setenv("MEM0_API_KEY", "")
	t.Setenv("MEM0_DEFAULT_USER_ID", "")

	_, _, err := ResolveCredentials("/nonexistent")
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestSignalFromMem0_RoundTrip(t *testing.T) {
	original := Signal{
		Type:      SignalTaskDispatch,
		Machine:   "wsl",
		TargetFor: "macbook",
		Message:   "Review the PR",
		Priority:  "high",
		Sprint:    "154",
	}

	meta := original.Mem0Metadata()
	metaIF := make(map[string]interface{}, len(meta))
	for k, v := range meta {
		metaIF[k] = v
	}
	result := mem0SearchResult{
		ID:       "mem-1",
		Memory:   original.Mem0Text(),
		Metadata: metaIF,
	}

	restored := signalFromMem0(result)
	if restored.Type != original.Type {
		t.Errorf("Type = %q, want %q", restored.Type, original.Type)
	}
	if restored.Machine != original.Machine {
		t.Errorf("Machine = %q, want %q", restored.Machine, original.Machine)
	}
	if restored.TargetFor != original.TargetFor {
		t.Errorf("TargetFor = %q, want %q", restored.TargetFor, original.TargetFor)
	}
	if restored.Priority != original.Priority {
		t.Errorf("Priority = %q, want %q", restored.Priority, original.Priority)
	}
	if restored.Sprint != original.Sprint {
		t.Errorf("Sprint = %q, want %q", restored.Sprint, original.Sprint)
	}
}

// --- slog + metrics tests ---

func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestAddSignal_SlogOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"ok"}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	c := NewClient("key", "user", srv.URL).WithLogger(newTestLogger(&buf))
	err := c.AddSignal(context.Background(), Signal{Type: SignalBlocker, Machine: "macos", Message: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "coordination signal added") {
		t.Errorf("slog output missing expected message, got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"type":"blocker"`) {
		t.Errorf("slog output missing signal type, got: %s", buf.String())
	}
}

func TestAddSignal_ErrorIncrementsCounter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`error`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	c := NewClient("key", "user", srv.URL).WithLogger(newTestLogger(&buf))
	_ = c.AddSignal(context.Background(), Signal{Type: SignalBlocker, Machine: "wsl", Message: "fail"})
	if c.Stats.Errors.Load() != 1 {
		t.Errorf("Errors = %d, want 1", c.Stats.Errors.Load())
	}
	if c.Stats.SignalsAdded.Load() != 0 {
		t.Errorf("SignalsAdded = %d, want 0 on failure", c.Stats.SignalsAdded.Load())
	}
	if !strings.Contains(buf.String(), "coordination signal add failed") {
		t.Errorf("slog missing warn message, got: %s", buf.String())
	}
}

func TestListSignals_CounterAndSlog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := mem0ListResult{
			Results: []mem0SearchResult{
				{ID: "1", Memory: "test", Metadata: map[string]interface{}{"type": "active-state", "machine": "wsl"}},
			},
			Total: 1,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	c := NewClient("key", "user", srv.URL).WithLogger(newTestLogger(&buf))
	signals, err := c.ListSignals(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("got %d signals, want 1", len(signals))
	}
	if c.Stats.SignalsListed.Load() != 1 {
		t.Errorf("SignalsListed = %d, want 1", c.Stats.SignalsListed.Load())
	}
	if !strings.Contains(buf.String(), "coordination signals listed") {
		t.Errorf("slog missing list message, got: %s", buf.String())
	}
}

func TestSearchSignals_CounterAndSlog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]mem0SearchResult{
			{ID: "1", Memory: "hit", Metadata: map[string]interface{}{"type": "blocker", "machine": "wsl"}},
		})
	}))
	defer srv.Close()

	var buf bytes.Buffer
	c := NewClient("key", "user", srv.URL).WithLogger(newTestLogger(&buf))
	signals, err := c.SearchSignals(context.Background(), "test", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(signals) != 1 {
		t.Fatalf("got %d, want 1", len(signals))
	}
	if c.Stats.SignalsSearched.Load() != 1 {
		t.Errorf("SignalsSearched = %d, want 1", c.Stats.SignalsSearched.Load())
	}
}

func TestDeleteSignal_CounterAndSlog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	c := NewClient("key", "user", srv.URL).WithLogger(newTestLogger(&buf))
	if err := c.DeleteSignal(context.Background(), "mem-99"); err != nil {
		t.Fatal(err)
	}
	if c.Stats.SignalsDeleted.Load() != 1 {
		t.Errorf("SignalsDeleted = %d, want 1", c.Stats.SignalsDeleted.Load())
	}
	if !strings.Contains(buf.String(), "coordination signal deleted") {
		t.Errorf("slog missing delete message, got: %s", buf.String())
	}
}

func TestCleanStaleSignals_CleanupCounters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusOK)
			return
		}
		resp := mem0ListResult{
			Results: []mem0SearchResult{
				{ID: "c1", Memory: "done", Metadata: map[string]interface{}{"type": "completed", "machine": "wsl"}},
				{ID: "a1", Memory: "active", Metadata: map[string]interface{}{"type": "active-state", "machine": "wsl"}},
			},
			Total: 2,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	c := NewClient("key", "user", srv.URL).WithLogger(newTestLogger(&buf))
	deleted, err := c.CleanStaleSignals(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}
	if c.Stats.CleanupRuns.Load() != 1 {
		t.Errorf("CleanupRuns = %d, want 1", c.Stats.CleanupRuns.Load())
	}
	if c.Stats.CleanupDeleted.Load() != 1 {
		t.Errorf("CleanupDeleted = %d, want 1", c.Stats.CleanupDeleted.Load())
	}
	if !strings.Contains(buf.String(), "coordination cleanup complete") {
		t.Errorf("slog missing cleanup message, got: %s", buf.String())
	}
}

func TestClientMetrics_Snapshot(t *testing.T) {
	m := &ClientMetrics{}
	m.SignalsAdded.Add(3)
	m.Errors.Add(1)
	snap := m.Snapshot()
	if snap["helixon_coordination_signals_added_total"] != 3 {
		t.Errorf("added = %d, want 3", snap["helixon_coordination_signals_added_total"])
	}
	if snap["helixon_coordination_errors_total"] != 1 {
		t.Errorf("errors = %d, want 1", snap["helixon_coordination_errors_total"])
	}
	if snap["helixon_coordination_signals_listed_total"] != 0 {
		t.Errorf("listed = %d, want 0", snap["helixon_coordination_signals_listed_total"])
	}
}

func TestWithLogger_Replaces(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf)
	c := NewClient("key", "user", "http://localhost").WithLogger(l)
	if c.Log != l {
		t.Error("WithLogger did not replace logger")
	}
}

// --- retry tests ---

func TestAddSignal_RetryOn500(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"Internal error processing memories"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"ok"}`))
	}))
	defer srv.Close()

	c := NewClient("key", "user", srv.URL)
	c.MaxRetries = 3
	c.RetryBaseDelay = time.Millisecond
	err := c.AddSignal(context.Background(), Signal{Type: SignalBlocker, Machine: "wsl", Message: "retry test"})
	if err != nil {
		t.Fatalf("AddSignal() should succeed after retries, got: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3 (2 failures + 1 success)", attempts)
	}
	if c.Stats.SignalsAdded.Load() != 1 {
		t.Errorf("SignalsAdded = %d, want 1", c.Stats.SignalsAdded.Load())
	}
}

func TestAddSignal_NoRetryOn400(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer srv.Close()

	c := NewClient("key", "user", srv.URL)
	c.MaxRetries = 3
	c.RetryBaseDelay = time.Millisecond
	err := c.AddSignal(context.Background(), Signal{Type: SignalBlocker, Machine: "wsl", Message: "bad"})
	if err == nil {
		t.Fatal("expected error on 400")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (no retry on 4xx)", attempts)
	}
}

func TestAddSignal_RetryExhausted(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"always failing"}`))
	}))
	defer srv.Close()

	c := NewClient("key", "user", srv.URL)
	c.MaxRetries = 2
	c.RetryBaseDelay = time.Millisecond
	err := c.AddSignal(context.Background(), Signal{Type: SignalBlocker, Machine: "wsl", Message: "exhaust"})
	if err == nil {
		t.Fatal("expected error when retries exhausted")
	}
	// 1 initial + 2 retries = 3 total attempts
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3 (1 initial + 2 retries)", attempts)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should contain status code, got: %v", err)
	}
}

func TestListSignals_RetryOn503(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"service unavailable"}`))
			return
		}
		resp := mem0ListResult{
			Results: []mem0SearchResult{
				{ID: "1", Memory: "test", Metadata: map[string]interface{}{"type": "active-state", "machine": "wsl"}},
			},
			Total: 1,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient("key", "user", srv.URL)
	c.MaxRetries = 3
	c.RetryBaseDelay = time.Millisecond
	signals, err := c.ListSignals(context.Background())
	if err != nil {
		t.Fatalf("ListSignals() should succeed after retry, got: %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("got %d signals, want 1", len(signals))
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2 (1 failure + 1 success)", attempts)
	}
}

func TestRetry_RespectsContextCancellation(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"fail"}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	c := NewClient("key", "user", srv.URL)
	c.MaxRetries = 10
	c.RetryBaseDelay = 50 * time.Millisecond

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := c.AddSignal(ctx, Signal{Type: SignalBlocker, Machine: "wsl", Message: "cancel"})
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
	if attempts > 5 {
		t.Errorf("attempts = %d, expected early cancellation (well below MaxRetries=10)", attempts)
	}
}

func TestRetry_CounterIncrement(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`error`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"ok"}`))
	}))
	defer srv.Close()

	c := NewClient("key", "user", srv.URL)
	c.MaxRetries = 3
	c.RetryBaseDelay = time.Millisecond
	_ = c.AddSignal(context.Background(), Signal{Type: SignalBlocker, Machine: "wsl", Message: "counter"})
	if c.Stats.Retries.Load() != 2 {
		t.Errorf("Retries = %d, want 2", c.Stats.Retries.Load())
	}
	snap := c.Stats.Snapshot()
	if snap["helixon_coordination_retries_total"] != 2 {
		t.Errorf("snapshot retries = %d, want 2", snap["helixon_coordination_retries_total"])
	}
}

func TestRetry_SlogOutput(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`error`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"ok"}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	c := NewClient("key", "user", srv.URL).WithLogger(newTestLogger(&buf))
	c.MaxRetries = 3
	c.RetryBaseDelay = time.Millisecond
	_ = c.AddSignal(context.Background(), Signal{Type: SignalBlocker, Machine: "wsl", Message: "slog"})
	logOut := buf.String()
	if !strings.Contains(logOut, "retrying") {
		t.Errorf("slog missing retry message, got: %s", logOut)
	}
	if !strings.Contains(logOut, "500") {
		t.Errorf("slog missing status code, got: %s", logOut)
	}
}
