package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRtkSessionSavings_NoRtk(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "rtk-session-savings.ndjson")
	err := collectRtkSavings("/nonexistent/rtk", logFile)
	if err != nil {
		t.Fatalf("unexpected error for missing rtk: %v", err)
	}
	if _, err := os.Stat(logFile); err == nil {
		t.Error("expected no log file when rtk is missing")
	}
}

func TestRtkSessionSavings_LogFormat(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "rtk-session-savings.ndjson")

	entry := rtkSavingsEntry{
		TotalCommands: 42,
		TokensSaved:   "1.2K",
		Efficiency:    "87%",
	}
	err := writeRtkSavingsNDJSON(logFile, entry)
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	line := strings.TrimSpace(string(data))
	if !strings.Contains(line, `"event":"rtk_session_savings"`) {
		t.Errorf("expected event field in NDJSON, got: %s", line)
	}
	if !strings.Contains(line, `"total_commands":42`) {
		t.Errorf("expected total_commands field, got: %s", line)
	}
	if !strings.Contains(line, `"tokens_saved":"1.2K"`) {
		t.Errorf("expected tokens_saved field, got: %s", line)
	}
}

func TestParseRtkGain(t *testing.T) {
	sample := `RTK Session Stats
Total commands:   157
Tokens saved:     4.8K
Efficiency meter: ████████░░ 82.3%`

	entry := parseRtkGainOutput(sample)
	if entry.TotalCommands != 157 {
		t.Errorf("expected 157 commands, got %d", entry.TotalCommands)
	}
	if entry.TokensSaved != "4.8K" {
		t.Errorf("expected 4.8K tokens, got %q", entry.TokensSaved)
	}
	if entry.Efficiency != "82.3%" {
		t.Errorf("expected 82.3%% efficiency, got %q", entry.Efficiency)
	}
}

func TestParseRtkGain_Empty(t *testing.T) {
	entry := parseRtkGainOutput("")
	if entry.TotalCommands != 0 {
		t.Errorf("expected 0 for empty, got %d", entry.TotalCommands)
	}
}
