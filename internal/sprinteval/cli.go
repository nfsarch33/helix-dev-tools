package sprinteval

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const defaultReportDir = "~/Code/global-kb/reports/research"

// NewSprintEvalCmd creates the "sprint-eval" cobra command for cursor-tools.
// It runs the full eval pipeline: parse agentrace events, fetch ticket
// snapshots from sprintboard (with histogram fallback for older binaries),
// compute the quality score, and write a Markdown + JSON report.
func NewSprintEvalCmd() *cobra.Command {
	var (
		sprintID      string
		sprintName    string
		agentracePath string
		sprintURL     string
		outputDir     string
		stdout        bool
	)

	cmd := &cobra.Command{
		Use:   "sprint-eval",
		Short: "Evaluate a sprint using agentrace events and sprintboard metrics",
		Long: `sprint-eval analyses agentrace NDJSON logs and sprintboard ticket data
to produce a quality evaluation report for the specified sprint.

The agentrace parser accepts both the legacy sprinteval schema and the
helixon TracedExecutor schema (event_type, agent_id, duration_ms, success,
error_message). When sprintboard's /sprints/{id}/tickets endpoint is
available, ticket snapshots are fetched directly; older binaries fall back
to the tickets_by_status histogram.

Examples:
  cursor-tools sprint-eval --sprint v8000
  cursor-tools sprint-eval --sprint v8000 --agentrace ~/logs/runx/agentrace-mcp.ndjson
  cursor-tools sprint-eval --sprint v8000 --output-dir ~/reports`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSprintEval(sprintEvalOpts{
				SprintID:      sprintID,
				SprintName:    sprintName,
				AgentracePath: agentracePath,
				SprintURL:     sprintURL,
				OutputDir:     outputDir,
				Stdout:        stdout,
			})
		},
	}

	cmd.Flags().StringVar(&sprintID, "sprint", "", "Sprint ID to evaluate (required)")
	cmd.Flags().StringVar(&sprintName, "sprint-name", "", "Optional human-readable sprint name (defaults to sprintboard name)")
	cmd.Flags().StringVar(&agentracePath, "agentrace", "", "Path to agentrace NDJSON log (default: ~/logs/runx/agentrace-mcp.ndjson)")
	cmd.Flags().StringVar(&sprintURL, "sprintboard-url", defaultSprintboardBaseURL, "Sprintboard API base URL")
	cmd.Flags().StringVar(&outputDir, "output-dir", defaultReportDir, "Output directory for sprint-eval-<id>.md/.json")
	cmd.Flags().BoolVar(&stdout, "stdout", false, "Print markdown report to stdout instead of writing files")

	_ = cmd.MarkFlagRequired("sprint")

	return cmd
}

type sprintEvalOpts struct {
	SprintID      string
	SprintName    string
	AgentracePath string
	SprintURL     string
	OutputDir     string
	Stdout        bool
}

func runSprintEval(opts sprintEvalOpts) error {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	logger.Info("starting sprint evaluation",
		slog.String("sprint", opts.SprintID),
		slog.String("agentrace", opts.AgentracePath),
	)

	events, err := ParseAgentrace(opts.AgentracePath)
	if err != nil {
		logger.Warn("agentrace parse failed (proceeding without events)",
			slog.String("error", err.Error()),
		)
		events = nil
	} else {
		logger.Info("agentrace parsed", slog.Int("events", len(events)))
	}

	sprintData, ticketSnapshots := loadSprintData(logger, opts.SprintURL, opts.SprintID)

	sprintName := opts.SprintName
	if sprintName == "" && sprintData != nil {
		sprintName = sprintData.SprintName
	}

	se := New(DefaultWeights(), logger)
	report, err := se.Run(SprintInput{
		SprintID:      opts.SprintID,
		SprintName:    sprintName,
		Tickets:       ticketSnapshots,
		AgentraceFile: "", // events already loaded above
	})
	if err != nil {
		return fmt.Errorf("evaluate: %w", err)
	}
	// Re-compute metrics with the events we already parsed so the
	// markdown reflects the loaded NDJSON, not just an empty slice.
	report.Metrics = ComputeMetrics(ticketSnapshots, events, nil, nil)

	if opts.Stdout {
		fmt.Print(se.renderMarkdown(report))
		return nil
	}

	dir := expandPath(opts.OutputDir)
	mdPath, err := se.WriteReport(reportWithFilename(report, "sprint-eval-"+opts.SprintID), dir)
	if err != nil {
		return fmt.Errorf("write markdown: %w", err)
	}
	jsonPath, err := se.WriteJSON(reportWithFilename(report, "sprint-eval-"+opts.SprintID), dir)
	if err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	logger.Info("sprint-eval written",
		slog.String("markdown", mdPath),
		slog.String("json", jsonPath),
		slog.Float64("quality_score", report.QualityScore),
		slog.String("grade", report.QualityGrade),
	)
	return nil
}

// loadSprintData fetches ticket snapshots, preferring the /tickets
// endpoint and falling back to the histogram from /sprints/{id} when
// the older sprintboard binary is in production.
func loadSprintData(logger *slog.Logger, baseURL, sprintID string) (*SprintData, []TicketSnapshot) {
	sprintData, err := FetchSprintMetrics(baseURL, sprintID)
	if err != nil {
		logger.Warn("sprintboard summary fetch failed", slog.String("error", err.Error()))
		return nil, nil
	}
	logger.Info("sprintboard summary",
		slog.Int("total", sprintData.TotalTickets),
		slog.Int("completed", sprintData.CompletedTickets),
	)

	tickets, terr := FetchSprintTickets(baseURL, sprintID)
	if terr == nil && len(tickets) > 0 {
		logger.Info("ticket snapshots fetched", slog.Int("count", len(tickets)))
		return sprintData, tickets
	}
	logger.Warn("ticket list endpoint unavailable; deriving from histogram",
		slog.String("error", fmt.Sprint(terr)),
	)
	return sprintData, SnapshotsFromHistogram(sprintData.TicketsByStatus)
}

// reportWithFilename clones a SprintReport and rewrites its SprintID so
// WriteReport/WriteJSON produce sprint-eval-<id>.{md,json} filenames
// while keeping the original SprintID visible inside the markdown.
func reportWithFilename(report *SprintReport, filenameID string) *SprintReport {
	clone := *report
	clone.SprintID = filenameID
	return &clone
}

func expandPath(p string) string {
	if len(p) > 1 && p[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
