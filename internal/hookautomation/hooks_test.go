package hookautomation

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := New(Config{HooksPath: "~/.cursor/hooks.json"})
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestRegisterHook(t *testing.T) {
	m := New(Config{})
	m.Register(Hook{Event: "agentStart", Name: "session-record", Command: "cursor-tools session start"})
	hooks := m.Hooks()
	if len(hooks) != 1 {
		t.Fatalf("got %d hooks, want 1", len(hooks))
	}
}

func TestHooksByEvent(t *testing.T) {
	m := New(Config{})
	m.Register(Hook{Event: "agentStart", Name: "a"})
	m.Register(Hook{Event: "agentStart", Name: "b"})
	m.Register(Hook{Event: "afterShellExecution", Name: "c"})
	start := m.ByEvent("agentStart")
	if len(start) != 2 {
		t.Errorf("got %d agentStart hooks, want 2", len(start))
	}
}

func TestValidateNoDuplicates(t *testing.T) {
	m := New(Config{})
	m.Register(Hook{Event: "agentStart", Name: "session-record"})
	m.Register(Hook{Event: "agentStart", Name: "session-record"})
	issues := m.Validate()
	if len(issues) != 1 {
		t.Errorf("got %d issues, want 1 duplicate", len(issues))
	}
}

func TestListEvents(t *testing.T) {
	m := New(Config{})
	m.Register(Hook{Event: "agentStart", Name: "a"})
	m.Register(Hook{Event: "afterShellExecution", Name: "b"})
	events := m.Events()
	if len(events) != 2 {
		t.Errorf("got %d events, want 2", len(events))
	}
}
