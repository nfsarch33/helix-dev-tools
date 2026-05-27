package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/fleetagent"
	"github.com/nfsarch33/helix-dev-tools/internal/fleeteval"
	"github.com/spf13/cobra"
)

var evalFleetRunCmd = &cobra.Command{
	Use:   "fleet-run [tasks.yaml]",
	Short: "Run fleet eval tasks against an LLM via the router and auto-grade responses",
	Long: `Reads YAML eval task definitions, sends each to an LLM via the cluster router,
grades responses with regex pattern matching and rubric scoring, and outputs NDJSON results.`,
	RunE: runEvalFleetRun,
}

var (
	fleetEvalRouter  string
	fleetEvalModel   string
	fleetEvalPrompt  string
	fleetEvalOutput  string
	fleetEvalTimeout int
)

func init() {
	evalFleetRunCmd.Flags().StringVar(&fleetEvalRouter, "router", "", "LLM router URL (default: LLM_ROUTER_URL or http://127.0.0.1:8787)")
	evalFleetRunCmd.Flags().StringVar(&fleetEvalModel, "model", "", "Model name (default: LLM_MODEL or MiniMax-M2.7)")
	evalFleetRunCmd.Flags().StringVar(&fleetEvalPrompt, "prompt", "", "Path to system prompt file")
	evalFleetRunCmd.Flags().StringVar(&fleetEvalOutput, "output", "", "NDJSON output path (default: ~/logs/runx/fleet-eval.ndjson)")
	evalFleetRunCmd.Flags().IntVar(&fleetEvalTimeout, "timeout", 120, "Timeout per task in seconds")
	evalCmd.AddCommand(evalFleetRunCmd)
}

func runEvalFleetRun(_ *cobra.Command, args []string) error {
	taskPath := resolveFleetTaskPath(args)
	routerURL := firstNonEmpty(fleetEvalRouter, os.Getenv("LLM_ROUTER_URL"), "http://127.0.0.1:8787")
	model := firstNonEmpty(fleetEvalModel, os.Getenv("LLM_MODEL"), "MiniMax-M2.7")
	outputPath := firstNonEmpty(fleetEvalOutput, defaultFleetEvalOutput())

	tf, err := fleeteval.LoadTaskFile(taskPath)
	if err != nil {
		return fmt.Errorf("load tasks: %w", err)
	}

	systemPrompt, err := loadSystemPrompt(fleetEvalPrompt)
	if err != nil {
		return fmt.Errorf("load system prompt: %w", err)
	}

	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	llm := fleetagent.NewHTTPLLMClient(routerURL, model, "")

	cfg := fleeteval.RunnerConfig{
		TaskFilePath:   taskPath,
		SystemPrompt:   systemPrompt,
		Model:          model,
		TimeoutSeconds: fleetEvalTimeout,
	}
	runner := fleeteval.NewRunner(llm, cfg, log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Printf("Fleet Eval: %d tasks, model=%s, router=%s\n", len(tf.Tasks), model, routerURL)
	fmt.Printf("Timeout: %ds per task, output: %s\n\n", fleetEvalTimeout, outputPath)

	start := time.Now()
	report, err := runner.RunAll(ctx, tf.Tasks)
	if err != nil {
		return fmt.Errorf("run eval: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		log.Warn("cannot create output dir", "error", err)
	}
	if err := fleeteval.WriteNDJSON(outputPath, report.Results); err != nil {
		log.Warn("write ndjson failed", "error", err)
	}

	md := fleeteval.FormatMarkdown(report)
	fmt.Print(md)
	fmt.Printf("\nCompleted in %s. Score: %d/%d (%s)\n",
		time.Since(start).Round(time.Millisecond),
		report.TotalScore, report.MaxScore, report.Verdict)

	if report.Verdict == "RED" {
		return fmt.Errorf("fleet eval FAILED: %d/%d", report.TotalScore, report.MaxScore)
	}
	return nil
}

func resolveFleetTaskPath(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), "Code", "global-kb", "eval", "fleet-agent-tasks.yaml"),
		"eval/fleet-agent-tasks.yaml",
		"fleet-agent-tasks.yaml",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return candidates[0]
}

func loadSystemPrompt(path string) (string, error) {
	if path == "" {
		candidates := []string{
			filepath.Join(os.Getenv("HOME"), "Code", "global-kb", "cursor-config", "fleet-agent-system-prompt.md"),
			"cursor-config/fleet-agent-system-prompt.md",
		}
		for _, c := range candidates {
			data, err := os.ReadFile(c)
			if err == nil {
				return string(data), nil
			}
		}
		return defaultSystemPrompt(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func defaultSystemPrompt() string {
	return `You are a fleet worker agent. Execute tasks precisely and concisely.
Respond with your answer only. Be factual and specific.`
}

func defaultFleetEvalOutput() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "logs", "runx", "fleet-eval.ndjson")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
