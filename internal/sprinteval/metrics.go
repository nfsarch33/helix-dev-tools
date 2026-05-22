package sprinteval

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

// MetricWeights controls how individual metrics contribute to the composite
// quality score. Weights should sum to 1.0.
type MetricWeights struct {
	CompletionRate     float64 `json:"completion_rate"`
	EstimationAccuracy float64 `json:"estimation_accuracy"`
	Coverage           float64 `json:"coverage"`
	ErrorRate          float64 `json:"error_rate"`
	ToolReliability    float64 `json:"tool_reliability"`
}

// DefaultWeights returns the standard metric weights per ADR-061.
func DefaultWeights() MetricWeights {
	return MetricWeights{
		CompletionRate:     0.30,
		EstimationAccuracy: 0.25,
		Coverage:           0.20,
		ErrorRate:          0.15,
		ToolReliability:    0.10,
	}
}

func (w MetricWeights) withDefaults() MetricWeights {
	if w.CompletionRate <= 0 && w.EstimationAccuracy <= 0 && w.Coverage <= 0 && w.ErrorRate <= 0 && w.ToolReliability <= 0 {
		return DefaultWeights()
	}
	return w
}

// SprintMetrics holds all computed metrics for a sprint evaluation.
type SprintMetrics struct {
	CompletionRate     float64 `json:"completion_rate"`
	EstimationAccuracy float64 `json:"estimation_accuracy"`
	Coverage           float64 `json:"coverage"`
	ErrorRate          float64 `json:"error_rate"`
	ToolReliability    float64 `json:"tool_reliability"`
	AvgTokensPerTask   int     `json:"avg_tokens_per_task"`
	TotalTickets       int     `json:"total_tickets"`
	CompletedTickets   int     `json:"completed_tickets"`
	StartedTickets     int     `json:"started_tickets"`
	TotalEvents        int     `json:"total_events"`
	ErrorEvents        int     `json:"error_events"`
	TotalToolCalls     int     `json:"total_tool_calls"`
	FailedToolCalls    int     `json:"failed_tool_calls"`
	TotalTokens        int     `json:"total_tokens"`
	TestsPassed        int     `json:"tests_passed"`
	TestsFailed        int     `json:"tests_failed"`
	TestsSkipped       int     `json:"tests_skipped"`
}

// ComputeMetrics calculates all sprint metrics from the raw data sources.
func ComputeMetrics(
	tickets []TicketSnapshot,
	events []AgentraceEvent,
	tests []TestResult,
	estimates map[string]Estimate,
) SprintMetrics {
	m := SprintMetrics{
		TotalTickets: len(tickets),
	}

	m.CompletionRate, m.CompletedTickets, m.StartedTickets = computeCompletion(tickets)
	m.Coverage = computeCoverage(tickets)
	m.ErrorRate, m.TotalEvents, m.ErrorEvents = computeErrorRate(events)
	m.ToolReliability, m.TotalToolCalls, m.FailedToolCalls = computeToolReliability(events)
	m.TotalTokens, m.AvgTokensPerTask = computeTokenUsage(events, m.CompletedTickets)
	m.EstimationAccuracy = computeEstimationAccuracy(estimates)
	m.TestsPassed, m.TestsFailed, m.TestsSkipped = computeTestResults(tests)

	return m
}

// computeCompletion returns done_tickets / total_tickets.
func computeCompletion(tickets []TicketSnapshot) (rate float64, completed, started int) {
	if len(tickets) == 0 {
		return 0, 0, 0
	}

	for _, t := range tickets {
		switch t.Status {
		case "done":
			completed++
			started++
		case "in_progress", "review", "blocked", "ready_for_handoff":
			started++
		}
	}

	return float64(completed) / float64(len(tickets)), completed, started
}

// computeCoverage returns (started + done) / planned.
func computeCoverage(tickets []TicketSnapshot) float64 {
	if len(tickets) == 0 {
		return 0
	}

	touched := 0
	for _, t := range tickets {
		if t.Status != "backlog" && t.Status != "ready" {
			touched++
		}
	}

	return float64(touched) / float64(len(tickets))
}

// computeErrorRate returns error_events / total_events.
func computeErrorRate(events []AgentraceEvent) (rate float64, total, errors int) {
	total = len(events)
	if total == 0 {
		return 0, 0, 0
	}

	for _, e := range events {
		if e.Error != "" {
			errors++
		}
	}

	return float64(errors) / float64(total), total, errors
}

