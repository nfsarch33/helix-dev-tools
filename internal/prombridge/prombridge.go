// Package prombridge formats Cursor hook rollups from metrics.jsonl summaries
// as Prometheus exposition text for Pushgateway (DRL Prometheus scrape).
package prombridge

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

var safeLabel = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// EvoloopSmoke holds optional HTTP probe results for observability.
type EvoloopSmoke struct {
	PrometheusHealthy bool
	DRLServiceHealthy bool
	CheckedAt         time.Time
}

// Format builds Prometheus text exposition (0.0.4) for a summary window.
// Metric prefix follows ironclaw_cursor_* for fleet dashboards.
func Format(s *metrics.Summary, window string, smoke *EvoloopSmoke) string {
	var b strings.Builder
	now := time.Now().UTC()
	if s != nil && !s.Until.IsZero() {
		now = s.Until
	}
	ts := now.UnixMilli()

	writeHelpType := func(name, help, typ string) {
		b.WriteString("# HELP ")
		b.WriteString(name)
		b.WriteString(" ")
		b.WriteString(help)
		b.WriteString("\n# TYPE ")
		b.WriteString(name)
		b.WriteString(" ")
		b.WriteString(typ)
		b.WriteString("\n")
	}

	w := escapeLabel(window)
	if w == "" {
		w = "unknown"
	}

	if s != nil {
		writeHelpType("ironclaw_cursor_metrics_jsonl_events_total",
			"Hook metrics events counted in the rollup window (gauge snapshot at push).", "gauge")
		b.WriteString(fmt.Sprintf("ironclaw_cursor_metrics_jsonl_events_total{window=%q} %d %d\n",
			w, s.TotalEvents, ts))

		writeHelpType("ironclaw_cursor_metrics_jsonl_intervention_rate_percent",
			"Percent of hook events that were deny or warn in the window.", "gauge")
		deny, warn, all := 0, 0, 0
		for _, h := range s.Hooks {
			deny += h.DenyCount
			warn += h.WarnCount
			all += h.Total
		}
		rate := 0.0
		if all > 0 {
			rate = float64(deny+warn) / float64(all) * 100
		}
		b.WriteString(fmt.Sprintf("ironclaw_cursor_metrics_jsonl_intervention_rate_percent{window=%q} %g %d\n",
			w, rate, ts))

		writeHelpType("ironclaw_cursor_hook_events_total",
			"Per-hook event count in the rollup window (gauge snapshot).", "gauge")
		for _, h := range s.Hooks {
			hk := sanitizeHookName(h.Hook)
			b.WriteString(fmt.Sprintf("ironclaw_cursor_hook_events_total{hook=%q,window=%q} %d %d\n",
				hk, w, h.Total, ts))
		}

		writeHelpType("ironclaw_cursor_hook_avg_latency_ms",
			"Average hook latency in milliseconds within the window.", "gauge")
		for _, h := range s.Hooks {
			hk := sanitizeHookName(h.Hook)
			b.WriteString(fmt.Sprintf("ironclaw_cursor_hook_avg_latency_ms{hook=%q,window=%q} %g %d\n",
				hk, w, h.AvgLatency, ts))
		}

		writeHelpType("ironclaw_cursor_task_skill_coverage_percent",
			"Percent of task groups with at least one skill activation.", "gauge")
		cov := 0.0
		if s.Tasks.Total > 0 {
			cov = float64(s.Tasks.SkillTasks) / float64(s.Tasks.Total) * 100
		}
		b.WriteString(fmt.Sprintf("ironclaw_cursor_task_skill_coverage_percent{window=%q} %g %d\n",
			w, cov, ts))

		writeHelpType("ironclaw_cursor_task_mcp_coverage_percent",
			"Percent of task groups with at least one MCP call.", "gauge")
		mcpCov := 0.0
		if s.Tasks.Total > 0 {
			mcpCov = float64(s.Tasks.MCPTasks) / float64(s.Tasks.Total) * 100
		}
		b.WriteString(fmt.Sprintf("ironclaw_cursor_task_mcp_coverage_percent{window=%q} %g %d\n",
			w, mcpCov, ts))
	}

	if smoke != nil && !smoke.CheckedAt.IsZero() {
		tss := smoke.CheckedAt.UnixMilli()
		writeHelpType("ironclaw_evoloop_smoke_prometheus_ok",
			"1 if DRL Prometheus /-/healthy probe succeeded at last scheduled smoke.", "gauge")
		v := 0.0
		if smoke.PrometheusHealthy {
			v = 1
		}
		b.WriteString(fmt.Sprintf("ironclaw_evoloop_smoke_prometheus_ok %g %d\n", v, tss))

		writeHelpType("ironclaw_evoloop_smoke_drl_service_ok",
			"1 if drl-service /healthz probe succeeded at last scheduled smoke.", "gauge")
		v2 := 0.0
		if smoke.DRLServiceHealthy {
			v2 = 1
		}
		b.WriteString(fmt.Sprintf("ironclaw_evoloop_smoke_drl_service_ok %g %d\n", v2, tss))
	}

	return b.String()
}

func escapeLabel(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

func sanitizeHookName(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return "unknown"
	}
	h = safeLabel.ReplaceAllString(h, "_")
	if h == "" {
		return "unknown"
	}
	return h
}
