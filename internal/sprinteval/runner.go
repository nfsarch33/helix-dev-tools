package sprinteval

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SprintEval computes aggregate quality metrics for a completed sprint
// by analysing agentrace events, sprintboard tickets, and test results.
type SprintEval struct {
	logger  *slog.Logger
	weights MetricWeights
}

// New creates a SprintEval runner with the given metric weights.
func New(weights MetricWeights, logger *slog.Logger) *SprintEval {
	if logger == nil {
		logger = slog.Default()
	}
	weights = weights.withDefaults()
	return &SprintEval{
		logger:  logger.With(slog.String("component", "sprinteval")),
		weights: weights,
	}
}

// SprintInput collects all data sources for evaluating a sprint.
type SprintInput struct {
	SprintID      string               `json:"sprint_id"`
	SprintName    string               `json:"sprint_name"`
	Tickets       []TicketSnapshot     `json:"tickets"`
	AgentraceFile string               `json:"agentrace_file"`
	TestResults   []TestResult         `json:"test_results"`
	Estimates     map[string]Estimate  `json:"estimates"`
	StartTime     time.Time            `json:"start_time"`
	EndTime       time.Time            `json:"end_time"`
}

// TicketSnapshot captures a ticket's state at sprint evaluation time.
type TicketSnapshot struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Status      string        `json:"status"`
	Assignee    string        `json:"assignee,omitempty"`
	Priority    int           `json:"priority"`
	CreatedAt   time.Time     `json:"created_at"`
	CompletedAt time.Time     `json:"completed_at,omitempty"`
	TimeInState time.Duration `json:"time_in_state,omitempty"`
}

// TestResult captures the outcome of a single test.
type TestResult struct {
	Package string  `json:"package"`
	Name    string  `json:"name"`
	Pass    bool    `json:"pass"`
	Elapsed float64 `json:"elapsed"`
	Output  string  `json:"output,omitempty"`
}

// Estimate captures planned vs actual time for a sprint phase.
type Estimate struct {
	PhaseName string        `json:"phase_name"`
	Naive     time.Duration `json:"naive"`
	Corrected time.Duration `json:"corrected"`
	Actual    time.Duration `json:"actual"`
}

// AgentraceEvent represents a single NDJSON event from the agentrace log.
type AgentraceEvent struct {
	Timestamp  time.Time `json:"ts"`
	Event      string    `json:"event"`
	Tool       string    `json:"tool,omitempty"`
	DurationMS int64     `json:"duration_ms,omitempty"`
	TokensIn   int       `json:"tokens_in,omitempty"`
	TokensOut  int       `json:"tokens_out,omitempty"`
	Model      string    `json:"model,omitempty"`
	SessionID  string    `json:"session,omitempty"`
	Error      string    `json:"error,omitempty"`
	SprintID   string    `json:"sprint_id,omitempty"`
}

// SprintReport is the complete evaluation output for a sprint.
type SprintReport struct {
	SprintID     string         `json:"sprint_id"`
	SprintName   string         `json:"sprint_name"`
	EvaluatedAt  time.Time      `json:"evaluated_at"`
	QualityScore float64        `json:"quality_score"`
	QualityGrade string         `json:"quality_grade"`
	Metrics      SprintMetrics  `json:"metrics"`
	Breakdown    []MetricDetail `json:"breakdown"`
	Trends       []TrendPoint   `json:"trends,omitempty"`
	Findings     []string       `json:"findings,omitempty"`
}

// MetricDetail provides per-metric breakdown in the report.
type MetricDetail struct {
	Name   string  `json:"name"`
	Value  float64 `json:"value"`
	Target float64 `json:"target"`
	Weight float64 `json:"weight"`
	Pass   bool    `json:"pass"`
}

// TrendPoint captures a historical data point for trend analysis.
type TrendPoint struct {
	SprintID     string  `json:"sprint_id"`
	QualityScore float64 `json:"quality_score"`
}

