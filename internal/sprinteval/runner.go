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
//
// Two schemas are accepted at decode time:
//   - Legacy sprinteval: {ts, event, tool, error, session, sprint_id, tokens_in, tokens_out, model}
//   - Helixon TracedExecutor: {ts, event_type, tool, server, agent_id, duration_ms, success, error_message}
//
// UnmarshalJSON normalises both onto the same field surface so downstream
// metrics (computeErrorRate, computeToolReliability) work unchanged.
type AgentraceEvent struct {
	Timestamp  time.Time `json:"ts"`
	Event      string    `json:"event"`
	EventType  string    `json:"event_type,omitempty"`
	Tool       string    `json:"tool,omitempty"`
	Server     string    `json:"server,omitempty"`
	AgentID    string    `json:"agent_id,omitempty"`
	DurationMS int64     `json:"duration_ms,omitempty"`
	Success    *bool     `json:"success,omitempty"`
	ErrorMsg   string    `json:"error_message,omitempty"`
	TokensIn   int       `json:"tokens_in,omitempty"`
	TokensOut  int       `json:"tokens_out,omitempty"`
	Model      string    `json:"model,omitempty"`
	SessionID  string    `json:"session,omitempty"`
	Error      string    `json:"error,omitempty"`
	SprintID   string    `json:"sprint_id,omitempty"`
}

// UnmarshalJSON decodes into the raw schema then normalises the helixon
// fields onto the legacy ones so existing metric code keeps working.
func (e *AgentraceEvent) UnmarshalJSON(data []byte) error {
	type rawEvent AgentraceEvent
	var raw rawEvent
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*e = AgentraceEvent(raw)
	e.normalise()
	return nil
}

// normalise maps helixon-schema fields onto the legacy fields when the
// legacy ones are empty. Helixon's success=false is treated as an error
// when no error_message is supplied so reliability metrics still count it.
func (e *AgentraceEvent) normalise() {
	if e.Event == "" && e.EventType != "" {
		e.Event = e.EventType
	}
	if e.Error == "" && e.ErrorMsg != "" {
		e.Error = e.ErrorMsg
	}
	if e.Success != nil && !*e.Success && e.Error == "" {
		e.Error = "tool failed"
	}
}

// SprintReport is the complete evaluation output for a sprint.
type SprintReport struct {
	SprintID     string             `json:"sprint_id"`
	SprintName   string             `json:"sprint_name"`
	EvaluatedAt  time.Time          `json:"evaluated_at"`
	QualityScore float64            `json:"quality_score"`
	QualityGrade string             `json:"quality_grade"`
	Metrics      SprintMetrics      `json:"metrics"`
	Breakdown    []MetricDetail     `json:"breakdown"`
	Latency      LatencyStats       `json:"latency"`
	ToolStats    []ToolLatencyEntry `json:"tool_stats,omitempty"`
	Trends       []TrendPoint       `json:"trends,omitempty"`
	Findings     []string           `json:"findings,omitempty"`
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
		Latency:      ComputeLatencyStats(events),
		ToolStats:    ComputeToolHistogram(events),
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

	if report.Latency.Count > 0 {
		sb.WriteString("\n## Latency (ms)\n\n")
		sb.WriteString("| Count | Min | P50 | P95 | Max |\n")
		sb.WriteString("|-------|-----|-----|-----|-----|\n")
		sb.WriteString(fmt.Sprintf("| %d | %d | %d | %d | %d |\n",
			report.Latency.Count,
			report.Latency.MinMs,
			report.Latency.P50Ms,
			report.Latency.P95Ms,
			report.Latency.MaxMs,
		))
	}

	if len(report.ToolStats) > 0 {
		sb.WriteString("\n## Tool Histogram\n\n")
		sb.WriteString("| Tool | Calls | Failures | P50 (ms) | P95 (ms) | Max (ms) |\n")
		sb.WriteString("|------|-------|----------|----------|----------|----------|\n")
		for _, t := range report.ToolStats {
			sb.WriteString(fmt.Sprintf("| %s | %d | %d | %d | %d | %d |\n",
				t.Tool, t.Calls, t.Failures, t.P50Ms, t.P95Ms, t.MaxMs,
			))
		}
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

// ParseAgentrace reads an NDJSON agentrace log file and returns all
// parsed events. Lines that fail to decode are skipped silently.
// Default path: ~/logs/runx/agentrace-mcp.ndjson
func ParseAgentrace(path string) ([]AgentraceEvent, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("agentrace: resolve home: %w", err)
		}
		path = filepath.Join(home, "logs", "runx", "agentrace-mcp.ndjson")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("agentrace: open %s: %w", path, err)
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
		var ev AgentraceEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		events = append(events, ev)
	}

	if err := scanner.Err(); err != nil {
		return events, fmt.Errorf("agentrace: scan: %w", err)
	}
	return events, nil
}

