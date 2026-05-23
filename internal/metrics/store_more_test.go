// runx-public-repo-gate: allow-file personal_path_id,fleet_host_alias — tests assert detection of literal personal-stack identifiers (gate test fixtures)

package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestSmallHelpers(t *testing.T) {
	if got := (Trend{}).arrow(1); got != "^" {
		t.Fatalf("arrow(1) = %q", got)
	}
	if got := (Trend{}).arrow(-1); got != "v" {
		t.Fatalf("arrow(-1) = %q", got)
	}
	if got := (Trend{}).arrow(0); got != "=" {
		t.Fatalf("arrow(0) = %q", got)
	}

	if got := humanBytes(512); got != "512 B" {
		t.Fatalf("humanBytes(512) = %q", got)
	}
	if got := humanBytes(2048); got != "2.0 KB" {
		t.Fatalf("humanBytes(2048) = %q", got)
	}
}

func TestAdoptionAndRenderingHelpers(t *testing.T) {
	now := time.Now().UTC()
	events := []Event{
		{Timestamp: now.Add(-1 * time.Hour), Hook: "track", Action: "record", Category: "skill", Detail: "skill-a", DurationMs: 20, TurnID: "task-1", TaskSource: "exact"},
		{Timestamp: now.Add(-1 * time.Hour), Hook: "track", Action: "record", Category: "skill", Detail: "skill-a", DurationMs: 40, TurnID: "task-1", TaskSource: "exact"},
		{Timestamp: now.Add(-1 * time.Hour), Hook: "skill-activate", Action: "read", Category: "skill", Detail: "skill-read-only", DurationMs: 1, TurnID: "task-3", TaskSource: "exact"},
		{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-mcp", Action: "allow", Category: "mcp", Detail: "helixon:helixon_health", LatencyMs: 15, TurnID: "task-1", TaskSource: "exact"},
		{Timestamp: now.Add(-1 * time.Hour), Hook: "track", Action: "record", Category: "subagent", Detail: "go-architect", DurationMs: 5, TurnID: "task-2", TaskSource: "turn"},
	}

	skills, mcpServers, subagents := buildAdoptionStats(events, now.Add(-24*time.Hour))
	if len(skills) != 2 {
		t.Fatalf("unexpected skills count: got %d, want 2: %+v", len(skills), skills)
	}
	if len(mcpServers) != 1 || mcpServers[0].Server != "helixon" || mcpServers[0].Tool != "helixon_health" {
		t.Fatalf("unexpected mcp servers: %+v", mcpServers)
	}
	if len(subagents) != 1 || subagents[0].Detail != "go-architect" {
		t.Fatalf("unexpected subagents: %+v", subagents)
	}

	summary := Summarise(events, now.Add(-24*time.Hour))
	if summary.Tasks.Total != 3 || summary.Tasks.SkillTasks != 2 || summary.Tasks.MCPTasks != 1 || summary.Tasks.SubagentTasks != 1 {
		t.Fatalf("unexpected task coverage: %+v", summary.Tasks)
	}
	if summary.Tasks.IronclawTasks != 1 || summary.Tasks.ExactTasks != 2 || summary.Tasks.TurnTasks != 1 {
		t.Fatalf("unexpected task confidence coverage: %+v", summary.Tasks)
	}
	md := summary.Markdown()
	if !strings.Contains(md, "System Performance Report") || !strings.Contains(md, "Operation Timing by Category") || !strings.Contains(md, "Helixon MCP task coverage") {
		t.Fatalf("Markdown() output missing sections: %q", md)
	}
	compact := summary.Compact(7)
	if !strings.Contains(compact, "events 7d") || !strings.Contains(compact, "iron=") {
		t.Fatalf("Compact() output = %q", compact)
	}

	top := topN(map[string]int{"b": 1, "a": 3, "c": 2}, 2)
	if len(top) != 2 || top[0].Detail != "a" || top[1].Detail != "c" {
		t.Fatalf("topN() = %+v", top)
	}
}

func TestAnalyseFlagsLowAdoption(t *testing.T) {
	now := time.Now().UTC()
	events := []Event{
		{Timestamp: now.Add(-50 * time.Minute), Hook: "guard-shell", Action: "allow", Category: "shell", LatencyMs: 1, TurnID: "task-1"},
		{Timestamp: now.Add(-40 * time.Minute), Hook: "guard-shell", Action: "allow", Category: "shell", LatencyMs: 1, TurnID: "task-2"},
		{Timestamp: now.Add(-30 * time.Minute), Hook: "guard-shell", Action: "allow", Category: "shell", LatencyMs: 1, TurnID: "task-3"},
		{Timestamp: now.Add(-20 * time.Minute), Hook: "guard-mcp", Action: "allow", Category: "mcp", Detail: "perplexity:perplexity_search", LatencyMs: 1, TurnID: "task-4"},
		{Timestamp: now.Add(-10 * time.Minute), Hook: "guard-shell", Action: "allow", Category: "shell", LatencyMs: 1, TurnID: "task-5"},
	}

	summary := Summarise(events, now.Add(-24*time.Hour))
	recs := summary.Analyse()
	joined := make([]string, 0, len(recs))
	for _, rec := range recs {
		joined = append(joined, rec.Category+":"+rec.Message)
	}
	text := strings.Join(joined, "\n")
	for _, want := range []string{
		"adoption:Low skill task coverage",
		"adoption:Low MCP task coverage",
		"adoption:No subagent usage recorded",
		"adoption:Low MCP diversity",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("Analyse() missing %q in %q", want, text)
		}
	}
}

