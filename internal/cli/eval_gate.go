package cli

import (
	"fmt"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/eval"
	"github.com/spf13/cobra"
)

var evalGateCmd = &cobra.Command{
	Use:   "gate [fixture.yaml ...]",
	Short: "Run eval gate and persist results (exit 1 on failure)",
	Long:  "Runs eval fixtures, persists results to eval.db, and exits non-zero if the gate fails.",
	RunE:  runEvalGate,
}

var evalHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Show recent eval run history from eval.db",
	RunE:  runEvalHistory,
}

func init() {
	evalCmd.AddCommand(evalGateCmd)
	evalCmd.AddCommand(evalHistoryCmd)
}

func runEvalGate(_ *cobra.Command, args []string) error {
	store, err := eval.OpenEvalStore(eval.DefaultEvalDBPath())
	if err != nil {
		return fmt.Errorf("open eval store: %w", err)
	}
	defer store.Close()

	var files []string
	if len(args) > 0 {
		files = args
	} else {
		dir := resolveEvalsDir()
		listed, err := eval.ListEvalFiles(dir)
		if err != nil {
			return fmt.Errorf("list eval files: %w", err)
		}
		files = listed
	}

	if len(files) == 0 {
		fmt.Println("no eval fixtures found")
		return nil
	}

	runID := fmt.Sprintf("gate-%s", time.Now().Format("20060102-150405"))
	fmt.Printf("Gate run %s: %d fixture(s)\n\n", runID, len(files))

	var results []eval.EvalResult
	for _, f := range files {
		result, err := eval.RunEvalFile(f)
		if err != nil {
			result = eval.EvalResult{EvalID: f, Pass: false, Error: err.Error()}
		}
		results = append(results, result)

		if saveErr := store.SaveResult(runID, result); saveErr != nil {
			fmt.Printf("  warn: failed to persist result: %v\n", saveErr)
		}

		status := "PASS"
		if !result.Pass {
			status = "FAIL"
		}
		fmt.Printf("  %s  %s (score=%.2f)\n", status, result.EvalName, result.Score)
	}

	report := eval.GenerateReport(runID, results)
	fmt.Printf("\n%s\n", report.ToMarkdown())

	if report.FailCount > 0 {
		return fmt.Errorf("gate FAILED: %d/%d evals failed", report.FailCount, report.EvalCount)
	}
	fmt.Println("Gate PASSED")
	return nil
}

func runEvalHistory(_ *cobra.Command, _ []string) error {
	store, err := eval.OpenEvalStore(eval.DefaultEvalDBPath())
	if err != nil {
		return fmt.Errorf("open eval store: %w", err)
	}
	defer store.Close()

	runs, err := store.RecentRuns(10)
	if err != nil {
		return fmt.Errorf("fetch history: %w", err)
	}

	if len(runs) == 0 {
		fmt.Println("no eval runs recorded yet")
		return nil
	}

	fmt.Printf("%-25s %5s %5s %8s %8s\n", "RUN ID", "TOTAL", "PASS", "RATE", "AVG")
	fmt.Printf("%-25s %5s %5s %8s %8s\n", "------", "-----", "----", "----", "---")
	for _, r := range runs {
		fmt.Printf("%-25s %5d %5d %7.1f%% %7.2f\n", r.RunID, r.Total, r.Passed, r.PassRate*100, r.AvgScore)
	}
	return nil
}
