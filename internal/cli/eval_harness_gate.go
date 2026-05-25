package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nfsarch33/helix-dev-tools/internal/evalharness"
	"github.com/spf13/cobra"
)

var evalHarnessGateCmd = &cobra.Command{
	Use:   "harness-gate [agentrace.ndjson]",
	Short: "Run ADR-065 graders against agentrace data (exit 1 on failure)",
	Long: `Evaluates agentrace NDJSON events through the 6 deterministic ADR-065 graders
(latency, error_rate, tool_coverage, token_efficiency, completion_rate, regression)
and emits results to EvoSpine. Exits non-zero if the quality gate fails.`,
	RunE: runEvalHarnessGate,
}

var evalHarnessFixtureCmd = &cobra.Command{
	Use:   "harness-fixtures [dir]",
	Short: "Run all evalharness YAML fixtures through graders",
	RunE:  runEvalHarnessFixtures,
}

var harnessSprintID string

func init() {
	evalHarnessGateCmd.Flags().StringVar(&harnessSprintID, "sprint", "", "Sprint ID for EvoSpine event tagging")
	evalCmd.AddCommand(evalHarnessGateCmd)
	evalCmd.AddCommand(evalHarnessFixtureCmd)
}

func runEvalHarnessGate(_ *cobra.Command, args []string) error {
	path := evalAgentracePath()
	if len(args) > 0 {
		path = args[0]
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read agentrace: %w", err)
	}

	events, err := parseNDJSON(data)
	if err != nil {
		return fmt.Errorf("parse agentrace: %w", err)
	}

	if len(events) == 0 {
		fmt.Println("no events found in agentrace file")
		return nil
	}

	cfg := evalharness.DefaultGraderConfig()
	graders := evalharness.ADR065Graders(cfg)
	gateCfg := evalharness.DefaultGateConfig()

	verdict := evalharness.EvaluateGate(events, graders, gateCfg)

	fmt.Print(evalharness.FormatGateVerdict(verdict))

	if harnessSprintID != "" {
		report := evalharness.GenerateSprintReport(harnessSprintID, events, evalharness.AllGraders(cfg), gateCfg)
		fmt.Printf("\n%s", evalharness.FormatMarkdownReport(report))

		if _, err := evalharness.EmitEvoSpineEvent(verdict, harnessSprintID); err != nil {
			fmt.Printf("warn: failed to emit EvoSpine event: %v\n", err)
		}
		if _, err := evalharness.EmitSprintReport(report); err != nil {
			fmt.Printf("warn: failed to emit sprint report: %v\n", err)
		}
	}

	if !verdict.Pass {
		return fmt.Errorf("harness gate FAILED: %.1f%% pass rate, %d failures", verdict.PassRate*100, verdict.FailCount)
	}
	fmt.Println("\nHarness gate PASSED")
	return nil
}

func runEvalHarnessFixtures(_ *cobra.Command, args []string) error {
	dir := "testdata/eval"
	if len(args) > 0 {
		dir = args[0]
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		dir = filepath.Join("internal", "evalharness", "fixtures")
	}

	fixtures, err := evalharness.LoadFixtureDir(dir)
	if err != nil {
		return fmt.Errorf("load fixtures: %w", err)
	}
	if len(fixtures) == 0 {
		fmt.Println("no fixtures found")
		return nil
	}

	cfg := evalharness.DefaultGraderConfig()
	graders := evalharness.AllGraders(cfg)
	hasFailure := false

	fmt.Printf("Running %d fixture(s)...\n\n", len(fixtures))
	for name, f := range fixtures {
		results, err := evalharness.RunFixture(f, graders)
		if err != nil {
			fmt.Printf("  ERROR  %s: %v\n", name, err)
			hasFailure = true
			continue
		}
		violations := evalharness.CheckExpectations(f, results)
		if len(violations) > 0 {
			fmt.Printf("  FAIL   %s\n", name)
			for _, v := range violations {
				fmt.Printf("           %s\n", v)
			}
			hasFailure = true
		} else {
			fmt.Printf("  PASS   %s\n", name)
		}
	}

	if hasFailure {
		return fmt.Errorf("some fixtures failed")
	}
	fmt.Println("\nAll fixtures passed")
	return nil
}

func evalAgentracePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "logs", "runx", "agentrace-mcp.ndjson")
}

func parseNDJSON(data []byte) ([]evalharness.AgentTraceEvent, error) {
	var events []evalharness.AgentTraceEvent
	lines := splitLines(data)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var event evalharness.AgentTraceEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		events = append(events, event)
	}
	return events, nil
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := range data {
		if data[i] == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
