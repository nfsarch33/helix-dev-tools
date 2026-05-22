package sprinteval

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

// NewSprintEvalCmd creates the "sprint-eval" cobra command for cursor-tools.
// It runs the eval pipeline: parse agentrace, fetch sprint metrics, and
// generate a markdown report.
func NewSprintEvalCmd() *cobra.Command {
	var (
		sprintID     string
		agentracePath string
		sprintURL    string
		outputPath   string
	)

	cmd := &cobra.Command{
		Use:   "sprint-eval",
		Short: "Evaluate a sprint using agentrace events and sprintboard metrics",
		Long: `sprint-eval analyses agentrace NDJSON logs and sprintboard ticket data
to produce a quality evaluation report for the specified sprint.

Examples:
  cursor-tools sprint-eval --sprint v8000
  cursor-tools sprint-eval --sprint v8000 --agentrace ~/logs/runx/agentrace-mcp.ndjson
  cursor-tools sprint-eval --sprint v8000 --output ~/reports/v8000.md`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSprintEval(sprintID, agentracePath, sprintURL, outputPath)
		},
	}

	cmd.Flags().StringVar(&sprintID, "sprint", "", "Sprint ID to evaluate (required)")
	cmd.Flags().StringVar(&agentracePath, "agentrace", "", "Path to agentrace NDJSON log (default: ~/logs/runx/agentrace-mcp.ndjson)")
	cmd.Flags().StringVar(&sprintURL, "sprintboard-url", "http://localhost:9400", "Sprintboard API base URL")
	cmd.Flags().StringVar(&outputPath, "output", "", "Output path for markdown report (default: stdout)")

	cmd.MarkFlagRequired("sprint")

	return cmd
}

func runSprintEval(sprintID, agentracePath, sprintURL, outputPath string) error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("starting sprint evaluation",
		slog.String("sprint", sprintID),
		slog.String("agentrace", agentracePath),
	)

	// Step 1: Parse agentrace events
	events, err := ParseAgentrace(agentracePath)
	if err != nil {
		logger.Warn("agentrace parse failed (proceeding without events)",
			slog.String("error", err.Error()),
		)
		events = nil
	} else {
		logger.Info("agentrace parsed", slog.Int("events", len(events)))
	}

	// Step 2: Fetch sprint metrics from sprintboard
	var sprint *SprintData
	sprint, err = FetchSprintMetrics(sprintURL, sprintID)
	if err != nil {
		logger.Warn("sprintboard fetch failed (proceeding without sprint data)",
			slog.String("error", err.Error()),
		)
	} else {
		logger.Info("sprint data fetched",
			slog.Int("tickets", sprint.TotalTickets),
			slog.Int("completed", sprint.CompletedTickets),
		)
	}

	// Step 3: Generate report
	report := GenerateReport(events, sprint)

	// Step 4: Output
	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(report), 0644); err != nil {
			return fmt.Errorf("write report to %s: %w", outputPath, err)
		}
		logger.Info("report written", slog.String("path", outputPath))
	} else {
		fmt.Print(report)
	}

	return nil
}
