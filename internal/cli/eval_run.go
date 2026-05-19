package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/eval"
	"github.com/spf13/cobra"
)

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Eval harness for MCP tool quality gates",
}

var evalRunCmd = &cobra.Command{
	Use:   "run [fixture.yaml ...]",
	Short: "Run eval fixtures and report pass/fail",
	Long:  "Runs YAML eval fixtures from evals/ directory or specified paths. Reports pass/fail with scores.",
	RunE:  runEvalRun,
}

var evalListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available eval fixtures",
	RunE:  runEvalList,
}

var evalSuite string

func init() {
	evalRunCmd.Flags().StringVar(&evalSuite, "suite", "", "Run a named suite (qa, regression, all)")
	evalCmd.AddCommand(evalRunCmd)
	evalCmd.AddCommand(evalListCmd)
}

func resolveEvalsDir() string {
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), "cursor-tools", "evals"),
		"evals",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return "evals"
}

func runEvalRun(_ *cobra.Command, args []string) error {
	start := time.Now()

	var files []string
	if len(args) > 0 {
		files = args
	} else {
		dir := resolveEvalsDir()
		listed, err := eval.ListEvalFiles(dir)
		if err != nil {
			return fmt.Errorf("list eval files in %s: %w", dir, err)
		}
		if evalSuite != "" && evalSuite != "all" {
			for _, f := range listed {
				def, err := eval.LoadEvalFile(f)
				if err != nil {
					continue
				}
				if string(def.Type) == evalSuite {
					files = append(files, f)
				}
			}
		} else {
			files = listed
		}
	}

	if len(files) == 0 {
		fmt.Println("no eval fixtures found")
		return nil
	}

	fmt.Printf("Running %d eval(s)...\n\n", len(files))

	var results []eval.EvalResult
	for _, f := range files {
		result, err := eval.RunEvalFile(f)
		if err != nil {
			results = append(results, eval.EvalResult{
				EvalID: filepath.Base(f),
				Pass:   false,
				Error:  err.Error(),
			})
			fmt.Printf("  FAIL  %s: %v\n", filepath.Base(f), err)
			continue
		}
		results = append(results, result)
		status := "PASS"
		if !result.Pass {
			status = "FAIL"
		}
		fmt.Printf("  %s  %s (score=%.2f, %dms)\n", status, result.EvalName, result.Score, result.DurationMS)
	}

	runID := fmt.Sprintf("eval-%s", time.Now().Format("20060102-150405"))
	report := eval.GenerateReport(runID, results)

	fmt.Printf("\n%s\n", report.ToMarkdown())
	fmt.Printf("Completed in %s. Badge: %s\n", time.Since(start).Round(time.Millisecond), eval.QualityBadge(report.PassRate))

	if report.FailCount > 0 {
		return fmt.Errorf("%d eval(s) failed", report.FailCount)
	}
	return nil
}

func runEvalList(_ *cobra.Command, _ []string) error {
	dir := resolveEvalsDir()
	fixtures, err := eval.ListFixtures(dir)
	if err != nil {
		return fmt.Errorf("list fixtures: %w", err)
	}

	if len(fixtures) == 0 {
		fmt.Println("no eval fixtures found")
		return nil
	}

	fmt.Printf("%-30s %-15s %s\n", "ID", "TYPE", "NAME")
	fmt.Printf("%-30s %-15s %s\n", "---", "----", "----")
	for _, f := range fixtures {
		fmt.Printf("%-30s %-15s %s\n", f.ID, f.Type, f.Name)
	}
	return nil
}
