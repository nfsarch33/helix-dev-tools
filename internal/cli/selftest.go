package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var selftestCmd = &cobra.Command{
	Use:   "selftest",
	Short: "Run hook unit tests (replaces test_hooks.py)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("selftest: not yet implemented (Phase 3)")
		return nil
	},
}
