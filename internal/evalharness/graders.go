package evalharness

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// GradeResult holds the output of a single grader evaluation.
type GradeResult struct {
	GraderName string  `json:"grader_name"`
	Pass       bool    `json:"pass"`
	Score      float64 `json:"score"`
	Reason     string  `json:"reason"`
	Timestamp  string  `json:"timestamp"`
}

// DeterministicGrader evaluates an agentrace event without LLM calls.
type DeterministicGrader interface {
	Name() string
	Grade(event AgentTraceEvent) GradeResult
}

// AgentTraceEvent is the minimal parsed shape of an agentrace NDJSON line.
type AgentTraceEvent struct {
	Timestamp      string          `json:"ts" yaml:"ts"`
	Event          string          `json:"event" yaml:"event"`
	Tool           string          `json:"tool,omitempty" yaml:"tool"`
	LatencyMS      float64         `json:"latency_ms,omitempty" yaml:"latency_ms"`
	Success        bool            `json:"success,omitempty" yaml:"success"`
	Error          string          `json:"error,omitempty" yaml:"error"`
	TokensUsed     int             `json:"tokens_used,omitempty" yaml:"tokens_used"`
	Coverage       float64         `json:"coverage,omitempty" yaml:"coverage"`
	LintClean      *bool           `json:"lint_clean,omitempty" yaml:"lint_clean"`
	TestPass       *bool           `json:"test_pass,omitempty" yaml:"test_pass"`
	ToolsTotal     int             `json:"tools_total,omitempty" yaml:"tools_total"`
	ToolsExercised int             `json:"tools_exercised,omitempty" yaml:"tools_exercised"`
	TasksTotal     int             `json:"tasks_total,omitempty" yaml:"tasks_total"`
	TasksCompleted int             `json:"tasks_completed,omitempty" yaml:"tasks_completed"`
	BaselineScore  float64         `json:"baseline_score,omitempty" yaml:"baseline_score"`
	CurrentScore   float64         `json:"current_score,omitempty" yaml:"current_score"`
	Raw            json.RawMessage `json:"raw,omitempty" yaml:"raw"`
}

// GraderConfig holds thresholds for all deterministic graders.
type GraderConfig struct {
	MaxP95LatencyMS     float64 `json:"max_p95_latency_ms"`
	MaxErrorRate        float64 `json:"max_error_rate"`
	MinCoverage         float64 `json:"min_coverage"`
	MaxTokensPerTask    int     `json:"max_tokens_per_task"`
	RequireConventional bool    `json:"require_conventional"`
	MinToolCoverage     float64 `json:"min_tool_coverage"`
	MinCompletionRate   float64 `json:"min_completion_rate"`
	MaxRegressionDelta  float64 `json:"max_regression_delta"`
}

// DefaultGraderConfig returns sensible defaults per ADR-065.
func DefaultGraderConfig() GraderConfig {
	return GraderConfig{
		MaxP95LatencyMS:     5000,
		MaxErrorRate:        0.05,
		MinCoverage:         0.70,
		MaxTokensPerTask:    50000,
		RequireConventional: true,
		MinToolCoverage:     0.60,
		MinCompletionRate:   0.80,
		MaxRegressionDelta:  0.10,
	}
}

// LatencyGrader checks tool-call latency against threshold.
type LatencyGrader struct {
	MaxMS float64
}

func (g *LatencyGrader) Name() string { return "latency" }

func (g *LatencyGrader) Grade(event AgentTraceEvent) GradeResult {
	now := time.Now().UTC().Format(time.RFC3339)
	if event.Event != "tool_call" {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "not a tool_call event", Timestamp: now}
	}
	if event.LatencyMS <= g.MaxMS {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: fmt.Sprintf("%.0fms <= %.0fms threshold", event.LatencyMS, g.MaxMS), Timestamp: now}
	}
	score := g.MaxMS / event.LatencyMS
	return GradeResult{GraderName: g.Name(), Pass: false, Score: score, Reason: fmt.Sprintf("%.0fms exceeds %.0fms threshold", event.LatencyMS, g.MaxMS), Timestamp: now}
}

// ErrorRateGrader flags events with errors.
type ErrorRateGrader struct{}

func (g *ErrorRateGrader) Name() string { return "error_rate" }

func (g *ErrorRateGrader) Grade(event AgentTraceEvent) GradeResult {
	now := time.Now().UTC().Format(time.RFC3339)
	if event.Error == "" && event.Success {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "no error", Timestamp: now}
	}
	if event.Error != "" {
		return GradeResult{GraderName: g.Name(), Pass: false, Score: 0.0, Reason: "error: " + event.Error, Timestamp: now}
	}
	return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "success field false but no error string", Timestamp: now}
}

