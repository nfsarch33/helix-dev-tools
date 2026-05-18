package cli

import (
	"fmt"
	"os"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/sprintboard"
	"github.com/nfsarch33/helix-dev-tools/internal/platform/sprintcli"
	"github.com/spf13/cobra"
)

var handoffGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate handoff document from a sprint board ticket",
	RunE: func(cmd *cobra.Command, args []string) error {
		ticket, _ := cmd.Flags().GetString("ticket")
		toAgent, _ := cmd.Flags().GetString("to")

		cli, err := sprintcli.New(sprintboard.DefaultDBPath())
		if err != nil {
			return err
		}
		defer cli.Close()

		content, err := cli.GenerateHandoff(ticket, toAgent)
		if err != nil {
			return err
		}

		fmt.Fprint(os.Stdout, content)
		return nil
	},
}

var handoffCmd = &cobra.Command{
	Use:   "handoff",
	Short: "Multi-agent handoff management",
}

func init() {
	handoffGenerateCmd.Flags().String("ticket", "", "Ticket ID to generate handoff for")
	handoffGenerateCmd.Flags().String("to", "", "Target agent (codex, claude-code, cursor-parent)")
	handoffGenerateCmd.MarkFlagRequired("ticket")
	handoffGenerateCmd.MarkFlagRequired("to")

	handoffCmd.AddCommand(handoffGenerateCmd)
}
