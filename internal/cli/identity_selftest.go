package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/identity"
)

var identityCmd = &cobra.Command{
	Use:   "identity",
	Short: "Personal-repo identity and shell hygiene helpers",
}

var identitySelftestCmd = &cobra.Command{
	Use:   "selftest",
	Short: "Verify the current shell is clean for personal-repo push gates",
	RunE: func(cmd *cobra.Command, _ []string) error {
		env := make(map[string]string, len(identity.TokenKeys))
		for _, key := range identity.TokenKeys {
			env[key] = os.Getenv(key)
		}
		result := identity.SelfTest(env)
		for _, failure := range result.Failures {
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "FAIL "+failure)
		}
		if !result.Pass {
			return fmt.Errorf("identity selftest failed: %d failure(s)", len(result.Failures))
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "PASS identity selftest")
		return nil
	},
}

func init() {
	identityCmd.AddCommand(identitySelftestCmd)
}
