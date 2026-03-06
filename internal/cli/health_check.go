package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var healthCheckCmd = &cobra.Command{
	Use:   "health-check",
	Short: "Run 19-suite integration test (replaces system-health-check.py)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("health-check: not yet implemented (Phase 3)")
		return nil
	},
}
