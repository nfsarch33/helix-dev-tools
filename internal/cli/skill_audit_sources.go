package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nfsarch33/helix-dev-tools/internal/sourceaudit"
	"github.com/spf13/cobra"
)

var sourceAuditJSON bool

var skillAuditSourcesCmd = &cobra.Command{
	Use:   "skill-audit-sources [skills-dir]",
	Short: "Audit agent skills for source URL retention",
	Long:  "Scans SKILL.md files for <!-- Source: URL --> markers. Reports skills missing source links for periodic upstream review.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		skillsDir := filepath.Join(home, ".cursor", "skills")
		if len(args) > 0 {
			skillsDir = args[0]
		}

		results, err := sourceaudit.ScanAll(skillsDir)
		if err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		if sourceAuditJSON {
			out, marshalErr := json.MarshalIndent(results, "", "  ")
			if marshalErr != nil {
				return marshalErr
			}
			fmt.Println(string(out))
			return nil
		}

		withSources := 0
		withoutSources := 0
		for _, r := range results {
			if r.HasSources {
				withSources++
			} else {
				withoutSources++
			}
		}

		fmt.Printf("Skill Source Audit: %s\n", skillsDir)
		fmt.Printf("  Total skills: %d\n", len(results))
		fmt.Printf("  With sources: %d\n", withSources)
		fmt.Printf("  Missing sources: %d\n\n", withoutSources)

		if withoutSources > 0 {
			fmt.Println("Skills missing source URLs:")
			for _, r := range results {
				if !r.HasSources {
					fmt.Printf("  - %s\n", r.Name)
				}
			}
		}

		if withSources > 0 {
			fmt.Println("\nSkills with tracked sources:")
			for _, r := range results {
				if r.HasSources {
					fmt.Printf("  %s:\n", r.Name)
					for _, s := range r.Sources {
						fmt.Printf("    -> %s\n", s)
					}
				}
			}
		}

		return nil
	},
}

func init() {
	skillAuditSourcesCmd.Flags().BoolVar(&sourceAuditJSON, "json", false, "Output as JSON")
}