// computeToolReliability returns successful_tool_calls / total_tool_calls.
func computeToolReliability(events []AgentraceEvent) (rate float64, total, failed int) {
	for _, e := range events {
		if e.Event != "tool_call" {
			continue
		}
		total++
		if e.Error != "" {
			failed++
		}
	}

	if total == 0 {
		return 1.0, 0, 0
	}
	return float64(total-failed) / float64(total), total, failed
}

// computeTokenUsage sums tokens across all LLM events.
func computeTokenUsage(events []AgentraceEvent, completedTasks int) (total, avgPerTask int) {
	for _, e := range events {
		total += e.TokensIn + e.TokensOut
	}

	if completedTasks > 0 {
		avgPerTask = total / completedTasks
	}
	return total, avgPerTask
}

// computeEstimationAccuracy returns 1 - abs(estimated - actual) / estimated,
// averaged across all phases that have actual measurements.
func computeEstimationAccuracy(estimates map[string]Estimate) float64 {
	if len(estimates) == 0 {
		return 1.0
	}

	var totalAccuracy float64
	var counted int

	for _, est := range estimates {
		if est.Actual <= 0 || est.Corrected <= 0 {
			continue
		}

		correctedSec := est.Corrected.Seconds()
		actualSec := est.Actual.Seconds()

		if correctedSec == 0 {
			continue
		}

		ratio := math.Abs(correctedSec-actualSec) / correctedSec
		accuracy := 1.0 - ratio
		if accuracy < 0 {
			accuracy = 0
		}

		totalAccuracy += accuracy
		counted++
	}

	if counted == 0 {
		return 1.0
	}
	return totalAccuracy / float64(counted)
}

// computeTestResults tallies pass/fail/skip counts from test output.
func computeTestResults(tests []TestResult) (passed, failed, skipped int) {
	for _, t := range tests {
		if t.Pass {
			passed++
		} else if t.Elapsed == 0 && t.Output == "" {
			skipped++
		} else {
			failed++
		}
	}
	return passed, failed, skipped
}

// EstimationRatio returns the overestimate/underestimate ratio.
// Values > 1.0 mean overestimate, < 1.0 mean underestimate.
func EstimationRatio(estimated, actual time.Duration) float64 {
	if actual <= 0 {
		return 0
	}
	return estimated.Seconds() / actual.Seconds()
}

// QualityTrend computes the direction of quality score change.
func QualityTrend(history []float64) string {
	if len(history) < 2 {
		return "insufficient_data"
	}

	last := history[len(history)-1]
	prev := history[len(history)-2]

	delta := last - prev
	switch {
	case delta > 0.05:
		return "improving"
	case delta < -0.05:
		return "degrading"
	default:
		return "stable"
	}
}

// SprintData holds metrics fetched from the Sprintboard REST API.
type SprintData struct {
	SprintID         string  `json:"sprint_id"`
	SprintName       string  `json:"name"`
	TotalTickets     int     `json:"total_tickets"`
	CompletedTickets int     `json:"completed_tickets"`
	InProgress       int     `json:"in_progress"`
	Blocked          int     `json:"blocked"`
	CompletionRate   float64 `json:"completion_rate"`
	AvgTimeToClaim   float64 `json:"avg_time_to_claim_seconds"`
}

// sprintAPIResponse wraps the Sprintboard REST response envelope.
type sprintAPIResponse struct {
	Data SprintData `json:"data"`
}

const defaultSprintboardBaseURL = "http://localhost:9400"

// FetchSprintMetrics retrieves sprint-level metrics from the Sprintboard
// REST API. If baseURL is empty, defaults to http://localhost:9400.
func FetchSprintMetrics(baseURL, sprintID string) (*SprintData, error) {
	if baseURL == "" {
		baseURL = defaultSprintboardBaseURL
	}
	if sprintID == "" {
		return nil, fmt.Errorf("sprinteval: sprint_id is required")
	}

	url := fmt.Sprintf("%s/api/v1/sprints/%s", baseURL, sprintID)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("sprinteval: fetch sprint %s: %w", sprintID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("sprinteval: sprint %s not found", sprintID)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("sprinteval: sprint %s: status %d: %s", sprintID, resp.StatusCode, string(body))
	}

	var envelope sprintAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("sprinteval: decode sprint %s: %w", sprintID, err)
	}

	data := envelope.Data
	if data.SprintID == "" {
		data.SprintID = sprintID
	}
	if data.TotalTickets > 0 && data.CompletionRate == 0 {
		data.CompletionRate = float64(data.CompletedTickets) / float64(data.TotalTickets)
	}

	return &data, nil
}
