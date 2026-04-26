package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/claude"
	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/evoloop"
	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

var metricsFlags struct {
	days    int
	export  string
	compact bool
	analyse bool
	doctor  bool
	fleet   bool
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
	metricsCmd.Flags().BoolVar(&metricsFlags.doctor, "doctor", false, "Include agent-doctor subsystem health summary")
	metricsCmd.Flags().BoolVar(&metricsFlags.fleet, "fleet", false, "Show fleet EvoLoop parity from shared Mem0")
}

func runMetrics(_ *cobra.Command, _ []string) error {
	if metricsFlags.fleet {
		return runFleetMetrics()
	}
	p := config.DefaultPaths()
	metricsPath := p.MetricsFile()

	events, err := metrics.LoadAll(metricsPath)
	if err != nil {
		return fmt.Errorf("loading metrics: %w", err)
	}

	since := time.Now().UTC().Add(-time.Duration(metricsFlags.days) * 24 * time.Hour)
	summary := metrics.Summarise(events, since)
	installedSkills := countSkillDirs(p.SkillsDir, map[string]bool{"00-index": true}) + countSkillDirs(p.AgentsSkillsDir, nil)
	installedSubagents := countFiles(p.AgentsDir, ".md")
	alwaysOnMCP := []string{
		"git-mcp-server", "context-mode", "mem0", "github-official",
		"context7", "duckduckgo", "perplexity", "fetch", "wolfram-alpha",
	}

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

	if len(summary.Checks) > 0 {
		fmt.Println("\n  Self-Check Pass Rates:")
		fmt.Printf("  %-18s %6s %6s %6s %12s %14s %8s\n", "Check", "Runs", "Pass", "Fail", "RunRate", "AssertRate", "Avg(ms)")
		clilog.Divider()
		for _, check := range summary.Checks {
			runRate := 0.0
			if check.Runs > 0 {
				runRate = float64(check.Passes) / float64(check.Runs) * 100
			}
			fmt.Printf("  %-18s %6d %6d %6d %11.1f%% %13.1f%% %8.0f\n",
				check.Name, check.Runs, check.Passes, check.Fails, runRate, check.AssertionPassRate, check.AvgMs)
		}
	}

	if len(summary.MemoryLayers) > 0 {
		fmt.Println("\n  Memory Layer KPIs:")
		fmt.Printf("  %-14s %6s %7s %6s %6s %7s %5s %5s %5s %7s %8s %9s %8s %9s\n",
			"Layer", "Uses", "Search", "Read", "Write", "Update", "Hit", "Miss", "Empty", "Unknown", "Observed", "Coverage", "HitRate", "AvgCount")
		clilog.Divider()
		for _, layer := range summary.MemoryLayers {
			hitRate := "n/a"
			if rate, ok := layer.ObservedHitRate(); ok {
				hitRate = fmt.Sprintf("%.1f%%", rate)
			}
			coverage := "n/a"
			if rate, ok := layer.OutcomeCoverage(); ok {
				coverage = fmt.Sprintf("%.1f%%", rate)
			}
			avgCount := "n/a"
			if layer.AvgResultCount > 0 {
				avgCount = fmt.Sprintf("%.1f", layer.AvgResultCount)
			}
			fmt.Printf("  %-14s %6d %7d %6d %6d %7d %5d %5d %5d %7d %8d %9s %8s %9s\n",
				layer.Layer, layer.Total, layer.Searches, layer.Reads, layer.WriteOps, layer.UpdateOps,
				layer.Hits, layer.Misses, layer.Empty, layer.Unknown, layer.Observed, coverage, hitRate, avgCount)
		}
		fmt.Println("    observed = explicit retrieval outcomes (hit/miss/empty)")
		fmt.Println("    coverage = observed outcomes divided by retrieval attempts (search + read)")
		fmt.Println("    unknown  = layer usage recorded without an explicit reported retrieval outcome")
	}

	// Adoption funnel
	skillCount := len(summary.Skills)
	skillUses := 0
	for _, sk := range summary.Skills {
		skillUses += sk.Uses
	}
	mcpServerSet := make(map[string]bool)
	mcpUses := 0
	ironclawUses := 0
	for _, m := range summary.MCPServers {
		if m.Server != "" {
			mcpServerSet[metrics.CanonicalMCPServerName(m.Server)] = true
		}
		mcpUses += m.Uses
		if metrics.CanonicalMCPServerName(m.Server) == "ironclaw" {
			ironclawUses += m.Uses
		}
	}
	subCount := len(summary.Subagents)
	subUses := 0
	for _, sa := range summary.Subagents {
		subUses += sa.Count
	}
	if summary.Tasks.Total > 0 || skillCount > 0 || len(mcpServerSet) > 0 || subCount > 0 {
		fmt.Println("\n  Adoption Funnel:")
		if summary.Tasks.Total > 0 {
			fmt.Printf("    Skill task coverage:    %d of %d tasks (%.1f%%)\n", summary.Tasks.SkillTasks, summary.Tasks.Total, percentage(summary.Tasks.SkillTasks, summary.Tasks.Total))
			fmt.Printf("    MCP task coverage:      %d of %d tasks (%.1f%%)\n", summary.Tasks.MCPTasks, summary.Tasks.Total, percentage(summary.Tasks.MCPTasks, summary.Tasks.Total))
			fmt.Printf("    IronClaw task coverage: %d of %d tasks (%.1f%%)\n", summary.Tasks.IronclawTasks, summary.Tasks.Total, percentage(summary.Tasks.IronclawTasks, summary.Tasks.Total))
			fmt.Printf("    Subagent task coverage: %d of %d tasks (%.1f%%)\n", summary.Tasks.SubagentTasks, summary.Tasks.Total, percentage(summary.Tasks.SubagentTasks, summary.Tasks.Total))
			fmt.Printf("    Task grouping confidence: exact=%d turn=%d heuristic=%d\n", summary.Tasks.ExactTasks, summary.Tasks.TurnTasks, summary.Tasks.HeuristicTasks)
		}
		fmt.Printf("    Skills activated:       %d of %d installed (%.1f%%)\n", skillCount, installedSkills, percentage(skillCount, installedSkills))
		fmt.Printf("    Skill hit rate:         %d of %d events (%.1f%%)\n", skillUses, summary.TotalEvents, percentage(skillUses, summary.TotalEvents))
		fmt.Printf("    MCP servers used:       %d of %d always-on (%.1f%%)\n", len(mcpServerSet), len(alwaysOnMCP), percentage(len(mcpServerSet), len(alwaysOnMCP)))
		fmt.Printf("    MCP hit rate:           %d of %d events (%.1f%%)\n", mcpUses, summary.TotalEvents, percentage(mcpUses, summary.TotalEvents))
		fmt.Printf("    IronClaw MCP share:     %d of %d MCP events (%.1f%%)\n", ironclawUses, mcpUses, percentage(ironclawUses, mcpUses))
		fmt.Printf("    Subagents invoked:      %d of %d available (%.1f%%)\n", subCount, installedSubagents, percentage(subCount, installedSubagents))
		fmt.Printf("    Subagent hit rate:      %d of %d events (%.1f%%)\n", subUses, summary.TotalEvents, percentage(subUses, summary.TotalEvents))
	}

	fmt.Println()

	printClaudeUsageSection(metricsFlags.days)

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

	if metricsFlags.doctor {
		printDoctorMetrics(p)
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

func runFleetMetrics() error {
	client, err := evoloopFactory(config.DefaultPaths(), nil)
	if err != nil {
		return fmt.Errorf("fleet metrics: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rollups, err := client.Recent(ctx, evoloop.RecentOptions{
		Kinds: []evoloop.CapsuleKind{evoloop.KindRollup},
		Limit: 20,
	})
	if err != nil {
		return fmt.Errorf("fleet metrics rollups: %w", err)
	}
	clilog.Header("cursor-tools metrics --fleet")
	if len(rollups) == 0 {
		clilog.Info("No EvoLoop rollups found.")
		return nil
	}
	fmt.Printf("\n  %-18s %-10s %8s %8s %10s\n", "Machine", "Day", "Cycles", "Improved", "LastKPI")
	clilog.Divider()
	for _, r := range rollups {
		fmt.Printf("  %-18s %-10s %8d %8d %10.3f\n", r.Machine, r.Day, r.Cycles, r.Improved, r.LastKPI)
	}
	fmt.Println()
	return nil
}

func printClaudeUsageSection(days int) {
	dir := claude.DefaultUsageDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return
	}

	now := time.Now().UTC()
	var allRecords []claude.Usage
	for i := 0; i < days; i++ {
		day := now.AddDate(0, 0, -i)
		path := claude.UsageFilePath(dir, day)
		records, err := claude.ReadUsage(path)
		if err != nil {
			continue
		}
		allRecords = append(allRecords, records...)
	}

	if len(allRecords) == 0 {
		return
	}

	var totalPromptBytes, totalOutputBytes int
	var totalDurMs int64
	var totalCost float64
	var errCount int
	backends := map[string]int{}

	for _, r := range allRecords {
		totalPromptBytes += r.PromptBytes
		totalOutputBytes += r.OutputBytes
		totalDurMs += r.DurationMs
		totalCost += r.Cost
		if r.ExitCode != 0 {
			errCount++
		}
		backends[r.Backend]++
	}

	fmt.Println("  Claude CLI Usage:")
	fmt.Printf("    Invocations:   %d\n", len(allRecords))
	fmt.Printf("    Prompt bytes:  %s\n", formatBytes(totalPromptBytes))
	fmt.Printf("    Output bytes:  %s\n", formatBytes(totalOutputBytes))
	fmt.Printf("    Duration:      %s\n", formatDuration(totalDurMs))
	if totalCost > 0 {
		fmt.Printf("    Cost:          $%.4f\n", totalCost)
	}
	fmt.Printf("    Errors:        %d / %d\n", errCount, len(allRecords))
	for b, c := range backends {
		fmt.Printf("    Backend %-10s %d calls\n", b+":", c)
	}
	fmt.Println()
}

func percentage(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator) * 100
}

