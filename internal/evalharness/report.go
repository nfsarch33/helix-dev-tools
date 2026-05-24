package evalharness

import (
	"fmt"
	"strings"
	"time"
)

// SprintReport represents a complete sprint evaluation report.
type SprintReport struct {
	SprintID    string        `json:"sprint_id"`
	GeneratedAt string        `json:"generated_at"`
	Verdict     GateVerdict   `json:"verdict"`
	GraderStats []GraderStat  `json:"grader_stats"`
	EventCount  int           `json:"event_count"`
}

// GraderStat aggregates pass/fail counts per grader across all events.
type GraderStat struct {
	Name      string  `json:"name"`
	Total     int     `json:"total"`
	Passed    int     `json:"passed"`
	Failed    int     `json:"failed"`
	PassRate  float64 `json:"pass_rate"`
}

// GenerateSprintReport evaluates events and produces a structured report.
func GenerateSprintReport(sprintID string, events []AgentTraceEvent, graders []DeterministicGrader, gateCfg GateConfig) SprintReport {
	verdict := EvaluateGate(events, graders, gateCfg)

	stats := make(map[string]*GraderStat)
	for _, g := range graders {
		stats[g.Name()] = &GraderStat{Name: g.Name()}
	}
	for _, event := range events {
		for _, g := range graders {
			result := g.Grade(event)
			s := stats[g.Name()]
			s.Total++
			if result.Pass {
				s.Passed++
			} else {
				s.Failed++
			}
		}
	}

	var graderStats []GraderStat
	for _, g := range graders {
		s := stats[g.Name()]
		if s.Total > 0 {
			s.PassRate = float64(s.Passed) / float64(s.Total)
		}
		graderStats = append(graderStats, *s)
	}

	return SprintReport{
		SprintID:    sprintID,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Verdict:     verdict,
		GraderStats: graderStats,
		EventCount:  len(events),
	}
}

// FormatMarkdownReport produces a Markdown KPI report for sprint closeout.
func FormatMarkdownReport(r SprintReport) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Eval Report: %s\n\n", r.SprintID))
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt))

	status := "PASS"
	if !r.Verdict.Pass {
		status = "FAIL"
	}
	sb.WriteString(fmt.Sprintf("## Gate Verdict: %s\n\n", status))
	sb.WriteString(fmt.Sprintf("- Events evaluated: %d\n", r.EventCount))
	sb.WriteString(fmt.Sprintf("- Overall pass rate: %.1f%%\n", r.Verdict.PassRate*100))
	sb.WriteString(fmt.Sprintf("- Total failures: %d / %d\n\n", r.Verdict.FailCount, r.Verdict.TotalCount))

	sb.WriteString("## Grader Breakdown\n\n")
	sb.WriteString("| Grader | Pass | Fail | Rate |\n")
	sb.WriteString("|--------|------|------|------|\n")
	for _, s := range r.GraderStats {
		sb.WriteString(fmt.Sprintf("| %s | %d | %d | %.0f%% |\n",
			s.Name, s.Passed, s.Failed, s.PassRate*100))
	}
	sb.WriteString("\n")

	if len(r.Verdict.Failures) > 0 {
		sb.WriteString("## Top Failures\n\n")
		shown := len(r.Verdict.Failures)
		if shown > 5 {
			shown = 5
		}
		for i := 0; i < shown; i++ {
			f := r.Verdict.Failures[i]
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", f.GraderName, f.Reason))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
