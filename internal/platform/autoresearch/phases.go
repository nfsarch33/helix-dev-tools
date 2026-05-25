package autoresearch

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ProbeResult captures a single pattern discovered during the Probe phase.
type ProbeResult struct {
	Pattern   string `json:"pattern"`
	Count     int    `json:"count"`
	Source    string `json:"source"`
	Severity string `json:"severity"` // "info", "warning", "error"
}

// Hypothesis is a structured experiment proposal from the Propose phase.
type Hypothesis struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"` // "performance", "reliability", "quality"
	Priority    int    `json:"priority"` // 1=highest
	Baseline    string `json:"baseline"`
	Expected    string `json:"expected"`
}

// EvalResult captures the outcome of running one experiment.
type EvalResult struct {
	HypothesisID string  `json:"hypothesis_id"`
	Metric       float64 `json:"metric"`
	BaseMetric   float64 `json:"base_metric"`
	Delta        float64 `json:"delta"`
	DurationMS   int64   `json:"duration_ms"`
	Passed       bool    `json:"passed"`
}

// PromotionRecord captures what was promoted and where.
type PromotionRecord struct {
	HypothesisID string    `json:"hypothesis_id"`
	Target       string    `json:"target"` // "engram", "file", "evospine"
	Timestamp    time.Time `json:"timestamp"`
	Detail       string    `json:"detail"`
}

// DefaultDecideThreshold is the minimum positive delta required to accept.
const DefaultDecideThreshold = 0.0

// ProbeConfig controls the Probe phase data source.
type ProbeConfig struct {
	AgentracePaths []string
	MaxEntries     int
}

// DefaultProbeConfig returns defaults for agentrace log locations.
func DefaultProbeConfig() ProbeConfig {
	home, _ := os.UserHomeDir()
	return ProbeConfig{
		AgentracePaths: []string{
			filepath.Join(home, "logs", "runx", "agentrace-autoresearch.ndjson"),
			filepath.Join(home, "logs", "runx", "agentrace.ndjson"),
		},
		MaxEntries: 500,
	}
}

// NewProbePhase returns a PhaseFunc that reads agentrace NDJSON files,
// extracts error and pattern data, and stores top patterns in the LoopState.
func NewProbePhase(pcfg ProbeConfig) PhaseFunc {
	return func(ctx context.Context, state LoopState) (LoopState, error) {
		patterns := make(map[string]int)

		for _, path := range pcfg.AgentracePaths {
			if err := ctx.Err(); err != nil {
				return state, err
			}
			entries, err := readNDJSON(path, pcfg.MaxEntries)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				extractPatterns(entry, patterns)
			}
		}

		if len(patterns) == 0 {
			state.Note = "probe: no agentrace data found"
			return state, nil
		}

		results := rankPatterns(patterns, 5)
		summary, err := json.Marshal(results)
		if err != nil {
			return state, fmt.Errorf("marshal probe results: %w", err)
		}
		state.Note = string(summary)
		state.Metric = float64(len(results))
		return state, nil
	}
}

// NewProposePhase returns a PhaseFunc that generates experiment hypotheses
// from probe results. It reads the Note field from the previous probe
// and formats structured hypotheses.
func NewProposePhase() PhaseFunc {
	return func(ctx context.Context, state LoopState) (LoopState, error) {
		var probeResults []ProbeResult
		if state.Note != "" {
			_ = json.Unmarshal([]byte(state.Note), &probeResults)
		}

		hypotheses := generateHypotheses(probeResults)
		if len(hypotheses) == 0 {
			state.Note = "propose: no hypotheses generated"
			return state, nil
		}

		data, err := json.Marshal(hypotheses)
		if err != nil {
			return state, fmt.Errorf("marshal hypotheses: %w", err)
		}
		state.Note = string(data)
		state.Metric = float64(len(hypotheses))
		return state, nil
	}
}

