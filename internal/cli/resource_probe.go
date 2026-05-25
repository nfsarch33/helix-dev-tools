package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// resourceProbeOnceCmd is the launchd-facing one-shot probe. The
// `cursor-tools daemon` resource-probe service writes the same
// NDJSON shape every 5 minutes via the long-lived loop; this command
// is what the launchd plist (com.user.cursor-resource-probe.plist)
// invokes when running outside the daemon, so resource-aware
// subagents and CLIs always have a fresh sample available.
//
// Output line schema:
//
//	{"ts":"<ISO8601>", "event":"memory_pressure_probe",
//	 "free_pct":<int>, "summary":"<vendor summary line>"}
//
// File: $HOME/logs/runx/resource-probe.ndjson (append-only).
// Compatible with the existing entries written by the cursor-resource-guard
// script so consumers can tail one stream.
var resourceProbeOnceCmd = &cobra.Command{
	Use:   "resource-probe-once",
	Short: "Capture one memory-pressure sample and append to resource-probe.ndjson",
	RunE:  runResourceProbeOnce,
}

func runResourceProbeOnce(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()
	if cmd != nil && cmd.Context() != nil {
		ctx = cmd.Context()
	}

	mp, err := captureMemoryPressure(ctx)
	if err != nil {
		return err
	}

	path := resourceProbePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("resource-probe: mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("resource-probe: open log: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	if err := enc.Encode(mp); err != nil {
		return fmt.Errorf("resource-probe: encode: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "free_pct=%d tier=%s sentrux_desktop_processes=%d sentrux_mcp_processes=%d\n",
		mp.FreePct, mp.Tier, mp.SentruxDesktopProcesses, mp.SentruxMCPProcesses)
	return nil
}

// resourceProbeSnapshot is the NDJSON shape persisted to disk and the
// read model consumed by session-start/session-handoff.
type resourceProbeSnapshot struct {
	Ts                      string `json:"ts,omitempty"`
	Event                   string `json:"event,omitempty"`
	Summary                 string `json:"summary,omitempty"`
	Tier                    string `json:"tier,omitempty"`
	FreePct                 int    `json:"free_pct"`
	SentruxDesktopProcesses int    `json:"sentrux_desktop_processes,omitempty"`
	SentruxMCPProcesses     int    `json:"sentrux_mcp_processes,omitempty"`
	Err                     string `json:"error,omitempty"`
}

// captureMemoryPressure runs the platform memory-pressure tool and
// parses its summary line. On macOS this is `memory_pressure`; on
// other platforms the function returns a stub with FreePct=-1 so
// downstream consumers can detect "no sample" without crashing.
func captureMemoryPressure(ctx context.Context) (resourceProbeSnapshot, error) {
	now := time.Now().Format(time.RFC3339)
	sample := resourceProbeSnapshot{
		Ts:      now,
		Event:   "memory_pressure_probe",
		FreePct: -1,
		Tier:    "UNKNOWN",
	}

	bin, lookErr := exec.LookPath("memory_pressure")
	if lookErr != nil {
		sample.Summary = "memory_pressure: not available on this platform"
		return sample, nil
	}

	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, bin)
	out, err := cmd.Output()
	if err != nil {
		return sample, fmt.Errorf("memory_pressure exec: %w", err)
	}
	summary := firstSummaryLine(string(out))
	sample.Summary = summary
	sample.FreePct = parseFreePct(summary)
	desktop, mcp, err := captureSentruxProcessCounts(ctx)
	if err != nil {
		sample.Err = "sentrux process probe: " + err.Error()
	} else {
		sample.SentruxDesktopProcesses = desktop
		sample.SentruxMCPProcesses = mcp
	}
	return normalizeResourceProbeSnapshot(sample), nil
}

// firstSummaryLine extracts the "System-wide memory free percentage"
// summary line from `memory_pressure` output.
func firstSummaryLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "System-wide memory free percentage") {
			return line
		}
	}
	// Fall back to first non-empty line so the NDJSON entry is
	// useful even if Apple changes the wording.
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}

var freePctRE = regexp.MustCompile(`(?i)free percentage:\s*(\d+)`)

func parseFreePct(summary string) int {
	m := freePctRE.FindStringSubmatch(summary)
	if len(m) < 2 {
		return -1
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return -1
	}
	return n
}

func resourceProbePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "resource-probe.ndjson"
	}
	return filepath.Join(home, "logs", "runx", "resource-probe.ndjson")
}

func normalizeResourceProbeSnapshot(sample resourceProbeSnapshot) resourceProbeSnapshot {
	if sample.Tier == "" || (sample.Tier == "UNKNOWN" && sample.FreePct >= 0) {
		sample.Tier = resourceProbeTier(sample.FreePct)
	}
	return sample
}

func resourceProbeTier(freePct int) string {
	switch {
	case freePct < 0:
		return "UNKNOWN"
	case freePct < 10:
		return "RED"
	case freePct < 20:
		return "YELLOW"
	default:
		return "GREEN"
	}
}

func captureSentruxProcessCounts(ctx context.Context) (desktop int, mcp int, err error) {
	bin, lookErr := exec.LookPath("pgrep")
	if lookErr != nil {
		return 0, 0, nil
	}

	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, bin, "-af", "sentrux").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "--mcp") {
			mcp++
			continue
		}
		desktop++
	}
	return desktop, mcp, nil
}

func init() {
	rootCmd.AddCommand(resourceProbeOnceCmd)
}
