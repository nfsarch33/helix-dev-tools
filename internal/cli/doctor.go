package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/health"
)

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

var doctorResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Verify intermediate-state sync, docs, and resume readiness",
	RunE: func(_ *cobra.Command, _ []string) error {
		return runDoctorProfile("resume")
	},
}

func init() {
	doctorCmd.AddCommand(doctorInstallCmd)
	doctorCmd.AddCommand(doctorMCPCmd)
	doctorCmd.AddCommand(doctorPlatformCmd)
	doctorCmd.AddCommand(doctorResumeCmd)
}

func runDoctorProfile(profile string) error {
	started := time.Now()
	p := config.DefaultPaths()

	if profile == "all" || profile == "install" || profile == "resume" {
		changes, _ := SyncCountsApply(true, true)
		if changes > 0 {
			clilog.Info("sync-counts: fixed %d index drift(s)", changes)
		}
	}

	title := "cursor-tools doctor"
	metricName := "doctor"
	if profile != "all" {
		title += " " + profile
		metricName += "-" + profile
	}

	pass, total := runSuites(title, health.BuildDoctorSuites(p, profile))
	recordCheckRun(metricName, started, pass, total)
	if pass < total {
		return fmt.Errorf("%s failed: %d/%d passed", metricName, pass, total)
	}
	return nil
}
