package cli

import "github.com/spf13/cobra"

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Cursor hook handlers (stdin JSON -> stdout JSON)",
}

func init() {
	hookCmd.AddCommand(guardShellCmd)
	hookCmd.AddCommand(sanitizeReadCmd)
	hookCmd.AddCommand(guardMcpCmd)
	hookCmd.AddCommand(postEditCmd)
	hookCmd.AddCommand(housekeepingCmd)
	hookCmd.AddCommand(guardNoShellLeakSyncCmd)
}
