package eval

import (
	"fmt"
	"strings"
	"time"
)

type Report struct {
	RunID           string        `json:"run_id"`
	EvalCount       int           `json:"eval_count"`
	PassCount       int           `json:"pass_count"`
	FailCount       int           `json:"fail_count"`
	PassRate        float64       `json:"pass_rate"`
	AverageScore    float64       `json:"average_score"`
	Results         []EvalResult  `json:"results"`
	TrendDelta      float64       `json:"trend_delta,omitempty"`
	PreviousRate    float64       `json:"previous_rate,omitempty"`
	GeneratedAt     time.Time     `json:"generated_at"`
}

func GenerateReport(runID string, results []EvalResult) Report {
	passCount := 0
	for _, r := range results {
		if r.Pass {
			passCount++
		}
	}

	return Report{
		RunID:        runID,
		EvalCount:    len(results),
		PassCount:    passCount,
		FailCount:    len(results) - passCount,
		PassRate:     PassRate(results),
		AverageScore: AverageScore(results),
		Results:      results,
		GeneratedAt:  time.Now(),
	}
}

func (r *Report) WithTrend(previousRate float64) *Report {
	r.PreviousRate = previousRate
	r.TrendDelta = r.PassRate - previousRate
	return r
}

func (r *Report) ToMarkdown() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Eval Report: %s\n\n", r.RunID))
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", r.GeneratedAt.Format(time.RFC3339)))
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("| Metric | Value |\n|---|---|\n"))
	sb.WriteString(fmt.Sprintf("| Evals | %d |\n", r.EvalCount))
	sb.WriteString(fmt.Sprintf("| Passed | %d |\n", r.PassCount))
	sb.WriteString(fmt.Sprintf("| Failed | %d |\n", r.FailCount))
	sb.WriteString(fmt.Sprintf("| Pass Rate | %.1f%% |\n", r.PassRate*100))
	sb.WriteString(fmt.Sprintf("| Avg Score | %.2f |\n", r.AverageScore))

	if r.TrendDelta != 0 {
		direction := "improved"
		if r.TrendDelta < 0 {
			direction = "regressed"
		}
		sb.WriteString(fmt.Sprintf("| Trend | %s (%.1f%%) |\n", direction, r.TrendDelta*100))
	}

	sb.WriteString("\n## Results\n\n")
	sb.WriteString("| Eval | Type | Pass | Score | Iterations |\n")
	sb.WriteString("|---|---|---|---|---|\n")
	for _, result := range r.Results {
		status := "FAIL"
		if result.Pass {
			status = "PASS"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %.2f | %d |\n",
			result.EvalName, result.EvalType, status, result.Score, result.Iterations))
	}

	return sb.String()
}

func QualityBadge(passRate float64) string {
	switch {
	case passRate >= 0.90:
		return "Platinum"
	case passRate >= 0.80:
		return "Gold"
	case passRate >= 0.65:
		return "Silver"
	case passRate >= 0.50:
		return "Bronze"
	default:
		return "None"
	}
}