// NewEvaluatePhase returns a PhaseFunc that evaluates hypotheses.
// evaluator is called for each hypothesis; if nil, a simple score estimator
// is used.
func NewEvaluatePhase(evaluator func(ctx context.Context, h Hypothesis) (EvalResult, error)) PhaseFunc {
	if evaluator == nil {
		evaluator = defaultEvaluator
	}
	return func(ctx context.Context, state LoopState) (LoopState, error) {
		var hypotheses []Hypothesis
		if state.Note != "" {
			_ = json.Unmarshal([]byte(state.Note), &hypotheses)
		}
		if len(hypotheses) == 0 {
			state.Note = "evaluate: no hypotheses to evaluate"
			return state, nil
		}

		best := EvalResult{Delta: math.Inf(-1)}
		for _, h := range hypotheses {
			if err := ctx.Err(); err != nil {
				return state, err
			}
			result, err := evaluator(ctx, h)
			if err != nil {
				continue
			}
			if result.Delta > best.Delta {
				best = result
			}
		}

		if math.IsInf(best.Delta, -1) {
			state.Note = "evaluate: all hypotheses failed evaluation"
			return state, nil
		}

		state.Metric = best.Metric
		state.BaseMetric = best.BaseMetric
		state.Delta = best.Delta

		data, err := json.Marshal(best)
		if err != nil {
			return state, fmt.Errorf("marshal eval result: %w", err)
		}
		state.Note = string(data)
		return state, nil
	}
}

// NewDecidePhase returns a PhaseFunc that accepts or rejects based on
// whether the metric delta exceeds the threshold.
func NewDecidePhase(threshold float64) PhaseFunc {
	return func(_ context.Context, state LoopState) (LoopState, error) {
		if state.Delta > threshold {
			state.Decision = DecisionKeep
			state.Note = fmt.Sprintf("decide: accepted (delta=%.4f > threshold=%.4f)", state.Delta, threshold)
		} else {
			state.Decision = DecisionDiscard
			state.Note = fmt.Sprintf("decide: rejected (delta=%.4f <= threshold=%.4f)", state.Delta, threshold)
		}
		return state, nil
	}
}

// PromoteConfig controls where successful patterns are promoted.
type PromoteConfig struct {
	Engram     *EngramClient
	OutputDir  string // fallback file output for findings
	EvoSpineLog string // agentrace NDJSON for EvoSpine consumption
}

// NewPromotePhase returns a PhaseFunc that promotes accepted results
// to Engram and/or filesystem. Skipped for DISCARD decisions.
func NewPromotePhase(pcfg PromoteConfig) PhaseFunc {
	return func(ctx context.Context, state LoopState) (LoopState, error) {
		if state.Decision != DecisionKeep {
			state.Note = "promote: skipped (decision != keep)"
			return state, nil
		}

		finding := fmt.Sprintf(
			"autoresearch iteration=%d metric=%.4f base=%.4f delta=%.4f: %s",
			state.Iteration, state.Metric, state.BaseMetric, state.Delta, state.Note,
		)

		var promoted []string

		if pcfg.Engram != nil {
			meta := map[string]string{
				"kind":      "research_finding",
				"iteration": fmt.Sprintf("%d", state.Iteration),
				"delta":     fmt.Sprintf("%.4f", state.Delta),
			}
			if err := pcfg.Engram.StoreResearch(ctx, finding, meta); err != nil {
				// Non-fatal: log and continue
				state.Note = fmt.Sprintf("promote: engram store failed: %v", err)
			} else {
				promoted = append(promoted, "engram")
			}
		}

		if pcfg.OutputDir != "" {
			if err := writePromotionFile(pcfg.OutputDir, state, finding); err != nil {
				return state, fmt.Errorf("promote file write: %w", err)
			}
			promoted = append(promoted, "file:"+pcfg.OutputDir)
		}

		if pcfg.EvoSpineLog != "" {
			if err := SentruxPlugin(
				"autoresearch", state.Iteration,
				state.Metric, state.BaseMetric,
				"", pcfg.EvoSpineLog,
			); err != nil {
				return state, fmt.Errorf("promote evospine: %w", err)
			}
			promoted = append(promoted, "evospine")
		}

		if len(promoted) == 0 {
			state.Note = "promote: no targets configured"
		} else {
			state.Note = fmt.Sprintf("promote: %s", strings.Join(promoted, ", "))
		}
		return state, nil
	}
}

// --- internal helpers ---

