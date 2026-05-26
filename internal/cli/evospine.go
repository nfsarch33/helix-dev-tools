package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var evospineCmd = &cobra.Command{
	Use:   "evospine",
	Short: "EvoSpine ORHEP self-improvement daemon and tools",
}

var evospineDaemonFlags struct {
	interval time.Duration
}

var evospineDaemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run periodic ORHEP cycles from agentrace data",
	RunE:  runEvospineDaemon,
}

func init() {
	evospineDaemonCmd.Flags().DurationVar(&evospineDaemonFlags.interval, "interval", 6*time.Hour, "Interval between ORHEP cycles")
	evospineCmd.AddCommand(evospineDaemonCmd)
}

func runEvospineDaemon(cmd *cobra.Command, _ []string) error {
	interval := evospineDaemonFlags.interval
	if interval < time.Minute {
		return fmt.Errorf("interval %v too short (minimum 1m)", interval)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	fmt.Fprintf(os.Stderr, "evospine daemon starting, interval=%v\n", interval)

	home, _ := os.UserHomeDir()
	if err := runEvospineCycle(home); err != nil {
		fmt.Fprintf(os.Stderr, "evospine: cycle: %v\n", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "evospine daemon shutting down\n")
			return nil
		case <-ticker.C:
			if err := runEvospineCycle(home); err != nil {
				fmt.Fprintf(os.Stderr, "evospine: cycle: %v\n", err)
			}
		}
	}
}

// runEvospineCycle performs one ORHEP cycle: read agentrace events,
// generate a capsule, and write it under home/logs/runx/.
// Returns nil when there are no events to process.
func runEvospineCycle(home string) error {
	logPath := filepath.Join(home, "logs", "runx", "agentrace-mcp.ndjson")

	events, err := readAgentraceEvents(logPath)
	if err != nil {
		return fmt.Errorf("read agentrace: %w", err)
	}
	if len(events) == 0 {
		fmt.Fprintf(os.Stderr, "evospine: no events, skipping cycle\n")
		return nil
	}

	capsule := generateORHEP(events)

	outDir := filepath.Join(home, "logs", "runx")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir capsules: %w", err)
	}
	outPath := filepath.Join(outDir, "evospine-capsule-"+time.Now().Format("2006-01-02T150405")+".md")
	if err := os.WriteFile(outPath, []byte(capsule), 0o644); err != nil {
		return fmt.Errorf("write capsule: %w", err)
	}
	fmt.Fprintf(os.Stderr, "evospine: capsule written to %s\n", outPath)
	return nil
}
