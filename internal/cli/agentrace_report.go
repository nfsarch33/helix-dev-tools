package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

var agentraceReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate agentrace telemetry report from NDJSON logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, _ := os.UserHomeDir()
		logPath := filepath.Join(home, "logs", "runx", "agentrace-mcp.ndjson")

		if p, _ := cmd.Flags().GetString("path"); p != "" {
			logPath = p
		}

		f, err := os.Open(logPath)
		if err != nil {
			return fmt.Errorf("open log: %w", err)
		}
		defer f.Close()

		type event struct {
			Tool     string `json:"tool"`
			AgentID  string `json:"agent_id"`
			Duration int64  `json:"duration_ms"`
			Success  bool   `json:"success"`
		}

		toolCounts := map[string]int{}
		toolLatency := map[string][]int64{}
		agentCounts := map[string]int{}
		var total, errors int

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			var e event
			if json.Unmarshal(scanner.Bytes(), &e) != nil {
				continue
			}
			total++
			toolCounts[e.Tool]++
			toolLatency[e.Tool] = append(toolLatency[e.Tool], e.Duration)
			agentCounts[e.AgentID]++
			if !e.Success {
				errors++
			}
		}

		if total == 0 {
			fmt.Println("No telemetry data found.")
			return nil
		}

		fmt.Printf("# Agentrace Report\n\nTotal events: %d | Errors: %d (%.1f%%)\n\n", total, errors, float64(errors)/float64(total)*100)

		fmt.Println("## Tool Usage")
		fmt.Printf("%-25s %6s %8s %8s\n", "Tool", "Count", "p50ms", "p95ms")
		fmt.Println("---")

		type toolStat struct {
			name  string
			count int
		}
		var stats []toolStat
		for name, count := range toolCounts {
			stats = append(stats, toolStat{name, count})
		}
		sort.Slice(stats, func(i, j int) bool { return stats[i].count > stats[j].count })

		for _, s := range stats {
			latencies := toolLatency[s.name]
			sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
			p50 := latencies[len(latencies)/2]
			p95 := latencies[int(float64(len(latencies))*0.95)]
			fmt.Printf("%-25s %6d %8d %8d\n", s.name, s.count, p50, p95)
		}

		fmt.Println("\n## Agent Breakdown")
		for agent, count := range agentCounts {
			fmt.Printf("- %s: %d events\n", agent, count)
		}

		return nil
	},
}

func init() {
	agentraceReportCmd.Flags().String("path", "", "Path to NDJSON log (default: ~/logs/runx/agentrace-mcp.ndjson)")
}
