package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

var trackFlags struct {
	category     string
	name         string
	durationMs   int64
	memoryLayer  string
	memoryOp     string
	memoryResult string
	resultCount  int
}

var trackCmd = &cobra.Command{
	Use:   "track [--cat CATEGORY] --name NAME [--ms DURATION] [-- COMMAND...]",
	Short: "Record timed operation metrics for analytics",
	Long: `Record execution timing for any operation: MCP tools, skills, subagents, scripts, and memory usage outcomes.

Two modes:
  Manual:  cursor-tools track --cat mcp --name context7.resolve --ms 1234
  Memory:  cursor-tools track --cat mcp --name mem0:search_memories --memory-layer mem0 --memory-op search --memory-result hit --result-count 3
  Wrapper: cursor-tools track --cat skill --name ui-ux-pro-max -- uiux search grid

Categories: mcp, shell, skill, subagent, script, tool, check, custom
When --cat is omitted, defaults to "custom".`,
	RunE:               runTrack,
	DisableFlagParsing: false,
}

func init() {
	trackCmd.Flags().StringVar(&trackFlags.category, "cat", "custom", "Operation category (mcp, shell, skill, subagent, script, tool, check, custom)")
	trackCmd.Flags().StringVar(&trackFlags.name, "name", "", "Operation name (required)")
	trackCmd.Flags().Int64Var(&trackFlags.durationMs, "ms", 0, "Pre-measured duration in milliseconds (manual mode)")
	trackCmd.Flags().StringVar(&trackFlags.memoryLayer, "memory-layer", "", "Memory layer for manual tracking (mem0, context_mode, git_kb, allpepper)")
	trackCmd.Flags().StringVar(&trackFlags.memoryOp, "memory-op", "", "Memory operation for manual tracking (search, read, write, update)")
	trackCmd.Flags().StringVar(&trackFlags.memoryResult, "memory-result", "", "Memory result for manual tracking (hit, miss, empty, write)")
	trackCmd.Flags().IntVar(&trackFlags.resultCount, "result-count", 0, "Number of results returned by the memory operation")
	_ = trackCmd.MarkFlagRequired("name")
}

// ValidCategories lists accepted category values.
var ValidCategories = []string{"mcp", "shell", "skill", "subagent", "script", "tool", "check", "custom"}

func isValidCategory(cat string) bool {
	for _, v := range ValidCategories {
		if v == cat {
			return true
		}
	}
	return false
}

func runTrack(cmd *cobra.Command, args []string) error {
	if !isValidCategory(trackFlags.category) {
		return fmt.Errorf("invalid category %q; valid: %s", trackFlags.category, strings.Join(ValidCategories, ", "))
	}
	if trackFlags.name == "" {
		return fmt.Errorf("--name is required")
	}
	if trackFlags.memoryLayer != "" && !metrics.IsValidMemoryLayer(trackFlags.memoryLayer) {
		return fmt.Errorf("invalid --memory-layer %q; valid: %s", trackFlags.memoryLayer, strings.Join(metrics.ValidMemoryLayers, ", "))
	}
	if trackFlags.memoryOp != "" && !metrics.IsValidMemoryOp(trackFlags.memoryOp) {
		return fmt.Errorf("invalid --memory-op %q; valid: %s", trackFlags.memoryOp, strings.Join(metrics.ValidMemoryOps, ", "))
	}
	if trackFlags.memoryResult != "" && !metrics.IsValidMemoryResult(trackFlags.memoryResult) {
		return fmt.Errorf("invalid --memory-result %q; valid: %s", trackFlags.memoryResult, strings.Join(metrics.ValidMemoryResults, ", "))
	}
	if trackFlags.resultCount < 0 {
		return fmt.Errorf("--result-count must be >= 0")
	}

	p := config.DefaultPaths()
	metricsPath := p.MetricsFile()

	if len(args) == 0 {
		return recordManual(metricsPath)
	}

	return recordWrapper(metricsPath, args)
}

func recordManual(metricsPath string) error {
	evt := metrics.Event{
		Hook:         "track",
		Action:       "record",
		Category:     trackFlags.category,
		Detail:       trackFlags.name,
		DurationMs:   trackFlags.durationMs,
		LatencyMs:    0,
		MemoryLayer:  trackFlags.memoryLayer,
		MemoryOp:     trackFlags.memoryOp,
		MemoryResult: trackFlags.memoryResult,
		ResultCount:  trackFlags.resultCount,
	}
	if err := metrics.Record(metricsPath, evt); err != nil {
		return fmt.Errorf("recording metric: %w", err)
	}
	clilog.Success("tracked %s/%s = %dms", trackFlags.category, trackFlags.name, trackFlags.durationMs)
	return nil
}

func recordWrapper(metricsPath string, args []string) error {
	cmdName := args[0]
	cmdArgs := args[1:]

	start := time.Now()
	c := exec.Command(cmdName, cmdArgs...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	err := c.Run()
	dur := time.Since(start).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	evt := metrics.Event{
		Hook:         "track",
		Action:       "record",
		Category:     trackFlags.category,
		Detail:       trackFlags.name,
		DurationMs:   dur,
		LatencyMs:    0,
		ExitCode:     exitCode,
		MemoryLayer:  trackFlags.memoryLayer,
		MemoryOp:     trackFlags.memoryOp,
		MemoryResult: trackFlags.memoryResult,
		ResultCount:  trackFlags.resultCount,
	}
	if recErr := metrics.Record(metricsPath, evt); recErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to record metric: %v\n", recErr)
	}

	actionWord := "completed"
	if exitCode != 0 {
		actionWord = fmt.Sprintf("failed (exit %d)", exitCode)
	}
	clilog.Info("tracked %s/%s = %dms [%s]", trackFlags.category, trackFlags.name, dur, actionWord)

	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}
