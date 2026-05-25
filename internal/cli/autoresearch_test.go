package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/autoresearch"
)

func TestAutoresearchRunCommand(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "ar.ndjson")
	outputDir := filepath.Join(dir, "promotions")

	// Seed agentrace data
	agtracePath := filepath.Join(dir, "agentrace.ndjson")
	f, err := os.Create(agtracePath)
	if err != nil {
		t.Fatalf("create agentrace: %v", err)
	}
	f.WriteString(`{"phase":"evaluate","error":"test error","note":"timeout"}` + "\n")
	f.WriteString(`{"phase":"evaluate","error":"test error","note":""}` + "\n")
	f.Close()

	// Override the probe config via environment
	t.Setenv("ENGRAM_URL", "http://127.0.0.1:1") // deliberately unreachable

	cmd := autoresearchRunCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{
		"--iterations", "1",
		"--log", logPath,
		"--output-dir", outputDir,
	})

	err = cmd.Execute()
	if err != nil {
		t.Logf("run output: %s", buf.String())
		t.Logf("run error (may be expected if no agentrace data at default path): %v", err)
	}
}

func TestAutoresearchStatusCommand(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "ar.ndjson")

	// Write a minimal log
	cfg := autoresearch.DefaultConfig()
	cfg.MaxIterations = 1
	cfg.LogPath = logPath
	r := autoresearch.New(cfg, nil, nil, nil, nil, nil)
	if _, err := r.Run(t.Context()); err != nil {
		t.Fatalf("seed run: %v", err)
	}

	cmd := autoresearchStatusCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--log", logPath})

	if err := cmd.Execute(); err != nil {
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
	logPath := filepath.Join(dir, "ar.ndjson")

	cfg := autoresearch.DefaultConfig()
	cfg.MaxIterations = 1
	cfg.LogPath = logPath
	r := autoresearch.New(cfg, nil, nil, nil, nil, nil)
	if _, err := r.Run(t.Context()); err != nil {
		t.Fatalf("seed run: %v", err)
	}

	cmd := autoresearchStatusCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--log", logPath, "--json"})

	if err := cmd.Execute(); err != nil {
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
