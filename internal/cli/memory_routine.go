package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/clilog"
	"github.com/nfsarch33/helix-dev-tools/internal/config"
	"github.com/nfsarch33/helix-dev-tools/internal/metrics"
)

var memoryRoutineFlags struct {
	days          int
	metricsExport string
	parityExport  string
	skipSync      bool
}

var memoryRoutineCmd = &cobra.Command{
	Use:   "memory-routine",
	Short: "Run the Mem0-first parity and metrics routine",
	Long:  "Runs the recurring memory routine: doctor mcp, metrics export and analysis, Mem0 parity proof export, and optional durable-doc sync.",
	RunE:  runMemoryRoutine,
}

var memoryRoutineExportMetrics = exportMemoryMetrics
var memoryRoutineRunParityExport = runMem0ParityExport
var memoryRoutineSyncDocs = syncMemoryRoutineDocs

func init() {
	p := config.DefaultPaths()
	defaultLogsDir := filepath.Join(p.Home, "logs")
	memoryRoutineCmd.Flags().IntVar(&memoryRoutineFlags.days, "days", 30, "Number of days of metrics history to include")
	memoryRoutineCmd.Flags().StringVar(&memoryRoutineFlags.metricsExport, "metrics-export", filepath.Join(defaultLogsDir, "memory-metrics.md"), "Path to export the memory metrics markdown report")
	memoryRoutineCmd.Flags().StringVar(&memoryRoutineFlags.parityExport, "parity-export", filepath.Join(defaultLogsDir, "memory-parity.md"), "Path to export the Mem0 parity markdown report")
	memoryRoutineCmd.Flags().BoolVar(&memoryRoutineFlags.skipSync, "skip-sync", false, "Skip the durable-doc git pull step")
}

func runMemoryRoutine(_ *cobra.Command, _ []string) error {
	started := time.Now()
	p := config.DefaultPaths()
	out := clilog.NewPrefixed("[memory-routine]")
	totalSteps := 4
	if memoryRoutineFlags.skipSync {
		totalSteps = 3
	}
	passedSteps := 0

	out.Info("step 1/%d: doctor mcp", totalSteps)
	if err := runDoctorProfile("mcp"); err != nil {
		recordCheckRunWithContext("memory-routine", "memory-routine", "", started, passedSteps, totalSteps)
		return err
	}
	passedSteps++

	out.Info("step 2/%d: metrics export", totalSteps)
	if err := memoryRoutineExportMetrics(p, memoryRoutineFlags.days, memoryRoutineFlags.metricsExport); err != nil {
		recordCheckRunWithContext("memory-routine", "memory-routine", "", started, passedSteps, totalSteps)
		return err
	}
	passedSteps++

	out.Info("step 3/%d: mem0 parity proof", totalSteps)
	if err := memoryRoutineRunParityExport(memoryRoutineFlags.parityExport); err != nil {
		recordCheckRunWithContext("memory-routine", "memory-routine", "", started, passedSteps, totalSteps)
		return err
	}
	passedSteps++

	if !memoryRoutineFlags.skipSync {
		out.Info("step 4/%d: durable-doc sync", totalSteps)
		if err := memoryRoutineSyncDocs(p); err != nil {
			recordCheckRunWithContext("memory-routine", "memory-routine", "", started, passedSteps, totalSteps)
			return fmt.Errorf("durable-doc sync: %w", err)
		}
		passedSteps++
	}

	recordCheckRunWithContext("memory-routine", "memory-routine", "", started, passedSteps, totalSteps)
	out.Info("done")
	return nil
}

func syncMemoryRoutineDocs(p config.Paths) error {
	if keyPath := p.SSHKeyPath(); keyPath != "" {
		if _, err := os.Stat(keyPath); err == nil {
			_ = os.Setenv("GIT_SSH_COMMAND", fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", keyPath))
		}
	}
	_, err := runCommandOutput(2*time.Minute, "git", "-C", p.GlobalKB, "pull", "--ff-only", "origin", "main")
	return err
}

func exportMemoryMetrics(p config.Paths, days int, exportPath string) error {
	events, err := metrics.LoadAll(p.MetricsFile())
	if err != nil {
		return fmt.Errorf("loading metrics: %w", err)
	}
	since := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	summary := metrics.Summarise(events, since)
	if err := os.MkdirAll(filepath.Dir(exportPath), 0o755); err != nil {
		return fmt.Errorf("create metrics export dir: %w", err)
	}
	if err := os.WriteFile(exportPath, []byte(summary.Markdown()), 0o644); err != nil {
		return fmt.Errorf("write metrics export: %w", err)
	}
	return nil
}

func runMem0ParityExport(exportPath string) error {
	if err := os.MkdirAll(filepath.Dir(exportPath), 0o755); err != nil {
		return fmt.Errorf("create parity export dir: %w", err)
	}
	saved := mem0ParityFlags
	defer func() {
		mem0ParityFlags = saved
	}()
	mem0ParityFlags.export = exportPath
	mem0ParityFlags.strict = true
	mem0ParityFlags.syncProvenance = false
	mem0ParityFlags.dryRun = false
	return runMem0Parity(nil, nil)
}
