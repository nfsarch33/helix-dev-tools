package search

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSearch_RanksRelevantEventsHigher(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	eventsFile := filepath.Join(tmp, "events.jsonl")

	now := time.Now().UTC().Format(time.RFC3339Nano)
	lines := []string{
		`{"ts":"` + now + `","event":"session_stall","session_id":"abc","stall_seconds":120}`,
		`{"ts":"` + now + `","event":"resource_probe","free_pct":80,"status":"GREEN"}`,
		`{"ts":"` + now + `","event":"session_stall","session_id":"def","stall_seconds":300}`,
		`{"ts":"` + now + `","event":"built","binary":"cursor-tools","duration_ms":4500}`,
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(eventsFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Search(eventsFile, "session stall", 24*time.Hour, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results for 'session stall', got %d", len(results))
	}
	for _, r := range results[:2] {
		if r.Event.EventType != "session_stall" {
			t.Errorf("top results should be session_stall events, got %s", r.Event.EventType)
		}
	}
}

func TestSearch_FiltersOldEvents(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	eventsFile := filepath.Join(tmp, "events.jsonl")

	old := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339Nano)
	recent := time.Now().UTC().Format(time.RFC3339Nano)
	content := `{"ts":"` + old + `","event":"old_event","data":"old"}` + "\n" +
		`{"ts":"` + recent + `","event":"recent_event","data":"recent"}` + "\n"
	if err := os.WriteFile(eventsFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Search(eventsFile, "event", 24*time.Hour, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (old filtered out), got %d", len(results))
	}
	if results[0].Event.EventType != "recent_event" {
		t.Errorf("expected recent_event, got %s", results[0].Event.EventType)
	}
}

func TestSearch_EmptyQueryReturnsNil(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	eventsFile := filepath.Join(tmp, "events.jsonl")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	content := `{"ts":"` + now + `","event":"test"}` + "\n"
	if err := os.WriteFile(eventsFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Search(eventsFile, "", 24*time.Hour, 10)
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Errorf("expected nil for empty query, got %d results", len(results))
	}
}

func TestSearch_RespectsMaxResults(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	eventsFile := filepath.Join(tmp, "events.jsonl")

	now := time.Now().UTC().Format(time.RFC3339Nano)
	content := ""
	for i := 0; i < 20; i++ {
		content += `{"ts":"` + now + `","event":"match","idx":` + string(rune('0'+i%10)) + `}` + "\n"
	}
	if err := os.WriteFile(eventsFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Search(eventsFile, "match", 24*time.Hour, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) > 5 {
		t.Errorf("expected at most 5 results, got %d", len(results))
	}
}
