package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/resourceguard"
	"github.com/spf13/cobra"
)

var resourceGuardCmd = &cobra.Command{
	Use:   "resource-guard",
	Short: "Resource guard utilities",
}

var staleCheckCmd = &cobra.Command{
	Use:   "stale-check",
	Short: "Detect orphan MCP server processes and log to resource-guard.ndjson",
	RunE:  runStaleCheck,
}

var staleCheckThreshold time.Duration

func init() {
	staleCheckCmd.Flags().DurationVar(&staleCheckThreshold, "threshold", resourceguard.DefaultThreshold, "Elapsed time threshold for stale detection")
	resourceGuardCmd.AddCommand(staleCheckCmd)
	rootCmd.AddCommand(resourceGuardCmd)
}

func runStaleCheck(cmd *cobra.Command, _ []string) error {
	lister := &psProcessLister{}
	d := resourceguard.NewDetector(lister, resourceguard.DetectorConfig{
		Threshold: staleCheckThreshold,
	})

	stale, err := d.Scan()
	if err != nil {
		return fmt.Errorf("stale-check: %w", err)
	}

	logPath := resourceGuardLogPath()
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return fmt.Errorf("mkdir log dir: %w", err)
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer f.Close()

	now := time.Now().Format(time.RFC3339)
	entry := map[string]any{
		"ts":          now,
		"event":       "stale_check",
		"stale_count": len(stale),
	}
	if len(stale) > 0 {
		entry["stale_processes"] = stale
	}

	enc := json.NewEncoder(f)
	if err := enc.Encode(entry); err != nil {
		return fmt.Errorf("encode log entry: %w", err)
	}

	out := cmd.OutOrStdout()
	if len(stale) == 0 {
		fmt.Fprintln(out, "stale_count=0")
		return nil
	}

	fmt.Fprintf(out, "stale_count=%d\n", len(stale))
	for _, s := range stale {
		fmt.Fprintf(out, "  pid=%d name=%s reason=%s elapsed=%s\n",
			s.PID, s.Name, s.Reason, s.Elapsed.Truncate(time.Second))
	}
	return nil
}

func resourceGuardLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "resource-guard.ndjson"
	}
	return filepath.Join(home, "logs", "runx", "resource-guard.ndjson")
}

// psProcessLister implements resourceguard.ProcessLister using `ps`.
type psProcessLister struct{}

func (p *psProcessLister) ListProcesses() ([]resourceguard.ProcessInfo, error) {
	out, err := exec.Command("ps", "-eo", "pid,ppid,etime,comm").Output()
	if err != nil {
		return nil, fmt.Errorf("ps: %w", err)
	}

	var procs []resourceguard.ProcessInfo
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i, line := range lines {
		if i == 0 {
			continue // skip header
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		elapsed := parseEtime(fields[2])
		name := fields[3]

		procs = append(procs, resourceguard.ProcessInfo{
			PID:     pid,
			PPID:    ppid,
			Name:    name,
			Elapsed: elapsed,
		})
	}
	return procs, nil
}

// parseEtime parses the ELAPSED column from `ps -eo etime`.
// Formats: MM:SS, HH:MM:SS, D-HH:MM:SS
func parseEtime(s string) time.Duration {
	var days, hours, minutes, seconds int

	// Split on dash for days
	parts := strings.SplitN(s, "-", 2)
	timePart := s
	if len(parts) == 2 {
		days, _ = strconv.Atoi(parts[0])
		timePart = parts[1]
	}

	segments := strings.Split(timePart, ":")
	switch len(segments) {
	case 3:
		hours, _ = strconv.Atoi(segments[0])
		minutes, _ = strconv.Atoi(segments[1])
		seconds, _ = strconv.Atoi(segments[2])
	case 2:
		minutes, _ = strconv.Atoi(segments[0])
		seconds, _ = strconv.Atoi(segments[1])
	case 1:
		seconds, _ = strconv.Atoi(segments[0])
	}

	return time.Duration(days)*24*time.Hour +
		time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second
}
