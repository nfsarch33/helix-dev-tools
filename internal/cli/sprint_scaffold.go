package cli

import (
	"fmt"
	"strings"

	"github.com/nfsarch33/helix-dev-tools/internal/sprintgen"
	"github.com/spf13/cobra"
)

var sprintScaffoldCmd = &cobra.Command{
	Use:   "sprint-scaffold [sprint-id]",
	Short: "Generate the 7-story sprint scaffold (5 themed + 2 universal)",
	Long: `Outputs the Markdown scaffold for one sprint per the Universal
Story Scaffold defined in cursor-config/rules/sprint-scaffold-7-stories.mdc:

  5 themed stories with: Type, Owner repo, Branch, Files, R1 OSS evidence,
                          RED test, GREEN summary, Validation, Acceptance,
                          Pause/resume notes, Calendar deadline.
  2 universal stories:    Hygiene KPI + EvoLoop capsule/retro.

Usage:
  cursor-tools sprint-scaffold v337 --type MVP --theme "Supervisor MVP"
  cursor-tools sprint-scaffold --next-sprint v338 --mode qa --theme "Mem0 Tokyo cutover + supervisor soak"

Either the positional <sprint-id> or the --next-sprint flag is required.
The --mode flag is an alias for --type (mvp/qa). Output is deterministic;
the golden test internal/sprintgen/testdata/sprint-v338-qa.expected.md
pins drift.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSprintScaffold,
}

var (
	scaffoldType       string
	scaffoldTheme      string
	scaffoldNextSprint string
	scaffoldMode       string
)

func init() {
	sprintScaffoldCmd.Flags().StringVar(&scaffoldType, "type", "MVP", "Sprint type (MVP or QA) -- alias for --mode")
	sprintScaffoldCmd.Flags().StringVar(&scaffoldMode, "mode", "", "Sprint mode (mvp or qa); preferred over --type")
	sprintScaffoldCmd.Flags().StringVar(&scaffoldTheme, "theme", "TBD", "Sprint theme description")
	sprintScaffoldCmd.Flags().StringVar(&scaffoldNextSprint, "next-sprint", "", "Sprint id (e.g. v338); alternative to positional arg")
}

func runSprintScaffold(cmd *cobra.Command, args []string) error {
	sprintID := scaffoldNextSprint
	if sprintID == "" && len(args) == 1 {
		sprintID = args[0]
	}
	if sprintID == "" {
		return fmt.Errorf("sprint-scaffold: --next-sprint or positional <sprint-id> required")
	}

	mode := scaffoldMode
	if mode == "" {
		mode = scaffoldType
	}
	mode = strings.ToUpper(strings.TrimSpace(mode))
	switch mode {
	case "MVP", "QA":
	default:
		return fmt.Errorf("sprint-scaffold: --mode must be mvp or qa (got %q)", mode)
	}

	out := sprintgen.Scaffold(sprintID, mode, scaffoldTheme)
	if out == "" {
		return fmt.Errorf("sprint-scaffold: empty output for sprint %q", sprintID)
	}
	fmt.Fprint(cmd.OutOrStdout(), out)
	return nil
}
