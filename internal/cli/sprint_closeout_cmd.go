package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nfsarch33/helix-dev-tools/internal/sprintcloseout"
	"github.com/spf13/cobra"
)

var sprintCloseoutCmd = &cobra.Command{
	Use:   "closeout",
	Short: "Verify sprint closeout evidence gate (7 required artefacts)",
	Long: `closeout checks that all 7 required sprint closeout artefacts exist
under the global-kb directory for the given sprint ID:

  1. sprint-retros/<id>-retro.md
  2. reports/<id>-kpi.md
  3. global-memories/capsules/<id>-capsule.md
  4. session-handoffs/<id>-handoff.md
  5. reports/evidence/<id>-evidence.md
  6. reports/<id>-badge.md
  7. global-memories/capsules/<id>-evospine.md

Exit code 1 if any artefact is missing.

Example:
  cursor-tools sprint closeout --id v6074 --kb ~/Code/global-kb`,
	RunE: runSprintCloseout,
}

func init() {
	sprintCloseoutCmd.Flags().String("id", "", "Sprint ID (e.g. v6074) (required)")
	sprintCloseoutCmd.Flags().String("kb", "", "Path to global-kb checkout (defaults to ~/Code/global-kb)")
	_ = sprintCloseoutCmd.MarkFlagRequired("id")
	sprintCmd.AddCommand(sprintCloseoutCmd)
}

func runSprintCloseout(cmd *cobra.Command, _ []string) error {
	id, _ := cmd.Flags().GetString("id")
	kb, _ := cmd.Flags().GetString("kb")

	if kb == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home: %w", err)
		}
		kb = filepath.Join(home, "Code", "global-kb")
	}

	result := sprintcloseout.Check(id, kb)
	fmt.Fprintln(cmd.OutOrStdout(), result.Report())

	if !result.OK {
		return fmt.Errorf("sprint closeout incomplete for %s", id)
	}
	return nil
}
