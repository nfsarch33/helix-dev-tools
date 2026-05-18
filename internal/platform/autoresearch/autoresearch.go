// Package autoresearch implements the 5-phase self-improvement loop:
// Probe -> Propose -> Evaluate -> Decide -> Promote.
// Each phase has a hard time budget; phases write to agentrace NDJSON and
// Mem0 OSS for cross-session pattern persistence.
package autoresearch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Phase names
const (
	PhaseProbe    = "probe"
	PhasePropose  = "propose"
	PhaseEvaluate = "evaluate"
	PhaseDecide   = "decide"
	PhasePromote  = "promote"
)

// Decision from the Decide phase.
type Decision string

const (
	DecisionKeep    Decision = "keep"
	DecisionDiscard Decision = "discard"
)

// Config controls loop budgets and output paths.
type Config struct {
	// MaxIterations caps the number of KEEP/DISCARD cycles.
	MaxIterations int
	// ProbeBudget is the wall-clock budget for the Probe phase.
	ProbeBudget time.Duration
	// ProposeBudget is the budget for LLM-driven proposal generation.
	ProposeBudget time.Duration
	// EvaluateBudget is the budget for running the metric suite.
	EvaluateBudget time.Duration
	// AgentID identifies the running agent in agentrace records.
	AgentID string
	// LogPath is the agentrace NDJSON output file.
	// Defaults to ~/logs/runx/agentrace-autoresearch.ndjson.
	LogPath string
}

// DefaultConfig returns a sensible default Config.
func DefaultConfig() Config {
	return Config{
		MaxIterations:  5,
		ProbeBudget:    30 * time.Second,
		ProposeBudget:  60 * time.Second,
		EvaluateBudget: 300 * time.Second,
		AgentID:        "autoresearch",
	}
}

// LoopState captures the result of one full research cycle.
type LoopState struct {
	Iteration  int       `json:"iteration"`
	Phase      string    `json:"phase"`
	Decision   Decision  `json:"decision,omitempty"`
	Metric     float64   `json:"metric,omitempty"`
	BaseMetric float64   `json:"base_metric,omitempty"`
	Delta      float64   `json:"delta,omitempty"`
	Note       string    `json:"note,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// PhaseFunc is a function implementing one research phase.
// It receives the current LoopState and returns an updated state.
type PhaseFunc func(ctx context.Context, state LoopState) (LoopState, error)

// Runner orchestrates the 5-phase loop.
type Runner struct {
	cfg      Config
	probe    PhaseFunc
	propose  PhaseFunc
	evaluate PhaseFunc
	decide   PhaseFunc
	promote  PhaseFunc
}

// New returns a Runner with the provided phase implementations.
// Pass nil for any phase to use the default no-op stub.
func New(cfg Config, probe, propose, evaluate, decide, promote PhaseFunc) *Runner {
	return &Runner{
		cfg:      cfg,
		probe:    orNoop(probe),
		propose:  orNoop(propose),
		evaluate: orNoop(evaluate),
		decide:   orNoop(decide),
		promote:  orNoop(promote),
	}
}

// Run executes the loop for up to cfg.MaxIterations cycles.
func (r *Runner) Run(ctx context.Context) ([]LoopState, error) {
	var history []LoopState

	for i := 0; i < r.cfg.MaxIterations; i++ {
		state := LoopState{Iteration: i + 1, Timestamp: time.Now()}

		phases := []struct {
			name   string
			budget time.Duration
			fn     PhaseFunc
		}{
			{PhaseProbe, r.cfg.ProbeBudget, r.probe},
			{PhasePropose, r.cfg.ProposeBudget, r.propose},
			{PhaseEvaluate, r.cfg.EvaluateBudget, r.evaluate},
			{PhaseDecide, 10 * time.Second, r.decide},
			{PhasePromote, 30 * time.Second, r.promote},
		}

		for _, p := range phases {
			if err := ctx.Err(); err != nil {
				return history, err
			}
			pCtx, cancel := context.WithTimeout(ctx, p.budget)
			state.Phase = p.name
			var err error
			state, err = p.fn(pCtx, state)
			cancel()
			if err != nil {
				r.appendLog(state, err.Error())
				return history, fmt.Errorf("phase %s iter %d: %w", p.name, i+1, err)
			}
			r.appendLog(state, "")
		}

		history = append(history, state)

		// Stop iterating after a DISCARD -- the proposed change was not useful.
		if state.Decision == DecisionDiscard {
			break
		}
	}

	return history, nil
}

// appendLog writes a LoopState record to the agentrace NDJSON log.
// Errors are non-fatal and printed to stderr only.
func (r *Runner) appendLog(state LoopState, errNote string) {
	logPath := r.cfg.LogPath
	if logPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		logPath = filepath.Join(home, "logs", "runx", "agentrace-autoresearch.ndjson")
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o750); err != nil {
		return
	}

	record := map[string]interface{}{
		"ts":         time.Now().UTC().Format(time.RFC3339),
		"agent_id":   r.cfg.AgentID,
		"iteration":  state.Iteration,
		"phase":      state.Phase,
		"decision":   state.Decision,
		"metric":     state.Metric,
		"base_metric": state.BaseMetric,
		"delta":      state.Delta,
		"note":       state.Note,
	}
	if errNote != "" {
		record["error"] = errNote
	}

	line, err := json.Marshal(record)
	if err != nil {
		return
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\n", line)
}

func orNoop(fn PhaseFunc) PhaseFunc {
	if fn != nil {
		return fn
	}
	return func(_ context.Context, state LoopState) (LoopState, error) {
		return state, nil
	}
}
