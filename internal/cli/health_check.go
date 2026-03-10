package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/health"
)

var healthCheckCmd = &cobra.Command{
	Use:   "health-check",
	Short: "Run the full integration health check suite",
	RunE:  runHealthCheck,
}

func runHealthCheck(_ *cobra.Command, _ []string) error {
	started := time.Now()

	changes, _ := SyncCountsApply(true, true)
	if changes > 0 {
		clilog.Info("sync-counts: fixed %d index drift(s)", changes)
	}

	p := config.DefaultPaths()
	pass, total := runSuites("cursor-tools health-check", health.BuildAllSuites(p))
	recordCheckRun("health-check", started, pass, total)
	if pass < total {
		return fmt.Errorf("health-check failed: %d/%d passed", pass, total)
	}
	return nil
}
