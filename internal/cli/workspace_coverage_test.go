package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestWorkspaceCoverageCommandReadsTrendFiles verifies the
// `cursor-tools workspace coverage --since 24h` output reads the
// trend JSONL files from the temp HOME and surfaces the parsed
// run count.
//
// Pre-v326 the test fixtures used hardcoded "2026-05-06T19:00:00Z"
// timestamps. That worked while the test ran within 24h of the
// fixture date and silently drifted into a 100% failure as the wall
// clock moved forward (runWorkspaceCoverage filters by
// time.Now().Add(-since)). v326-4 makes the fixtures dynamic so the
// test is time-of-day-of-year independent: every run anchors its
// trend timestamps to a fresh `time.Now().UTC()`.
func TestWorkspaceCoverageCommandReadsTrendFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	hooksDir := filepath.Join(home, ".cursor", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("mkdir hooks: %v", err)
	}

	// Anchor fixtures one hour before the test's notion of "now" so
	// the row is comfortably inside the default 24h window without
	// being so close it could brush against a future-dated guard.
	now := time.Now().UTC()
	fixtureTS := now.Add(-1 * time.Hour).Format(time.RFC3339)

	workspaceLine := fmt.Sprintf(
		`{"generated_at":%q,"score":100,"tier":"GREEN","findings":0}`+"\n",
		fixtureTS,
	)
	metricsLine := fmt.Sprintf(
		`{"ts":%q,"hook":"post-shell","cat":"workspace","detail":"workspace doctor"}`+"\n",
		fixtureTS,
	)

	if err := os.WriteFile(filepath.Join(hooksDir, "workspace-doctor.jsonl"), []byte(workspaceLine), 0o644); err != nil {
		t.Fatalf("write workspace log: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hooksDir, "metrics.jsonl"), []byte(metricsLine), 0o644); err != nil {
		t.Fatalf("write metrics: %v", err)
	}

	defer resetWorkspaceCoverageFlags()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"workspace", "coverage", "--since", "24h"})
	defer func() {
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	}()
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("workspace coverage: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "Workspace hygiene coverage") {
		t.Fatalf("output missing header: %q", out.String())
	}
	if !strings.Contains(out.String(), "Workspace runs: 1") {
		t.Fatalf("output missing run count: %q", out.String())
	}
}
