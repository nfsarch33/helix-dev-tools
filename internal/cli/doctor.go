package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/clilog"
	"github.com/nfsarch33/helix-dev-tools/internal/config"
	"github.com/nfsarch33/helix-dev-tools/internal/health"
)

var doctorSyncCountsApply = SyncCountsApply
var doctorBuildSuites = health.BuildDoctorSuites
var doctorRunSuites = runSuites
var doctorRecordCheckRun = recordCheckRunWithContext
var doctorWriteCheckJSON = writeCheckJSON
var doctorOutputJSON bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run targeted install, MCP, platform, and resume checks",
	Long:  "Doctor reuses the shared health suites to verify fresh installs, MCP readiness, platform assumptions, and resume-state drift.",
	RunE: func(_ *cobra.Command, _ []string) error {
		return runDoctorProfile("all")
	},
}

var doctorInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Verify fresh-install prerequisites and symlinked tooling",
	RunE: func(_ *cobra.Command, _ []string) error {
		return runDoctorProfile("install")
	},
}

var doctorMCPCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Verify MCP config, commands, env placeholders, and absolute paths",
	RunE: func(_ *cobra.Command, _ []string) error {
		return runDoctorProfile("mcp")
	},
}

var doctorPlatformCmd = &cobra.Command{
	Use:   "platform",
	Short: "Verify macOS or WSL/Linux platform assumptions",
	RunE: func(_ *cobra.Command, _ []string) error {
		return runDoctorProfile("platform")
	},
}

var doctorDepsCmd = &cobra.Command{
	Use:   "deps",
	Short: "Verify required CLI dependencies and core toolchain versions",
	RunE: func(_ *cobra.Command, _ []string) error {
		return runDoctorProfile("deps")
	},
}

var doctorResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Verify intermediate-state sync, docs, and resume readiness",
	RunE: func(_ *cobra.Command, _ []string) error {
		return runDoctorProfile("resume")
	},
}

var doctorStackCmd = &cobra.Command{
	Use:   "stack",
	Short: "Run agent-doctor checks across evolver, research, selfimprove subsystems",
	RunE: func(_ *cobra.Command, _ []string) error {
		return runDoctorProfile("stack")
	},
}

var doctorDRLCmd = &cobra.Command{
	Use:   "drl",
	Short: "Probe DRL Prometheus, healthz, Pushgateway; Mem0, signals, self-improve pipeline",
	Long:  "Soft by default when the DRL stack is down. Set FLEET_DRL_DOCTOR_STRICT=1 to require HTTP 2xx on probes. FLEET_DRL_DOCTOR_SKIP=1 skips HTTP probes.",
	RunE: func(_ *cobra.Command, _ []string) error {
		return runDoctorProfile("drl")
	},
}

func init() {
	doctorCmd.AddCommand(doctorInstallCmd)
	doctorCmd.AddCommand(doctorMCPCmd)
	doctorCmd.AddCommand(doctorPlatformCmd)
	doctorCmd.AddCommand(doctorDepsCmd)
	doctorCmd.AddCommand(doctorResumeCmd)
	doctorCmd.AddCommand(doctorStackCmd)
	doctorCmd.AddCommand(doctorDRLCmd)
	doctorCmd.PersistentFlags().BoolVar(&doctorOutputJSON, "json", false, "Output results as JSON")
}

func runDoctorProfile(profile string) error {
	started := time.Now()
	p := config.DefaultPaths()

	if profile == "all" || profile == "install" || profile == "resume" {
		changes, _ := doctorSyncCountsApply(true, true)
		if changes > 0 {
			clilog.Info("sync-counts: fixed %d index drift(s)", changes)
		}
	}

	title := "cursor-tools doctor"
	metricName := "doctor"
	metricProfile := profile
	if profile != "all" {
		title += " " + profile
		metricName += "-" + profile
	} else {
		metricProfile = ""
	}

	suites := doctorBuildSuites(p, profile)
	pass, total := summarizeSuites(suites)
	if !doctorOutputJSON {
		pass, total = doctorRunSuites(title, suites)
	}
	runID := doctorRecordCheckRun(metricName, "doctor", metricProfile, started, pass, total)
	recordCheckSuiteRuns("doctor", metricProfile, runID, suites)
	if doctorOutputJSON {
		if err := doctorWriteCheckJSON(title, "doctor", metricProfile, runID, suites); err != nil {
			return err
		}
	}
	if pass < total {
		return fmt.Errorf("%s failed: %d/%d passed", metricName, pass, total)
	}
	return nil
}
