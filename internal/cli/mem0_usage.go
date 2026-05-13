package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var mem0UsageFlags struct {
	logPath string
	days    int
}

var mem0UsageCmd = &cobra.Command{
	Use:   "mem0-usage",
	Short: "Show Mem0 API usage statistics from NDJSON logs",
	Long: `Reads ~/logs/runx/mem0-usage.ndjson and prints summary statistics
including total adds, searches, dedup hits, cache hits, rate-limited events,
and daily burn rate.`,
	RunE: runMem0Usage,
}

func init() {
	mem0UsageCmd.Flags().StringVar(&mem0UsageFlags.logPath, "log", "", "Path to usage NDJSON (default: ~/logs/runx/mem0-usage.ndjson)")
	mem0UsageCmd.Flags().IntVar(&mem0UsageFlags.days, "days", 7, "Number of days to analyse")
}

type usageEvent struct {
	Timestamp string                 `json:"ts"`
	Event     string                 `json:"event"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
}

type usageSummary struct {
	totalAdds         int
	totalSearches     int
	dedupHits         int
	dedupMisses       int
	cacheHits         int
	cacheMisses       int
	batchFlushes      int
	batchEntries      int
	rateLimitedAdds   int
	rateLimitedSearch int
	firstEvent        time.Time
	lastEvent         time.Time
}

func runMem0Usage(_ *cobra.Command, _ []string) error {
	logPath := mem0UsageFlags.logPath
	if logPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home: %w", err)
		}
		logPath = filepath.Join(home, "logs", "runx", "mem0-usage.ndjson")
	}

	f, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No usage log found at", logPath)
			fmt.Println("The mem0cache proxy has not been activated yet.")
			return nil
		}
		return fmt.Errorf("open log: %w", err)
	}
	defer f.Close()

	cutoff := time.Now().AddDate(0, 0, -mem0UsageFlags.days)
	var s usageSummary

	scanner := bufio.NewScanner(f)
	totalLines := 0
	for scanner.Scan() {
		var ev usageEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		totalLines++

		ts, err := time.Parse(time.RFC3339, ev.Timestamp)
		if err != nil {
			continue
		}

		if s.firstEvent.IsZero() || ts.Before(s.firstEvent) {
			s.firstEvent = ts
		}
		if ts.After(s.lastEvent) {
			s.lastEvent = ts
		}

		if ts.Before(cutoff) {
			continue
		}

		switch ev.Event {
		case "mem0_add":
			s.totalAdds++
			if isDedupHit(ev.Meta) {
				s.dedupHits++
			} else {
				s.dedupMisses++
			}
		case "mem0_search":
			s.totalSearches++
			if isCacheHit(ev.Meta) {
				s.cacheHits++
			} else {
				s.cacheMisses++
			}
		case "mem0_batch_flush":
			s.batchFlushes++
			if c, ok := ev.Meta["count"]; ok {
				if n, ok := c.(float64); ok {
					s.batchEntries += int(n)
				}
			}
		case "mem0_rate_limited":
			op, _ := ev.Meta["op"].(string)
			switch op {
			case "add":
				s.rateLimitedAdds++
			case "search":
				s.rateLimitedSearch++
			}
		}
	}

	fmt.Println("=== Mem0 Usage Summary ===")
	fmt.Printf("Log file: %s\n", logPath)
	fmt.Printf("Period: last %d days\n", mem0UsageFlags.days)
	fmt.Printf("Total events: %d\n\n", totalLines)

	if !s.firstEvent.IsZero() {
		fmt.Printf("First event: %s\n", s.firstEvent.Format(time.RFC3339))
		fmt.Printf("Last event:  %s\n\n", s.lastEvent.Format(time.RFC3339))
	}

	fmt.Println("--- Adds ---")
	fmt.Printf("Total add calls:     %d\n", s.totalAdds)
	fmt.Printf("  Dedup hits (saved): %d\n", s.dedupHits)
	fmt.Printf("  Dedup misses (new): %d\n", s.dedupMisses)

	fmt.Println("\n--- Searches ---")
	fmt.Printf("Total search calls:  %d\n", s.totalSearches)
	fmt.Printf("  Cache hits (saved): %d\n", s.cacheHits)
	fmt.Printf("  Cache misses (API): %d\n", s.cacheMisses)

	fmt.Println("\n--- Batch Flushes ---")
	fmt.Printf("Flush events:        %d\n", s.batchFlushes)
	fmt.Printf("Entries flushed:     %d\n", s.batchEntries)

	fmt.Println("\n--- Rate Limited ---")
	fmt.Printf("Add operations:      %d\n", s.rateLimitedAdds)
	fmt.Printf("Search operations:   %d\n", s.rateLimitedSearch)

	totalSaved := s.dedupHits + s.cacheHits
	totalAPI := s.dedupMisses + s.cacheMisses
	if totalAPI+totalSaved > 0 {
		pct := float64(totalSaved) / float64(totalAPI+totalSaved) * 100
		fmt.Printf("\n--- Efficiency ---\n")
		fmt.Printf("API calls saved:     %d / %d (%.1f%%)\n", totalSaved, totalAPI+totalSaved, pct)
	}

	if !s.firstEvent.IsZero() && !s.lastEvent.IsZero() {
		span := s.lastEvent.Sub(s.firstEvent)
		if span > 0 {
			days := span.Hours() / 24
			if days > 0 {
				fmt.Printf("Daily API burn rate: %.1f adds/day, %.1f searches/day\n",
					float64(s.dedupMisses)/days, float64(s.cacheMisses)/days)
			}
		}
	}

	return nil
}

func isDedupHit(meta map[string]interface{}) bool {
	v, ok := meta["dedup_hit"]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func isCacheHit(meta map[string]interface{}) bool {
	v, ok := meta["cache_hit"]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}
