package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nfsarch33/cursor-tools/internal/outcomes"
)

func TestOutcomeCmdRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "outcome" {
			found = true
			break
		}
	}
	if !found {
		t.Error("outcome command not registered on rootCmd")
	}
}

func TestOutcomeSubcommandsRegistered(t *testing.T) {
	expected := []string{"emit", "recent"}
	nameSet := map[string]bool{}
	for _, cmd := range outcomeCmd.Commands() {
		nameSet[cmd.Name()] = true
	}
	for _, name := range expected {
		if !nameSet[name] {
			t.Errorf("outcome subcommand %q not registered", name)
		}
	}
}

func TestParseMetaPairs(t *testing.T) {
	m, err := parseMetaPairs([]string{"k=v", "hook=guard-shell", "  spaced = value  "})
	if err != nil {
		t.Fatalf("parseMetaPairs: %v", err)
	}
	if m["k"] != "v" {
		t.Errorf("k=%q", m["k"])
	}
	if m["hook"] != "guard-shell" {
		t.Errorf("hook=%q", m["hook"])
	}
	if m["spaced"] != "value" {
		t.Errorf("spaced=%q", m["spaced"])
	}

	if _, err := parseMetaPairs([]string{"no-equals"}); err == nil {
		t.Error("expected error for bare key")
	}
	if _, err := parseMetaPairs([]string{"=value"}); err == nil {
		t.Error("expected error for empty key")
	}

	if m2, err := parseMetaPairs(nil); err != nil || m2 != nil {
		t.Errorf("expected nil map, got %v/%v", m2, err)
	}
}

func TestRunOutcomeEmit_RequiresEvent(t *testing.T) {
	resetOutcomeFlags(t)
	if err := runOutcomeEmit(nil, nil); err == nil {
		t.Fatal("expected error when --event missing")
	}
}

func TestRunOutcomeEmit_BufferedSink_WritesFile(t *testing.T) {
	resetOutcomeFlags(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "outcomes.ndjson")
	t.Setenv("CURSOR_TOOLS_OUTCOMES_PATH", path)
	t.Setenv("CURSOR_TOOLS_OUTCOMES_DISABLED", "")
	outcomes.ResetDefaultEmitter()

	outcomeFlags.event = "test:unit"
	outcomeFlags.actor = outcomes.ActorCursorTools
	outcomeFlags.detail = "hello from outcome_test"
	outcomeFlags.latencyMs = 42
	outcomeFlags.sink = "buffered"
	outcomeFlags.meta = []string{"category=test", "component=cli"}

	if err := runOutcomeEmit(nil, nil); err != nil {
		t.Fatalf("runOutcomeEmit: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading buffer file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("buffer file is empty")
	}
	var parsed outcomes.Outcome
	line := strings.TrimSpace(strings.Split(string(data), "\n")[0])
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("unmarshal NDJSON: %v (raw=%q)", err, line)
	}
	if parsed.Event != "test:unit" {
		t.Errorf("Event=%q", parsed.Event)
	}
	if parsed.Actor != outcomes.ActorCursorTools {
		t.Errorf("Actor=%q", parsed.Actor)
	}
	if parsed.Kind != outcomes.KindAgentOutcome {
		t.Errorf("Kind=%q", parsed.Kind)
	}
	if parsed.Meta["category"] != "test" {
		t.Errorf("meta.category=%q", parsed.Meta["category"])
	}
	if parsed.Machine == "" {
		t.Error("Machine must be auto-filled")
	}
}

func TestRunOutcomeEmit_UnknownActor(t *testing.T) {
	resetOutcomeFlags(t)
	outcomeFlags.event = "test:unit"
	outcomeFlags.actor = "martian"
	outcomeFlags.sink = "buffered"
	err := runOutcomeEmit(nil, nil)
	if err == nil {
		t.Fatal("expected error for unknown actor")
	}
	if !strings.Contains(err.Error(), "unknown actor") {
		t.Errorf("err=%v", err)
	}
}

func TestRunOutcomeEmit_InvalidSink(t *testing.T) {
	resetOutcomeFlags(t)
	outcomeFlags.event = "test:unit"
	outcomeFlags.sink = "bogus"
	err := runOutcomeEmit(nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid sink")
	}
	if !strings.Contains(err.Error(), "invalid --sink") {
		t.Errorf("err=%v", err)
	}
}

func TestRunOutcomeEmit_SkillHitFlag(t *testing.T) {
	resetOutcomeFlags(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "outcomes.ndjson")
	t.Setenv("CURSOR_TOOLS_OUTCOMES_PATH", path)
	outcomes.ResetDefaultEmitter()

	outcomeFlags.event = "test:skill"
	outcomeFlags.actor = outcomes.ActorCursorHook
	outcomeFlags.skillHit = "true"
	outcomeFlags.sink = "buffered"

	if err := runOutcomeEmit(nil, nil); err != nil {
		t.Fatalf("runOutcomeEmit: %v", err)
	}

	data, _ := os.ReadFile(path)
	var parsed outcomes.Outcome
	line := strings.TrimSpace(strings.Split(string(data), "\n")[0])
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.SkillHit == nil || !*parsed.SkillHit {
		t.Errorf("expected SkillHit=true, got %v", parsed.SkillHit)
	}
}

func TestSprintFromEnv(t *testing.T) {
	t.Setenv("CURSOR_TOOLS_SPRINT", "")
	if got := sprintFromEnv(); got != "v253" {
		t.Errorf("default sprint=%q, want v253", got)
	}
	t.Setenv("CURSOR_TOOLS_SPRINT", "v300")
	if got := sprintFromEnv(); got != "v300" {
		t.Errorf("overridden sprint=%q, want v300", got)
	}
}

func TestTruncateForOutcome(t *testing.T) {
	s := strings.Repeat("x", outcomes.MaxDetailChars+10)
	out := truncateForOutcome(s)
	if len(out) != outcomes.MaxDetailChars {
		t.Errorf("truncated len=%d, want %d", len(out), outcomes.MaxDetailChars)
	}
	if out := truncateForOutcome("hi"); out != "hi" {
		t.Errorf("short string truncated unexpectedly: %q", out)
	}
}

func resetOutcomeFlags(t *testing.T) {
	t.Helper()
	outcomeFlags.actor = ""
	outcomeFlags.machine = ""
	outcomeFlags.event = ""
	outcomeFlags.detail = ""
	outcomeFlags.latencyMs = 0
	outcomeFlags.mcpTool = ""
	outcomeFlags.kpiDelta = 0
	outcomeFlags.skillHit = ""
	outcomeFlags.sessionID = ""
	outcomeFlags.sprint = ""
	outcomeFlags.meta = nil
	outcomeFlags.sink = ""
	outcomeFlags.jsonOut = false
	t.Setenv("CURSOR_TOOLS_OUTCOMES_DISABLED", "")
	t.Setenv("CURSOR_TOOLS_OUTCOMES_SINK", "")
	outcomes.ResetDefaultEmitter()
}
