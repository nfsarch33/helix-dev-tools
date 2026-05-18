package kbfallback

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestProbe_Online(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := New(srv.URL, "")
	if f.Probe() != StateOnline {
		t.Error("expected StateOnline")
	}
	if f.CurrentState() != StateOnline {
		t.Error("expected state to be Online")
	}
}

func TestProbe_Fallback_Unreachable(t *testing.T) {
	f := New("http://127.0.0.1:19999", "")
	if f.Probe() != StateFallback {
		t.Error("expected StateFallback for unreachable host")
	}
}

func TestProbe_Fallback_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	f := New(srv.URL, "")
	if f.Probe() != StateFallback {
		t.Error("expected StateFallback for 503 response")
	}
}

func TestWriteEvent_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	f := New("", dir)
	ev := Event{ID: "e1", AgentID: "a1", EventType: "handoff", Data: map[string]string{"ticket": "t1"}}
	if err := f.WriteEvent(ev); err != nil {
		t.Fatalf("WriteEvent: %v", err)
	}

	evDir := filepath.Join(dir, "coordination-events")
	entries, err := os.ReadDir(evDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file, got %d", len(entries))
	}
}

func TestReadEvents_ReturnsAll(t *testing.T) {
	dir := t.TempDir()
	evDir := filepath.Join(dir, "coordination-events")
	os.MkdirAll(evDir, 0755)

	events := []Event{
		{ID: "e1", AgentID: "a1", EventType: "handoff"},
		{ID: "e2", AgentID: "a2", EventType: "signal"},
	}
	for _, ev := range events {
		data, _ := json.Marshal(ev)
		os.WriteFile(filepath.Join(evDir, ev.ID+".json"), data, 0644)
	}

	f := New("", dir)
	got, err := f.ReadEvents()
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 events, got %d", len(got))
	}
}

func TestReadEvents_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	f := New("", dir)
	got, err := f.ReadEvents()
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 events, got %d", len(got))
	}
}
