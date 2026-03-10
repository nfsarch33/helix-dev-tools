package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

func TestRunMetricsRicherBranches(t *testing.T) {
	oldHome := os.Getenv("HOME")
	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Setenv("HOME", oldHome)
		metricsFlags.days = 7
		metricsFlags.export = ""
		metricsFlags.compact = false
		metricsFlags.analyse = false
	}()

	p := config.DefaultPaths()
	for _, dir := range []string{p.HooksDir, p.SkillsDir, p.AgentsDir, p.AgentsSkillsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(p.SkillsDir, "skill-a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.SkillsDir, "skill-a", "SKILL.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(p.AgentsSkillsDir, "skill-b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.AgentsSkillsDir, "skill-b", "SKILL.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.AgentsDir, "go-architect.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	events := []metrics.Event{
		{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-shell", Action: "deny", Category: "shell", LatencyMs: 10, Detail: "rm -rf /"},
		{Timestamp: now.Add(-1 * time.Hour), Hook: "track", Action: "record", Category: "skill", DurationMs: 42, Detail: "skill-a"},
		{Timestamp: now.Add(-1 * time.Hour), Hook: "guard-mcp", Action: "allow", Category: "mcp", LatencyMs: 30, Detail: "perplexity:perplexity_search"},
		{Timestamp: now.Add(-1 * time.Hour), Hook: "track", Action: "record", Category: "subagent", DurationMs: 5, Detail: "go-architect"},
		{Timestamp: now.Add(-1 * time.Hour), Hook: "doctor", Action: "pass", Category: "check", DurationMs: 11, Detail: "doctor", PassedCount: 5, TotalCount: 5},
	}
	for _, event := range events {
		if err := metrics.Record(p.MetricsFile(), event); err != nil {
			t.Fatal(err)
		}
	}

	metricsFlags.analyse = true
	if err := runMetrics(nil, nil); err != nil {
		t.Fatalf("runMetrics() error = %v", err)
	}

	exportDir := filepath.Join(home, "export")
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		t.Fatal(err)
	}
	metricsFlags.export = filepath.Join(exportDir, "report.md")
	if err := runMetrics(nil, nil); err != nil {
		t.Fatalf("runMetrics() export error = %v", err)
	}
	report, err := os.ReadFile(metricsFlags.export)
	if err != nil {
		t.Fatal(err)
	}
	reportText := string(report)
	for _, want := range []string{"Task Adoption Coverage", "Skill task coverage", "MCP task coverage"} {
		if !strings.Contains(reportText, want) {
			t.Fatalf("export missing %q in %q", want, reportText)
		}
	}

	metricsFlags.export = filepath.Join("/proc", "report.md")
	if err := runMetrics(nil, nil); err == nil {
		t.Fatal("runMetrics() expected export write failure")
	}
}
