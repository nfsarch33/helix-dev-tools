package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/config"
	"github.com/nfsarch33/helix-dev-tools/internal/workspace"
)

var workspaceCoverageFlags struct {
	since string
	json  bool
}

var workspaceCoverageCmd = &cobra.Command{
	Use:   "coverage",
	Short: "Summarise Workspace Doctor hook coverage and tier distribution",
	RunE: func(cmd *cobra.Command, _ []string) error {
		summary, err := runWorkspaceCoverage()
		if err != nil {
			return err
		}
		if workspaceCoverageFlags.json {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(summary)
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(),
			"Workspace hygiene coverage\nWorkspace runs: %d\nGREEN/YELLOW/RED: %d/%d/%d\nGit mutation events: %d\nPost-shell events: %d\nHook hit rate: %.1f%%\n",
			summary.WorkspaceRuns,
			summary.GreenCount,
			summary.YellowCount,
			summary.RedCount,
			summary.GitMutationEvents,
			summary.PostShellEvents,
			summary.HookHitRate,
		)
		return err
	},
}

func init() {
	workspaceCoverageCmd.Flags().StringVar(&workspaceCoverageFlags.since, "since", "7d", "Coverage window")
	workspaceCoverageCmd.Flags().BoolVar(&workspaceCoverageFlags.json, "json", false, "Output JSON")
	workspaceCmd.AddCommand(workspaceCoverageCmd)
}

func runWorkspaceCoverage() (workspace.CoverageSummary, error) {
	p := config.DefaultPaths()
	since, err := parseWorkspaceSince(workspaceCoverageFlags.since)
	if err != nil {
		return workspace.CoverageSummary{}, err
	}
	workspaceLog, err := os.Open(filepath.Join(p.HooksDir, "workspace-doctor.jsonl"))
	if err != nil && !os.IsNotExist(err) {
		return workspace.CoverageSummary{}, err
	}
	if workspaceLog != nil {
		defer workspaceLog.Close()
	}
	metricsLog, err := os.Open(p.MetricsFile())
	if err != nil && !os.IsNotExist(err) {
		return workspace.CoverageSummary{}, err
	}
	if metricsLog != nil {
		defer metricsLog.Close()
	}
	now := time.Now().UTC()
	return workspace.SummariseCoverage(workspace.CoverageInput{
		WorkspaceEvents: workspaceLog,
		MetricsEvents:   metricsLog,
		Since:           now.Add(-since),
		Now:             now,
	})
}

func parseWorkspaceSince(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, "d") {
		days := strings.TrimSuffix(value, "d")
		parsed, err := time.ParseDuration(days + "h")
		if err != nil {
			return 0, err
		}
		return parsed * 24, nil
	}
	return time.ParseDuration(value)
}

func resetWorkspaceCoverageFlags() {
	workspaceCoverageFlags.since = "7d"
	workspaceCoverageFlags.json = false
}
