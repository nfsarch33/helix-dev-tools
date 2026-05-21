package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/platform/sprintboard"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

var fleetReportFlags struct {
	sprintID string
	hours    int
	logsDir  string
}

var fleetReportNow = time.Now

var fleetReportCmd = &cobra.Command{
	Use:   "fleet-report",
	Short: "Aggregated fleet report: sprintboard, agentrace, semble coverage",
	Long: `Generates a unified markdown report combining:
  1. Sprintboard ticket completions (from local DB)
  2. Agentrace event counts (from ~/logs/runx/agentrace-mcp.ndjson)
  3. Semble coverage ratio (from ~/logs/runx/semble-discipline.ndjson)`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home: %w", err)
		}
		logsDir := fleetReportFlags.logsDir
		if logsDir == "" {
			logsDir = filepath.Join(home, "logs", "runx")
		}
		return runFleetReport(cmd.OutOrStdout(),
			fleetReportFlags.sprintID,
			fleetReportFlags.hours,
			logsDir,
		)
	},
}

func init() {
	fleetReportCmd.Flags().StringVar(&fleetReportFlags.sprintID, "sprint", "", "Sprint ID for ticket summary (auto-detects active sprint if omitted)")
	fleetReportCmd.Flags().IntVar(&fleetReportFlags.hours, "hours", 24, "Look-back window in hours")
	fleetReportCmd.Flags().StringVar(&fleetReportFlags.logsDir, "logs-dir", "", "NDJSON logs directory (default: ~/logs/runx/)")
}

type fleetSprintSection struct {
	SprintID     string
	Total        int
	Done         int
	InProgress   int
	Blocked      int
	ByStatus     map[string]int
	ProgressPct  float64
	Available    bool
}

type fleetAgentraceSection struct {
	TotalEvents int
	Errors      int
	ByTool      map[string]int
	ByAgent     map[string]int
	Available   bool
}

type fleetSembleSection struct {
	SembleSearches int
	GrepFallbacks  int
	Total          int
	CoveragePct    float64
	Status         string
	Available      bool
}

func runFleetReport(w io.Writer, sprintID string, hours int, logsDir string) error {
	now := fleetReportNow()
	cutoff := now.Add(-time.Duration(hours) * time.Hour)

	sprint := collectSprintSection(sprintID)
	agentrace := collectAgentraceSection(filepath.Join(logsDir, "agentrace-mcp.ndjson"), cutoff)
	semble := collectSembleSection(
		filepath.Join(logsDir, "agentrace-mcp.ndjson"),
		filepath.Join(logsDir, "semble-discipline.ndjson"),
		cutoff,
	)

	formatFleetReport(w, now, hours, sprint, agentrace, semble)
	return nil
}

func collectSprintSection(sprintID string) fleetSprintSection {
	store, err := sprintboard.Open(sprintboard.DefaultDBPath())
	if err != nil {
		return fleetSprintSection{}
	}
	defer store.Close()

	if sprintID == "" {
		sprintID = detectActiveSprint(store)
	}
	if sprintID == "" {
		return fleetSprintSection{}
	}

	summary, err := store.SprintSummary(sprintID)
	if err != nil {
		return fleetSprintSection{}
	}

	sect := fleetSprintSection{
		SprintID:  sprintID,
		Total:     summary.TotalTickets,
		ByStatus:  make(map[string]int),
		Available: true,
	}
	for status, count := range summary.TicketsByStatus {
		sect.ByStatus[string(status)] = count
		switch status {
		case sprintboard.StatusDone:
			sect.Done = count
		case sprintboard.StatusInProgress:
			sect.InProgress = count
		case sprintboard.StatusBlocked:
			sect.Blocked = count
		}
	}
	if sect.Total > 0 {
		sect.ProgressPct = float64(sect.Done) / float64(sect.Total) * 100
	}
	return sect
}

func detectActiveSprint(store *sprintboard.Store) string {
	sprints, err := store.ListSprints()
	if err != nil || len(sprints) == 0 {
		return ""
	}
	for _, s := range sprints {
		if s.Status == sprintboard.SprintActive {
			return s.ID
		}
	}
	return sprints[len(sprints)-1].ID
}

func collectAgentraceSection(path string, cutoff time.Time) fleetAgentraceSection {
	f, err := os.Open(path)
	if err != nil {
		return fleetAgentraceSection{}
	}
	defer f.Close()

	type agentraceEvent struct {
		TS      string `json:"ts"`
		Time    string `json:"time"`
		Tool    string `json:"tool"`
		AgentID string `json:"agent_id"`
		Success *bool  `json:"success"`
	}

	sect := fleetAgentraceSection{
		ByTool:    make(map[string]int),
		ByAgent:   make(map[string]int),
		Available: true,
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<16), 1<<16)
	for scanner.Scan() {
		var e agentraceEvent
		if json.Unmarshal(scanner.Bytes(), &e) != nil {
			continue
		}
		tsRaw := e.TS
		if tsRaw == "" {
			tsRaw = e.Time
		}
		ts := parseFleetTime(tsRaw)
		if ts.IsZero() || ts.Before(cutoff) {
			continue
		}
		sect.TotalEvents++
		if e.Success != nil && !*e.Success {
			sect.Errors++
		}
		if e.Tool != "" {
			sect.ByTool[e.Tool]++
		}
		if e.AgentID != "" {
			sect.ByAgent[e.AgentID]++
		}
	}
	return sect
}

