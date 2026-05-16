package cli

import "github.com/spf13/cobra"

var version = "dev"

// SetVersion sets the version string for the version command.
func SetVersion(v string) {
	version = v
}

var rootCmd = &cobra.Command{
	Use:   "cursor-tools",
	Short: "Cursor IDE memory system tools",
	Long:  "Single binary for Cursor hooks, git hooks, health checks, and memory system management.",
}

func init() {
	rootCmd.AddCommand(hookCmd)
	rootCmd.AddCommand(githookCmd)
	rootCmd.AddCommand(syncCountsCmd)
	rootCmd.AddCommand(promoteCmd)
	rootCmd.AddCommand(healthCheckCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(identityCmd)
	rootCmd.AddCommand(selftestCmd)
	rootCmd.AddCommand(bootstrapCmd)
	rootCmd.AddCommand(dailyRefreshCmd)
	rootCmd.AddCommand(mcpIndexCmd)
	rootCmd.AddCommand(metricsCmd)
	rootCmd.AddCommand(trackCmd)
	rootCmd.AddCommand(mem0ParityCmd)
	rootCmd.AddCommand(mem0OutboxCmd)
	rootCmd.AddCommand(mem0CanaryCmd)
	rootCmd.AddCommand(mem0DrainCmd)
	rootCmd.AddCommand(memoryRoutineCmd)
	rootCmd.AddCommand(safeCmd)
	rootCmd.AddCommand(skillvetAuditCmd)
	rootCmd.AddCommand(skillAuditSourcesCmd)
	rootCmd.AddCommand(worktreeCmd)
	rootCmd.AddCommand(workspaceCmd)
	rootCmd.AddCommand(openclawCmd)
	rootCmd.AddCommand(sessionHandoffCmd)
	rootCmd.AddCommand(handoffReviewCmd)
	rootCmd.AddCommand(signalCmd)
	rootCmd.AddCommand(outcomeCmd)
	rootCmd.AddCommand(evoloopCmd)
	rootCmd.AddCommand(autoUpdateCmd)
	rootCmd.AddCommand(claudeUsageCmd)
	rootCmd.AddCommand(claudeRunCmd)
	rootCmd.AddCommand(tokenUsageCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(promPushCmd)
	rootCmd.AddCommand(fleetPreflightCmd)
	rootCmd.AddCommand(replicateCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(sprintScaffoldCmd)
	rootCmd.AddCommand(docsyncCmd)
	rootCmd.AddCommand(docsCheckCmd)
	rootCmd.AddCommand(branchCleanupCmd)
	rootCmd.AddCommand(sessionIndexCmd)
	rootCmd.AddCommand(mem0UsageCmd)
	rootCmd.AddCommand(mem0ExportCmd)
	rootCmd.AddCommand(mcpFilterCmd)
	rootCmd.AddCommand(observabilityReportCmd)
	rootCmd.AddCommand(agentraceSearchCmd)
	rootCmd.AddCommand(githubCmd)
	rootCmd.AddCommand(rebrandCmd)
	rootCmd.AddCommand(installCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		printVersion(cmd.OutOrStdout(), version)
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
