package agentrace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentraceSearch_FindsMatchingEntries(t *testing.T) {
	tmp := t.TempDir()

	writeNDJSON(t, tmp, "agentrace-events.ndjson", []string{
		`{"time":"2026-05-14T01:00:00+10:00","kind":"session_start","msg":"session started for sprint v374"}`,
		`{"time":"2026-05-14T01:30:00+10:00","kind":"tool_call","msg":"Read file internal/cli/root.go"}`,
		`{"time":"2026-05-14T02:00:00+10:00","kind":"error","msg":"failed to connect to Temporal server"}`,
		`{"time":"2026-05-14T02:30:00+10:00","kind":"tool_call","msg":"Shell: go test ./..."}`,
		`{"time":"2026-05-14T03:00:00+10:00","kind":"session_end","msg":"session completed for sprint v374"}`,
	})
	writeNDJSON(t, tmp, "evoloop-smoke.ndjson", []string{
		`{"time":"2026-05-14T01:10:00+10:00","kind":"smoke","msg":"evoloop cycle 42 started"}`,
		`{"time":"2026-05-14T01:20:00+10:00","kind":"smoke","msg":"sprint v374 evoloop health OK"}`,
	})

	idx, err := BuildIndex(tmp)
	if err != nil {
		t.Fatalf("BuildIndex() error = %v", err)
	}

	results := idx.Search("sprint v374", 10)
	if len(results) == 0 {
		t.Fatal("expected at least one result for 'sprint v374'")
	}

	found := false
	for _, r := range results {
		if strings.Contains(r.Line, "sprint v374") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a result containing 'sprint v374', got %d results", len(results))
	}
}

func TestAgentraceSearch_RanksRelevantHigher(t *testing.T) {
	tmp := t.TempDir()

	writeNDJSON(t, tmp, "events.ndjson", []string{
		`{"time":"2026-05-14T01:00:00+10:00","msg":"unrelated log entry about docker pull"}`,
		`{"time":"2026-05-14T01:30:00+10:00","msg":"Temporal workflow failed with timeout error in production"}`,
		`{"time":"2026-05-14T02:00:00+10:00","msg":"Temporal server connection refused on port 7233"}`,
	})

	idx, err := BuildIndex(tmp)
	if err != nil {
		t.Fatalf("BuildIndex() error = %v", err)
	}

	results := idx.Search("Temporal", 10)
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results for 'Temporal', got %d", len(results))
	}

	for _, r := range results {
		if !strings.Contains(strings.ToLower(r.Line), "temporal") {
			t.Errorf("result should contain 'temporal': %q", r.Line)
		}
	}
}

func TestAgentraceSearch_LimitResults(t *testing.T) {
	tmp := t.TempDir()

	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, `{"time":"2026-05-14T01:00:00+10:00","msg":"repeated keyword match"}`)
	}
	writeNDJSON(t, tmp, "events.ndjson", lines)

	idx, err := BuildIndex(tmp)
	if err != nil {
		t.Fatalf("BuildIndex() error = %v", err)
	}

	results := idx.Search("keyword", 5)
	if len(results) != 5 {
		t.Errorf("expected 5 results with limit=5, got %d", len(results))
	}
}

func TestAgentraceSearch_EmptyDirectory(t *testing.T) {
	tmp := t.TempDir()

	idx, err := BuildIndex(tmp)
	if err != nil {
		t.Fatalf("BuildIndex() error = %v", err)
	}

	results := idx.Search("anything", 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty index, got %d", len(results))
	}
}

func TestAgentraceSearch_NoMatch(t *testing.T) {
	tmp := t.TempDir()

	writeNDJSON(t, tmp, "events.ndjson", []string{
		`{"time":"2026-05-14T01:00:00+10:00","msg":"alpha beta gamma"}`,
	})

	idx, err := BuildIndex(tmp)
	if err != nil {
		t.Fatalf("BuildIndex() error = %v", err)
	}

	results := idx.Search("zzzznotfound", 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-matching query, got %d", len(results))
	}
}

func TestAgentraceSearch_CaseInsensitive(t *testing.T) {
	tmp := t.TempDir()

	writeNDJSON(t, tmp, "events.ndjson", []string{
		`{"time":"2026-05-14T01:00:00+10:00","msg":"Error in TEMPORAL workflow"}`,
	})

	idx, err := BuildIndex(tmp)
	if err != nil {
		t.Fatalf("BuildIndex() error = %v", err)
	}

	results := idx.Search("temporal", 10)
	if len(results) == 0 {
		t.Error("expected case-insensitive match for 'temporal' against 'TEMPORAL'")
	}
}

func writeNDJSON(t *testing.T, dir, name string, lines []string) {
	t.Helper()
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
