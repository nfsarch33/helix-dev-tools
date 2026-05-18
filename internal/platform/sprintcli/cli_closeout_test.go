package sprintcli

import (
	"os"
	"path/filepath"
	"testing"
)

// writeEvalFixture writes a minimal YAML eval fixture to dir and returns the
// dir path.  command="true" produces a passing eval; command="false" a
// failing one.
func writeEvalFixture(t *testing.T, dir, id, command string) {
	t.Helper()
	content := "id: " + id + "\n" +
		"name: " + id + "\n" +
		"type: capability\n" +
		"task: test\n" +
		"criteria:\n" +
		"  - name: shell-check\n" +
		"    grader_type: shell\n" +
		"    command: \"" + command + "\"\n"
	if err := os.WriteFile(filepath.Join(dir, id+".yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

func TestCloseout_NoEvalsDir(t *testing.T) {
	t.Parallel()
	c := testCLI(t)
	c.CreateSprint("v6206", "QA Sprint", "eval")

	res, err := c.Closeout("v6206", "", 0.8)
	if err != nil {
		t.Fatalf("Closeout: %v", err)
	}
	if !res.EvalPass {
		t.Error("expected EvalPass=true when evalsDir is empty")
	}
	if res.Blocked {
		t.Error("expected Blocked=false when evalsDir is empty")
	}
	if res.PassRate != 1.0 {
		t.Errorf("PassRate = %f, want 1.0", res.PassRate)
	}
	if res.SprintID != "v6206" {
		t.Errorf("SprintID = %q, want v6206", res.SprintID)
	}
}

func TestCloseout_AllPass(t *testing.T) {
	t.Parallel()
	c := testCLI(t)
	c.CreateSprint("v6206", "QA Sprint", "eval")

	dir := t.TempDir()
	writeEvalFixture(t, dir, "pass-eval", "true")

	res, err := c.Closeout("v6206", dir, 1.0)
	if err != nil {
		t.Fatalf("Closeout: %v", err)
	}
	if !res.EvalPass {
		t.Error("expected EvalPass=true for all-pass eval")
	}
	if res.Blocked {
		t.Errorf("expected Blocked=false, got BlockReason=%q", res.BlockReason)
	}
	if res.PassRate < 0.999 {
		t.Errorf("PassRate = %f, want 1.0", res.PassRate)
	}
	if res.EvalReport == "" {
		t.Error("expected non-empty EvalReport markdown")
	}
}

func TestCloseout_BelowThreshold(t *testing.T) {
	t.Parallel()
	c := testCLI(t)
	c.CreateSprint("v6206", "QA Sprint", "eval")

	dir := t.TempDir()
	writeEvalFixture(t, dir, "fail-eval", "false")

	res, err := c.Closeout("v6206", dir, 1.0)
	if err != nil {
		t.Fatalf("Closeout: %v", err)
	}
	if !res.Blocked {
		t.Error("expected Blocked=true when pass rate < minPassRate")
	}
	if res.BlockReason == "" {
		t.Error("expected non-empty BlockReason when blocked")
	}
}

func TestCloseout_ZeroThreshold(t *testing.T) {
	t.Parallel()
	c := testCLI(t)
	c.CreateSprint("v6206", "QA Sprint", "eval")

	dir := t.TempDir()
	writeEvalFixture(t, dir, "fail-eval-zero", "false")

	// minPassRate=0.0 means gate is disabled; must never block.
	res, err := c.Closeout("v6206", dir, 0.0)
	if err != nil {
		t.Fatalf("Closeout: %v", err)
	}
	if res.Blocked {
		t.Errorf("expected Blocked=false with zero threshold, got BlockReason=%q", res.BlockReason)
	}
}