// GenerateReport produces a Markdown report from agentrace events and
// optional sprintboard data. The report includes tool call statistics,
// error breakdown, and sprint metrics when available.
func GenerateReport(events []AgentraceEvent, sprint *SprintData) string {
	var sb strings.Builder

	sb.WriteString("# Sprint Evaluation Report\n\n")
	sb.WriteString(fmt.Sprintf("**Generated**: %s\n\n", time.Now().Format(time.RFC3339)))

	if sprint != nil {
		sb.WriteString("## Sprint Overview\n\n")
		sb.WriteString(fmt.Sprintf("- **Sprint**: %s\n", sprint.SprintID))
		sb.WriteString(fmt.Sprintf("- **Total Tickets**: %d\n", sprint.TotalTickets))
		sb.WriteString(fmt.Sprintf("- **Completed**: %d\n", sprint.CompletedTickets))
		sb.WriteString(fmt.Sprintf("- **Completion Rate**: %.1f%%\n", sprint.CompletionRate*100))
		if sprint.AvgTimeToClaim > 0 {
			sb.WriteString(fmt.Sprintf("- **Avg Time-to-Claim**: %s\n", time.Duration(sprint.AvgTimeToClaim)*time.Second))
		}
		sb.WriteString("\n")
	}

	if len(events) == 0 {
		sb.WriteString("## Agentrace\n\nNo events found.\n")
		return sb.String()
	}

	// Compute tool call stats
	toolCounts := make(map[string]int)
	toolErrors := make(map[string]int)
	var totalDuration int64
	errorCount := 0

	for _, ev := range events {
		if ev.Tool != "" {
			toolCounts[ev.Tool]++
			totalDuration += ev.DurationMS
		}
		if ev.Error != "" {
			errorCount++
			if ev.Tool != "" {
				toolErrors[ev.Tool]++
			}
		}
	}

	sb.WriteString("## Agentrace Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Events**: %d\n", len(events)))
	sb.WriteString(fmt.Sprintf("- **Tool Calls**: %d\n", len(toolCounts)))
	sb.WriteString(fmt.Sprintf("- **Errors**: %d (%.1f%%)\n", errorCount, float64(errorCount)/float64(len(events))*100))
	if len(toolCounts) > 0 {
		avgDur := totalDuration / int64(len(events))
		sb.WriteString(fmt.Sprintf("- **Avg Duration**: %dms\n", avgDur))
	}
	sb.WriteString("\n")

	// Tool breakdown table
	if len(toolCounts) > 0 {
		sb.WriteString("## Tool Call Breakdown\n\n")
		sb.WriteString("| Tool | Calls | Errors | Error Rate |\n")
		sb.WriteString("|------|-------|--------|------------|\n")
		for tool, count := range toolCounts {
			errs := toolErrors[tool]
			rate := float64(errs) / float64(count) * 100
			sb.WriteString(fmt.Sprintf("| %s | %d | %d | %.1f%% |\n", tool, count, errs, rate))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
