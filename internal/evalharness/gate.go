package evalharness

import (
	"fmt"
	"strings"
	"time"
)

// GateVerdict represents the overall result of a quality gate check.
type GateVerdict struct {
	Pass        bool          `json:"pass"`
	FailCount   int           `json:"fail_count"`
	TotalCount  int           `json:"total_count"`
	PassRate    float64       `json:"pass_rate"`
	Failures    []GradeResult `json:"failures,omitempty"`
	CheckedAt   string        `json:"checked_at"`
}

// GateConfig specifies thresholds for the quality gate.
type GateConfig struct {
	MinPassRate float64 `json:"min_pass_rate"`
	MaxFailures int     `json:"max_failures"`
}

// DefaultGateConfig returns the ADR-065 quality gate defaults.
func DefaultGateConfig() GateConfig {
	return GateConfig{
		MinPassRate: 0.80,
		MaxFailures: 5,
	}
}

// EvaluateGate runs all graders against a batch of events and returns
// a gate verdict (pass/fail for pre-push or closeout).
func EvaluateGate(events []AgentTraceEvent, graders []DeterministicGrader, cfg GateConfig) GateVerdict {
	var results []GradeResult
	var failures []GradeResult

	for _, event := range events {
		for _, grader := range graders {
			result := grader.Grade(event)
			results = append(results, result)
			if !result.Pass {
				failures = append(failures, result)
			}
		}
	}

	total := len(results)
	failCount := len(failures)
	passRate := 1.0
	if total > 0 {
		passRate = float64(total-failCount) / float64(total)
	}

	pass := passRate >= cfg.MinPassRate && failCount <= cfg.MaxFailures
	return GateVerdict{
		Pass:       pass,
		FailCount:  failCount,
		TotalCount: total,
		PassRate:   passRate,
		Failures:   failures,
		CheckedAt:  time.Now().UTC().Format(time.RFC3339),
	}
}

// FormatGateVerdict produces a human-readable summary.
func FormatGateVerdict(v GateVerdict) string {
	var sb strings.Builder
	status := "PASS"
	if !v.Pass {
		status = "FAIL"
	}
	sb.WriteString(fmt.Sprintf("Gate: %s (%.1f%% pass rate, %d failures / %d total)\n",
		status, v.PassRate*100, v.FailCount, v.TotalCount))

	if len(v.Failures) > 0 {
		sb.WriteString("Failures:\n")
		shown := v.FailCount
		if shown > 10 {
			shown = 10
		}
		for i := 0; i < shown; i++ {
			f := v.Failures[i]
			sb.WriteString(fmt.Sprintf("  - [%s] %s\n", f.GraderName, f.Reason))
		}
		if v.FailCount > 10 {
			sb.WriteString(fmt.Sprintf("  ... and %d more\n", v.FailCount-10))
		}
	}
	return sb.String()
}
