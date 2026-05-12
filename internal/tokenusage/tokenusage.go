package tokenusage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Record represents a single agentrace event with token usage data.
// Fields are a superset of the metrics.Event token-relevant fields and
// the claude.Usage token fields, unified into one parse target.
type Record struct {
	Timestamp    time.Time `json:"ts"`
	Hook         string    `json:"hook,omitempty"`
	Action       string    `json:"action,omitempty"`
	Category     string    `json:"cat,omitempty"`
	Detail       string    `json:"detail,omitempty"`
	BytesIn      int64     `json:"bytes_in,omitempty"`
	BytesOut     int64     `json:"bytes_out,omitempty"`
	InputTokens  int       `json:"input_tokens,omitempty"`
	OutputTokens int       `json:"output_tokens,omitempty"`
	CacheRead    int       `json:"cache_read,omitempty"`
	CacheWrite   int       `json:"cache_write,omitempty"`
	DurationMs   int64     `json:"dur_ms,omitempty"`
	LatencyMs    int64     `json:"latency_ms,omitempty"`
	Model        string    `json:"model,omitempty"`
	Cost         float64   `json:"cost,omitempty"`
	TurnID       string    `json:"turn_id,omitempty"`
	RunID        string    `json:"run_id,omitempty"`
}

// ToolBreakdown aggregates token usage for a single tool/hook/category key.
type ToolBreakdown struct {
	Key          string  `json:"key"`
	Calls        int     `json:"calls"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	BytesIn      int64   `json:"bytes_in"`
	BytesOut     int64   `json:"bytes_out"`
	CacheRead    int     `json:"cache_read"`
	CacheWrite   int     `json:"cache_write"`
	TotalCost    float64 `json:"total_cost,omitempty"`
}

// Summary is the aggregate output of the token-usage subcommand.
type Summary struct {
	Since         time.Time       `json:"since"`
	Until         time.Time       `json:"until"`
	TotalCalls    int             `json:"total_calls"`
	TotalInput    int             `json:"total_input_tokens"`
	TotalOutput   int             `json:"total_output_tokens"`
	TotalTokens   int             `json:"total_tokens"`
	TotalBytesIn  int64           `json:"total_bytes_in"`
	TotalBytesOut int64           `json:"total_bytes_out"`
	TotalCost     float64         `json:"total_cost,omitempty"`
	Breakdown     []ToolBreakdown `json:"breakdown"`
}

// LoadRecords reads NDJSON records from a file, skipping malformed lines.
func LoadRecords(path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var records []Record
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}
		records = append(records, r)
	}
	return records, scanner.Err()
}

// LoadGlob reads NDJSON records from all files matching the glob pattern.
func LoadGlob(pattern string) ([]Record, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %q: %w", pattern, err)
	}
	var all []Record
	for _, path := range matches {
		records, err := LoadRecords(path)
		if err != nil {
			continue
		}
		all = append(all, records...)
	}
	return all, nil
}

// recordKey returns the aggregation key for a record.
// Priority: category:detail > hook:action > hook > "unknown".
func recordKey(r Record) string {
	if r.Category != "" && r.Detail != "" {
		return r.Category + ":" + r.Detail
	}
	if r.Category != "" {
		return r.Category
	}
	if r.Hook != "" && r.Action != "" {
		return r.Hook + ":" + r.Action
	}
	if r.Hook != "" {
		return r.Hook
	}
	return "unknown"
}

// hasTokenData returns true if the record carries any token or byte usage.
func hasTokenData(r Record) bool {
	return r.InputTokens > 0 || r.OutputTokens > 0 ||
		r.BytesIn > 0 || r.BytesOut > 0 ||
		r.CacheRead > 0 || r.CacheWrite > 0
}

// Aggregate computes a Summary from records within [since, until).
// If since is zero, all records are included. If until is zero, now is used.
func Aggregate(records []Record, since, until time.Time) *Summary {
	if until.IsZero() {
		until = time.Now().UTC()
	}

	s := &Summary{Since: since, Until: until}
	acc := make(map[string]*ToolBreakdown)

	for _, r := range records {
		if !since.IsZero() && r.Timestamp.Before(since) {
			continue
		}
		if r.Timestamp.After(until) {
			continue
		}
		if !hasTokenData(r) {
			continue
		}

		key := recordKey(r)
		tb, ok := acc[key]
		if !ok {
			tb = &ToolBreakdown{Key: key}
			acc[key] = tb
		}

		tb.Calls++
		tb.InputTokens += r.InputTokens
		tb.OutputTokens += r.OutputTokens
		tb.TotalTokens += r.InputTokens + r.OutputTokens
		tb.BytesIn += r.BytesIn
		tb.BytesOut += r.BytesOut
		tb.CacheRead += r.CacheRead
		tb.CacheWrite += r.CacheWrite
		tb.TotalCost += r.Cost

		s.TotalCalls++
		s.TotalInput += r.InputTokens
		s.TotalOutput += r.OutputTokens
		s.TotalTokens += r.InputTokens + r.OutputTokens
		s.TotalBytesIn += r.BytesIn
		s.TotalBytesOut += r.BytesOut
		s.TotalCost += r.Cost
	}

	breakdown := make([]ToolBreakdown, 0, len(acc))
	for _, tb := range acc {
		breakdown = append(breakdown, *tb)
	}
	sort.Slice(breakdown, func(i, j int) bool {
		return breakdown[i].TotalTokens > breakdown[j].TotalTokens
	})
	s.Breakdown = breakdown
	return s
}

// FormatTable renders the summary as a human-readable table.
func FormatTable(s *Summary) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Token Usage: %s to %s\n",
		s.Since.Format("2006-01-02 15:04"), s.Until.Format("2006-01-02 15:04")))
	b.WriteString(fmt.Sprintf("Total: %d calls, %d input, %d output, %d total tokens\n",
		s.TotalCalls, s.TotalInput, s.TotalOutput, s.TotalTokens))
	if s.TotalBytesIn > 0 || s.TotalBytesOut > 0 {
		b.WriteString(fmt.Sprintf("Bytes: %d in, %d out\n", s.TotalBytesIn, s.TotalBytesOut))
	}
	if s.TotalCost > 0 {
		b.WriteString(fmt.Sprintf("Cost:  $%.4f\n", s.TotalCost))
	}
	b.WriteString("\n")

	if len(s.Breakdown) == 0 {
		b.WriteString("No token usage data found.\n")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("%-40s %6s %10s %10s %10s\n",
		"Tool/Hook", "Calls", "Input", "Output", "Total"))
	b.WriteString(strings.Repeat("─", 80) + "\n")

	for _, tb := range s.Breakdown {
		key := tb.Key
		if len(key) > 40 {
			key = key[:37] + "..."
		}
		b.WriteString(fmt.Sprintf("%-40s %6d %10d %10d %10d\n",
			key, tb.Calls, tb.InputTokens, tb.OutputTokens, tb.TotalTokens))
	}

	return b.String()
}

// DefaultLogPattern returns the default glob pattern for agentrace NDJSON files.
func DefaultLogPattern() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "agentrace*.ndjson"
	}
	return filepath.Join(home, "logs", "runx", "agentrace*.ndjson")
}

// DefaultMetricsPath returns the path to the cursor-tools metrics NDJSON file.
func DefaultMetricsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "cursor-tools-metrics.jsonl"
	}
	return filepath.Join(home, "logs", "cursor-tools-metrics.jsonl")
}
