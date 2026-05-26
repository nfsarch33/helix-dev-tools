package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
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

	runCycle := func() {
		home, _ := os.UserHomeDir()
		logPath := home + "/logs/runx/agentrace-mcp.ndjson"

		events, err := readAgentraceEvents(logPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "evospine: read agentrace: %v\n", err)
			return
		}
		if len(events) == 0 {
			fmt.Fprintf(os.Stderr, "evospine: no events, skipping cycle\n")
			return
		}

		capsule := generateORHEP(events)

		outDir := home + "/logs/runx"
		os.MkdirAll(outDir, 0o755)
		outPath := outDir + "/evospine-capsule-" + time.Now().Format("2006-01-02T150405") + ".md"
		if err := os.WriteFile(outPath, []byte(capsule), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "evospine: write capsule: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "evospine: capsule written to %s\n", outPath)
	}

	runCycle()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "evospine daemon shutting down\n")
			return nil
		case <-ticker.C:
			runCycle()
		}
	}
}