// Run evaluates a sprint and produces a SprintReport.
func (se *SprintEval) Run(input SprintInput) (*SprintReport, error) {
	if input.SprintID == "" {
		return nil, fmt.Errorf("sprinteval: sprint_id is required")
	}

	se.logger.Info("evaluating sprint",
		slog.String("sprint_id", input.SprintID),
		slog.Int("tickets", len(input.Tickets)),
	)

	events, err := se.loadAgentrace(input.AgentraceFile, input.StartTime, input.EndTime)
	if err != nil {
		se.logger.Warn("agentrace load failed (proceeding without)",
			slog.String("error", err.Error()),
		)
	}

	metrics := ComputeMetrics(input.Tickets, events, input.TestResults, input.Estimates)

	breakdown := []MetricDetail{
		{Name: "Completion Rate", Value: metrics.CompletionRate, Target: 0.80, Weight: se.weights.CompletionRate, Pass: metrics.CompletionRate >= 0.80},
		{Name: "Estimation Accuracy", Value: metrics.EstimationAccuracy, Target: 0.50, Weight: se.weights.EstimationAccuracy, Pass: metrics.EstimationAccuracy >= 0.50},
		{Name: "Coverage", Value: metrics.Coverage, Target: 0.90, Weight: se.weights.Coverage, Pass: metrics.Coverage >= 0.90},
		{Name: "Error Rate", Value: metrics.ErrorRate, Target: 0.05, Weight: se.weights.ErrorRate, Pass: metrics.ErrorRate <= 0.05},
		{Name: "Tool Reliability", Value: metrics.ToolReliability, Target: 0.95, Weight: se.weights.ToolReliability, Pass: metrics.ToolReliability >= 0.95},
	}

	qualityScore := se.computeQualityScore(metrics)

	report := &SprintReport{
		SprintID:     input.SprintID,
		SprintName:   input.SprintName,
		EvaluatedAt:  time.Now(),
		QualityScore: qualityScore,
		QualityGrade: gradeScore(qualityScore),
		Metrics:      metrics,
		Breakdown:    breakdown,
		Findings:     se.generateFindings(metrics, input),
	}

	se.logger.Info("sprint evaluation complete",
		slog.String("sprint_id", input.SprintID),
		slog.Float64("quality_score", qualityScore),
		slog.String("grade", report.QualityGrade),
	)

	return report, nil
}

// WriteReport writes the sprint report as a markdown file.
func (se *SprintEval) WriteReport(report *SprintReport, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create report dir: %w", err)
	}

	filename := filepath.Join(dir, fmt.Sprintf("%s.md", report.SprintID))
	md := se.renderMarkdown(report)

	if err := os.WriteFile(filename, []byte(md), 0644); err != nil {
		return "", fmt.Errorf("write report: %w", err)
	}

	se.logger.Info("report written", slog.String("path", filename))
	return filename, nil
}

// WriteJSON writes the sprint report as a JSON file alongside the markdown.
func (se *SprintEval) WriteJSON(report *SprintReport, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create report dir: %w", err)
	}

	filename := filepath.Join(dir, fmt.Sprintf("%s.json", report.SprintID))
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal report: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return "", fmt.Errorf("write JSON report: %w", err)
	}
	return filename, nil
}

func (se *SprintEval) loadAgentrace(path string, start, end time.Time) ([]AgentraceEvent, error) {
	if path == "" {
		return nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open agentrace: %w", err)
	}
	defer f.Close()

	var events []AgentraceEvent
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var evt AgentraceEvent
		if err := json.Unmarshal(line, &evt); err != nil {
			se.logger.Debug("skip malformed agentrace line", slog.String("error", err.Error()))
			continue
		}

		if !start.IsZero() && evt.Timestamp.Before(start) {
			continue
		}
		if !end.IsZero() && evt.Timestamp.After(end) {
			continue
		}

		events = append(events, evt)
	}

	return events, scanner.Err()
}

func (se *SprintEval) computeQualityScore(m SprintMetrics) float64 {
	score := 0.0
	score += m.CompletionRate * se.weights.CompletionRate
	score += m.EstimationAccuracy * se.weights.EstimationAccuracy
	score += m.Coverage * se.weights.Coverage

	invertedErrorRate := 1.0 - m.ErrorRate
	if invertedErrorRate < 0 {
		invertedErrorRate = 0
	}
	score += invertedErrorRate * se.weights.ErrorRate

	score += m.ToolReliability * se.weights.ToolReliability
	return score
}

