package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nfsarch33/helix-dev-tools/internal/skillvet"
	"github.com/spf13/cobra"
)

var skillvetOutputJSON bool
var skillvetSummary bool

var skillvetAuditCmd = &cobra.Command{
	Use:   "skillvet-audit <skill-path>",
	Short: "Security scan an agent skill directory",
	Long:  "Static security scanner for agent skills. 20 critical + 5 warning pattern checks. Go port of skill-audit.sh.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		result, err := skillvet.ScanSkill(args[0])
		if err != nil {
			return err
		}

		if skillvetOutputJSON {
			out, marshalErr := json.MarshalIndent(result, "", "  ")
			if marshalErr != nil {
				return marshalErr
			}
			fmt.Println(string(out))
		} else if skillvetSummary {
			status := "CLEAN"
			if result.Warnings > 0 {
				status = fmt.Sprintf("WARN(%d)", result.Warnings)
			}
			if result.Criticals > 0 {
				status = fmt.Sprintf("CRITICAL(%d)", result.Criticals)
			}
			fmt.Printf("%s: %s\n", result.Skill, status)
		} else {
			fmt.Printf("Scanning: %s (%s)\n", result.Skill, args[0])
			for _, f := range result.Findings {
				fmt.Printf("  %s: %s (%s)\n", f.Severity, f.Description, f.File)
			}
			fmt.Printf("  Result: %d critical, %d warning\n", result.Criticals, result.Warnings)
		}

		if result.Criticals > 0 {
			os.Exit(2)
		}
		if result.Warnings > 0 {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	skillvetAuditCmd.Flags().BoolVar(&skillvetOutputJSON, "json", false, "Output results as JSON")
	skillvetAuditCmd.Flags().BoolVar(&skillvetSummary, "summary", false, "Output one-line summary")
}
