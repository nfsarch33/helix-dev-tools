package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/workspace"
)

var workspaceDoctorJSON bool
var workspaceDoctorQuick bool
var workspaceDoctorRepos []string
var workspaceDoctorNDJSON string
var workspaceDoctorConfig string
var workspaceReportSince string

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Audit multi-repo workspace cleanliness",
}

var workspaceDoctorCmd = &cobra.Command{
	Use:          "doctor",
	Short:        "Score dirty worktrees, unpushed commits, branch drift, and stale refs",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		score, err := runWorkspaceDoctor(cmd.Context())
		if err != nil {
			return err
		}
		if workspaceDoctorJSON {
			return workspace.WriteJSONReport(cmd.OutOrStdout(), score)
		}
		return workspace.WriteHumanReport(cmd.OutOrStdout(), score)
	},
}

var workspaceScoreCmd = &cobra.Command{
	Use:          "score",
	Short:        "Run workspace doctor and fail when the cleanliness tier is not GREEN",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		score, err := runWorkspaceDoctor(cmd.Context())
		if err != nil {
			return err
		}
		if workspaceDoctorJSON {
			if err := workspace.WriteJSONReport(cmd.OutOrStdout(), score); err != nil {
				return err
			}
		} else if err := workspace.WriteHumanReport(cmd.OutOrStdout(), score); err != nil {
			return err
		}
		if score.Tier != workspace.TierGreen {
			return fmt.Errorf("workspace cleanliness tier %s score %d", score.Tier, score.Score)
		}
		return nil
	},
}

var workspaceReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Print workspace doctor trend log path",
	RunE: func(cmd *cobra.Command, _ []string) error {
		path := defaultWorkspaceNDJSONPath()
		if workspaceDoctorNDJSON != "" {
			path = workspaceDoctorNDJSON
		}
		if workspaceReportSince != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "workspace report since %s\n", workspaceReportSince)
		}
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "NDJSON trend log: %s\n", path)
		return err
	},
}

func init() {
	workspaceDoctorCmd.Flags().BoolVar(&workspaceDoctorJSON, "json", false, "Output JSON")
	workspaceDoctorCmd.Flags().BoolVar(&workspaceDoctorQuick, "quick", false, "Skip remote-state checks")
	workspaceDoctorCmd.Flags().StringSliceVar(&workspaceDoctorRepos, "repo", nil, "Runx repo alias to include; repeatable")
	workspaceDoctorCmd.Flags().StringVar(&workspaceDoctorNDJSON, "ndjson", "", "Append one NDJSON trend entry to this path")
	workspaceDoctorCmd.Flags().StringVar(&workspaceDoctorConfig, "config", "", "runx config path")
	workspaceScoreCmd.Flags().BoolVar(&workspaceDoctorJSON, "json", false, "Output JSON")
	workspaceScoreCmd.Flags().BoolVar(&workspaceDoctorQuick, "quick", false, "Skip remote-state checks")
	workspaceScoreCmd.Flags().StringSliceVar(&workspaceDoctorRepos, "repo", nil, "Runx repo alias to include; repeatable")
	workspaceScoreCmd.Flags().StringVar(&workspaceDoctorNDJSON, "ndjson", "", "Append one NDJSON trend entry to this path")
	workspaceScoreCmd.Flags().StringVar(&workspaceDoctorConfig, "config", "", "runx config path")
	workspaceReportCmd.Flags().StringVar(&workspaceReportSince, "since", "7d", "Trend window")
	workspaceReportCmd.Flags().StringVar(&workspaceDoctorNDJSON, "ndjson", "", "NDJSON trend log path")
	workspaceCmd.AddCommand(workspaceDoctorCmd)
	workspaceCmd.AddCommand(workspaceScoreCmd)
	workspaceCmd.AddCommand(workspaceReportCmd)
}

func runWorkspaceDoctor(ctx context.Context) (workspace.Score, error) {
	auditor := workspace.NewAuditor(nil)
	report, err := auditor.Audit(ctx, workspace.AuditOptions{
		ConfigPath: workspaceDoctorConfig,
		RepoFilter: workspaceDoctorRepos,
		Quick:      workspaceDoctorQuick,
		Timeout:    10 * time.Second,
	})
	if err != nil {
		return workspace.Score{}, err
	}
	score := workspace.ScoreReport(report)
	if workspaceDoctorNDJSON != "" {
		if err := appendWorkspaceNDJSON(workspaceDoctorNDJSON, score); err != nil {
			return workspace.Score{}, err
		}
	}
	return score, nil
}

func appendWorkspaceNDJSON(path string, score workspace.Score) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	return workspace.WriteNDJSONReport(f, score)
}

func defaultWorkspaceNDJSONPath() string {
	home := os.Getenv("HOME")
	if home == "" {
		home = "~"
	}
	return filepath.Join(home, "logs", "runx", "workspace-doctor.ndjson")
}
