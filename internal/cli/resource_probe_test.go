package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFreePct(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"The system has 51539607552 (3145728 pages with a page size of 16384). System-wide memory free percentage: 69%", 69},
		{"System-wide memory free percentage: 5%", 5},
		{"free percentage: 0", 0},
		{"some other line", -1},
		{"", -1},
	}
	for _, tc := range cases {
		got := parseFreePct(tc.in)
		if got != tc.want {
			t.Errorf("parseFreePct(%q)=%d want %d", tc.in, got, tc.want)
		}
	}
}

func TestFirstSummaryLine_PrefersFreePercentage(t *testing.T) {
	raw := `header noise
some intermediate output
The system has 12345. System-wide memory free percentage: 42%
trailing
`
	got := firstSummaryLine(raw)
	want := "The system has 12345. System-wide memory free percentage: 42%"
	if got != want {
		t.Errorf("firstSummaryLine got %q want %q", got, want)
	}
}

func TestFirstSummaryLine_FallbackFirstLine(t *testing.T) {
	raw := "first non-empty\nsecond"
	got := firstSummaryLine(raw)
	if got != "first non-empty" {
		t.Errorf("got %q want first non-empty", got)
	}
}

func TestResourceProbeCmd_Registered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"resource-probe-once"})
	if err != nil {
		t.Fatalf("rootCmd.Find(resource-probe-once): %v", err)
	}
	if cmd == nil || cmd.Use != "resource-probe-once" {
		t.Fatalf("got %#v", cmd)
	}
}

func TestRunResourceProbeOnce_WritesSanitizedSentruxCounts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	binDir := t.TempDir()
	restorePath := prependPath(t, binDir)
	defer restorePath()

	writeExecutable(t, binDir, "memory_pressure", `#!/bin/sh
cat <<'EOF'
System-wide memory free percentage: 42%
EOF
`)
	writeExecutable(t, binDir, "pgrep", `#!/bin/sh
cat <<'EOF'
123 sentrux
456 sentrux --mcp
789 sentrux --mcp --stdio
EOF
`)

	var stdout bytes.Buffer
	cmd := resourceProbeOnceCmd
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)

	if err := runResourceProbeOnce(cmd, nil); err != nil {
		t.Fatalf("runResourceProbeOnce: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(home, "logs", "runx", "resource-probe.ndjson"))
	if err != nil {
		t.Fatalf("read resource-probe.ndjson: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("probe lines = %d, want 1", len(lines))
	}

	var got resourceProbeSnapshot
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("unmarshal probe line: %v", err)
	}
	if got.FreePct != 42 {
		t.Fatalf("free_pct = %d, want 42", got.FreePct)
	}
	if got.Tier != "GREEN" {
		t.Fatalf("tier = %q, want GREEN", got.Tier)
	}
	if got.SentruxDesktopProcesses != 1 {
		t.Fatalf("sentrux_desktop_processes = %d, want 1", got.SentruxDesktopProcesses)
	}
	if got.SentruxMCPProcesses != 2 {
		t.Fatalf("sentrux_mcp_processes = %d, want 2", got.SentruxMCPProcesses)
	}

	text := stdout.String()
	for _, want := range []string{"free_pct=42", "tier=GREEN", "sentrux_desktop_processes=1", "sentrux_mcp_processes=2"} {
		if !strings.Contains(text, want) {
			t.Fatalf("stdout missing %q in %q", want, text)
		}
	}
}

func TestCaptureMemoryPressure_WithoutToolReturnsUnknownTier(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	got, err := captureMemoryPressure(context.Background())
	if err != nil {
		t.Fatalf("captureMemoryPressure: %v", err)
	}
	if got.FreePct != -1 {
		t.Fatalf("free_pct = %d, want -1", got.FreePct)
	}
	if got.Tier != "UNKNOWN" {
		t.Fatalf("tier = %q, want UNKNOWN", got.Tier)
	}
	if !strings.Contains(got.Summary, "not available") {
		t.Fatalf("summary = %q, want not available", got.Summary)
	}
}
