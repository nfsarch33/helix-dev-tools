package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/health"
)

var healthCheckCmd = &cobra.Command{
	Use:   "health-check",
	Short: "Run 19-suite integration health check",
	RunE:  runHealthCheck,
}

func runHealthCheck(_ *cobra.Command, _ []string) error {
	clilog.Header("cursor-tools health-check")

	changes, _ := SyncCountsApply(true, true)
	if changes > 0 {
		clilog.Info("sync-counts: fixed %d index drift(s)", changes)
	}

	p := config.DefaultPaths()
	runner := health.NewRunner()

	for _, s := range health.BuildAllSuites(p) {
		runner.Add(s)
	}

	pass, total := runner.Run()
	if pass < total {
		os.Exit(1)
	}
	return nil
}
