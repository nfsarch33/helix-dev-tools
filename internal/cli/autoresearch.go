package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/autoresearch"
)

var autoresearchFlags struct {
	iterations int
	threshold  float64
	logPath    string
	outputDir  string
	jsonOutput bool
}

var autoresearchCmd = &cobra.Command{
	Use:   "autoresearch",
	Short: "Autonomous research loop (Probe → Propose → Evaluate → Decide → Promote)",
	Long: `Run the 5-phase autoresearch loop based on karpathy/autoresearch methodology.

Phases:
  1. Probe    - scan agentrace NDJSON for error/pattern data
  2. Propose  - generate structured experiment hypotheses
  3. Evaluate - score hypotheses against baseline
  4. Decide   - accept/reject based on delta threshold
  5. Promote  - persist findings to Engram, files, or EvoSpine`,
}

var autoresearchRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Execute one autoresearch cycle",
	RunE:  runAutoresearch,
}

var autoresearchStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show autoresearch history and metrics",
	RunE:  showAutoresearchStatus,
}

func init() {
	autoresearchRunCmd.Flags().IntVar(&autoresearchFlags.iterations, "iterations", 3, "max research iterations per cycle")
	autoresearchRunCmd.Flags().Float64Var(&autoresearchFlags.threshold, "threshold", 0.0, "minimum delta to accept a finding")
	autoresearchRunCmd.Flags().StringVar(&autoresearchFlags.logPath, "log", "", "agentrace NDJSON output path")
	autoresearchRunCmd.Flags().StringVar(&autoresearchFlags.outputDir, "output-dir", "", "directory for promotion files")
	autoresearchRunCmd.Flags().BoolVar(&autoresearchFlags.jsonOutput, "json", false, "output results as JSON")

	autoresearchStatusCmd.Flags().StringVar(&autoresearchFlags.logPath, "log", "", "agentrace NDJSON log to read")
	autoresearchStatusCmd.Flags().BoolVar(&autoresearchFlags.jsonOutput, "json", false, "output as JSON")

	autoresearchCmd.AddCommand(autoresearchRunCmd)
	autoresearchCmd.AddCommand(autoresearchStatusCmd)
}

func runAutoresearch(cmd *cobra.Command, _ []string) error {
	cfg := autoresearch.DefaultConfig()
	if autoresearchFlags.iterations > 0 {
		cfg.MaxIterations = autoresearchFlags.iterations
	}
	if autoresearchFlags.logPath != "" {
		cfg.LogPath = autoresearchFlags.logPath
	}

	pcfg := autoresearch.DefaultProbeConfig()

	var promoteCfg autoresearch.PromoteConfig
	if autoresearchFlags.outputDir != "" {
		promoteCfg.OutputDir = autoresearchFlags.outputDir
	} else {
		home, _ := os.UserHomeDir()
		promoteCfg.OutputDir = filepath.Join(home, "logs", "runx", "autoresearch-promotions")
	}

	home, _ := os.UserHomeDir()
	promoteCfg.EvoSpineLog = filepath.Join(home, "logs", "runx", "sentrux-autoresearch.ndjson")

	promoteCfg.Engram = autoresearch.NewEngramClient()

	runner := autoresearch.New(cfg,
		autoresearch.NewProbePhase(pcfg),
		autoresearch.NewProposePhase(),
		autoresearch.NewEvaluatePhase(nil),
		autoresearch.NewDecidePhase(autoresearchFlags.threshold),
		autoresearch.NewPromotePhase(promoteCfg),
	)

	history, err := runner.Run(cmd.Context())
	if err != nil {
		return fmt.Errorf("autoresearch run: %w", err)
	}

	status := autoresearch.BuildStatus(history, cfg.LogPath)

	if autoresearchFlags.jsonOutput {
		data, _ := json.MarshalIndent(status, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Autoresearch cycle complete\n")
	fmt.Fprintf(w, "  Iterations: %d\n", status.TotalIterations)
	fmt.Fprintf(w, "  Keep:       %d\n", status.KeepCount)
	fmt.Fprintf(w, "  Discard:    %d\n", status.DiscardCount)
	if status.LastDecision != "" {
		fmt.Fprintf(w, "  Last:       %s (metric=%.4f, delta=%.4f)\n",
			status.LastDecision, status.LastMetric, status.LastDelta)
	}
	fmt.Fprintf(w, "  Log:        %s (%d entries)\n", status.LogPath, status.LogEntries)
	return nil
}

func showAutoresearchStatus(cmd *cobra.Command, _ []string) error {
	logPath := autoresearchFlags.logPath
	if logPath == "" {
		home, _ := os.UserHomeDir()
		logPath = filepath.Join(home, "logs", "runx", "agentrace-autoresearch.ndjson")
	}

	status, err := autoresearch.LoadStatusFromLog(logPath)
	if err != nil {
		return fmt.Errorf("load status: %w", err)
	}

	if autoresearchFlags.jsonOutput {
		data, _ := json.MarshalIndent(status, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Autoresearch Status\n")
	fmt.Fprintf(w, "  Total iterations: %d\n", status.TotalIterations)
	fmt.Fprintf(w, "  Keep decisions:   %d\n", status.KeepCount)
	fmt.Fprintf(w, "  Discard decisions:%d\n", status.DiscardCount)
	if status.LastDecision != "" {
		fmt.Fprintf(w, "  Last decision:    %s\n", status.LastDecision)
		fmt.Fprintf(w, "  Last metric:      %.4f\n", status.LastMetric)
		fmt.Fprintf(w, "  Last delta:       %.4f\n", status.LastDelta)
	}
	if !status.LastRun.IsZero() {
		fmt.Fprintf(w, "  Last run:         %s\n", status.LastRun.Format("2006-01-02 15:04:05 UTC"))
	}
	fmt.Fprintf(w, "  Log entries:      %d\n", status.LogEntries)
	fmt.Fprintf(w, "  Log path:         %s\n", status.LogPath)
	return nil
}
