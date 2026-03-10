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
		{Timestamp: now.Add(-1 * time.Hour), Hook: "track", Action: "record", Category: "skill", Detail: "skill-a", DurationMs: 20},
		{Timestamp: now.Add(-1 * time.Hour), Hook: "track", Action: "record", Category: "skill", Detail: "skill-a", DurationMs: 40},
		{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-mcp", Action: "allow", Category: "mcp", Detail: "perplexity:perplexity_search", LatencyMs: 15},
		{Timestamp: now.Add(-1 * time.Hour), Hook: "track", Action: "record", Category: "subagent", Detail: "go-architect", DurationMs: 5},
	}

	skills, mcpServers, subagents := buildAdoptionStats(events, now.Add(-24*time.Hour))
	if len(skills) != 1 || skills[0].Name != "skill-a" || skills[0].Uses != 2 {
		t.Fatalf("unexpected skills: %+v", skills)
	}
	if len(mcpServers) != 1 || mcpServers[0].Server != "perplexity" || mcpServers[0].Tool != "perplexity_search" {
		t.Fatalf("unexpected mcp servers: %+v", mcpServers)
	}
	if len(subagents) != 1 || subagents[0].Detail != "go-architect" {
		t.Fatalf("unexpected subagents: %+v", subagents)
	}

	summary := Summarise(events, now.Add(-24*time.Hour))
	md := summary.Markdown()
	if !strings.Contains(md, "System Performance Report") || !strings.Contains(md, "Operation Timing by Category") {
		t.Fatalf("Markdown() output missing sections: %q", md)
	}
	compact := summary.Compact(7)
	if !strings.Contains(compact, "events 7d") {
		t.Fatalf("Compact() output = %q", compact)
	}

	top := topN(map[string]int{"b": 1, "a": 3, "c": 2}, 2)
	if len(top) != 2 || top[0].Detail != "a" || top[1].Detail != "c" {
		t.Fatalf("topN() = %+v", top)
	}
}
