package fleetagent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// DailySummary generates a concise daily status from the NDJSON execution log.
type DailySummary struct {
	LogPath string
	AgentID string
}

// SummaryStats holds aggregated daily metrics.
type SummaryStats struct {
	Date            string  `json:"date"`
	AgentID         string  `json:"agent_id"`
	TotalTasks      int     `json:"total_tasks"`
	Succeeded       int     `json:"succeeded"`
	Failed          int     `json:"failed"`
	SuccessRate     float64 `json:"success_rate"`
	TotalDurationMS int64   `json:"total_duration_ms"`
	AvgDurationMS   int64   `json:"avg_duration_ms"`
}

// Generate reads the NDJSON log and produces stats for the given day.
func (d *DailySummary) Generate(day time.Time) (SummaryStats, error) {
	dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)

	entries, err := d.readEntries(dayStart, dayEnd)
	if err != nil {
		return SummaryStats{}, err
	}

	stats := SummaryStats{
		Date:    dayStart.Format("2006-01-02"),
		AgentID: d.AgentID,
	}

	for _, e := range entries {
		stats.TotalTasks++
		stats.TotalDurationMS += e.DurationMS
		if e.Success {
			stats.Succeeded++
		} else {
			stats.Failed++
		}
	}

	if stats.TotalTasks > 0 {
		stats.SuccessRate = float64(stats.Succeeded) / float64(stats.TotalTasks)
		stats.AvgDurationMS = stats.TotalDurationMS / int64(stats.TotalTasks)
	}

	return stats, nil
}

// ToMarkdown renders summary stats as a markdown section.
func (s SummaryStats) ToMarkdown() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Fleet Agent Daily Summary: %s\n\n", s.Date))
	sb.WriteString(fmt.Sprintf("**Agent:** %s\n\n", s.AgentID))
	sb.WriteString("| Metric | Value |\n|---|---|\n")
	sb.WriteString(fmt.Sprintf("| Total tasks | %d |\n", s.TotalTasks))
	sb.WriteString(fmt.Sprintf("| Succeeded | %d |\n", s.Succeeded))
	sb.WriteString(fmt.Sprintf("| Failed | %d |\n", s.Failed))
	sb.WriteString(fmt.Sprintf("| Success rate | %.1f%% |\n", s.SuccessRate*100))
	sb.WriteString(fmt.Sprintf("| Total duration | %dms |\n", s.TotalDurationMS))
	sb.WriteString(fmt.Sprintf("| Avg duration | %dms |\n", s.AvgDurationMS))
	return sb.String()
}

func (d *DailySummary) readEntries(from, to time.Time) ([]NDJSONEntry, error) {
	f, err := os.Open(d.LogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open log: %w", err)
	}
	defer f.Close()

	var entries []NDJSONEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e NDJSONEntry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		if d.AgentID != "" && e.AgentID != d.AgentID {
			continue
		}
		if !e.Timestamp.Before(from) && e.Timestamp.Before(to) {
			entries = append(entries, e)
		}
	}
	return entries, scanner.Err()
}