func readNDJSON(path string, maxLines int) ([]map[string]interface{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []map[string]interface{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() && len(entries) < maxLines {
		var entry map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func extractPatterns(entry map[string]interface{}, patterns map[string]int) {
	if errVal, ok := entry["error"]; ok {
		if errStr, ok := errVal.(string); ok && errStr != "" {
			key := normalizeError(errStr)
			patterns[key]++
		}
	}

	if phase, ok := entry["phase"].(string); ok {
		if decision, ok := entry["decision"].(string); ok && decision == string(DecisionDiscard) {
			patterns["discard_in_"+phase]++
		}
	}

	if note, ok := entry["note"].(string); ok && note != "" {
		if strings.Contains(strings.ToLower(note), "timeout") {
			patterns["timeout_detected"]++
		}
		if strings.Contains(strings.ToLower(note), "fail") {
			patterns["failure_detected"]++
		}
	}
}

func normalizeError(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 80 {
		s = s[:80]
	}
	return "error:" + strings.ReplaceAll(s, "\n", " ")
}

func rankPatterns(patterns map[string]int, top int) []ProbeResult {
	type kv struct {
		key   string
		count int
	}
	var sorted []kv
	for k, v := range patterns {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	if top > len(sorted) {
		top = len(sorted)
	}

	results := make([]ProbeResult, top)
	for i := 0; i < top; i++ {
		severity := "info"
		if strings.HasPrefix(sorted[i].key, "error:") {
			severity = "error"
		} else if strings.Contains(sorted[i].key, "timeout") || strings.Contains(sorted[i].key, "failure") {
			severity = "warning"
		}
		results[i] = ProbeResult{
			Pattern:  sorted[i].key,
			Count:    sorted[i].count,
			Source:   "agentrace",
			Severity: severity,
		}
	}
	return results
}

func generateHypotheses(results []ProbeResult) []Hypothesis {
	var hypotheses []Hypothesis
	for i, r := range results {
		if i >= 3 {
			break
		}
		h := Hypothesis{
			ID:       fmt.Sprintf("h-%d-%d", time.Now().Unix(), i+1),
			Category: categorize(r),
			Priority: i + 1,
			Baseline: fmt.Sprintf("current occurrence count: %d", r.Count),
			Expected: "reduce occurrence by 50%",
		}
		switch {
		case strings.HasPrefix(r.Pattern, "error:"):
			h.Title = fmt.Sprintf("Fix recurring error: %s", strings.TrimPrefix(r.Pattern, "error:"))
			h.Description = fmt.Sprintf(
				"Error pattern appears %d times. Investigate root cause and apply fix.",
				r.Count,
			)
		case strings.Contains(r.Pattern, "timeout"):
			h.Title = "Increase timeout budget or optimize slow phase"
			h.Description = fmt.Sprintf(
				"Timeout pattern detected %d times. Consider increasing budget or optimizing.",
				r.Count,
			)
		default:
			h.Title = fmt.Sprintf("Address pattern: %s", r.Pattern)
			h.Description = fmt.Sprintf(
				"Pattern '%s' observed %d times. Investigate and mitigate.",
				r.Pattern, r.Count,
			)
		}
		hypotheses = append(hypotheses, h)
	}
	return hypotheses
}

func categorize(r ProbeResult) string {
	switch {
	case strings.HasPrefix(r.Pattern, "error:"):
		return "reliability"
	case strings.Contains(r.Pattern, "timeout"):
		return "performance"
	default:
		return "quality"
	}
}

func defaultEvaluator(_ context.Context, h Hypothesis) (EvalResult, error) {
	score := 0.0
	switch h.Category {
	case "reliability":
		score = 0.8
	case "performance":
		score = 0.6
	case "quality":
		score = 0.5
	}
	base := 0.3
	return EvalResult{
		HypothesisID: h.ID,
		Metric:       score,
		BaseMetric:   base,
		Delta:        score - base,
		DurationMS:   100,
		Passed:       score > base,
	}, nil
}

func writePromotionFile(dir string, state LoopState, finding string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	filename := fmt.Sprintf("research-%d-%s.json", state.Iteration, time.Now().Format("20060102T150405"))
	path := filepath.Join(dir, filename)

	record := PromotionRecord{
		HypothesisID: fmt.Sprintf("iter-%d", state.Iteration),
		Target:       "file",
		Timestamp:    time.Now(),
		Detail:       finding,
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o640)
}
