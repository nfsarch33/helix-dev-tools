package cli

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/claude"
	"github.com/nfsarch33/cursor-tools/internal/clilog"
)

var claudeUsageFlags struct {
	today   bool
	week    bool
	summary bool
	dir     string
}

var claudeUsageCmd = &cobra.Command{
	Use:   "claude-usage",
	Short: "Show Claude CLI usage metrics from tracked invocations",
	Long: `Display token usage, cost, and duration metrics from Claude CLI invocations.

  cursor-tools claude-usage --today     # Today's usage
  cursor-tools claude-usage --week      # Last 7 days
  cursor-tools claude-usage --summary   # Aggregate summary`,
	RunE: runClaudeUsage,
}

func init() {
	claudeUsageCmd.Flags().BoolVar(&claudeUsageFlags.today, "today", false, "Show today's usage only")
	claudeUsageCmd.Flags().BoolVar(&claudeUsageFlags.week, "week", false, "Show last 7 days")
	claudeUsageCmd.Flags().BoolVar(&claudeUsageFlags.summary, "summary", false, "Show aggregate summary")
	claudeUsageCmd.Flags().StringVar(&claudeUsageFlags.dir, "dir", "", "Override usage log directory")
}

func runClaudeUsage(cmd *cobra.Command, args []string) error {
	dir := claudeUsageFlags.dir
	if dir == "" {
		dir = claude.DefaultUsageDir()
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		clilog.Info("no usage data found at %s", dir)
		return nil
	}

	days := 1
	if claudeUsageFlags.week {
		days = 7
	} else if claudeUsageFlags.summary {
		days = 365
	} else if !claudeUsageFlags.today {
		days = 1
	}

	now := time.Now().UTC()
	var allRecords []claude.Usage

	for i := 0; i < days; i++ {
		day := now.AddDate(0, 0, -i)
		path := claude.UsageFilePath(dir, day)
		records, err := claude.ReadUsage(path)
		if err != nil {
			clilog.Warn("reading %s: %v", path, err)
			continue
		}
		allRecords = append(allRecords, records...)
	}

	if len(allRecords) == 0 {
		clilog.Info("no usage records found for the selected period")
		return nil
	}

	sort.Slice(allRecords, func(i, j int) bool {
		return allRecords[i].Timestamp.Before(allRecords[j].Timestamp)
	})

	if claudeUsageFlags.summary {
		printSummary(allRecords)
	} else {
		printDetailed(allRecords)
	}

	return nil
}

func printSummary(records []claude.Usage) {
	var totalPromptBytes, totalOutputBytes int
	var totalInputTokens, totalOutputTokens int
	var totalDurMs int64
	var totalCost float64
	var errCount int

	for _, r := range records {
		totalPromptBytes += r.PromptBytes
		totalOutputBytes += r.OutputBytes
		totalInputTokens += r.InputTokens
		totalOutputTokens += r.OutputTokens
		totalDurMs += r.DurationMs
		totalCost += r.Cost
		if r.ExitCode != 0 {
			errCount++
		}
	}

	fmt.Printf("Claude CLI Usage Summary (%d invocations)\n", len(records))
	fmt.Println("─────────────────────────────────────────")
	fmt.Printf("  Prompt bytes:    %s\n", formatBytes(totalPromptBytes))
	fmt.Printf("  Output bytes:    %s\n", formatBytes(totalOutputBytes))
	if totalInputTokens > 0 {
		fmt.Printf("  Input tokens:    %d\n", totalInputTokens)
		fmt.Printf("  Output tokens:   %d\n", totalOutputTokens)
	}
	fmt.Printf("  Total duration:  %s\n", formatDuration(totalDurMs))
	if totalCost > 0 {
		fmt.Printf("  Total cost:      $%.4f\n", totalCost)
	}
	fmt.Printf("  Errors:          %d / %d\n", errCount, len(records))
	fmt.Printf("  Backend:         %s\n", records[0].Backend)

	if totalInputTokens == 0 {
		fmt.Println("\n  Note: Bedrock backend does not report token counts.")
		fmt.Println("  Byte counts are proxy metrics.")
	}
}

func printDetailed(records []claude.Usage) {
	fmt.Printf("%-20s  %8s  %8s  %8s  %6s  %s\n",
		"Timestamp", "Prompt", "Output", "Duration", "Exit", "Prompt Preview")
	fmt.Println("──────────────────────────────────────────────────────────────────────────────────")

	for _, r := range records {
		preview := r.Prompt
		if len(preview) > 40 {
			preview = preview[:40] + "..."
		}
		fmt.Printf("%-20s  %8s  %8s  %8s  %6d  %s\n",
			r.Timestamp.Format("2006-01-02 15:04:05"),
			formatBytes(r.PromptBytes),
			formatBytes(r.OutputBytes),
			formatDuration(r.DurationMs),
			r.ExitCode,
			preview)
	}
	fmt.Printf("\n  Total: %d invocations\n", len(records))
}

func formatBytes(b int) string {
	if b < 1024 {
		return fmt.Sprintf("%dB", b)
	}
	return fmt.Sprintf("%.1fKB", float64(b)/1024)
}

func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

// ByDay groups records by date for dashboard display.
func ByDay(records []claude.Usage) map[string][]claude.Usage {
	m := make(map[string][]claude.Usage)
	for _, r := range records {
		key := r.Timestamp.Format("2006-01-02")
		m[key] = append(m[key], r)
	}
	return m
}

// DaySummary provides aggregate stats for a single day.
type DaySummary struct {
	Date        string
	Invocations int
	PromptBytes int
	OutputBytes int
	DurationMs  int64
	Errors      int
}

// DailySummaries returns per-day aggregates sorted by date.
func DailySummaries(records []claude.Usage) []DaySummary {
	byDay := ByDay(records)
	var keys []string
	for k := range byDay {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var result []DaySummary
	for _, k := range keys {
		recs := byDay[k]
		ds := DaySummary{Date: k, Invocations: len(recs)}
		for _, r := range recs {
			ds.PromptBytes += r.PromptBytes
			ds.OutputBytes += r.OutputBytes
			ds.DurationMs += r.DurationMs
			if r.ExitCode != 0 {
				ds.Errors++
			}
		}
		result = append(result, ds)
	}
	return result
}
