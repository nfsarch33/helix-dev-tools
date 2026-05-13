package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var observabilityReportFlags struct {
	logsDir string
	since   string
}

var observabilityReportCmd = &cobra.Command{
	Use:   "observability-report",
	Short: "Unified NDJSON observability report across all runx streams",
	Long:  "Reads all .ndjson files from ~/logs/runx/ and generates a structured markdown report with per-stream summaries and an hourly cross-correlation timeline.",
	RunE: func(cmd *cobra.Command, _ []string) error {
		dir := observabilityReportFlags.logsDir
		if dir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolve home: %w", err)
			}
			dir = filepath.Join(home, "logs", "runx")
		}
		return runObservabilityReport(cmd.OutOrStdout(), dir, observabilityReportFlags.since)
	},
}

func init() {
	observabilityReportCmd.Flags().StringVar(&observabilityReportFlags.logsDir, "logs-dir", "", "NDJSON logs directory (default: ~/logs/runx)")
	observabilityReportCmd.Flags().StringVar(&observabilityReportFlags.since, "since", "7d", "Time window, e.g. 7d, 24h, 30d")
}

type ndjsonEntry struct {
	Time  string `json:"time"`
	Level string `json:"level"`
	Msg   string `json:"msg"`
}

type streamSummary struct {
	Name      string
	Count     int
	Errors    int
	Earliest  time.Time
	Latest    time.Time
	FileBytes int64
}

func runObservabilityReport(w io.Writer, logsDir, since string) error {
	sinceD, err := parseSinceDuration(since)
	if err != nil {
		return fmt.Errorf("invalid --since: %w", err)
	}
	cutoff := time.Now().Add(-sinceD)

	pattern := filepath.Join(logsDir, "*.ndjson")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("glob logs: %w", err)
	}

	if len(files) == 0 {
		_, _ = fmt.Fprintln(w, "observability-report: no NDJSON streams found")
		return nil
	}

	var summaries []streamSummary
	hourlyMap := make(map[string]map[string]int)

	for _, f := range files {
		name := filepath.Base(f)
		info, statErr := os.Stat(f)
		if statErr != nil {
			slog.Warn("skipping file", "file", f, "error", statErr)
			continue
		}
		if info.Size() == 0 {
			summaries = append(summaries, streamSummary{Name: name, FileBytes: 0})
			continue
		}

		s, hours := scanNDJSONStream(f, name, cutoff)
		summaries = append(summaries, s)

		for h, count := range hours {
			if hourlyMap[h] == nil {
				hourlyMap[h] = make(map[string]int)
			}
			hourlyMap[h][name] += count
		}
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Count > summaries[j].Count
	})

	formatObservabilityReport(w, summaries, hourlyMap, since)
	return nil
}

func scanNDJSONStream(path, name string, cutoff time.Time) (streamSummary, map[string]int) {
	s := streamSummary{Name: name}
	hours := make(map[string]int)

	info, err := os.Stat(path)
	if err == nil {
		s.FileBytes = info.Size()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("read failed", "file", path, "error", err)
		return s, hours
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var e ndjsonEntry
		if json.Unmarshal(line, &e) != nil {
			continue
		}

		t := parseNDJSONTime(e.Time)
		if t.IsZero() {
			s.Count++
			continue
		}

		if t.Before(cutoff) {
			continue
		}

		s.Count++
		if strings.EqualFold(e.Level, "ERROR") || strings.EqualFold(e.Level, "WARN") {
			s.Errors++
		}

		if s.Earliest.IsZero() || t.Before(s.Earliest) {
			s.Earliest = t
		}
		if t.After(s.Latest) {
			s.Latest = t
		}

		hourKey := t.UTC().Format("2006-01-02T15")
		hours[hourKey]++
	}
	return s, hours
}

func parseNDJSONTime(raw string) time.Time {
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05+10:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000000+10:00",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Time{}
}