// CoverageGrader checks test coverage meets minimum.
type CoverageGrader struct {
	MinCoverage float64
}

func (g *CoverageGrader) Name() string { return "coverage" }

func (g *CoverageGrader) Grade(event AgentTraceEvent) GradeResult {
	now := time.Now().UTC().Format(time.RFC3339)
	if event.Event != "test_run" {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "not a test_run event", Timestamp: now}
	}
	if event.Coverage >= g.MinCoverage {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: event.Coverage, Reason: fmt.Sprintf("%.1f%% >= %.1f%% minimum", event.Coverage*100, g.MinCoverage*100), Timestamp: now}
	}
	return GradeResult{GraderName: g.Name(), Pass: false, Score: event.Coverage, Reason: fmt.Sprintf("%.1f%% below %.1f%% minimum", event.Coverage*100, g.MinCoverage*100), Timestamp: now}
}

// LintGrader checks that lint passed.
type LintGrader struct{}

func (g *LintGrader) Name() string { return "lint" }

func (g *LintGrader) Grade(event AgentTraceEvent) GradeResult {
	now := time.Now().UTC().Format(time.RFC3339)
	if event.LintClean == nil {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "no lint data", Timestamp: now}
	}
	if *event.LintClean {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "lint clean", Timestamp: now}
	}
	return GradeResult{GraderName: g.Name(), Pass: false, Score: 0.0, Reason: "lint failures present", Timestamp: now}
}

// TestPassGrader checks that tests passed.
type TestPassGrader struct{}

func (g *TestPassGrader) Name() string { return "test_pass" }

func (g *TestPassGrader) Grade(event AgentTraceEvent) GradeResult {
	now := time.Now().UTC().Format(time.RFC3339)
	if event.TestPass == nil {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "no test data", Timestamp: now}
	}
	if *event.TestPass {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "tests pass", Timestamp: now}
	}
	return GradeResult{GraderName: g.Name(), Pass: false, Score: 0.0, Reason: "test failures", Timestamp: now}
}

// TokenEfficiencyGrader checks tokens used per task.
type TokenEfficiencyGrader struct {
	MaxTokens int
}

func (g *TokenEfficiencyGrader) Name() string { return "token_efficiency" }

func (g *TokenEfficiencyGrader) Grade(event AgentTraceEvent) GradeResult {
	now := time.Now().UTC().Format(time.RFC3339)
	if event.TokensUsed == 0 {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "no token data", Timestamp: now}
	}
	if event.TokensUsed <= g.MaxTokens {
		score := 1.0 - (float64(event.TokensUsed) / float64(g.MaxTokens))
		return GradeResult{GraderName: g.Name(), Pass: true, Score: score, Reason: fmt.Sprintf("%d tokens <= %d max", event.TokensUsed, g.MaxTokens), Timestamp: now}
	}
	return GradeResult{GraderName: g.Name(), Pass: false, Score: 0.0, Reason: fmt.Sprintf("%d tokens exceeds %d max", event.TokensUsed, g.MaxTokens), Timestamp: now}
}

// CommitMessageGrader checks conventional commit compliance.
type CommitMessageGrader struct {
	RequireConventional bool
}

func (g *CommitMessageGrader) Name() string { return "commit_message" }

func (g *CommitMessageGrader) Grade(event AgentTraceEvent) GradeResult {
	now := time.Now().UTC().Format(time.RFC3339)
	if event.Event != "commit" {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "not a commit event", Timestamp: now}
	}

	var msg string
	if err := json.Unmarshal(event.Raw, &msg); err != nil {
		msg = strings.Trim(string(event.Raw), "\"")
	}
	if !g.RequireConventional {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "conventional commits not required", Timestamp: now}
	}

	if isConventionalCommit(msg) {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "conventional commit format", Timestamp: now}
	}
	return GradeResult{GraderName: g.Name(), Pass: false, Score: 0.0, Reason: "not conventional commit format", Timestamp: now}
}

func isConventionalCommit(msg string) bool {
	prefixes := []string{"feat", "fix", "docs", "style", "refactor", "test", "chore", "perf", "ci", "build", "revert"}
	msg = strings.TrimSpace(msg)
	for _, p := range prefixes {
		if strings.HasPrefix(msg, p+"(") || strings.HasPrefix(msg, p+":") {
			return true
		}
	}
	return false
}

// ToolCoverageGrader checks what percentage of available tools were exercised.
type ToolCoverageGrader struct {
	MinCoverage float64
}

func (g *ToolCoverageGrader) Name() string { return "tool_coverage" }

