package hookmetrics

import (
	"strings"
	"testing"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

func TestExporter_HitRateAndMutationLabels(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	events := []metrics.Event{
		{Timestamp: now.Add(-time.Hour), Category: "git", Action: "mutation", Detail: "runx git commit"},
		{Timestamp: now.Add(-time.Hour), Category: "git", Action: "mutation", Detail: "runx git push"},
		{Timestamp: now.Add(-time.Hour), Hook: "pre-push", Action: "allow"},
		{Timestamp: now.Add(-time.Hour), Hook: "post-shell", Action: "allow"},
		{Timestamp: now.Add(-48 * time.Hour), Category: "git", Action: "mutation", Detail: "old"},
	}

	out := ExportPrometheus(events, now.Add(-24*time.Hour))

	for _, want := range []string{
		`cursor_hook_git_mutations_total 2`,
		`cursor_hook_fires_total{hook="pre-push"} 1`,
		`cursor_hook_fires_total{hook="post-shell"} 1`,
		`cursor_hook_hit_rate 1`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("export missing %q:\n%s", want, out)
		}
	}
}

func TestExporter_SkillCoverageAndMCPDiversity(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	events := []metrics.Event{
		{Timestamp: now.Add(-time.Hour), Category: "skill", Action: "activate", Detail: "monitoring-observability", TurnID: "task-1", TaskSource: "exact"},
		{Timestamp: now.Add(-time.Hour), Category: "mcp", Action: "call", Detail: "mem0:search_memories", TurnID: "task-1", TaskSource: "exact"},
		{Timestamp: now.Add(-time.Hour), Category: "mcp", Action: "call", Detail: "context-mode:ctx_search", TurnID: "task-2", TaskSource: "exact"},
		{Timestamp: now.Add(-time.Hour), Category: "subagent", Action: "invoke", Detail: "go-tester", TurnID: "task-3", TaskSource: "exact"},
		{Timestamp: now.Add(-time.Hour), Category: "shell", Action: "allow", Detail: "test", TurnID: "task-4", TaskSource: "exact"},
	}

	out := ExportPrometheus(events, now.Add(-24*time.Hour))

	for _, want := range []string{
		`cursor_skill_task_coverage_ratio 0.25`,
		`cursor_mcp_diversity_servers 2`,
		`cursor_subagent_invocations_total{agent="go-tester"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("export missing %q:\n%s", want, out)
		}
	}
}
