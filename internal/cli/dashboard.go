package cli

import "github.com/spf13/cobra"

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Helix dashboard subcommands",
	Long:  "Parent command for dashboard operations. Use `dashboard serve` to start the HTTP server.",
}

var dashboardServeChildCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the helix dashboard HTTP server",
	Long: `Serves the helix dashboard on the specified port.
This is the canonical form; the top-level dashboard-serve command remains as a backward-compatible alias.`,
	RunE: runDashboardServe,
}

func init() {
	dashboardServeChildCmd.Flags().IntVar(&dashboardPort, "port", 9095, "HTTP listen port")
	dashboardServeChildCmd.Flags().StringVar(&dashboardManifest, "manifest", "", "roadmap YAML manifest path")
	dashboardServeChildCmd.Flags().StringVar(&dashboardEngramURL, "engram-url", "http://localhost:8281", "Engram base URL")
	dashboardCmd.AddCommand(dashboardServeChildCmd)
}
