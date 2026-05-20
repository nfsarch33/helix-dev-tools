package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/nfsarch33/helix-dev-tools/internal/hookio"
	"github.com/nfsarch33/helix-dev-tools/internal/semblediscipline"
	"github.com/spf13/cobra"
)

var sembleDisciplineCmd = &cobra.Command{
	Use:   "semble-discipline",
	Short: "beforeShellExecution: advisory Semble-first check for exploratory rg/grep/find",
	Long: `Reads hook JSON from stdin (same contract as guard-shell).
Logs exploratory search commands to ~/logs/runx/semble-discipline.ndjson.
When CURSOR_TOOLS_SEMBLE_STRICT=1, returns permission=ask for exploratory matches.

Typically chained from guard-shell or invoked directly from hooks.json.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runSembleDiscipline(os.Stdin, os.Stdout)
	},
}

func init() {
	hookCmd.AddCommand(sembleDisciplineCmd)
}

func runSembleDiscipline(stdin *os.File, stdout *os.File) error {
	input, err := hookio.ReadInput(stdin)
	if err != nil {
		_ = hookio.WriteResponse(stdout, hookio.Allow())
		return nil
	}
	resp := evaluateSembleDiscipline(input.Command, "semble-discipline")
	_ = hookio.WriteResponse(stdout, resp)
	return nil
}

// evaluateSembleDiscipline is the shared classifier used by guard-shell and the hook subcommand.
func evaluateSembleDiscipline(command, hookName string) *hookio.Response {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return hookio.Allow()
	}

	result := semblediscipline.Classify(cmd)
	if result.Verdict != semblediscipline.VerdictExploratory {
		return hookio.Allow()
	}

	cmdShort := cmd
	if len(cmdShort) > 200 {
		cmdShort = cmdShort[:200]
	}

	strict := semblediscipline.StrictModeEnabled()
	_ = semblediscipline.AppendEvent("", semblediscipline.Event{
		Event:   "semble_discipline",
		Tool:    result.Tool,
		Verdict: string(result.Verdict),
		Reason:  result.Reason,
		Command: cmdShort,
		Hook:    hookName,
		Strict:  strict,
	})

	userMsg := fmt.Sprintf(
		"Semble-first: exploratory %s detected. Prefer `semble search \"<intent>\" <repo-path>` before rg/grep/find.",
		result.Tool,
	)
	agentMsg := "Use the Semble MCP search tool for semantic discovery. Reserve rg/grep for literal matches with -F or a single explicit file path."

	if strict {
		return hookio.Deny(
			userMsg+" (strict mode)",
			agentMsg+" Re-run with semble search, or use rg/grep -F against a single known file path.",
		)
	}
	return hookio.Ask(userMsg, agentMsg)
}

// sembleDisciplineAdvisory is invoked from guard-shell before pattern matching.
func sembleDisciplineAdvisory(command string) *hookio.Response {
	return evaluateSembleDiscipline(command, "guard-shell")
}
