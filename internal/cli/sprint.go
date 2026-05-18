package cli

import (
	"fmt"
	"os"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/sprintboard"
	"github.com/nfsarch33/helix-dev-tools/internal/platform/sprintcli"
	"github.com/spf13/cobra"
)

var sprintCmd = &cobra.Command{
	Use:   "sprint",
	Short: "Multi-agent sprint board management",
}

var sprintCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new sprint",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		name, _ := cmd.Flags().GetString("name")
		theme, _ := cmd.Flags().GetString("theme")

		cli, err := sprintcli.New(sprintboard.DefaultDBPath())
		if err != nil {
			return err
		}
		defer cli.Close()

		msg, err := cli.CreateSprint(id, name, theme)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, msg)
		return nil
	},
}

var sprintListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sprints",
	RunE: func(cmd *cobra.Command, args []string) error {
		cli, err := sprintcli.New(sprintboard.DefaultDBPath())
		if err != nil {
			return err
		}
		defer cli.Close()

		output, err := cli.ListSprints()
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, output)
		return nil
	},
}

var sprintStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sprint status with ticket summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		cli, err := sprintcli.New(sprintboard.DefaultDBPath())
		if err != nil {
			return err
		}
		defer cli.Close()

		output, err := cli.SprintStatus(id)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, output)
		return nil
	},
}

var sprintAssignCmd = &cobra.Command{
	Use:   "assign",
	Short: "Assign a ticket to an agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		ticket, _ := cmd.Flags().GetString("ticket")
		agent, _ := cmd.Flags().GetString("agent")

		cli, err := sprintcli.New(sprintboard.DefaultDBPath())
		if err != nil {
			return err
		}
		defer cli.Close()

		msg, err := cli.AssignTicket(ticket, agent)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, msg)
		return nil
	},
}

var sprintKickoffCmd = &cobra.Command{
	Use:   "kickoff",
	Short: "Generate agent kickoff prompt for a sprint",
	RunE: func(cmd *cobra.Command, args []string) error {
		sprintID, _ := cmd.Flags().GetString("sprint")
		agent, _ := cmd.Flags().GetString("agent")

		cli, err := sprintcli.New(sprintboard.DefaultDBPath())
		if err != nil {
			return err
		}
		defer cli.Close()

		output, err := cli.GenerateKickoff(sprintID, agent)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, output)
		return nil
	},
}

func init() {
	sprintCreateCmd.Flags().String("id", "", "Sprint ID (e.g. v6140)")
	sprintCreateCmd.Flags().String("name", "", "Sprint name")
	sprintCreateCmd.Flags().String("theme", "", "Sprint theme")
	sprintCreateCmd.MarkFlagRequired("id")
	sprintCreateCmd.MarkFlagRequired("name")

	sprintStatusCmd.Flags().String("id", "", "Sprint ID")
	sprintStatusCmd.MarkFlagRequired("id")

	sprintAssignCmd.Flags().String("ticket", "", "Ticket ID")
	sprintAssignCmd.Flags().String("agent", "", "Agent ID")
	sprintAssignCmd.MarkFlagRequired("ticket")
	sprintAssignCmd.MarkFlagRequired("agent")

	sprintKickoffCmd.Flags().String("sprint", "", "Sprint ID")
	sprintKickoffCmd.Flags().String("agent", "", "Agent ID")
	sprintKickoffCmd.MarkFlagRequired("sprint")
	sprintKickoffCmd.MarkFlagRequired("agent")

	sprintCmd.AddCommand(sprintCreateCmd)
	sprintCmd.AddCommand(sprintListCmd)
	sprintCmd.AddCommand(sprintStatusCmd)
	sprintCmd.AddCommand(sprintAssignCmd)
	sprintCmd.AddCommand(sprintKickoffCmd)
}
