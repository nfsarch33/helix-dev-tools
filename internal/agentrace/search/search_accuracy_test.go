package search

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSearchAccuracy_100EntryCorpus(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	f := filepath.Join(tmp, "events.jsonl")
	now := time.Now().UTC()
	var content string
	for i := 0; i < 100; i++ {
		ts := now.Add(-time.Duration(i) * time.Minute).Format(time.RFC3339Nano)
		switch {
		case i < 5:
			content += fmt.Sprintf("{\"ts\":\"%s\",\"event\":\"deploy_failure\",\"service\":\"api-gateway\",\"error\":\"connection refused port 8080\"}\n", ts)
		case i < 15:
			content += fmt.Sprintf("{\"ts\":\"%s\",\"event\":\"resource_probe\",\"free_pct\":%d,\"status\":\"GREEN\"}\n", ts, 50+i)
		case i < 30:
			content += fmt.Sprintf("{\"ts\":\"%s\",\"event\":\"session_stall\",\"session_id\":\"sess_%d\",\"stall_seconds\":%d}\n", ts, i, i*10)
		case i < 50:
			content += fmt.Sprintf("{\"ts\":\"%s\",\"event\":\"built\",\"binary\":\"cursor-tools\",\"duration_ms\":%d}\n", ts, 1000+i*100)
		default:
			content += fmt.Sprintf("{\"ts\":\"%s\",\"event\":\"evoloop_cycle\",\"phase\":\"observe\",\"cycle\":%d}\n", ts, i)
		}
	}
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Search(f, "deploy failure connection refused", 24*time.Hour, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 1 {
		t.Fatal("expected at least 1 result for deploy_failure search")
	}
	for _, r := range results[:min(5, len(results))] {
		if r.Event.EventType != "deploy_failure" {
			t.Errorf("top results should be deploy_failure, got %s", r.Event.EventType)
		}
	}
}

func TestSearchAccuracy_RankingOrder(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	f := filepath.Join(tmp, "events.jsonl")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	content := fmt.Sprintf("{\"ts\":\"%s\",\"event\":\"unrelated\",\"data\":\"nothing here\"}\n", now) +
		fmt.Sprintf("{\"ts\":\"%s\",\"event\":\"partial_match\",\"data\":\"contains deploy word\"}\n", now) +
		fmt.Sprintf("{\"ts\":\"%s\",\"event\":\"deploy_failure\",\"service\":\"deploy\",\"error\":\"deploy crashed deploy\"}\n", now)
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Search(f, "deploy", 24*time.Hour, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	if results[0].Score <= results[1].Score {
		t.Error("first result should score higher than second")
	}
}

func TestSearchAccuracy_EmptyIndex(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	f := filepath.Join(tmp, "events.jsonl")
	if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	results, err := Search(f, "anything", 24*time.Hour, 10)
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Errorf("expected nil for empty index, got %d results", len(results))
	}
}

func TestSearchAccuracy_NoMatchingResults(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	f := filepath.Join(tmp, "events.jsonl")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	content := fmt.Sprintf("{\"ts\":\"%s\",\"event\":\"alpha\",\"data\":\"bravo charlie\"}\n", now) +
		fmt.Sprintf("{\"ts\":\"%s\",\"event\":\"delta\",\"data\":\"echo foxtrot\"}\n", now)
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	results, err := Search(f, "zebra xylophone", 24*time.Hour, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for no-match query, got %d", len(results))
	}
}
