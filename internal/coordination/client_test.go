package coordination

import (
	"context"
	"encoding/json"
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
				Metadata: map[string]string{
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
				Metadata: map[string]string{
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
					Metadata: map[string]string{"type": "active-state", "machine": "wsl"},
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
					Metadata: map[string]string{"type": "active-state", "machine": "wsl"},
				},
				{
					ID:       "completed-1",
					Memory:   "done item",
					Metadata: map[string]string{"type": "completed", "machine": "wsl"},
				},
				{
					ID:       "fresh-1",
					Memory:   "current task",
					Metadata: map[string]string{"type": "task-dispatch", "machine": "wsl", "target_for": "macbook"},
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

	result := mem0SearchResult{
		ID:       "mem-1",
		Memory:   original.Mem0Text(),
		Metadata: original.Mem0Metadata(),
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
