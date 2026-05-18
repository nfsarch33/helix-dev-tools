package mem0bridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnqueue_AddsToPending(t *testing.T) {
	b := New("http://localhost:18888", "")
	s := Signal{ID: "s1", From: "agent1", To: "agent2", Ticket: "t1", Summary: "done"}
	b.Enqueue(s)
	if b.PendingCount() != 1 {
		t.Errorf("expected 1 pending, got %d", b.PendingCount())
	}
}

func TestEnqueue_SetsCreatedAt(t *testing.T) {
	b := New("http://localhost:18888", "")
	before := time.Now()
	b.Enqueue(Signal{ID: "s1"})
	b.mu.Lock()
	ts := b.outbox[0].Signal.CreatedAt
	b.mu.Unlock()
	if ts.Before(before) {
		t.Errorf("expected CreatedAt >= before, got %v", ts)
	}
}

func TestFlush_DeliversToMem0(t *testing.T) {
	received := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/memories/" {
			received++
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer srv.Close()

	b := New(srv.URL, "")
	b.Enqueue(Signal{ID: "s1", From: "a", To: "b", Ticket: "t1", Summary: "ok"})
	delivered, err := b.Flush()
	if err != nil {
		t.Fatalf("Flush error: %v", err)
	}
	if delivered != 1 {
		t.Errorf("expected 1 delivered, got %d", delivered)
	}
	if received != 1 {
		t.Errorf("expected 1 HTTP call, got %d", received)
	}
	if b.PendingCount() != 0 {
		t.Errorf("expected 0 pending after flush, got %d", b.PendingCount())
	}
}

func TestFlush_FallsBackToGitKB(t *testing.T) {
	dir := t.TempDir()
	b := New("http://127.0.0.1:19999", dir) // unreachable port
	b.Enqueue(Signal{ID: "s1", From: "a", To: "b", Ticket: "t1", Summary: "test"})

	delivered, _ := b.Flush()
	if delivered != 1 {
		t.Errorf("expected 1 delivered (via fallback), got %d", delivered)
	}

	// Verify file written to kbDir
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("expected 1 file in kbDir, got %d", len(entries))
	}
}

func TestSubscribe_ReturnsFromMem0(t *testing.T) {
	signals := []Signal{
		{ID: "s1", From: "a", To: "target", Ticket: "t1", Summary: "msg1"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"memory": signals[0].Summary,
					"metadata": map[string]string{
						"signal_id": signals[0].ID,
						"from":      signals[0].From,
						"ticket":    signals[0].Ticket,
						"branch":    signals[0].Branch,
					},
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer srv.Close()

	b := New(srv.URL, "")
	got, err := b.Subscribe("target")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if len(got) != 1 || got[0].ID != "s1" {
		t.Errorf("expected signal s1, got %v", got)
	}
}

func TestSubscribe_FallsBackToGitKB(t *testing.T) {
	dir := t.TempDir()
	s := Signal{ID: "s1", From: "a", To: "target", Ticket: "t1", Summary: "kb msg"}
	data, _ := json.Marshal(s)
	os.WriteFile(filepath.Join(dir, "2026-05-19T120000-s1.json"), data, 0644)

	b := New("http://127.0.0.1:19999", dir) // unreachable
	got, err := b.Subscribe("target")
	if err != nil {
		t.Fatalf("Subscribe fallback: %v", err)
	}
	if len(got) != 1 || got[0].ID != "s1" {
		t.Errorf("expected signal s1 from KB, got %v", got)
	}
}

func TestReconcile_DeduplicatesByID(t *testing.T) {
	dir := t.TempDir()
	s := Signal{ID: "s1", From: "a", To: "target", Ticket: "t1", Summary: "both"}
	data, _ := json.Marshal(s)
	os.WriteFile(filepath.Join(dir, "2026-05-19T120000-s1.json"), data, 0644)

	// Mem0 server returns the same signal
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"memory": s.Summary,
					"metadata": map[string]string{
						"signal_id": s.ID,
						"from":      s.From,
						"ticket":    s.Ticket,
						"branch":    "",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer srv.Close()

	b := New(srv.URL, dir)
	got, err := b.Reconcile("target")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 deduplicated signal, got %d", len(got))
	}
}

func TestReadFromGitKB_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	b := New("", dir)
	signals, err := b.readFromGitKB("anyone")
	if err != nil {
		t.Fatalf("readFromGitKB: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0, got %d", len(signals))
	}
}
