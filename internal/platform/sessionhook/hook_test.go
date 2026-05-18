package sessionhook

import (
	"path/filepath"
	"testing"
)

func TestRunSessionHookStart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	t.Setenv("AGENTRACE_ENABLED", "true")
	t.Setenv("CURSOR", "1")

	result, err := RunSessionHook(ActionStart, dbPath)
	if err != nil {
		t.Fatalf("RunSessionHook start: %v", err)
	}
	if result.Action != "start" {
		t.Errorf("expected action start, got %q", result.Action)
	}
	if result.TicketID == "" {
		t.Error("expected a ticket ID")
	}
}

func TestRunSessionHookStop(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	t.Setenv("AGENTRACE_ENABLED", "true")
	t.Setenv("CURSOR", "1")

	RunSessionHook(ActionStart, dbPath)

	result, err := RunSessionHook(ActionStop, dbPath)
	if err != nil {
		t.Fatalf("RunSessionHook stop: %v", err)
	}
	if result.Action != "stop" {
		t.Errorf("expected action stop, got %q", result.Action)
	}
	if result.TicketID == "" {
		t.Error("expected ticket ID from stop")
	}
}

func TestRunSessionHookDisabled(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	t.Setenv("AGENTRACE_ENABLED", "false")

	result, err := RunSessionHook(ActionStart, dbPath)
	if err != nil {
		t.Fatalf("should not error when disabled: %v", err)
	}
	if result.TicketID != "" {
		t.Error("should not create ticket when disabled")
	}
}

func TestRunSessionHookStopNoActive(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	t.Setenv("AGENTRACE_ENABLED", "true")
	t.Setenv("CURSOR", "1")

	result, err := RunSessionHook(ActionStop, dbPath)
	if err != nil {
		t.Fatalf("stop with no active: %v", err)
	}
	if result.TicketID != "" {
		t.Error("should not find ticket when none active")
	}
}

func TestRunSessionHookInvalidAction(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	t.Setenv("AGENTRACE_ENABLED", "true")

	_, err := RunSessionHook("invalid", dbPath)
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
}
