package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSprintboardMonitor_AppendsNDJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dbDir := filepath.Join(home, ".config", "helix-dev-tools")
	if err := os.MkdirAll(dbDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Use production schema minimal: monitor only needs sprintboard.Open to work;
	// skip if no DB — copy is optional. Run against empty DB will fail migrate.
	// Instead test appendMonitorNDJSON in isolation.
	path, err := appendMonitorNDJSON(map[string]interface{}{
		"event": "sprintboard_monitor", "sprint_id": "v7100",
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "sprintboard_monitor") {
		t.Fatalf("missing event in %s", string(data))
	}
}

func TestSprintboardMonitor_CommandHelp(t *testing.T) {
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"sprintboard-monitor", "--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "sprintboard-monitor.ndjson") {
		t.Fatalf("help missing ndjson path: %s", out.String())
	}
}
