package agentidentity

import (
	"os"
	"testing"
)

func TestResolveDefault(t *testing.T) {
	clearEnv(t)
	info := Resolve()
	if info.ID != Operator {
		t.Errorf("expected operator, got %q", info.ID)
	}
	if info.Surface != "terminal" {
		t.Errorf("expected terminal surface, got %q", info.Surface)
	}
}

func TestResolveCursor(t *testing.T) {
	clearEnv(t)
	t.Setenv("CURSOR", "1")
	info := Resolve()
	if info.ID != CursorParent {
		t.Errorf("expected cursor-parent, got %q", info.ID)
	}
	if info.Surface != "cursor" {
		t.Errorf("expected cursor surface, got %q", info.Surface)
	}
}

func TestResolveCursorExplicit(t *testing.T) {
	clearEnv(t)
	t.Setenv("CURSOR_AGENT_ID", "custom-agent")
	info := Resolve()
	if info.ID != "custom-agent" {
		t.Errorf("expected custom-agent, got %q", info.ID)
	}
}

func TestResolveCodex(t *testing.T) {
	clearEnv(t)
	t.Setenv("CODEX_SESSION", "sess-123")
	info := Resolve()
	if info.ID != Codex {
		t.Errorf("expected codex, got %q", info.ID)
	}
	if info.Surface != "codex-cli" {
		t.Errorf("expected codex-cli surface, got %q", info.Surface)
	}
	if info.Session != "sess-123" {
		t.Errorf("expected session sess-123, got %q", info.Session)
	}
}

func TestResolveCodexProfile(t *testing.T) {
	clearEnv(t)
	t.Setenv("CODEX_SESSION", "sess-456")
	t.Setenv("CODEX_PROFILE", "EC")
	info := Resolve()
	if info.ID != "codex-ec" {
		t.Errorf("expected codex-ec, got %q", info.ID)
	}
}

func TestResolveClaudeCode(t *testing.T) {
	clearEnv(t)
	t.Setenv("CLAUDE_CODE", "1")
	t.Setenv("CLAUDE_SESSION_ID", "claude-abc")
	info := Resolve()
	if info.ID != ClaudeCode {
		t.Errorf("expected claude-code, got %q", info.ID)
	}
	if info.Session != "claude-abc" {
		t.Errorf("expected session claude-abc, got %q", info.Session)
	}
}

func TestIsKnown(t *testing.T) {
	if !IsKnown(CursorParent) {
		t.Error("cursor-parent should be known")
	}
	if !IsKnown(Codex) {
		t.Error("codex should be known")
	}
	if IsKnown("random-agent") {
		t.Error("random-agent should not be known")
	}
}

func TestAllAgents(t *testing.T) {
	agents := AllAgents()
	if len(agents) != 6 {
		t.Errorf("expected 6 agents, got %d", len(agents))
	}
}

func TestPriorityOrder(t *testing.T) {
	clearEnv(t)
	t.Setenv("CURSOR_AGENT_ID", "explicit")
	t.Setenv("CODEX_SESSION", "sess")
	t.Setenv("CLAUDE_CODE", "1")

	info := Resolve()
	if info.ID != "explicit" {
		t.Errorf("CURSOR_AGENT_ID should take priority, got %q", info.ID)
	}
}

func TestHostname(t *testing.T) {
	clearEnv(t)
	info := Resolve()
	if info.Hostname == "" {
		t.Skip("hostname unavailable in test environment")
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"CURSOR_AGENT_ID", "CURSOR", "CURSOR_SESSION_ID",
		"CODEX_SESSION", "CODEX_PROFILE",
		"CLAUDE_CODE", "CLAUDE_SESSION_ID",
	} {
		os.Unsetenv(key)
		t.Cleanup(func() { os.Unsetenv(key) })
	}
}
