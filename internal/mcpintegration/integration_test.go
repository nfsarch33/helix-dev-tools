package mcpintegration

import (
	"testing"
	"time"
)

func TestNewTracer(t *testing.T) {
	tr := NewTracer(Config{Enabled: true, LogPath: "/tmp/test-agentrace.ndjson"})
	if tr == nil {
		t.Fatal("expected non-nil tracer")
	}
}

func TestRecordToolCall(t *testing.T) {
	tr := NewTracer(Config{Enabled: true})
	tr.RecordToolCall(ToolCall{
		Server:   "user-sprintboard",
		Tool:     "sprint_create",
		Duration: 50 * time.Millisecond,
		Success:  true,
	})
	calls := tr.ToolCalls()
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}
}

func TestDisabled(t *testing.T) {
	tr := NewTracer(Config{Enabled: false})
	tr.RecordToolCall(ToolCall{Server: "test", Tool: "test"})
	calls := tr.ToolCalls()
	if len(calls) != 0 {
		t.Error("expected no calls when disabled")
	}
}

func TestServerStats(t *testing.T) {
	tr := NewTracer(Config{Enabled: true})
	tr.RecordToolCall(ToolCall{Server: "sprintboard", Tool: "sprint_create", Duration: 50 * time.Millisecond, Success: true})
	tr.RecordToolCall(ToolCall{Server: "sprintboard", Tool: "ticket_create", Duration: 30 * time.Millisecond, Success: true})
	tr.RecordToolCall(ToolCall{Server: "mem0-oss", Tool: "mem0_search", Duration: 2 * time.Second, Success: false})

	stats := tr.ServerStats()
	if stats["sprintboard"].TotalCalls != 2 {
		t.Errorf("sprintboard calls %d, want 2", stats["sprintboard"].TotalCalls)
	}
	if stats["mem0-oss"].FailureCount != 1 {
		t.Errorf("mem0 failures %d, want 1", stats["mem0-oss"].FailureCount)
	}
}

func TestConfigurable(t *testing.T) {
	tr := NewTracer(Config{
		Enabled:     true,
		LogPath:     "/tmp/test.ndjson",
		IncludeArgs: false,
		Servers:     []string{"sprintboard", "mem0-oss"},
	})
	if !tr.IsServerTracked("sprintboard") {
		t.Error("expected sprintboard to be tracked")
	}
	if tr.IsServerTracked("unknown") {
		t.Error("expected unknown to not be tracked")
	}
}
