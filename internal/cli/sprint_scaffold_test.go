package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestSprintScaffold_PositionalArg(t *testing.T) {
	resetSprintScaffoldFlags()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"sprint-scaffold", "v337", "--type", "MVP", "--theme", "Supervisor MVP"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "## v337 (MVP) -- Supervisor MVP") {
		t.Errorf("missing header in output:\n%s", out.String())
	}
}

func TestSprintScaffold_NextSprintFlag(t *testing.T) {
	resetSprintScaffoldFlags()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"sprint-scaffold", "--next-sprint", "v338", "--mode", "qa", "--theme", "Mem0 Tokyo cutover + supervisor soak"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "## v338 (QA) -- Mem0 Tokyo cutover") {
		head := out.String()
		if len(head) > 200 {
			head = head[:200]
		}
		t.Errorf("missing v338 QA header:\n%s", head)
	}
}

func TestSprintScaffold_RequiresSprintID(t *testing.T) {
	resetSprintScaffoldFlags()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"sprint-scaffold", "--mode", "mvp"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing sprint id")
	}
	if !strings.Contains(err.Error(), "sprint-id") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSprintScaffold_RejectsUnknownMode(t *testing.T) {
	resetSprintScaffoldFlags()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"sprint-scaffold", "v337", "--mode", "spaghetti"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected mode validation error")
	}
	if !strings.Contains(err.Error(), "mvp or qa") {
		t.Errorf("unexpected error: %v", err)
	}
}

// resetSprintScaffoldFlags is needed because the cobra Command's flag
// values are package-level and persist across tests in the same run.
func resetSprintScaffoldFlags() {
	scaffoldType = "MVP"
	scaffoldMode = ""
	scaffoldTheme = "TBD"
	scaffoldNextSprint = ""
}