func (se *SprintEval) generateFindings(m SprintMetrics, input SprintInput) []string {
	var findings []string

	if m.CompletionRate < 0.80 {
		notDone := 0
		for _, t := range input.Tickets {
			if t.Status != "done" {
				notDone++
			}
		}
		findings = append(findings, fmt.Sprintf("Completion rate %.0f%% below 80%% target: %d tickets not done", m.CompletionRate*100, notDone))
	}

	if m.EstimationAccuracy < 0.50 {
		findings = append(findings, fmt.Sprintf("Estimation accuracy %.0f%% below 50%% target: consider breaking phases into smaller tickets", m.EstimationAccuracy*100))
	}

	if m.ErrorRate > 0.05 {
		findings = append(findings, fmt.Sprintf("Error rate %.1f%% exceeds 5%% threshold: investigate recurring failures", m.ErrorRate*100))
	}

	if m.ToolReliability < 0.95 {
		findings = append(findings, fmt.Sprintf("Tool reliability %.1f%% below 95%%: check for flaky tools or network issues", m.ToolReliability*100))
	}

	if m.AvgTokensPerTask > 50000 {
		findings = append(findings, fmt.Sprintf("Average tokens per task (%d) is high: consider optimising prompts", m.AvgTokensPerTask))
	}

	if len(findings) == 0 {
		findings = append(findings, "All metrics within target ranges")
	}

	return findings
}

func (se *SprintEval) renderMarkdown(report *SprintReport) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Sprint Eval: %s\n\n", report.SprintID))
	if report.SprintName != "" {
		sb.WriteString(fmt.Sprintf("**Sprint**: %s\n", report.SprintName))
	}
	sb.WriteString(fmt.Sprintf("**Quality Score**: %.2f (%s)\n", report.QualityScore, report.QualityGrade))
	sb.WriteString(fmt.Sprintf("**Evaluated**: %s\n\n", report.EvaluatedAt.Format(time.RFC3339)))

	sb.WriteString("## Metrics\n\n")
	sb.WriteString("| Metric | Value | Target | Status |\n")
	sb.WriteString("|--------|-------|--------|--------|\n")

	for _, d := range report.Breakdown {
		status := "PASS"
		if !d.Pass {
			status = "FAIL"
		}
		if d.Name == "Error Rate" {
			sb.WriteString(fmt.Sprintf("| %s | %.1f%% | <= %.0f%% | %s |\n", d.Name, d.Value*100, d.Target*100, status))
		} else {
			sb.WriteString(fmt.Sprintf("| %s | %.1f%% | >= %.0f%% | %s |\n", d.Name, d.Value*100, d.Target*100, status))
		}
	}

	sb.WriteString("\n## Summary Statistics\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Tickets**: %d\n", report.Metrics.TotalTickets))
	sb.WriteString(fmt.Sprintf("- **Completed**: %d\n", report.Metrics.CompletedTickets))
	sb.WriteString(fmt.Sprintf("- **Total Events**: %d\n", report.Metrics.TotalEvents))
	sb.WriteString(fmt.Sprintf("- **Error Events**: %d\n", report.Metrics.ErrorEvents))
	sb.WriteString(fmt.Sprintf("- **Total Tokens**: %d\n", report.Metrics.TotalTokens))
	if report.Metrics.AvgTokensPerTask > 0 {
		sb.WriteString(fmt.Sprintf("- **Avg Tokens/Task**: %d\n", report.Metrics.AvgTokensPerTask))
	}

	if len(report.Findings) > 0 {
		sb.WriteString("\n## Findings\n\n")
		for _, f := range report.Findings {
			sb.WriteString(fmt.Sprintf("- %s\n", f))
		}
	}

	if len(report.Trends) > 0 {
		sb.WriteString("\n## Trends\n\n")
		sb.WriteString("| Sprint | Quality Score |\n")
		sb.WriteString("|--------|---------------|\n")
		for _, tp := range report.Trends {
			sb.WriteString(fmt.Sprintf("| %s | %.2f |\n", tp.SprintID, tp.QualityScore))
		}
	}

	return sb.String()
}

func gradeScore(score float64) string {
	switch {
	case score >= 0.90:
		return "EXCELLENT"
	case score >= 0.80:
		return "GREEN"
	case score >= 0.60:
		return "AMBER"
	default:
		return "RED"
	}
}
