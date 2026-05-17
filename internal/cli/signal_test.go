package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/config"
	"github.com/nfsarch33/helix-dev-tools/internal/coordination"
)

func TestBuildSignal_State(t *testing.T) {
	signalFlags.to = ""
	signalFlags.priority = ""
	signalFlags.sprint = "154"

	s := buildSignal(coordination.SignalActiveState, []string{"Working", "on", "fuzz", "targets"})

	if s.Type != coordination.SignalActiveState {
		t.Errorf("Type = %q, want active-state", s.Type)
	}
	if s.Message != "Working on fuzz targets" {
		t.Errorf("Message = %q, want joined args", s.Message)
	}
	if s.Machine == "" {
		t.Error("Machine should not be empty")
	}
	if s.Sprint != "154" {
		t.Errorf("Sprint = %q, want 154", s.Sprint)
	}
}

func TestBuildSignal_Task(t *testing.T) {
	signalFlags.to = "macbook"
	signalFlags.priority = "high"
	signalFlags.sprint = ""

	s := buildSignal(coordination.SignalTaskDispatch, []string{"Review", "the", "PR"})

	if s.Type != coordination.SignalTaskDispatch {
		t.Errorf("Type = %q, want task-dispatch", s.Type)
	}
	if s.TargetFor != "macbook" {
		t.Errorf("TargetFor = %q, want macbook", s.TargetFor)
	}
	if s.Priority != "high" {
		t.Errorf("Priority = %q, want high", s.Priority)
	}
}

func TestRunSignalTask_RequiresTo(t *testing.T) {
	signalFlags.to = ""
	signalFlags.priority = "normal"

	// Mock the client to avoid real API calls
	origClient := newCoordinationClient
	newCoordinationClient = func(_ config.Paths) (*coordination.Client, error) {
		return coordination.NewClient("key", "user", "http://localhost:1"), nil
	}
	defer func() { newCoordinationClient = origClient }()

	err := runSignalTask(nil, []string{"test"})
	if err == nil {
		t.Fatal("expected error when --to is empty")
	}
	if err.Error() != "--to flag is required for task dispatch (e.g. --to macbook)" {
		t.Errorf("error = %q", err.Error())
	}
}

func TestSignalIcon(t *testing.T) {
	tests := []struct {
		sType coordination.SignalType
		want  string
	}{
		{coordination.SignalActiveState, ">"},
		{coordination.SignalTaskDispatch, "T"},
		{coordination.SignalDecision, "D"},
		{coordination.SignalBlocker, "!"},
		{coordination.SignalCompleted, "*"},
		{coordination.SignalType("unknown"), "?"},
	}
	for _, tc := range tests {
		if got := signalIcon(tc.sType); got != tc.want {
			t.Errorf("signalIcon(%q) = %q, want %q", tc.sType, got, tc.want)
		}
	}
}

func TestSignalCmdRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "signal" {
			found = true
			break
		}
	}
	if !found {
		t.Error("signal command not registered on rootCmd")
	}
}

func TestSignalSubcommandsRegistered(t *testing.T) {
	expected := []string{"state", "task", "blocker", "alert-webhook", "decision", "completed", "list", "search"}
	subCmds := signalCmd.Commands()
	nameSet := make(map[string]bool)
	for _, cmd := range subCmds {
		nameSet[cmd.Name()] = true
	}
	for _, name := range expected {
		if !nameSet[name] {
			t.Errorf("subcommand %q not registered on signalCmd", name)
		}
	}
}

func TestReadSignalWebhookPayload_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "alert.json")
	if err := os.WriteFile(path, []byte(`{"status":"firing"}`), 0600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	data, err := readSignalWebhookPayload(path)
	if err != nil {
		t.Fatalf("readSignalWebhookPayload returned error: %v", err)
	}
	if string(data) != `{"status":"firing"}` {
		t.Fatalf("payload = %q", string(data))
	}
}
