package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestSprintDispatch_DryRunCodex(t *testing.T) {
	resetSprintDispatchFlags()
	dir := t.TempDir()
	kickoff := dir + "/kickoff.md"
	if err := writeTestFile(kickoff, "# test kickoff\n"); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{
		"sprint-dispatch",
		"--agent", "codex",
		"--kickoff", kickoff,
		"--sprint", "v7100",
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "codex exec") || !strings.Contains(s, "v7100") {
		t.Fatalf("unexpected output:\n%s", s)
	}
}

func resetSprintDispatchFlags() {
	sprintDispatchFlags.agent = ""
	sprintDispatchFlags.kickoff = ""
	sprintDispatchFlags.sprintID = ""
	sprintDispatchFlags.exec = false
}

func writeTestFile(path, body string) error {
	return os.WriteFile(path, []byte(body), 0o644)
}
