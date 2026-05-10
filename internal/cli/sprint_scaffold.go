package cli

import (
	"fmt"

	"github.com/nfsarch33/cursor-tools/internal/sprintgen"
	"github.com/spf13/cobra"
)

var sprintScaffoldCmd = &cobra.Command{
	Use:   "sprint-scaffold <sprint-id>",
	Short: "Generate a sprint story scaffold from the Universal Story template",
	Long:  "Outputs a Markdown scaffold with 5 themed story slots plus the 2 universal stories (Hygiene KPI + EvoLoop capsule/retro).",
	Args:  cobra.ExactArgs(1),
	RunE:  runSprintScaffold,
}

var (
	scaffoldType  string
	scaffoldTheme string
)

func init() {
	sprintScaffoldCmd.Flags().StringVar(&scaffoldType, "type", "MVP", "Sprint type (MVP or QA)")
	sprintScaffoldCmd.Flags().StringVar(&scaffoldTheme, "theme", "TBD", "Sprint theme description")
}

func runSprintScaffold(cmd *cobra.Command, args []string) error {
	out := sprintgen.Scaffold(args[0], scaffoldType, scaffoldTheme)
	if out == "" {
		return fmt.Errorf("sprint-scaffold: empty sprint ID")
	}
	fmt.Print(out)
	return nil
}
