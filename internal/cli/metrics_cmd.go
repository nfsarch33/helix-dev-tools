package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

var metricsFlags struct {
	days   int
	export string
}

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Show hook and system performance metrics",
	Long:  "Analyse metrics.jsonl to show hook latency, intervention rates, and top blocked commands.",
	RunE:  runMetrics,
}

func init() {
	metricsCmd.Flags().IntVar(&metricsFlags.days, "days", 7, "Number of days to include in the report")
	metricsCmd.Flags().StringVar(&metricsFlags.export, "export", "", "Export markdown report to file (e.g. ~/memo/global-memories/system-performance.md)")
}

func runMetrics(_ *cobra.Command, _ []string) error {
	p := config.DefaultPaths()
	metricsPath := p.MetricsFile()

	events, err := metrics.Load(metricsPath)
	if err != nil {
		return fmt.Errorf("loading metrics: %w", err)
	}

	since := time.Now().UTC().Add(-time.Duration(metricsFlags.days) * 24 * time.Hour)
	summary := metrics.Summarise(events, since)

	clilog.Header("cursor-tools metrics")
	fmt.Printf("\n  Period: last %d days (%s to %s)\n", metricsFlags.days,
		since.Format("2006-01-02"), time.Now().UTC().Format("2006-01-02"))
	fmt.Printf("  Total events: %d\n\n", summary.TotalEvents)

	if summary.TotalEvents == 0 {
		clilog.Info("No metrics data for this period. Metrics are recorded automatically by hooks.")
		return nil
	}

	fmt.Printf("  %-18s %6s %5s %5s %5s %8s %8s\n", "Hook", "Total", "Deny", "Warn", "Allow", "Avg(ms)", "Max(ms)")
	clilog.Divider()

	totalDeny, totalWarn, totalAll := 0, 0, 0
	for _, h := range summary.Hooks {
		fmt.Printf("  %-18s %6d %5d %5d %5d %8.1f %8d\n",
			h.Hook, h.Total, h.DenyCount, h.WarnCount, h.AllowCount, h.AvgLatency, h.MaxLatency)
		totalDeny += h.DenyCount
		totalWarn += h.WarnCount
		totalAll += h.Total
	}

	clilog.Divider()
	if totalAll > 0 {
		rate := float64(totalDeny+totalWarn) / float64(totalAll) * 100
		fmt.Printf("\n  Intervention rate: %d/%d = %.1f%% (deny+warn / total)\n",
			totalDeny+totalWarn, totalAll, rate)
	}

	if len(summary.TopDenied) > 0 {
		fmt.Println("\n  Top blocked (deny):")
		for i, d := range summary.TopDenied {
			if i >= 5 {
				break
			}
			detail := d.Detail
			if len(detail) > 60 {
				detail = detail[:60] + "..."
			}
			fmt.Printf("    %d. %s (%dx)\n", i+1, detail, d.Count)
		}
	}
	fmt.Println()

	if metricsFlags.export != "" {
		md := summary.Markdown()
		if err := os.WriteFile(metricsFlags.export, []byte(md), 0o644); err != nil {
			return fmt.Errorf("writing export: %w", err)
		}
		clilog.Success("exported to %s", metricsFlags.export)
	}

	return nil
}
