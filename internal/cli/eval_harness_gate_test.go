package cli

import (
	"encoding/json"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/evalharness"
)

func TestParseNDJSON_Valid(t *testing.T) {
	events := []evalharness.AgentTraceEvent{
		{Event: "tool_call", LatencyMS: 100, Success: true},
		{Event: "test_run", Coverage: 0.85},
	}
	var buf []byte
	for _, e := range events {
		b, _ := json.Marshal(e)
		buf = append(buf, b...)
		buf = append(buf, '\n')
	}

	parsed, err := parseNDJSON(buf)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(parsed) != 2 {
		t.Errorf("expected 2 events, got %d", len(parsed))
	}
}

func TestParseNDJSON_SkipsInvalid(t *testing.T) {
	input := []byte("not json\n{\"event\":\"tool_call\",\"latency_ms\":50}\ninvalid\n")
	parsed, err := parseNDJSON(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(parsed) != 1 {
		t.Errorf("expected 1 valid event, got %d", len(parsed))
	}
}

func TestParseNDJSON_Empty(t *testing.T) {
	parsed, err := parseNDJSON(nil)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(parsed) != 0 {
		t.Errorf("expected 0 events, got %d", len(parsed))
	}
}

func TestSplitLines(t *testing.T) {
	cases := []struct {
		input    string
		expected int
	}{
		{"line1\nline2\n", 2},
		{"single", 1},
		{"", 0},
		{"\n\n", 0},
		{"a\nb\nc", 3},
	}
	for _, tc := range cases {
		lines := splitLines([]byte(tc.input))
		if len(lines) != tc.expected {
			t.Errorf("splitLines(%q): expected %d lines, got %d", tc.input, tc.expected, len(lines))
		}
	}
}
