package cli

import (
	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/learnings"
)

var promoteWorkspace string
var promoteDryRun bool

var promoteCmd = &cobra.Command{
	Use:   "promote",
	Short: "Promote learnings through the memory hierarchy",
	RunE:  runPromote,
}

func init() {
	promoteCmd.Flags().StringVar(&promoteWorkspace, "workspace", "", "Workspace path to promote from (project -> global)")
	promoteCmd.Flags().BoolVar(&promoteDryRun, "dry-run", false, "Show what would happen without writing")
}

func runPromote(_ *cobra.Command, _ []string) error {
	p := config.DefaultPaths()

	results := learnings.PromoteResults{}

	if promoteWorkspace != "" {
		results.Promoted = learnings.PromoteWorkspace(promoteWorkspace, p.GlobalLearningsDir(), promoteDryRun)
	}

	results.L1Digest = learnings.GenerateL1Digest(p.GlobalLearningsDir(), p.GlobalMemoriesDir(), promoteDryRun)
	results.L2SOP = learnings.GenerateL2SOP(p.GlobalLearningsDir(), p.SOPDir(), promoteDryRun)

	total := results.Total()
	if total > 0 {
		prefix := ""
		if promoteDryRun {
			prefix = "[DRY-RUN] "
		}
		clilog.Success("%sPromoted: %s", prefix, results.Summary())
	} else {
		clilog.Info("No promotions needed.")
	}

	return nil
}