func printDoctorMetrics(p config.Paths) {
	agentDoctorBin := filepath.Join(p.Home, "ai-agent-business-stack", "go", "bin", "agent-doctor")
	if _, err := os.Stat(agentDoctorBin); err != nil {
		var lookErr error
		agentDoctorBin, lookErr = exec.LookPath("agent-doctor")
		if lookErr != nil {
			clilog.Info("agent-doctor not installed, skipping --doctor section")
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, agentDoctorBin, "all", "--json")
	out, _ := cmd.CombinedOutput()
	if len(out) == 0 {
		clilog.Warn("agent-doctor produced no output")
		return
	}

	var report struct {
		Suites []struct {
			Name   string `json:"name"`
			Checks []struct {
				Name    string `json:"name"`
				Status  string `json:"status"`
				Message string `json:"message"`
			} `json:"checks"`
			DurationMs int64 `json:"duration_ms"`
		} `json:"suites"`
		Overall    string `json:"overall"`
		DurationMs int64  `json:"duration_ms"`
	}
	if err := json.Unmarshal(out, &report); err != nil {
		clilog.Warn("agent-doctor output not parseable: %v", err)
		return
	}

	fmt.Println("  Agent Doctor Health:")
	fmt.Printf("  %-16s %-6s %5s %5s %5s %10s\n", "Suite", "Health", "Pass", "Warn", "Fail", "Time(ms)")
	clilog.Divider()
	for _, suite := range report.Suites {
		pass, warn, fail := 0, 0, 0
		for _, check := range suite.Checks {
			switch check.Status {
			case "pass", "ok":
				pass++
			case "warn", "warning":
				warn++
			case "fail", "critical":
				fail++
			}
		}
		health := "ok"
		if fail > 0 {
			health = "FAIL"
		} else if warn > 0 {
			health = "warn"
		}
		fmt.Printf("  %-16s %-6s %5d %5d %5d %10d\n", suite.Name, health, pass, warn, fail, suite.DurationMs)
	}
	fmt.Printf("\n  Overall: %s (ran in %dms)\n\n", report.Overall, report.DurationMs)
}
