package prombridge

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/metrics"
)

func TestFormat_EmptySummary(t *testing.T) {
	s := &metrics.Summary{
		TotalEvents: 0,
		Until:       time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC),
	}
	out := Format(s, "24h", nil)
	if !strings.Contains(out, "helixon_cursor_metrics_jsonl_events_total") {
		t.Fatalf("missing events metric: %q", out)
	}
	if !strings.Contains(out, `window="24h"`) {
		t.Fatalf("missing window label: %q", out)
	}
}

func TestFormat_HookTotals(t *testing.T) {
	s := &metrics.Summary{
		Until:       time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC),
		TotalEvents: 10,
		Hooks: []metrics.HookStats{
			{Hook: "guard-shell", Total: 6, DenyCount: 1, WarnCount: 1, AllowCount: 4, AvgLatency: 12.5, MaxLatency: 40},
			{Hook: "guard-mcp", Total: 4, DenyCount: 0, WarnCount: 0, AllowCount: 4, AvgLatency: 8, MaxLatency: 20},
		},
		Tasks: metrics.TaskCoverage{Total: 5, SkillTasks: 2, MCPTasks: 3},
	}
	out := Format(s, "168h", nil)
	if !strings.Contains(out, `helixon_cursor_hook_events_total{hook="guard_shell",window="168h"} 6`) {
		t.Fatalf("expected guard_shell count: %s", out)
	}
	if !strings.Contains(out, "helixon_cursor_metrics_jsonl_intervention_rate_percent") {
		t.Fatal("missing intervention rate")
	}
	// 2 interventions / 10 total = 20%
	if !strings.Contains(out, "20") {
		t.Fatalf("expected intervention rate ~20%% in output: %s", out)
	}
}

func TestFormat_Smoke(t *testing.T) {
	smoke := &EvoloopSmoke{
		PrometheusHealthy: true,
		DRLServiceHealthy: false,
		CheckedAt:         time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC),
	}
	out := Format(nil, "24h", smoke)
	if !strings.Contains(out, "helixon_evoloop_smoke_prometheus_ok 1") {
		t.Fatalf("expected prom ok=1: %s", out)
	}
	if !strings.Contains(out, "helixon_evoloop_smoke_drl_service_ok 0") {
		t.Fatalf("expected drl ok=0: %s", out)
	}
}

// Pushgateway rejects exposition lines that include a sample timestamp (third field).
var pushgatewayForbiddenSampleTS = regexp.MustCompile(` \d{13,}$`)

func TestFormat_NoSampleTimestampsForPushgateway(t *testing.T) {
	s := &metrics.Summary{
		Until:       time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC),
		TotalEvents: 3,
		Hooks:       []metrics.HookStats{{Hook: "x", Total: 1, AvgLatency: 1}},
		Tasks:       metrics.TaskCoverage{Total: 1, SkillTasks: 1, MCPTasks: 0},
	}
	smoke := &EvoloopSmoke{PrometheusHealthy: true, DRLServiceHealthy: true, CheckedAt: s.Until}
	out := Format(s, "24h", smoke)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if pushgatewayForbiddenSampleTS.MatchString(line) {
			t.Fatalf("forbidden Pushgateway sample timestamp suffix: %q", line)
		}
	}
}

func TestSanitizeHookName(t *testing.T) {
	if got := sanitizeHookName("guard-shell"); got != "guard_shell" {
		t.Fatalf("got %q", got)
	}
	if got := sanitizeHookName(""); got != "unknown" {
		t.Fatalf("got %q", got)
	}
}
