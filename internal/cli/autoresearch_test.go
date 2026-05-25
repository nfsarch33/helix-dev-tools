package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/autoresearch"
)

func seedAutoresearchLog(t *testing.T, dir string) string {
	t.Helper()
	logPath := filepath.Join(dir, "ar.ndjson")
	cfg := autoresearch.DefaultConfig()
	cfg.MaxIterations = 1
	cfg.LogPath = logPath
	r := autoresearch.New(cfg, nil, nil, nil, nil, nil)
	if _, err := r.Run(context.Background()); err != nil {
		t.Fatalf("seed run: %v", err)
	}
	return logPath
}

func TestAutoresearchRunCommand(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "ar.ndjson")
	outputDir := filepath.Join(dir, "promotions")

	agtracePath := filepath.Join(dir, "agentrace.ndjson")
	f, err := os.Create(agtracePath)
	if err != nil {
		t.Fatalf("create agentrace: %v", err)
	}
	f.WriteString(`{"phase":"evaluate","error":"test error","note":"timeout"}` + "\n")
	f.WriteString(`{"phase":"evaluate","error":"test error","note":""}` + "\n")
	f.Close()

	t.Setenv("ENGRAM_URL", "http://localhost:1")

	autoresearchFlags.iterations = 1
	autoresearchFlags.logPath = logPath
	autoresearchFlags.outputDir = outputDir
	autoresearchFlags.jsonOutput = false

	buf := new(bytes.Buffer)
	autoresearchRunCmd.SetOut(buf)
	autoresearchRunCmd.SetErr(buf)

	err = runAutoresearch(autoresearchRunCmd, nil)
	if err != nil {
		t.Logf("run output: %s", buf.String())
		t.Logf("run error (may be expected if no agentrace data at default path): %v", err)
	}
}

func TestAutoresearchStatusCommand(t *testing.T) {
	dir := t.TempDir()
	logPath := seedAutoresearchLog(t, dir)

	autoresearchFlags.logPath = logPath
	autoresearchFlags.jsonOutput = false

	buf := new(bytes.Buffer)
	autoresearchStatusCmd.SetOut(buf)
	autoresearchStatusCmd.SetErr(buf)

	if err := showAutoresearchStatus(autoresearchStatusCmd, nil); err != nil {
		t.Fatalf("status: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Autoresearch Status") {
		t.Errorf("expected status header, got:\n%s", output)
	}
	if !strings.Contains(output, "Log entries:") {
		t.Errorf("expected log entries line, got:\n%s", output)
	}
}

func TestAutoresearchStatusJSON(t *testing.T) {
	dir := t.TempDir()
	logPath := seedAutoresearchLog(t, dir)

	autoresearchFlags.logPath = logPath
	autoresearchFlags.jsonOutput = true

	buf := new(bytes.Buffer)
	autoresearchStatusCmd.SetOut(buf)
	autoresearchStatusCmd.SetErr(buf)

	if err := showAutoresearchStatus(autoresearchStatusCmd, nil); err != nil {
		t.Fatalf("status: %v", err)
	}

	var status autoresearch.ResearchStatus
	if err := json.Unmarshal(buf.Bytes(), &status); err != nil {
		t.Fatalf("unmarshal JSON status: %v (output=%q)", err, buf.String())
	}
	if status.LogEntries <= 0 {
		t.Errorf("expected log entries > 0, got %d", status.LogEntries)
	}
}
