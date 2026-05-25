package autoresearch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTestAgentrace(t *testing.T, dir string, entries []map[string]interface{}) string {
	t.Helper()
	path := filepath.Join(dir, "agentrace.ndjson")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create test agentrace: %v", err)
	}
	defer f.Close()
	for _, e := range entries {
		line, _ := json.Marshal(e)
		fmt.Fprintf(f, "%s\n", line)
	}
	return path
}

func TestProbePhaseExtractsPatterns(t *testing.T) {
	dir := t.TempDir()
	entries := []map[string]interface{}{
		{"phase": "evaluate", "error": "connection timeout", "note": "timeout during eval"},
		{"phase": "evaluate", "error": "connection timeout", "note": ""},
		{"phase": "propose", "error": "", "note": "all good"},
		{"phase": "decide", "decision": "discard", "note": "rejected"},
		{"phase": "decide", "decision": "discard", "note": "rejected again"},
	}
	path := writeTestAgentrace(t, dir, entries)

	pcfg := ProbeConfig{
		AgentracePaths: []string{path},
		MaxEntries:     100,
	}

	probe := NewProbePhase(pcfg)
	state := LoopState{Iteration: 1, Timestamp: time.Now()}
	result, err := probe(context.Background(), state)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}

	var patterns []ProbeResult
	if err := json.Unmarshal([]byte(result.Note), &patterns); err != nil {
		t.Fatalf("unmarshal probe results: %v (note=%q)", err, result.Note)
	}
	if len(patterns) == 0 {
		t.Fatal("expected at least one pattern")
	}

	found := false
	for _, p := range patterns {
		if p.Pattern == "error:connection timeout" && p.Count == 2 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'error:connection timeout' with count=2, got %+v", patterns)
	}
}

func TestProbePhaseNoData(t *testing.T) {
	pcfg := ProbeConfig{
		AgentracePaths: []string{"/nonexistent/path.ndjson"},
		MaxEntries:     100,
	}
	probe := NewProbePhase(pcfg)
	state := LoopState{Iteration: 1, Timestamp: time.Now()}
	result, err := probe(context.Background(), state)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if result.Note != "probe: no agentrace data found" {
		t.Errorf("expected no-data note, got %q", result.Note)
	}
}

func TestProposePhaseGeneratesHypotheses(t *testing.T) {
	probeResults := []ProbeResult{
		{Pattern: "error:connection timeout", Count: 5, Source: "agentrace", Severity: "error"},
		{Pattern: "timeout_detected", Count: 3, Source: "agentrace", Severity: "warning"},
	}
	probeJSON, _ := json.Marshal(probeResults)

	propose := NewProposePhase()
	state := LoopState{Iteration: 1, Note: string(probeJSON), Timestamp: time.Now()}
	result, err := propose(context.Background(), state)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}

	var hypotheses []Hypothesis
	if err := json.Unmarshal([]byte(result.Note), &hypotheses); err != nil {
		t.Fatalf("unmarshal hypotheses: %v", err)
	}
	if len(hypotheses) == 0 {
		t.Fatal("expected at least one hypothesis")
	}
	if hypotheses[0].Category != "reliability" {
		t.Errorf("first hypothesis should be reliability, got %q", hypotheses[0].Category)
	}
}

func TestProposePhaseEmptyInput(t *testing.T) {
	propose := NewProposePhase()
	state := LoopState{Iteration: 1, Note: "", Timestamp: time.Now()}
	result, err := propose(context.Background(), state)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if result.Note != "propose: no hypotheses generated" {
		t.Errorf("expected no-hypotheses note, got %q", result.Note)
	}
}

func TestEvaluatePhaseDefaultEvaluator(t *testing.T) {
	hypotheses := []Hypothesis{
		{ID: "h1", Title: "Fix timeout", Category: "performance", Priority: 1},
		{ID: "h2", Title: "Fix error", Category: "reliability", Priority: 2},
	}
	hJSON, _ := json.Marshal(hypotheses)

	evaluate := NewEvaluatePhase(nil)
	state := LoopState{Iteration: 1, Note: string(hJSON), Timestamp: time.Now()}
	result, err := evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if result.Delta <= 0 {
		t.Errorf("expected positive delta, got %.4f", result.Delta)
	}
	if result.Metric <= result.BaseMetric {
		t.Errorf("metric (%.4f) should exceed base (%.4f)", result.Metric, result.BaseMetric)
	}
}

