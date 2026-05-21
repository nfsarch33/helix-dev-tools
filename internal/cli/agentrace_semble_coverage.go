package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

var sembleCoverageFlags struct {
	hours int
}

var sembleCoverageCmd = &cobra.Command{
	Use:   "semble-coverage",
	Short: "Semble-first coverage from agentrace + semble-discipline logs",
	RunE: func(cmd *cobra.Command, _ []string) error {
		home, _ := os.UserHomeDir()
		return runSembleCoverage(
			filepath.Join(home, "logs", "runx", "agentrace-mcp.ndjson"),
			filepath.Join(home, "logs", "runx", "semble-discipline.ndjson"),
			sembleCoverageFlags.hours,
		)
	},
}

func init() {
	sembleCoverageCmd.Flags().IntVar(&sembleCoverageFlags.hours, "hours", 24, "Look-back window in hours")
}

type coverageEvent struct {
	TS        string `json:"ts"`
	Event     string `json:"event"`
	EventType string `json:"event_type"`
	Tool      string `json:"tool"`
	Verdict   string `json:"verdict"`
	Command   string `json:"command"`
}

func runSembleCoverage(agentracePath, disciplinePath string, hours int) error {
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)

	var sembleSearchCount, grepFallbackCount int
	patternCounts := map[string]int{}

	for _, logPath := range []string{agentracePath, disciplinePath} {
		f, err := os.Open(logPath)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1<<16), 1<<16)
		for scanner.Scan() {
			var e coverageEvent
			if json.Unmarshal(scanner.Bytes(), &e) != nil {
				continue
			}

			ts, err := time.Parse(time.RFC3339, e.TS)
			if err != nil {
				continue
			}
			if ts.Before(cutoff) {
				continue
			}

			eventKey := e.Event
			if eventKey == "" {
				eventKey = e.EventType
			}

			switch {
			case eventKey == "semble_search":
				sembleSearchCount++
			case eventKey == "grep_fallback" || eventKey == "semble_discipline":
				grepFallbackCount++
				if e.Command != "" {
					short := e.Command
					if len(short) > 80 {
						short = short[:80]
					}
					patternCounts[short]++
				}
			}
		}
		f.Close()
	}

	total := sembleSearchCount + grepFallbackCount
	var coverage float64
	if total > 0 {
		coverage = float64(sembleSearchCount) / float64(total) * 100
	}

	target := 80.0
	status := "GREEN"
	if coverage < 50 {
		status = "RED"
	} else if coverage < target {
		status = "YELLOW"
	}

	fmt.Printf("# Semble-First Coverage Report (last %dh)\n\n", hours)
	fmt.Printf("| Metric | Value |\n")
	fmt.Printf("|--------|-------|\n")
	fmt.Printf("| Semble searches | %d |\n", sembleSearchCount)
	fmt.Printf("| Grep fallbacks | %d |\n", grepFallbackCount)
	fmt.Printf("| Total code searches | %d |\n", total)
	fmt.Printf("| Coverage | %.1f%% |\n", coverage)
	fmt.Printf("| Target | %.0f%% |\n", target)
	fmt.Printf("| Status | **%s** |\n", status)

	if len(patternCounts) > 0 {
		fmt.Printf("\n## Top Grep Patterns (should use Semble)\n\n")
		type pc struct {
			pattern string
			count   int
		}
		var sorted []pc
		for p, c := range patternCounts {
			sorted = append(sorted, pc{p, c})
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })
		limit := 5
		if len(sorted) < limit {
			limit = len(sorted)
		}
		for i := 0; i < limit; i++ {
			fmt.Printf("- %dx: `%s`\n", sorted[i].count, sorted[i].pattern)
		}
	}

	return nil
}