func formatObservabilityReport(w io.Writer, summaries []streamSummary, hourlyMap map[string]map[string]int, since string) {
	_, _ = fmt.Fprintf(w, "# Observability Report (--since %s)\n\n", since)
	_, _ = fmt.Fprintf(w, "Generated: %s\n\n", time.Now().UTC().Format(time.RFC3339))

	_, _ = fmt.Fprintln(w, "## Per-Stream Summary")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "| Stream | Entries | Errors | Size | Earliest | Latest | Rate/hr |")
	_, _ = fmt.Fprintln(w, "|--------|---------|--------|------|----------|--------|---------|")

	totalEntries := 0
	totalErrors := 0
	for _, s := range summaries {
		totalEntries += s.Count
		totalErrors += s.Errors
		rate := ""
		if s.Count > 0 && !s.Earliest.IsZero() && !s.Latest.IsZero() {
			dur := s.Latest.Sub(s.Earliest)
			if dur > 0 {
				r := float64(s.Count) / dur.Hours()
				rate = fmt.Sprintf("%.1f", r)
			}
		}

		earliest := "-"
		latest := "-"
		if !s.Earliest.IsZero() {
			earliest = s.Earliest.Format("2006-01-02 15:04")
		}
		if !s.Latest.IsZero() {
			latest = s.Latest.Format("2006-01-02 15:04")
		}

		_, _ = fmt.Fprintf(w, "| %s | %d | %d | %s | %s | %s | %s |\n",
			s.Name, s.Count, s.Errors, formatFileBytes(s.FileBytes),
			earliest, latest, rate)
	}

	_, _ = fmt.Fprintf(w, "\n**Total:** %d entries across %d streams (%d errors)\n\n",
		totalEntries, len(summaries), totalErrors)

	if len(hourlyMap) == 0 {
		return
	}

	_, _ = fmt.Fprintln(w, "## Hourly Cross-Correlation Timeline")
	_, _ = fmt.Fprintln(w)

	var hourKeys []string
	streamNames := make(map[string]bool)
	for h, streams := range hourlyMap {
		hourKeys = append(hourKeys, h)
		for name := range streams {
			streamNames[name] = true
		}
	}
	sort.Strings(hourKeys)

	var sortedStreams []string
	for name := range streamNames {
		sortedStreams = append(sortedStreams, name)
	}
	sort.Strings(sortedStreams)

	maxStreams := 10
	if len(sortedStreams) > maxStreams {
		sortedStreams = sortedStreams[:maxStreams]
	}

	header := "| Hour |"
	sep := "|------|"
	for _, s := range sortedStreams {
		short := strings.TrimSuffix(s, ".ndjson")
		if len(short) > 12 {
			short = short[:12]
		}
		header += fmt.Sprintf(" %s |", short)
		sep += "------|"
	}
	header += " Total |"
	sep += "------|"
	_, _ = fmt.Fprintln(w, header)
	_, _ = fmt.Fprintln(w, sep)

	maxRows := 48
	start := 0
	if len(hourKeys) > maxRows {
		start = len(hourKeys) - maxRows
	}
	for _, h := range hourKeys[start:] {
		total := 0
		row := fmt.Sprintf("| %s |", h[5:])
		for _, s := range sortedStreams {
			c := hourlyMap[h][s]
			total += c
			if c == 0 {
				row += " . |"
			} else {
				row += fmt.Sprintf(" %d |", c)
			}
		}
		row += fmt.Sprintf(" %d |", total)
		_, _ = fmt.Fprintln(w, row)
	}
	_, _ = fmt.Fprintln(w)
}

func formatFileBytes(b int64) string {
	switch {
	case b >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	default:
		return fmt.Sprintf("%dB", b)
	}
}

func parseSinceDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 7 * 24 * time.Hour, nil
	}

	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	if len(s) > 1 {
		numStr := s[:len(s)-1]
		unit := s[len(s)-1:]
		var n int
		if _, err := fmt.Sscanf(numStr, "%d", &n); err == nil {
			switch unit {
			case "d":
				return time.Duration(n) * 24 * time.Hour, nil
			case "w":
				return time.Duration(n) * 7 * 24 * time.Hour, nil
			}
		}
	}
	return 0, fmt.Errorf("unrecognized duration %q (examples: 7d, 24h, 4w)", s)
}
