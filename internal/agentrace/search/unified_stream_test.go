package search

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSearch_UnifiedStreamFromMultipleServers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agentrace-mcp.ndjson")

	events := []string{
		`{"ts":"2026-05-20T10:00:00+10:00","tool":"sprint_create","agent_id":"cursor-parent","duration_ms":5,"success":true}`,
		`{"ts":"2026-05-20T10:00:01+10:00","tool":"ticket_create","agent_id":"cursor-parent","duration_ms":2,"success":true}`,
		`{"ts":"2026-05-20T10:00:02+10:00","server":"mem0-oss","tool":"mem0_search","agent":"cursor-parent","latency_ms":42,"success":true}`,
		`{"ts":"2026-05-20T10:00:03+10:00","server":"mem0-oss","tool":"mem0_add","agent":"cursor-parent","latency_ms":150,"success":false,"error":"timeout"}`,
		`{"ts":"2026-05-20T10:00:04+10:00","tool":"agent_register","agent_id":"cursor-parent","duration_ms":10,"success":true}`,
	}
	var content string
	for _, ev := range events {
		content += ev + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Search(path, "mem0", 24*time.Hour, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for 'mem0' query, got none")
	}
	for _, r := range results {
		if r.Event.Data["tool"] != "mem0_search" && r.Event.Data["tool"] != "mem0_add" {
			t.Errorf("unexpected tool in mem0 search result: %v", r.Event.Data["tool"])
		}
	}

	results2, err := Search(path, "sprint", 24*time.Hour, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results2) == 0 {
		t.Fatal("expected results for 'sprint' query, got none")
	}

	allResults, err := Search(path, "cursor", 24*time.Hour, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(allResults) < 3 {
		t.Fatalf("expected at least 3 results for 'cursor' query across both servers, got %d", len(allResults))
	}
}

func TestSearch_ReadsLiveUnifiedStream(t *testing.T) {
	path := filepath.Join(os.Getenv("HOME"), "logs", "runx", "agentrace-mcp.ndjson")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("agentrace-mcp.ndjson not present, skip live verification")
	}

	results, err := Search(path, "mem0", 24*time.Hour, 10)
	if err != nil {
		t.Fatalf("Search live stream: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected mem0 events in live stream after simulated append")
	}
	t.Logf("Found %d mem0 events in live unified stream", len(results))
	for _, r := range results {
		t.Logf("  tool=%v ts=%v", r.Event.Data["tool"], r.Event.Timestamp)
	}
}