func (g *ToolCoverageGrader) Grade(event AgentTraceEvent) GradeResult {
	now := time.Now().UTC().Format(time.RFC3339)
	if event.Event != "session_summary" && event.ToolsTotal == 0 {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "no tool coverage data", Timestamp: now}
	}
	if event.ToolsTotal == 0 {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "no tools registered", Timestamp: now}
	}
	ratio := float64(event.ToolsExercised) / float64(event.ToolsTotal)
	if ratio >= g.MinCoverage {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: ratio, Reason: fmt.Sprintf("%.0f%% tools exercised >= %.0f%% minimum", ratio*100, g.MinCoverage*100), Timestamp: now}
	}
	return GradeResult{GraderName: g.Name(), Pass: false, Score: ratio, Reason: fmt.Sprintf("%.0f%% tools exercised < %.0f%% minimum", ratio*100, g.MinCoverage*100), Timestamp: now}
}

// CompletionRateGrader checks task completion percentage.
type CompletionRateGrader struct {
	MinRate float64
}

func (g *CompletionRateGrader) Name() string { return "completion_rate" }

func (g *CompletionRateGrader) Grade(event AgentTraceEvent) GradeResult {
	now := time.Now().UTC().Format(time.RFC3339)
	if event.Event != "session_summary" && event.TasksTotal == 0 {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "no task completion data", Timestamp: now}
	}
	if event.TasksTotal == 0 {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "no tasks assigned", Timestamp: now}
	}
	rate := float64(event.TasksCompleted) / float64(event.TasksTotal)
	if rate >= g.MinRate {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: rate, Reason: fmt.Sprintf("%.0f%% completed >= %.0f%% minimum", rate*100, g.MinRate*100), Timestamp: now}
	}
	return GradeResult{GraderName: g.Name(), Pass: false, Score: rate, Reason: fmt.Sprintf("%.0f%% completed < %.0f%% minimum", rate*100, g.MinRate*100), Timestamp: now}
}

// RegressionGrader detects score regressions against a baseline.
type RegressionGrader struct {
	MaxDelta float64
}

func (g *RegressionGrader) Name() string { return "regression" }

func (g *RegressionGrader) Grade(event AgentTraceEvent) GradeResult {
	now := time.Now().UTC().Format(time.RFC3339)
	if event.Event != "eval_comparison" && event.BaselineScore == 0 {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "no baseline data", Timestamp: now}
	}
	if event.BaselineScore == 0 {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "no baseline established", Timestamp: now}
	}
	delta := event.BaselineScore - event.CurrentScore
	if delta <= g.MaxDelta {
		score := 1.0
		if delta > 0 {
			score = 1.0 - (delta / event.BaselineScore)
		}
		return GradeResult{GraderName: g.Name(), Pass: true, Score: score, Reason: fmt.Sprintf("delta %.2f within %.2f threshold", delta, g.MaxDelta), Timestamp: now}
	}
	score := 1.0 - (delta / event.BaselineScore)
	if score < 0 {
		score = 0
	}
	return GradeResult{GraderName: g.Name(), Pass: false, Score: score, Reason: fmt.Sprintf("regression delta %.2f exceeds %.2f threshold (baseline=%.2f, current=%.2f)", delta, g.MaxDelta, event.BaselineScore, event.CurrentScore), Timestamp: now}
}

// AllGraders returns all deterministic graders with the given config.
func AllGraders(cfg GraderConfig) []DeterministicGrader {
	return []DeterministicGrader{
		&LatencyGrader{MaxMS: cfg.MaxP95LatencyMS},
		&ErrorRateGrader{},
		&CoverageGrader{MinCoverage: cfg.MinCoverage},
		&LintGrader{},
		&TestPassGrader{},
		&TokenEfficiencyGrader{MaxTokens: cfg.MaxTokensPerTask},
		&ToolCoverageGrader{MinCoverage: cfg.MinToolCoverage},
		&CompletionRateGrader{MinRate: cfg.MinCompletionRate},
		&RegressionGrader{MaxDelta: cfg.MaxRegressionDelta},
	}
}

// ADR065Graders returns the 6 graders specified in ADR-065 for gate evaluation.
func ADR065Graders(cfg GraderConfig) []DeterministicGrader {
	return []DeterministicGrader{
		&LatencyGrader{MaxMS: cfg.MaxP95LatencyMS},
		&ErrorRateGrader{},
		&ToolCoverageGrader{MinCoverage: cfg.MinToolCoverage},
		&TokenEfficiencyGrader{MaxTokens: cfg.MaxTokensPerTask},
		&CompletionRateGrader{MinRate: cfg.MinCompletionRate},
		&RegressionGrader{MaxDelta: cfg.MaxRegressionDelta},
	}
}
