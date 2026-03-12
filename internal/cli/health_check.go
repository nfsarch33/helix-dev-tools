package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/health"
)

var healthCheckSyncCountsApply = SyncCountsApply
var healthCheckBuildSuites = health.BuildAllSuites
var healthCheckRunSuites = runSuites
var healthCheckRecordCheckRun = recordCheckRunWithContext
var healthCheckWriteCheckJSON = writeCheckJSON
var healthCheckOutputJSON bool

var healthCheckCmd = &cobra.Command{
	Use:   "health-check",
	Short: "Run the full integration health check suite",
	RunE:  runHealthCheck,
}

func init() {
	healthCheckCmd.Flags().BoolVar(&healthCheckOutputJSON, "json", false, "Output results as JSON")
}

func runHealthCheck(_ *cobra.Command, _ []string) error {
	started := time.Now()

	changes, _ := healthCheckSyncCountsApply(true, true)
	if changes > 0 {
		clilog.Info("sync-counts: fixed %d index drift(s)", changes)
	}

	p := config.DefaultPaths()
	suites := healthCheckBuildSuites(p)
	pass, total := summarizeSuites(suites)
	if !healthCheckOutputJSON {
		pass, total = healthCheckRunSuites("cursor-tools health-check", suites)
	}
	runID := healthCheckRecordCheckRun("health-check", "health-check", "", started, pass, total)
	recordCheckSuiteRuns("health-check", "", runID, suites)
	if healthCheckOutputJSON {
		if err := healthCheckWriteCheckJSON("cursor-tools health-check", "health-check", "", runID, suites); err != nil {
			return err
		}
	}
	if pass < total {
		return fmt.Errorf("health-check failed: %d/%d passed", pass, total)
	}
	return nil
}
