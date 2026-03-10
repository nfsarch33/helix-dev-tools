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
	days    int
	export  string
	compact bool
	analyse bool
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
	metricsCmd.Flags().BoolVar(&metricsFlags.compact, "compact", false, "Single-line output for embedding in prompts")
	metricsCmd.Flags().BoolVar(&metricsFlags.analyse, "analyse", false, "Show actionable recommendations from data patterns")
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

	if metricsFlags.compact {
		fmt.Println(summary.Compact(metricsFlags.days))
		return nil
	}

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

	if len(summary.Categories) > 0 {
		fmt.Println("\n  Operation Timing by Category:")
		fmt.Printf("  %-12s %6s %10s %10s %10s %10s\n", "Category", "Count", "Avg(ms)", "P95(ms)", "Max(ms)", "Total(s)")
		clilog.Divider()
		for _, c := range summary.Categories {
			fmt.Printf("  %-12s %6d %10.0f %10d %10d %10.1f\n",
				c.Category, c.Count, c.AvgDuration, c.P95Duration, c.MaxDuration, float64(c.TotalMs)/1000)
		}
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

	if len(summary.Skills) > 0 {
		fmt.Println("\n  Skill Activations (last 7d):")
		fmt.Printf("  %-30s %6s %8s\n", "Skill", "Uses", "Avg(ms)")
		clilog.Divider()
		for i, sk := range summary.Skills {
			if i >= 15 {
				break
			}
			fmt.Printf("  %-30s %6d %8.0f\n", sk.Name, sk.Uses, sk.AvgMs)
		}
	}

	if len(summary.MCPServers) > 0 {
		fmt.Println("\n  MCP Server Calls (last 7d):")
		fmt.Printf("  %-18s %-28s %6s %8s\n", "Server", "Tool", "Uses", "Avg(ms)")
		clilog.Divider()
		for i, m := range summary.MCPServers {
			if i >= 15 {
				break
			}
			server := m.Server
			if server == "" {
				server = "(unknown)"
			}
			fmt.Printf("  %-18s %-28s %6d %8.0f\n", server, m.Tool, m.Uses, m.AvgMs)
		}
	}

	if len(summary.Subagents) > 0 {
		fmt.Println("\n  Subagent Invocations (last 7d):")
		fmt.Printf("  %-30s %6s\n", "Agent", "Uses")
		clilog.Divider()
		for _, sa := range summary.Subagents {
			fmt.Printf("  %-30s %6d\n", sa.Detail, sa.Count)
		}
	}

	// Adoption funnel
	skillCount := len(summary.Skills)
	mcpServerSet := make(map[string]bool)
	for _, m := range summary.MCPServers {
		if m.Server != "" {
			mcpServerSet[m.Server] = true
		}
	}
	subCount := len(summary.Subagents)
	if skillCount > 0 || len(mcpServerSet) > 0 || subCount > 0 {
		fmt.Println("\n  Adoption Funnel:")
		fmt.Printf("    Skills activated:       %d of 89 installed\n", skillCount)
		fmt.Printf("    MCP servers used:       %d of 9 always-on\n", len(mcpServerSet))
		fmt.Printf("    Subagents invoked:      %d of 6 available\n", subCount)
	}

	fmt.Println()

	recs := summary.Analyse()
	if metricsFlags.analyse || len(recs) > 0 {
		if len(recs) > 0 {
			fmt.Println("  Recommendations:")
			for _, r := range recs {
				icon := "  INFO "
				if r.Severity == "warn" {
					icon = "  WARN "
				} else if r.Severity == "critical" {
					icon = "  CRIT "
				}
				fmt.Printf("  %s [%s] %s\n", icon, r.Category, r.Message)
			}
			fmt.Println()
		} else if metricsFlags.analyse {
			clilog.Success("No recommendations. System is healthy.")
			fmt.Println()
		}
	}

	if metricsFlags.export != "" {
		md := summary.Markdown()
		if err := os.WriteFile(metricsFlags.export, []byte(md), 0o644); err != nil {
			return fmt.Errorf("writing export: %w", err)
		}
		clilog.Success("exported to %s", metricsFlags.export)
	}

	return nil
}