func TestEvaluatePhaseCustomEvaluator(t *testing.T) {
	hypotheses := []Hypothesis{
		{ID: "h1", Title: "Test", Category: "quality", Priority: 1},
	}
	hJSON, _ := json.Marshal(hypotheses)

	customEval := func(_ context.Context, h Hypothesis) (EvalResult, error) {
		return EvalResult{
			HypothesisID: h.ID,
			Metric:       0.95,
			BaseMetric:   0.80,
			Delta:        0.15,
			DurationMS:   50,
			Passed:       true,
		}, nil
	}

	evaluate := NewEvaluatePhase(customEval)
	state := LoopState{Iteration: 1, Note: string(hJSON), Timestamp: time.Now()}
	result, err := evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Delta != 0.15 {
		t.Errorf("expected delta=0.15, got %.4f", result.Delta)
	}
}

func TestEvaluatePhaseNoHypotheses(t *testing.T) {
	evaluate := NewEvaluatePhase(nil)
	state := LoopState{Iteration: 1, Note: "[]", Timestamp: time.Now()}
	result, err := evaluate(context.Background(), state)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if result.Note != "evaluate: no hypotheses to evaluate" {
		t.Errorf("expected no-hypotheses note, got %q", result.Note)
	}
}

func TestDecidePhaseAccepts(t *testing.T) {
	decide := NewDecidePhase(0.1)
	state := LoopState{Iteration: 1, Delta: 0.5, Timestamp: time.Now()}
	result, err := decide(context.Background(), state)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if result.Decision != DecisionKeep {
		t.Errorf("expected keep, got %q", result.Decision)
	}
}

func TestDecidePhaseRejects(t *testing.T) {
	decide := NewDecidePhase(0.5)
	state := LoopState{Iteration: 1, Delta: 0.1, Timestamp: time.Now()}
	result, err := decide(context.Background(), state)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if result.Decision != DecisionDiscard {
		t.Errorf("expected discard, got %q", result.Decision)
	}
}

func TestDecidePhaseZeroThreshold(t *testing.T) {
	decide := NewDecidePhase(0.0)
	state := LoopState{Iteration: 1, Delta: 0.001, Timestamp: time.Now()}
	result, err := decide(context.Background(), state)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if result.Decision != DecisionKeep {
		t.Errorf("expected keep with positive delta, got %q", result.Decision)
	}
}

func TestPromotePhaseFile(t *testing.T) {
	dir := t.TempDir()
	pcfg := PromoteConfig{OutputDir: dir}
	promote := NewPromotePhase(pcfg)
	state := LoopState{
		Iteration: 1,
		Decision:  DecisionKeep,
		Metric:    0.9,
		BaseMetric: 0.5,
		Delta:     0.4,
		Timestamp: time.Now(),
	}
	result, err := promote(context.Background(), state)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if result.Note == "" {
		t.Error("expected non-empty note")
	}

	matches, _ := filepath.Glob(filepath.Join(dir, "research-*.json"))
	if len(matches) != 1 {
		t.Errorf("expected 1 promotion file, got %d", len(matches))
	}
}

func TestPromotePhaseSkipsDiscard(t *testing.T) {
	promote := NewPromotePhase(PromoteConfig{OutputDir: t.TempDir()})
	state := LoopState{
		Iteration: 1,
		Decision:  DecisionDiscard,
		Timestamp: time.Now(),
	}
	result, err := promote(context.Background(), state)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if result.Note != "promote: skipped (decision != keep)" {
		t.Errorf("expected skip note, got %q", result.Note)
	}

	matches, _ := filepath.Glob(filepath.Join(t.TempDir(), "research-*.json"))
	if len(matches) != 0 {
		t.Errorf("expected no promotion file for discard, got %d", len(matches))
	}
}

