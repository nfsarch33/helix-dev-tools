package cli

import "github.com/spf13/cobra"

var githookCmd = &cobra.Command{
	Use:   "githook",
	Short: "Git hook handlers",
}

func init() {
	githookCmd.AddCommand(commitMsgCmd)
	githookCmd.AddCommand(prePushCmd)
	githookCmd.AddCommand(preCommitCmd)
}
