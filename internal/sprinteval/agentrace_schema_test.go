package sprinteval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestAgentraceEvent_UnmarshalLegacySchema confirms the original
// sprinteval NDJSON schema continues to decode unchanged.
func TestAgentraceEvent_UnmarshalLegacySchema(t *testing.T) {
	line := `{"ts":"2026-05-22T10:00:00Z","event":"tool_call","tool":"search","tokens_in":100,"tokens_out":50,"error":"timeout"}`

	var ev AgentraceEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		t.Fatalf("unmarshal legacy: %v", err)
	}
	if ev.Event != "tool_call" {
		t.Errorf("Event = %q, want tool_call", ev.Event)
	}
	if ev.Tool != "search" {
		t.Errorf("Tool = %q, want search", ev.Tool)
	}
	if ev.TokensIn != 100 || ev.TokensOut != 50 {
		t.Errorf("tokens = (%d,%d), want (100,50)", ev.TokensIn, ev.TokensOut)
	}
	if ev.Error != "timeout" {
		t.Errorf("Error = %q, want timeout", ev.Error)
	}
}

// TestAgentraceEvent_UnmarshalHelixonSchema confirms the helixon
// TracedExecutor NDJSON schema is normalised onto the legacy fields
// so downstream metrics work without code changes.
func TestAgentraceEvent_UnmarshalHelixonSchema(t *testing.T) {
	line := `{"ts":"2026-05-22T10:00:00Z","event_type":"tool_call","tool":"memory.search","server":"helixon","agent_id":"claude-code","duration_ms":42,"success":true}`

	var ev AgentraceEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		t.Fatalf("unmarshal helixon: %v", err)
	}
	if ev.Event != "tool_call" {
		t.Errorf("Event = %q, want tool_call (normalised from event_type)", ev.Event)
	}
	if ev.EventType != "tool_call" {
		t.Errorf("EventType = %q, want tool_call (raw)", ev.EventType)
	}
	if ev.Tool != "memory.search" {
		t.Errorf("Tool = %q, want memory.search", ev.Tool)
	}
	if ev.Server != "helixon" {
		t.Errorf("Server = %q, want helixon", ev.Server)
	}
	if ev.AgentID != "claude-code" {
		t.Errorf("AgentID = %q, want claude-code", ev.AgentID)
	}
	if ev.DurationMS != 42 {
		t.Errorf("DurationMS = %d, want 42", ev.DurationMS)
	}
	if ev.Success == nil || !*ev.Success {
		t.Errorf("Success = %v, want pointer-to-true", ev.Success)
	}
	if ev.Error != "" {
		t.Errorf("Error = %q, want empty for success=true", ev.Error)
	}
}

// TestAgentraceEvent_HelixonFailureMapsToError confirms a helixon event
// with success=false (and either a populated error_message or none) is
// counted as a failure by computeToolReliability / computeErrorRate.
func TestAgentraceEvent_HelixonFailureMapsToError(t *testing.T) {
	cases := []struct {
		name      string
		line      string
		wantError string
	}{
		{
			name:      "explicit error_message",
			line:      `{"ts":"2026-05-22T10:00:00Z","event_type":"tool_call","tool":"x","success":false,"error_message":"boom"}`,
			wantError: "boom",
		},
		{
			name:      "success false without message",
			line:      `{"ts":"2026-05-22T10:00:00Z","event_type":"tool_call","tool":"x","success":false}`,
			wantError: "tool failed",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ev AgentraceEvent
			if err := json.Unmarshal([]byte(tc.line), &ev); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if ev.Error != tc.wantError {
				t.Errorf("Error = %q, want %q", ev.Error, tc.wantError)
			}
		})
	}
}

// TestComputeToolReliability_HelixonSchema feeds the metric computer a
// mix of helixon-schema events to prove the normaliser is the only
// bridge required between the two schemas.
func TestComputeToolReliability_HelixonSchema(t *testing.T) {
	lines := []string{
		`{"ts":"2026-05-22T10:00:00Z","event_type":"tool_call","tool":"a","success":true}`,
		`{"ts":"2026-05-22T10:00:01Z","event_type":"tool_call","tool":"b","success":true}`,
		`{"ts":"2026-05-22T10:00:02Z","event_type":"tool_call","tool":"c","success":false,"error_message":"net err"}`,
	}
	events := make([]AgentraceEvent, 0, len(lines))
	for _, l := range lines {
		var ev AgentraceEvent
		if err := json.Unmarshal([]byte(l), &ev); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		events = append(events, ev)
	}

	rate, total, failed := computeToolReliability(events)
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if failed != 1 {
		t.Errorf("failed = %d, want 1", failed)
	}
	if rate < 0.66 || rate > 0.67 {
		t.Errorf("rate = %f, want ~0.667", rate)
	}
}

// TestLoadAgentrace_MixedSchemas writes a single NDJSON file with both
// schemas interleaved and asserts both are parsed and time-window
// filtered correctly.
func TestLoadAgentrace_MixedSchemas(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agentrace.ndjson")

	body := strings.Join([]string{
		`{"ts":"2026-05-22T09:00:00Z","event":"tool_call","tool":"legacy"}`,
		`{"ts":"2026-05-22T10:00:00Z","event_type":"tool_call","tool":"helixon-new","success":true,"duration_ms":7}`,
		`{"ts":"2026-05-22T11:00:00Z","event_type":"tool_call","tool":"helixon-fail","success":false,"error_message":"x"}`,
		``, // blank line should be skipped
	}, "\n")

	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	se := New(DefaultWeights(), nil)
	start := time.Date(2026, 5, 22, 9, 30, 0, 0, time.UTC)
	end := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)

	events, err := se.loadAgentrace(path, start, end)
	if err != nil {
		t.Fatalf("loadAgentrace: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2 (legacy filtered out by time window)", len(events))
	}
	if events[0].Tool != "helixon-new" {
		t.Errorf("events[0].Tool = %q, want helixon-new", events[0].Tool)
	}
	if events[0].DurationMS != 7 {
		t.Errorf("events[0].DurationMS = %d, want 7", events[0].DurationMS)
	}
	if events[1].Tool != "helixon-fail" {
		t.Errorf("events[1].Tool = %q, want helixon-fail", events[1].Tool)
	}
	if events[1].Error != "x" {
		t.Errorf("events[1].Error = %q, want x (normalised from error_message)", events[1].Error)
	}
}
