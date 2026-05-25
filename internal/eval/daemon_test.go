package eval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewDaemon_Defaults(t *testing.T) {
	dir := t.TempDir()
	evalDir := filepath.Join(dir, "evals")
	os.MkdirAll(evalDir, 0700)

	d, err := NewDaemon(DaemonConfig{
		EvalDir:   evalDir,
		StorePath: filepath.Join(dir, "eval.db"),
	}, nil)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	defer d.Close()

	if d.cfg.Interval != time.Hour {
		t.Errorf("expected 1h default interval, got %v", d.cfg.Interval)
	}
	if d.cfg.NDJSONPath == "" {
		t.Error("expected default NDJSON path")
	}
}

func TestDaemon_RunOnce_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	evalDir := filepath.Join(dir, "evals")
	os.MkdirAll(evalDir, 0700)
	ndjsonPath := filepath.Join(dir, "eval-results.ndjson")

	d, err := NewDaemon(DaemonConfig{
		EvalDir:    evalDir,
		NDJSONPath: ndjsonPath,
		StorePath:  filepath.Join(dir, "eval.db"),
	}, nil)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	defer d.Close()

	report, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if report.EvalCount != 0 {
		t.Errorf("expected 0 evals, got %d", report.EvalCount)
	}

	stats := d.Stats()
	if stats.RunCount != 1 {
		t.Errorf("expected 1 run, got %d", stats.RunCount)
	}
}

func TestDaemon_RunOnce_WithFixtures(t *testing.T) {
	dir := t.TempDir()
	evalDir := filepath.Join(dir, "evals")
	os.MkdirAll(evalDir, 0700)
	ndjsonPath := filepath.Join(dir, "eval-results.ndjson")

	writeFixture(t, evalDir, "pass.yaml", `
id: pass1
name: passing eval
type: capability
task: "func main() {}"
criteria:
  - name: has_func
    grader_type: code
    pattern: "func main"
`)

	d, err := NewDaemon(DaemonConfig{
		EvalDir:    evalDir,
		NDJSONPath: ndjsonPath,
		StorePath:  filepath.Join(dir, "eval.db"),
	}, nil)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}
	defer d.Close()

	report, err := d.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if report.EvalCount != 1 {
		t.Errorf("expected 1 eval, got %d", report.EvalCount)
	}
	if report.PassCount != 1 {
		t.Errorf("expected 1 pass, got %d", report.PassCount)
	}

	data, err := os.ReadFile(ndjsonPath)
	if err != nil {
		t.Fatalf("read ndjson: %v", err)
	}

	var entry NDJSONEvalEntry
	if err := json.Unmarshal(data[:len(data)-1], &entry); err != nil {
		t.Fatalf("unmarshal ndjson: %v", err)
	}
	if entry.EvalID != "pass1" {
		t.Errorf("expected eval_id pass1, got %s", entry.EvalID)
	}
	if !entry.Pass {
		t.Error("expected pass=true")
	}
	if !strings.HasPrefix(entry.RunID, "daemon-") {
		t.Errorf("expected daemon- prefix, got %s", entry.RunID)
	}
}

func TestDaemon_Run_CancelledContext(t *testing.T) {
	dir := t.TempDir()
	evalDir := filepath.Join(dir, "evals")
	os.MkdirAll(evalDir, 0700)

	d, err := NewDaemon(DaemonConfig{
		EvalDir:    evalDir,
		Interval:   50 * time.Millisecond,
		NDJSONPath: filepath.Join(dir, "eval.ndjson"),
		StorePath:  filepath.Join(dir, "eval.db"),
	}, nil)
	if err != nil {
		t.Fatalf("NewDaemon: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	err = d.Run(ctx)
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("expected context error, got %v", err)
	}
}

func TestDaemon_Stats_IncrementOnRun(t *testing.T) {
	dir := t.TempDir()
	evalDir := filepath.Join(dir, "evals")
	os.MkdirAll(evalDir, 0700)

	d, err := NewDaemon(DaemonConfig{
		EvalDir:    evalDir,
		NDJSONPath: filepath.Join(dir, "eval.ndjson"),
		StorePath:  filepath.Join(dir, "eval.db"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	if d.Stats().RunCount != 0 {
		t.Error("initial run count should be 0")
	}

	d.RunOnce(context.Background())
	d.RunOnce(context.Background())

	if d.Stats().RunCount != 2 {
		t.Errorf("expected 2 runs, got %d", d.Stats().RunCount)
	}
}

func TestDaemon_NDJSONMultipleResults(t *testing.T) {
	dir := t.TempDir()
	evalDir := filepath.Join(dir, "evals")
	os.MkdirAll(evalDir, 0700)
	ndjsonPath := filepath.Join(dir, "eval.ndjson")

	writeFixture(t, evalDir, "e1.yaml", `
id: e1
name: eval one
type: capability
task: "func main"
criteria:
  - name: has_func
    pattern: "func main"
`)
	writeFixture(t, evalDir, "e2.yaml", `
id: e2
name: eval two
type: capability
task: "no match here"
criteria:
  - name: missing
    pattern: "ZZZZZ"
    weight: 1.0
`)

	d, err := NewDaemon(DaemonConfig{
		EvalDir:    evalDir,
		NDJSONPath: ndjsonPath,
		StorePath:  filepath.Join(dir, "eval.db"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	report, _ := d.RunOnce(context.Background())
	if report.EvalCount != 2 {
		t.Errorf("expected 2 evals, got %d", report.EvalCount)
	}
	if report.PassCount != 1 {
		t.Errorf("expected 1 pass, got %d", report.PassCount)
	}

	data, _ := os.ReadFile(ndjsonPath)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 ndjson lines, got %d", len(lines))
	}
}

func TestDaemon_RunOnStart(t *testing.T) {
	dir := t.TempDir()
	evalDir := filepath.Join(dir, "evals")
	os.MkdirAll(evalDir, 0700)

	d, err := NewDaemon(DaemonConfig{
		EvalDir:    evalDir,
		Interval:   time.Hour,
		RunOnStart: true,
		NDJSONPath: filepath.Join(dir, "eval.ndjson"),
		StorePath:  filepath.Join(dir, "eval.db"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	d.Run(ctx)

	if d.Stats().RunCount < 1 {
		t.Error("RunOnStart should trigger at least 1 run")
	}
}

func TestNDJSONEvalEntry_Fields(t *testing.T) {
	entry := NDJSONEvalEntry{
		Timestamp:  time.Now(),
		RunID:      "daemon-test",
		EvalID:     "e1",
		EvalName:   "test eval",
		EvalType:   EvalCapability,
		Pass:       true,
		Score:      0.95,
		DurationMS: 1234,
		Iterations: 2,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}

	var decoded NDJSONEvalEntry
	json.Unmarshal(data, &decoded)

	if decoded.RunID != "daemon-test" {
		t.Errorf("expected run_id daemon-test, got %s", decoded.RunID)
	}
	if decoded.Score != 0.95 {
		t.Errorf("expected score 0.95, got %f", decoded.Score)
	}
}
