package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/helix-dev-tools/internal/tokenusage"
)

var tokenUsageFlags struct {
	last   int
	since  string
	json   bool
	source string
}

var tokenUsageCmd = &cobra.Command{
	Use:   "token-usage",
	Short: "Aggregate and display token usage from agentrace NDJSON logs",
	Long: `Reads agentrace NDJSON files and aggregates per-tool-call token counts
(input/output/total), with optional cost tracking.

  cursor-tools token-usage              # All available data
  cursor-tools token-usage --last 24    # Last 24 hours
  cursor-tools token-usage --json       # JSON output
  cursor-tools token-usage --source <path>  # Custom NDJSON file`,
	RunE: runTokenUsage,
}

func init() {
	tokenUsageCmd.Flags().IntVar(&tokenUsageFlags.last, "last", 0, "Show last N hours of data (0 = all)")
	tokenUsageCmd.Flags().StringVar(&tokenUsageFlags.since, "since", "", "Start time (RFC3339 or YYYY-MM-DD)")
	tokenUsageCmd.Flags().BoolVar(&tokenUsageFlags.json, "json", false, "Output as JSON")
	tokenUsageCmd.Flags().StringVar(&tokenUsageFlags.source, "source", "", "Custom NDJSON file path (default: ~/logs/runx/agentrace*.ndjson)")
}

func runTokenUsage(cmd *cobra.Command, args []string) error {
	var records []tokenusage.Record
	var err error

	if tokenUsageFlags.source != "" {
		records, err = tokenusage.LoadRecords(tokenUsageFlags.source)
	} else {
		pattern := tokenusage.DefaultLogPattern()
		records, err = tokenusage.LoadGlob(pattern)
		if err == nil && len(records) == 0 {
			metricsPath := tokenusage.DefaultMetricsPath()
			records, err = tokenusage.LoadRecords(metricsPath)
		}
	}
	if err != nil {
		return fmt.Errorf("load records: %w", err)
	}

	if len(records) == 0 {
		fmt.Fprintln(os.Stderr, "No token usage data found.")
		return nil
	}

	var since time.Time
	if tokenUsageFlags.last > 0 {
		since = time.Now().UTC().Add(-time.Duration(tokenUsageFlags.last) * time.Hour)
	} else if tokenUsageFlags.since != "" {
		since, err = parseTime(tokenUsageFlags.since)
		if err != nil {
			return fmt.Errorf("parse --since: %w", err)
		}
	}

	summary := tokenusage.Aggregate(records, since, time.Time{})

	if tokenUsageFlags.json {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}

	fmt.Print(tokenusage.FormatTable(summary))
	return nil
}

func parseTime(s string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised time format %q (use RFC3339 or YYYY-MM-DD)", s)
}
