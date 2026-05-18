package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var evospineAutoCmd = &cobra.Command{
	Use:   "auto",
	Short: "Generate an automated EvoSpine ORHEP capsule from agentrace data",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, _ := os.UserHomeDir()
		logPath := filepath.Join(home, "logs", "runx", "agentrace-mcp.ndjson")

		events, err := readAgentraceEvents(logPath)
		if err != nil {
			return fmt.Errorf("read agentrace: %w", err)
		}

		if len(events) == 0 {
			fmt.Println("No agentrace events found. Run some MCP tools first.")
			return nil
		}

		capsule := generateORHEP(events)
		fmt.Fprint(os.Stdout, capsule)
		return nil
	},
}

type agentraceEvent struct {
	Timestamp  string `json:"ts"`
	Tool       string `json:"tool"`
	AgentID    string `json:"agent_id"`
	DurationMS int64  `json:"duration_ms"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
}

func readAgentraceEvents(path string) ([]agentraceEvent, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var events []agentraceEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e agentraceEvent
		if json.Unmarshal(scanner.Bytes(), &e) == nil {
			events = append(events, e)
		}
	}
	return events, scanner.Err()
}

func generateORHEP(events []agentraceEvent) string {
	var b strings.Builder
	now := time.Now().Format(time.RFC3339)

	b.WriteString(fmt.Sprintf("# EvoSpine Auto-ORHEP Capsule\n\n> Generated: %s\n> Events: %d\n\n", now, len(events)))

	// Observe
	b.WriteString("## Observe\n\n")
	toolCounts := map[string]int{}
	toolLatency := map[string]int64{}
	agentCounts := map[string]int{}
	var errors int
	for _, e := range events {
		toolCounts[e.Tool]++
		toolLatency[e.Tool] += e.DurationMS
		agentCounts[e.AgentID]++
		if !e.Success {
			errors++
		}
	}
	b.WriteString(fmt.Sprintf("- Total events: %d\n- Error rate: %.1f%%\n- Agents active: %d\n\n",
		len(events), float64(errors)/float64(len(events))*100, len(agentCounts)))

	// Reflect
	b.WriteString("## Reflect\n\n")
	type toolStat struct {
		name   string
		count  int
		avgMS  int64
	}
	var stats []toolStat
	for name, count := range toolCounts {
		stats = append(stats, toolStat{name, count, toolLatency[name] / int64(count)})
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].count > stats[j].count })

	b.WriteString("Top tools by usage:\n")
	limit := 5
	if len(stats) < limit {
		limit = len(stats)
	}
	for _, s := range stats[:limit] {
		b.WriteString(fmt.Sprintf("- %s: %d calls, avg %dms\n", s.name, s.count, s.avgMS))
	}

	// Heal
	b.WriteString("\n## Heal\n\n")
	if errors > 0 {
		b.WriteString(fmt.Sprintf("- %d errors detected. Review error patterns in agentrace log.\n", errors))
	} else {
		b.WriteString("- No errors detected. System healthy.\n")
	}

	// Evolve
	b.WriteString("\n## Evolve\n\n")
	var slowTools []toolStat
	for _, s := range stats {
		if s.avgMS > 100 {
			slowTools = append(slowTools, s)
		}
	}
	if len(slowTools) > 0 {
		b.WriteString("Slow operations (>100ms avg):\n")
		for _, s := range slowTools {
			b.WriteString(fmt.Sprintf("- %s: avg %dms -- consider caching or optimization\n", s.name, s.avgMS))
		}
	} else {
		b.WriteString("- All tools within performance targets.\n")
	}

	// Promote
	b.WriteString("\n## Promote\n\n")
	b.WriteString("- Capsule auto-generated from agentrace telemetry\n")
	b.WriteString(fmt.Sprintf("- Agent distribution: %v\n", agentCounts))

	return b.String()
}

func init() {
	evoloopCmd.AddCommand(evospineAutoCmd)
}
