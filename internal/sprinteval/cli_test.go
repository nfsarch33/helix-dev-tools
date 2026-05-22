package sprinteval

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunSprintEval_EndToEndWritesMarkdownAndJSON exercises the full
// pipeline against a stub sprintboard and a fixture agentrace file.
func TestRunSprintEval_EndToEndWritesMarkdownAndJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/sprints/v8000":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sprint": map[string]any{
					"id":   "v8000",
					"name": "v8000 overnight",
				},
				"tickets_by_status": map[string]int{"done": 4, "in_progress": 1},
				"total_tickets":     5,
			})
		case "/api/v1/sprints/v8000/tickets":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sprint_id": "v8000",
				"tickets": []map[string]any{
					{"id": "T-8000-B1", "title": "helixon", "status": "done", "priority": 1},
					{"id": "T-8000-B2", "title": "sprinteval", "status": "done", "priority": 1},
					{"id": "T-8000-B3", "title": "orderwf", "status": "done", "priority": 1},
					{"id": "T-8000-B4", "title": "sembleproxy", "status": "done", "priority": 1},
					{"id": "T-8000-B9", "title": "ndjson", "status": "in_progress", "priority": 1},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	tmp := t.TempDir()
	agentrace := filepath.Join(tmp, "agentrace.ndjson")
	body := strings.Join([]string{
		`{"ts":"2026-05-22T10:00:00Z","event_type":"tool_call","tool":"memory.search","success":true,"duration_ms":12,"agent_id":"claude-code"}`,
		`{"ts":"2026-05-22T10:00:01Z","event_type":"tool_call","tool":"sprintboard.claim_ticket","success":true,"duration_ms":8}`,
		`{"ts":"2026-05-22T10:00:02Z","event_type":"tool_call","tool":"sprintboard.complete_ticket","success":false,"error_message":"boom"}`,
	}, "\n")
	if err := os.WriteFile(agentrace, []byte(body), 0o644); err != nil {
		t.Fatalf("write agentrace: %v", err)
	}

	out := filepath.Join(tmp, "reports")
	err := runSprintEval(sprintEvalOpts{
		SprintID:      "v8000",
		AgentracePath: agentrace,
		SprintURL:     srv.URL,
		OutputDir:     out,
	})
	if err != nil {
		t.Fatalf("runSprintEval: %v", err)
	}

	mdPath := filepath.Join(out, "sprint-eval-v8000.md")
	jsonPath := filepath.Join(out, "sprint-eval-v8000.json")

	mdBytes, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	md := string(mdBytes)
	for _, want := range []string{
		"Sprint Eval: sprint-eval-v8000",
		"v8000 overnight",
		"Completion Rate",
		"Tool Reliability",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\n---\n%s", want, md)
		}
	}

	jsonBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	var parsed SprintReport
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	// 4/5 done -> 80% completion. Tool reliability: 2/3 -> ~66.7%.
	if parsed.Metrics.CompletedTickets != 4 || parsed.Metrics.TotalTickets != 5 {
		t.Errorf("metrics totals = %d/%d, want 4/5", parsed.Metrics.CompletedTickets, parsed.Metrics.TotalTickets)
	}
	if parsed.Metrics.TotalToolCalls != 3 || parsed.Metrics.FailedToolCalls != 1 {
		t.Errorf("tool calls = %d/%d, want 3/1", parsed.Metrics.FailedToolCalls, parsed.Metrics.TotalToolCalls)
	}
	if parsed.QualityGrade == "" {
		t.Error("quality grade should be set")
	}
}

// TestRunSprintEval_HistogramFallback exercises the older-server path
// where /sprints/{id}/tickets is missing. The CLI must fall back to
// SnapshotsFromHistogram so the report still grades correctly.
func TestRunSprintEval_HistogramFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/sprints/v8000":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sprint":            map[string]any{"id": "v8000", "name": "fallback test"},
				"tickets_by_status": map[string]int{"done": 8, "in_progress": 2},
				"total_tickets":     10,
			})
		default:
			http.NotFound(w, r) // /tickets returns 404 (older server)
		}
	}))
	defer srv.Close()

	tmp := t.TempDir()
	out := filepath.Join(tmp, "reports")

	err := runSprintEval(sprintEvalOpts{
		SprintID:  "v8000",
		SprintURL: srv.URL,
		OutputDir: out,
	})
	if err != nil {
		t.Fatalf("runSprintEval: %v", err)
	}

	mdBytes, err := os.ReadFile(filepath.Join(out, "sprint-eval-v8000.md"))
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	md := string(mdBytes)
	if !strings.Contains(md, "Sprint Eval: sprint-eval-v8000") {
		t.Errorf("markdown missing header\n---\n%s", md)
	}
	if !strings.Contains(md, "10") {
		t.Errorf("markdown missing total ticket count\n---\n%s", md)
	}
}

func TestExpandPath(t *testing.T) {
	got := expandPath("/abs/path")
	if got != "/abs/path" {
		t.Errorf("expandPath /abs/path = %q, want unchanged", got)
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		got = expandPath("~/foo/bar")
		want := filepath.Join(home, "foo/bar")
		if got != want {
			t.Errorf("expandPath ~/foo/bar = %q, want %q", got, want)
		}
	}
}
