package fleeteval

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// WriteNDJSON writes results as newline-delimited JSON to the given path.
func WriteNDJSON(path string, results []Result) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, r := range results {
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("encode result %s: %w", r.TaskID, err)
		}
	}
	return nil
}

// FormatMarkdown produces a human-readable markdown report.
func FormatMarkdown(report *RunReport) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Fleet Eval Report: %s\n\n", report.RunID))
	b.WriteString(fmt.Sprintf("**Model**: %s\n", report.Model))
	b.WriteString(fmt.Sprintf("**Timestamp**: %s\n", report.Timestamp.Format("2006-01-02T15:04:05-07:00")))
	b.WriteString(fmt.Sprintf("**Verdict**: %s\n\n", report.Verdict))

	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("| Metric | Value |\n"))
	b.WriteString(fmt.Sprintf("| --- | --- |\n"))
	b.WriteString(fmt.Sprintf("| Total Score | %d / %d |\n", report.TotalScore, report.MaxScore))
	pct := 0.0
	if report.MaxScore > 0 {
		pct = float64(report.TotalScore) / float64(report.MaxScore) * 100
	}
	b.WriteString(fmt.Sprintf("| Percentage | %.1f%% |\n", pct))
	b.WriteString(fmt.Sprintf("| Pass / Fail / Error | %d / %d / %d |\n",
		report.PassCount, report.FailCount, report.ErrorCount))
	b.WriteString(fmt.Sprintf("| Avg Duration | %d ms |\n\n", report.AvgDurationMS))

	b.WriteString("## Per-Task Results\n\n")
	b.WriteString("| ID | Lvl | Title | Score | Pass | Pattern | Duration |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range report.Results {
		status := "PASS"
		if !r.Pass {
			status = "FAIL"
		}
		if r.Error != "" {
			status = "ERROR"
		}
		pattern := "yes"
		if !r.PatternMatch {
			pattern = "no"
		}
		b.WriteString(fmt.Sprintf("| %s | L%d | %s | %d/%d | %s | %s | %dms |\n",
			r.TaskID, r.Level, r.Title, r.Score, r.MaxScore, status, pattern, r.DurationMS))
	}

	failures := filterFailures(report.Results)
	if len(failures) > 0 {
		b.WriteString("\n## Failure Details\n\n")
		for _, r := range failures {
			b.WriteString(fmt.Sprintf("### %s: %s\n\n", r.TaskID, r.Title))
			if r.Error != "" {
				b.WriteString(fmt.Sprintf("**Error**: %s\n\n", r.Error))
			}
			if r.Response != "" {
				b.WriteString(fmt.Sprintf("**Response** (first 500 chars):\n```\n%s\n```\n\n",
					truncate(r.Response, 500)))
			}
			for _, g := range r.GradeDetail {
				b.WriteString(fmt.Sprintf("- %s: %.2f (weight=%.1f) %s\n", g.Metric, g.Score, g.Weight, g.Note))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

func filterFailures(results []Result) []Result {
	var out []Result
	for _, r := range results {
		if !r.Pass || r.Error != "" {
			out = append(out, r)
		}
	}
	return out
}