func TestAnalyseFlagsHeuristicTrackingGaps(t *testing.T) {
	now := time.Now().UTC()
	events := make([]Event, 0, 60)
	for i := 0; i < 60; i++ {
		events = append(events, Event{
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
			Hook:      "sanitize-read",
			Action:    "allow",
			Category:  "tool",
			LatencyMs: 1,
			Detail:    "README.md",
		})
	}

	summary := Summarise(events, now.Add(-24*time.Hour))
	if summary.Tasks.ExplicitTasks != 0 {
		t.Fatalf("expected no explicit tasks, got %+v", summary.Tasks)
	}
	recs := summary.Analyse()
	text := make([]string, 0, len(recs))
	for _, rec := range recs {
		text = append(text, rec.Category+":"+rec.Message)
	}
	joined := strings.Join(text, "\n")
	for _, want := range []string{
		"adoption:Low skill coverage by events",
		"adoption:No subagent usage recorded",
		"adoption:Task grouping is still heuristic",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("Analyse() missing %q in %q", want, joined)
		}
	}
}

func TestMemoryLayerHelpersAndSummary(t *testing.T) {
	now := time.Now().UTC()
	if layer, op := InferMemoryContextFromMCPDetail("search_memories"); layer != MemoryLayerMem0 || op != MemoryOpSearch {
		t.Fatalf("InferMemoryContextFromMCPDetail(search_memories) = %q/%q", layer, op)
	}
	if layer, op, result := InferMemoryContextFromReadPath("/Users/jason.lian/Code/global-kb/global-memories/daily-startup-prompt.md"); layer != MemoryLayerGitKB || op != MemoryOpRead || result != MemoryResultHit {
		t.Fatalf("InferMemoryContextFromReadPath() = %q/%q/%q, want git_kb/read/hit", layer, op, result)
	}

	events := []Event{
		{Timestamp: now.Add(-5 * time.Minute), Hook: "guard-mcp", Action: "allow", Category: "mcp", Detail: "mem0:search_memories", MemoryLayer: MemoryLayerMem0, MemoryOp: MemoryOpSearch},
		{Timestamp: now.Add(-4 * time.Minute), Hook: "track", Action: "record", Category: "mcp", Detail: "mem0:search_memories", MemoryLayer: MemoryLayerMem0, MemoryOp: MemoryOpSearch, MemoryResult: MemoryResultHit, ResultCount: 3},
		{Timestamp: now.Add(-3 * time.Minute), Hook: "track", Action: "record", Category: "mcp", Detail: "context-mode:ctx_search", MemoryLayer: MemoryLayerContextMode, MemoryOp: MemoryOpSearch, MemoryResult: MemoryResultMiss},
		{Timestamp: now.Add(-2 * time.Minute), Hook: "sanitize-read", Action: "allow", Category: "tool", Detail: "/Users/jason.lian/Code/global-kb/sop/mcp-tools-reference.md", MemoryLayer: MemoryLayerGitKB, MemoryOp: MemoryOpRead, MemoryResult: MemoryResultHit},
		{Timestamp: now.Add(-1 * time.Minute), Hook: "track", Action: "record", Category: "tool", Detail: "git_kb:mcp-tools-reference", MemoryLayer: MemoryLayerGitKB, MemoryOp: MemoryOpRead, MemoryResult: MemoryResultHit},
	}

	summary := Summarise(events, now.Add(-24*time.Hour))
	if len(summary.MemoryLayers) < 3 {
		t.Fatalf("expected memory layers, got %+v", summary.MemoryLayers)
	}
	if md := summary.Markdown(); !strings.Contains(md, "Coverage") || !strings.Contains(md, "Observed Hit Rate") {
		t.Fatalf("Markdown() missing coverage columns: %q", md)
	}

	md := summary.Markdown()
	if !strings.Contains(md, "Memory Layer KPIs") || !strings.Contains(md, MemoryLayerMem0) || !strings.Contains(md, MemoryLayerGitKB) {
		t.Fatalf("Markdown() missing memory section: %q", md)
	}

	compact := summary.Compact(7)
	if !strings.Contains(compact, "memory=") || !strings.Contains(compact, "mem0=") {
		t.Fatalf("Compact() missing memory summary: %q", compact)
	}
}