func TestPromotePhaseEvoSpineLog(t *testing.T) {
	dir := t.TempDir()
	evospinePath := filepath.Join(dir, "sentrux.ndjson")
	pcfg := PromoteConfig{EvoSpineLog: evospinePath}
	promote := NewPromotePhase(pcfg)
	state := LoopState{
		Iteration:  1,
		Decision:   DecisionKeep,
		Metric:     0.85,
		BaseMetric: 0.60,
		Delta:      0.25,
		Timestamp:  time.Now(),
	}
	result, err := promote(context.Background(), state)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if result.Note == "" {
		t.Error("expected non-empty note")
	}
	if _, err := os.Stat(evospinePath); err != nil {
		t.Errorf("evospine log not created: %v", err)
	}
}

func TestReadNDJSON(t *testing.T) {
	dir := t.TempDir()
	entries := []map[string]interface{}{
		{"phase": "probe", "note": "test"},
		{"phase": "propose", "note": "test2"},
	}
	path := writeTestAgentrace(t, dir, entries)

	result, err := readNDJSON(path, 100)
	if err != nil {
		t.Fatalf("readNDJSON: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result))
	}
}

func TestReadNDJSONMaxLines(t *testing.T) {
	dir := t.TempDir()
	var entries []map[string]interface{}
	for i := 0; i < 10; i++ {
		entries = append(entries, map[string]interface{}{"i": i})
	}
	path := writeTestAgentrace(t, dir, entries)

	result, err := readNDJSON(path, 3)
	if err != nil {
		t.Fatalf("readNDJSON: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 entries (capped), got %d", len(result))
	}
}

func TestNormalizeError(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple error", "error:simple error"},
		{"line1\nline2", "error:line1 line2"},
		{
			"a very long error message that exceeds eighty characters and should be truncated at the boundary",
			"error:a very long error message that exceeds eighty characters and should be tru",
		},
	}
	for _, tc := range tests {
		got := normalizeError(tc.input)
		if got != tc.want {
			t.Errorf("normalizeError(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestRankPatterns(t *testing.T) {
	patterns := map[string]int{
		"error:timeout": 10,
		"error:oom":     3,
		"info:success":  50,
	}
	results := rankPatterns(patterns, 2)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Pattern != "info:success" {
		t.Errorf("expected top pattern 'info:success', got %q", results[0].Pattern)
	}
	if results[0].Count != 50 {
		t.Errorf("expected count=50, got %d", results[0].Count)
	}
}

func TestGenerateHypotheses(t *testing.T) {
	results := []ProbeResult{
		{Pattern: "error:timeout", Count: 5, Severity: "error"},
		{Pattern: "timeout_detected", Count: 3, Severity: "warning"},
		{Pattern: "discard_in_decide", Count: 2, Severity: "info"},
		{Pattern: "extra_pattern", Count: 1, Severity: "info"},
	}
	hypotheses := generateHypotheses(results)
	if len(hypotheses) != 3 {
		t.Errorf("expected 3 hypotheses (capped), got %d", len(hypotheses))
	}
	if hypotheses[0].Category != "reliability" {
		t.Errorf("first hypothesis should be reliability, got %q", hypotheses[0].Category)
	}
	if hypotheses[1].Category != "performance" {
		t.Errorf("second hypothesis should be performance, got %q", hypotheses[1].Category)
	}
}

func TestCategorize(t *testing.T) {
	tests := []struct {
		pattern string
		want    string
	}{
		{"timeout_detected", "performance"},
		{"error:connection refused", "reliability"},
		{"info:general", "quality"},
	}
	for _, tc := range tests {
		got := categorize(ProbeResult{Pattern: tc.pattern})
		if got != tc.want {
			t.Errorf("categorize(%q) = %q, want %q", tc.pattern, got, tc.want)
		}
	}
}

func TestExtractPatterns(t *testing.T) {
	patterns := make(map[string]int)

	entry := map[string]interface{}{
		"error":    "connection refused",
		"phase":    "decide",
		"decision": "discard",
		"note":     "timeout detected in evaluation",
	}
	extractPatterns(entry, patterns)

	if patterns["error:connection refused"] != 1 {
		t.Errorf("expected error pattern count=1, got %d", patterns["error:connection refused"])
	}
	if patterns["discard_in_decide"] != 1 {
		t.Errorf("expected discard_in_decide count=1, got %d", patterns["discard_in_decide"])
	}
	if patterns["timeout_detected"] != 1 {
		t.Errorf("expected timeout_detected count=1, got %d", patterns["timeout_detected"])
	}
}