func collectSembleSection(agentracePath, disciplinePath string, cutoff time.Time) fleetSembleSection {
	type coverageEvt struct {
		TS        string `json:"ts"`
		Time      string `json:"time"`
		Event     string `json:"event"`
		EventType string `json:"event_type"`
	}

	sect := fleetSembleSection{Available: true}

	for _, logPath := range []string{agentracePath, disciplinePath} {
		f, err := os.Open(logPath)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 1<<16), 1<<16)
		for scanner.Scan() {
			var e coverageEvt
			if json.Unmarshal(scanner.Bytes(), &e) != nil {
				continue
			}
			tsRaw := e.TS
			if tsRaw == "" {
				tsRaw = e.Time
			}
			ts := parseFleetTime(tsRaw)
			if ts.IsZero() || ts.Before(cutoff) {
				continue
			}
			evtKey := e.Event
			if evtKey == "" {
				evtKey = e.EventType
			}
			switch evtKey {
			case "semble_search":
				sect.SembleSearches++
			case "grep_fallback", "semble_discipline":
				sect.GrepFallbacks++
			}
		}
		f.Close()
	}

	sect.Total = sect.SembleSearches + sect.GrepFallbacks
	if sect.Total > 0 {
		sect.CoveragePct = float64(sect.SembleSearches) / float64(sect.Total) * 100
	}
	if sect.Total == 0 {
		sect.Status = "NO DATA"
		sect.Available = false
	} else if sect.CoveragePct >= 80 {
		sect.Status = "GREEN"
	} else if sect.CoveragePct >= 50 {
		sect.Status = "YELLOW"
	} else {
		sect.Status = "RED"
	}
	return sect
}

func parseFleetTime(raw string) time.Time {
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05+10:00",
		"2006-01-02T15:04:05Z",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Time{}
}

func formatFleetReport(w io.Writer, now time.Time, hours int, sprint fleetSprintSection, agentrace fleetAgentraceSection, semble fleetSembleSection) {
	fmt.Fprintf(w, "# Fleet Report (last %dh)\n\n", hours)
	fmt.Fprintf(w, "Generated: %s\n\n", now.Format(time.RFC3339))

	fmt.Fprintln(w, "## Sprintboard")
	fmt.Fprintln(w)
	if sprint.Available {
		fmt.Fprintf(w, "**Sprint:** %s | **Progress:** %.0f%% (%d/%d done)\n\n", sprint.SprintID, sprint.ProgressPct, sprint.Done, sprint.Total)
		fmt.Fprintln(w, "| Status | Count |")
		fmt.Fprintln(w, "|--------|-------|")
		keys := make([]string, 0, len(sprint.ByStatus))
		for k := range sprint.ByStatus {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(w, "| %s | %d |\n", k, sprint.ByStatus[k])
		}
		if sprint.Blocked > 0 {
			fmt.Fprintf(w, "\n**Blocked tickets:** %d\n", sprint.Blocked)
		}
	} else {
		fmt.Fprintln(w, "_No sprintboard data available._")
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Agentrace")
	fmt.Fprintln(w)
	if agentrace.Available && agentrace.TotalEvents > 0 {
		errorRate := float64(0)
		if agentrace.TotalEvents > 0 {
			errorRate = float64(agentrace.Errors) / float64(agentrace.TotalEvents) * 100
		}
		fmt.Fprintf(w, "**Total events:** %d | **Errors:** %d (%.1f%%)\n\n", agentrace.TotalEvents, agentrace.Errors, errorRate)

		if len(agentrace.ByAgent) > 0 {
			fmt.Fprintln(w, "### By Agent")
			fmt.Fprintln(w, "| Agent | Events |")
			fmt.Fprintln(w, "|-------|--------|")
			type agentCount struct {
				name  string
				count int
			}
			var sorted []agentCount
			for name, count := range agentrace.ByAgent {
				sorted = append(sorted, agentCount{name, count})
			}
			sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })
			for _, a := range sorted {
				fmt.Fprintf(w, "| %s | %d |\n", a.name, a.count)
			}
		}

		if len(agentrace.ByTool) > 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "### Top Tools")
			fmt.Fprintln(w, "| Tool | Count |")
			fmt.Fprintln(w, "|------|-------|")
			type toolCount struct {
				name  string
				count int
			}
			var sorted []toolCount
			for name, count := range agentrace.ByTool {
				sorted = append(sorted, toolCount{name, count})
			}
			sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })
			limit := 10
			if len(sorted) < limit {
				limit = len(sorted)
			}
			for i := 0; i < limit; i++ {
				fmt.Fprintf(w, "| %s | %d |\n", sorted[i].name, sorted[i].count)
			}
		}
	} else {
		fmt.Fprintln(w, "_No agentrace data in window._")
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "## Semble Coverage")
	fmt.Fprintln(w)
	if semble.Available {
		fmt.Fprintln(w, "| Metric | Value |")
		fmt.Fprintln(w, "|--------|-------|")
		fmt.Fprintf(w, "| Semble searches | %d |\n", semble.SembleSearches)
		fmt.Fprintf(w, "| Grep fallbacks | %d |\n", semble.GrepFallbacks)
		fmt.Fprintf(w, "| Coverage | %.1f%% |\n", semble.CoveragePct)
		fmt.Fprintf(w, "| Status | **%s** |\n", semble.Status)
	} else {
		fmt.Fprintln(w, "_No semble coverage data in window._")
	}
	fmt.Fprintln(w)
}
