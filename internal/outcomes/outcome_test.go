package outcomes

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestOutcomeJSONShape verifies the canonical JSON serialization of an Outcome
// against a golden file. The schema is locked: any breaking change must update
// the golden file deliberately.
func TestOutcomeJSONShape(t *testing.T) {
	skill := true
	o := Outcome{
		Timestamp:  time.Date(2026, 4, 24, 22, 30, 0, 0, time.UTC),
		Kind:       KindAgentOutcome,
		Actor:      ActorCursorHook,
		Machine:    "macbook",
		Event:      "guard-shell:allow",
		McpTool:    "",
		LatencyMs:  12,
		SkillHit:   &skill,
		KPIDelta:   0.05,
		SessionID:  "turn-abc-001",
		Detail:     "ls -la",
		Meta:       map[string]string{"hook": "guard-shell", "category": "shell"},
		Sprint:     "v253",
		ExitCode:   nil,
		BytesIn:    8,
		BytesOut:   1024,
		DurationMs: 0,
	}

	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	goldenPath := filepath.Join("testdata", "golden_outcome.json")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	got := strings.TrimSpace(string(data))
	wantStr := strings.TrimSpace(string(want))
	if got != wantStr {
		t.Errorf("Outcome JSON mismatch.\nGot:\n%s\n\nWant:\n%s\n", got, wantStr)
	}
}

func TestOutcomeValidate(t *testing.T) {
	tests := []struct {
		name    string
		o       Outcome
		wantErr error
	}{
		{
			name: "valid_minimal",
			o: Outcome{
				Kind:    KindAgentOutcome,
				Actor:   ActorCursorHook,
				Machine: "macbook",
				Event:   "guard-shell:allow",
			},
		},
		{
			name: "missing_kind",
			o: Outcome{
				Actor:   ActorCursorHook,
				Machine: "macbook",
				Event:   "guard-shell:allow",
			},
			wantErr: ErrMissingKind,
		},
		{
			name: "wrong_kind",
			o: Outcome{
				Kind:    "evoloop_cycle",
				Actor:   ActorCursorHook,
				Machine: "macbook",
				Event:   "guard-shell:allow",
			},
			wantErr: ErrInvalidKind,
		},
		{
			name: "missing_actor",
			o: Outcome{
				Kind:    KindAgentOutcome,
				Machine: "macbook",
				Event:   "guard-shell:allow",
			},
			wantErr: ErrMissingActor,
		},
		{
			name: "missing_machine",
			o: Outcome{
				Kind:  KindAgentOutcome,
				Actor: ActorCursorHook,
				Event: "guard-shell:allow",
			},
			wantErr: ErrMissingMachine,
		},
		{
			name: "missing_event",
			o: Outcome{
				Kind:    KindAgentOutcome,
				Actor:   ActorCursorHook,
				Machine: "macbook",
			},
			wantErr: ErrMissingEvent,
		},
		{
			name: "unknown_actor",
			o: Outcome{
				Kind:    KindAgentOutcome,
				Actor:   "rogue-bot",
				Machine: "macbook",
				Event:   "test",
			},
			wantErr: ErrInvalidActor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.o.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected nil err, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected err %v, got nil", tt.wantErr)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected err %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestOutcomeMem0Text(t *testing.T) {
	o := Outcome{
		Kind:    KindAgentOutcome,
		Actor:   ActorCursorHook,
		Machine: "macbook",
		Event:   "guard-shell:allow",
		Detail:  "git status",
	}
	got := o.Mem0Text()
	if !strings.Contains(got, "macbook") {
		t.Errorf("Mem0Text should mention machine, got: %q", got)
	}
	if !strings.Contains(got, "guard-shell:allow") {
		t.Errorf("Mem0Text should mention event, got: %q", got)
	}
	if !strings.Contains(got, "git status") {
		t.Errorf("Mem0Text should include detail, got: %q", got)
	}
}

func TestOutcomeMem0Metadata(t *testing.T) {
	skill := true
	o := Outcome{
		Kind:      KindAgentOutcome,
		Actor:     ActorCursorHook,
		Machine:   "macbook",
		Event:     "guard-shell:allow",
		Detail:    "git status",
		LatencyMs: 42,
		SkillHit:  &skill,
		McpTool:   "user-mem0:search_memories",
		Sprint:    "v253",
		KPIDelta:  0.1,
		SessionID: "turn-7",
		Meta:      map[string]string{"hook": "guard-shell"},
	}
	m := o.Mem0Metadata()
	if m["kind"] != KindAgentOutcome {
		t.Errorf("expected kind=%s, got %q", KindAgentOutcome, m["kind"])
	}
	if m["actor"] != ActorCursorHook {
		t.Errorf("expected actor=%s, got %q", ActorCursorHook, m["actor"])
	}
	if m["machine"] != "macbook" {
		t.Errorf("expected machine=macbook, got %q", m["machine"])
	}
	if m["event"] != "guard-shell:allow" {
		t.Errorf("expected event=guard-shell:allow, got %q", m["event"])
	}
	if m["sprint"] != "v253" {
		t.Errorf("expected sprint=v253, got %q", m["sprint"])
	}
	if m["session_id"] != "turn-7" {
		t.Errorf("expected session_id=turn-7, got %q", m["session_id"])
	}
	if m["latency_ms"] != "42" {
		t.Errorf("expected latency_ms=42, got %q", m["latency_ms"])
	}
	if m["skill_hit"] != "true" {
		t.Errorf("expected skill_hit=true, got %q", m["skill_hit"])
	}
	if m["mcp_tool"] != "user-mem0:search_memories" {
		t.Errorf("expected mcp_tool=user-mem0:search_memories, got %q", m["mcp_tool"])
	}
	if m["kpi_delta"] != "0.1" {
		t.Errorf("expected kpi_delta=0.1, got %q", m["kpi_delta"])
	}
	if m["hook"] != "guard-shell" {
		t.Errorf("expected hook=guard-shell, got %q", m["hook"])
	}
}

func TestOutcomeDetailTruncation(t *testing.T) {
	long := strings.Repeat("x", 500)
	o := Outcome{
		Kind:    KindAgentOutcome,
		Actor:   ActorCursorHook,
		Machine: "macbook",
		Event:   "post-edit:ok",
		Detail:  long,
	}
	o.Normalize()
	if len(o.Detail) > MaxDetailChars {
		t.Errorf("detail not truncated: len=%d > %d", len(o.Detail), MaxDetailChars)
	}
}

func TestKnownActors(t *testing.T) {
	want := []string{
		ActorCursorHook,
		ActorCursorTools,
		ActorFleetCLI,
		ActorMCBridge,
		ActorIronclawDaemon,
		ActorEvoloopDaemon,
	}
	got := KnownActors()
	if len(got) != len(want) {
		t.Fatalf("KnownActors mismatch: got %v, want %v", got, want)
	}
	for i, a := range want {
		if got[i] != a {
			t.Errorf("KnownActors[%d]=%q, want %q", i, got[i], a)
		}
	}
}
