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
	Timestamp  string          `json:"ts"`
	Event      string          `json:"event"`
	Tool       string          `json:"tool,omitempty"`
	LatencyMS  float64         `json:"latency_ms,omitempty"`
	Success    bool            `json:"success,omitempty"`
	Error      string          `json:"error,omitempty"`
	TokensUsed int             `json:"tokens_used,omitempty"`
	Coverage   float64         `json:"coverage,omitempty"`
	LintClean  *bool           `json:"lint_clean,omitempty"`
	TestPass   *bool           `json:"test_pass,omitempty"`
	Raw        json.RawMessage `json:"raw,omitempty"`
}

// GraderConfig holds thresholds for all deterministic graders.
type GraderConfig struct {
	MaxP95LatencyMS   float64 `json:"max_p95_latency_ms"`
	MaxErrorRate      float64 `json:"max_error_rate"`
	MinCoverage       float64 `json:"min_coverage"`
	MaxTokensPerTask  int     `json:"max_tokens_per_task"`
	RequireConventional bool  `json:"require_conventional"`
}

// DefaultGraderConfig returns sensible defaults per ADR-065.
func DefaultGraderConfig() GraderConfig {
	return GraderConfig{
		MaxP95LatencyMS:    5000,
		MaxErrorRate:       0.05,
		MinCoverage:        0.70,
		MaxTokensPerTask:   50000,
		RequireConventional: true,
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

	raw := string(event.Raw)
	if !g.RequireConventional {
		return GradeResult{GraderName: g.Name(), Pass: true, Score: 1.0, Reason: "conventional commits not required", Timestamp: now}
	}

	if isConventionalCommit(raw) {
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

// AllGraders returns the 6 deterministic graders with the given config.
func AllGraders(cfg GraderConfig) []DeterministicGrader {
	return []DeterministicGrader{
		&LatencyGrader{MaxMS: cfg.MaxP95LatencyMS},
		&ErrorRateGrader{},
		&CoverageGrader{MinCoverage: cfg.MinCoverage},
		&LintGrader{},
		&TestPassGrader{},
		&TokenEfficiencyGrader{MaxTokens: cfg.MaxTokensPerTask},
	}
}
