package handoffgen

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateBasic(t *testing.T) {
	ctx := HandoffContext{
		TicketID:    "t-001",
		TicketTitle: "Build MCP Server",
		SprintID:    "v6090",
		FromAgent:   "cursor-parent",
		ToAgent:     "codex-ec",
		Status:      "ready_for_handoff",
		WorkDone:    "Implemented the core store layer with SQLite backend.",
		Timestamp:   time.Date(2026, 5, 18, 13, 0, 0, 0, time.FixedZone("AEST", 10*3600)),
	}

	result := Generate(ctx)

	if !strings.HasPrefix(result.Filename, "session-handoffs/2026-05-18-t-001-handoff.md") {
		t.Errorf("unexpected filename: %s", result.Filename)
	}

	if !strings.Contains(result.Content, "# Handoff: Build MCP Server") {
		t.Error("missing title")
	}
	if !strings.Contains(result.Content, "From: cursor-parent -> To: codex-ec") {
		t.Error("missing agent info")
	}
	if !strings.Contains(result.Content, "ready_for_handoff") {
		t.Error("missing status")
	}
}

func TestGenerateWithCarryItems(t *testing.T) {
	ctx := HandoffContext{
		TicketID:    "t-002",
		TicketTitle: "Fix timeout",
		SprintID:    "v6092",
		FromAgent:   "codex",
		ToAgent:     "cursor-parent",
		Status:      "blocked",
		CarryItems: []string{
			"HTTP client timeout needs to be 120s",
			"Retry logic for infer endpoint",
		},
		Blockers: []string{
			"vLLM not running on gpu-host-1",
		},
		Timestamp: time.Date(2026, 5, 18, 14, 0, 0, 0, time.FixedZone("AEST", 10*3600)),
	}

	result := Generate(ctx)

	if !strings.Contains(result.Content, "## Carry-Forward Items") {
		t.Error("missing carry-forward section")
	}
	if !strings.Contains(result.Content, "1. HTTP client timeout") {
		t.Error("missing carry item 1")
	}
	if !strings.Contains(result.Content, "## Blockers") {
		t.Error("missing blockers section")
	}
	if !strings.Contains(result.Content, "vLLM not running on gpu-host-1") {
		t.Error("missing blocker content")
	}
}

func TestGenerateWithFilesChanged(t *testing.T) {
	ctx := HandoffContext{
		TicketID:     "t-003",
		TicketTitle:  "Add platform packages",
		SprintID:     "v6080",
		FromAgent:    "cursor-parent",
		ToAgent:      "claude-code",
		Status:       "done",
		FilesChanged: []string{"internal/platform/sprintboard/store.go", "cmd/sprintboard-mcp/main.go"},
		Timestamp:    time.Date(2026, 5, 18, 15, 0, 0, 0, time.FixedZone("AEST", 10*3600)),
	}

	result := Generate(ctx)

	if !strings.Contains(result.Content, "## Files Changed") {
		t.Error("missing files section")
	}
	if !strings.Contains(result.Content, "`internal/platform/sprintboard/store.go`") {
		t.Error("missing file path")
	}
}

func TestGenerateNextSteps(t *testing.T) {
	ctx := HandoffContext{
		TicketID:    "t-004",
		TicketTitle: "Review PR",
		SprintID:    "v6094",
		FromAgent:   "cursor-parent",
		ToAgent:     "codex",
		Status:      "review",
		Timestamp:   time.Now(),
	}

	result := Generate(ctx)

	if !strings.Contains(result.Content, "## Next Steps for Receiving Agent") {
		t.Error("missing next steps")
	}
	if !strings.Contains(result.Content, "sprint_status v6094") {
		t.Error("missing sprint status command")
	}
	if !strings.Contains(result.Content, "ticket_update t-004 in_progress") {
		t.Error("missing ticket update command")
	}
}

func TestGenerateDefaultTimestamp(t *testing.T) {
	ctx := HandoffContext{
		TicketID:    "t-005",
		TicketTitle: "Auto-timestamp",
		FromAgent:   "cursor-parent",
		ToAgent:     "codex",
	}

	result := Generate(ctx)

	if !strings.Contains(result.Content, "Created:") {
		t.Error("missing timestamp")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"t-001", "t-001"},
		{"ticket/123", "ticket-123"},
		{"My Ticket", "my-ticket"},
		{"win:path", "win-path"},
	}

	for _, tc := range tests {
		got := sanitizeFilename(tc.input)
		if got != tc.want {
			t.Errorf("sanitize(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestGenerateEmptyOptionals(t *testing.T) {
	ctx := HandoffContext{
		TicketID:    "minimal",
		TicketTitle: "Minimal handoff",
		FromAgent:   "a",
		ToAgent:     "b",
		Status:      "done",
		Timestamp:   time.Now(),
	}

	result := Generate(ctx)

	if strings.Contains(result.Content, "## Files Changed") {
		t.Error("should not have files section when empty")
	}
	if strings.Contains(result.Content, "## Carry-Forward Items") {
		t.Error("should not have carry-forward section when empty")
	}
	if strings.Contains(result.Content, "## Blockers") {
		t.Error("should not have blockers section when empty")
	}
}
