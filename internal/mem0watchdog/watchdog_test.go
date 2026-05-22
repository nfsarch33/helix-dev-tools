// runx-public-repo-gate: allow-file network_topology
// Mem0 OSS canonical local port is 18888 — watchdog default; not a private host.
package mem0watchdog

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

func TestProbe_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	w := New(Config{
		ProbeURL:     srv.URL,
		ProbeTimeout: 5 * time.Second,
	})
	if err := w.Probe(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestProbe_Timeout(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	w := New(Config{
		ProbeURL:     srv.URL,
		ProbeTimeout: 50 * time.Millisecond,
	})
	err := w.Probe(context.Background())
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "deadline exceeded") && !strings.Contains(err.Error(), "Client.Timeout") {
		t.Fatalf("expected timeout-related error, got %v", err)
	}
}

func TestProbe_ServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	w := New(Config{
		ProbeURL:     srv.URL,
		ProbeTimeout: 5 * time.Second,
	})
	err := w.Probe(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 status, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected error mentioning 500, got %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	if cfg.ProbeURL != "http://127.0.0.1:18888/docs" {
		t.Errorf("ProbeURL = %q, want http://127.0.0.1:18888/docs", cfg.ProbeURL)
	}
	if cfg.ProbeInterval != 60*time.Second {
		t.Errorf("ProbeInterval = %v, want 60s", cfg.ProbeInterval)
	}
	if cfg.ProbeTimeout != 10*time.Second {
		t.Errorf("ProbeTimeout = %v, want 10s", cfg.ProbeTimeout)
	}
	if cfg.FailThreshold != 3 {
		t.Errorf("FailThreshold = %d, want 3", cfg.FailThreshold)
	}
	if cfg.PortPattern != "18888" {
		t.Errorf("PortPattern = %q, want 18888", cfg.PortPattern)
	}
	if !strings.Contains(cfg.LogPath, "mem0-watchdog.ndjson") {
		t.Errorf("LogPath = %q, want to contain mem0-watchdog.ndjson", cfg.LogPath)
	}
}

func TestLogEntry_Format(t *testing.T) {
	t.Parallel()
	logPath := filepath.Join(t.TempDir(), "test-watchdog.ndjson")

	w := New(Config{
		ProbeURL:    "http://127.0.0.1:0/unused",
		LogPath:     logPath,
		PortPattern: "18888",
	})

	w.log(LogEntry{Event: "probe_ok", Status: "ok"})
	w.log(LogEntry{Event: "probe_fail", Status: "fail", Fails: 2, Error: "connection refused"})
	w.log(LogEntry{Event: "tunnel_restart", Status: "killed", Killed: []int{1234, 5678}})

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 log lines, got %d", len(lines))
	}

	var entry0 LogEntry
	if err := json.Unmarshal([]byte(lines[0]), &entry0); err != nil {
		t.Fatalf("unmarshal line 0: %v", err)
	}
	if entry0.Event != "probe_ok" {
		t.Errorf("line 0 event = %q, want probe_ok", entry0.Event)
	}
	if entry0.Timestamp == "" {
		t.Error("line 0 missing timestamp")
	}

	var entry1 LogEntry
	if err := json.Unmarshal([]byte(lines[1]), &entry1); err != nil {
		t.Fatalf("unmarshal line 1: %v", err)
	}
	if entry1.Fails != 2 {
		t.Errorf("line 1 fails = %d, want 2", entry1.Fails)
	}
	if entry1.Error != "connection refused" {
		t.Errorf("line 1 error = %q, want 'connection refused'", entry1.Error)
	}

	var entry2 LogEntry
	if err := json.Unmarshal([]byte(lines[2]), &entry2); err != nil {
		t.Fatalf("unmarshal line 2: %v", err)
	}
	if len(entry2.Killed) != 2 || entry2.Killed[0] != 1234 || entry2.Killed[1] != 5678 {
		t.Errorf("line 2 killed = %v, want [1234 5678]", entry2.Killed)
	}
}
