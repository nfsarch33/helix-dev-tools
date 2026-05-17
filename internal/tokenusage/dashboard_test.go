package tokenusage_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/tokenusage"
)

func TestDashboard_7DayTrend(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Now().UTC()

	var records []tokenusage.Record
	for day := 0; day < 7; day++ {
		ts := now.Add(-time.Duration(6-day) * 24 * time.Hour)
		for call := 0; call < 10; call++ {
			records = append(records, tokenusage.Record{
				Timestamp:    ts.Add(time.Duration(call) * time.Minute),
				Hook:         "afterModelResponse",
				Action:       "token_log",
				Category:     "mcp",
				Detail:       "user-mem0",
				InputTokens:  1000 + day*100,
				OutputTokens: 500 + day*50,
				BytesIn:      4000,
				BytesOut:     2000,
				Model:        "claude-4-opus",
				Cost:         0.01 * float64(day+1),
			})
		}
	}

	path := filepath.Join(dir, "agentrace-test.ndjson")
	writeNDJSON(t, path, records)

	loaded, err := tokenusage.LoadRecords(path)
	if err != nil {
		t.Fatalf("LoadRecords: %v", err)
	}
	if len(loaded) != 70 {
		t.Fatalf("loaded %d records, want 70", len(loaded))
	}

	since := now.Add(-7 * 24 * time.Hour)
	summary := tokenusage.Aggregate(loaded, since, now.Add(time.Hour))

	if summary.TotalCalls != 70 {
		t.Errorf("TotalCalls=%d, want 70", summary.TotalCalls)
	}
	if summary.TotalInput == 0 {
		t.Error("TotalInput should be > 0")
	}
	if summary.TotalOutput == 0 {
		t.Error("TotalOutput should be > 0")
	}
	if summary.TotalCost == 0 {
		t.Error("TotalCost should be > 0")
	}

	if len(summary.Breakdown) == 0 {
		t.Fatal("Breakdown should have entries")
	}
	if summary.Breakdown[0].Key != "mcp:user-mem0" {
		t.Errorf("top breakdown key=%q, want mcp:user-mem0", summary.Breakdown[0].Key)
	}
}

func TestDashboard_FormatTableOutput(t *testing.T) {
	t.Parallel()
	summary := &tokenusage.Summary{
		Since:       time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
		Until:       time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC),
		TotalCalls:  100,
		TotalInput:  50000,
		TotalOutput: 25000,
		TotalTokens: 75000,
		TotalCost:   1.50,
		Breakdown: []tokenusage.ToolBreakdown{
			{Key: "mcp:user-mem0", Calls: 40, InputTokens: 20000, OutputTokens: 10000, TotalTokens: 30000},
			{Key: "mcp:user-exa", Calls: 30, InputTokens: 15000, OutputTokens: 8000, TotalTokens: 23000},
			{Key: "hook:afterModelResponse", Calls: 30, InputTokens: 15000, OutputTokens: 7000, TotalTokens: 22000},
		},
	}

	table := tokenusage.FormatTable(summary)

	if !strings.Contains(table, "Token Usage:") {
		t.Error("table should contain header")
	}
	if !strings.Contains(table, "100") {
		t.Error("table should show total calls")
	}
	if !strings.Contains(table, "mcp:user-mem0") {
		t.Error("table should show breakdown entries")
	}
	if !strings.Contains(table, "$1.5000") {
		t.Error("table should show cost")
	}
}

func TestDashboard_EmptyRecords(t *testing.T) {
	t.Parallel()
	summary := tokenusage.Aggregate(nil, time.Time{}, time.Time{})
	if summary.TotalCalls != 0 {
		t.Error("empty records should produce 0 calls")
	}
	table := tokenusage.FormatTable(summary)
	if !strings.Contains(table, "No token usage data") {
		t.Error("empty summary should say no data")
	}
}

func TestDashboard_TimeRangeFilter(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	records := []tokenusage.Record{
		{Timestamp: now.Add(-48 * time.Hour), Category: "old", InputTokens: 100, OutputTokens: 50},
		{Timestamp: now.Add(-12 * time.Hour), Category: "recent", InputTokens: 200, OutputTokens: 100},
		{Timestamp: now.Add(-1 * time.Hour), Category: "latest", InputTokens: 300, OutputTokens: 150},
	}

	since := now.Add(-24 * time.Hour)
	summary := tokenusage.Aggregate(records, since, now.Add(time.Hour))

	if summary.TotalCalls != 2 {
		t.Errorf("TotalCalls=%d, want 2 (only recent + latest)", summary.TotalCalls)
	}
}

func TestDashboard_PerSessionBreakdown(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	records := []tokenusage.Record{
		{Timestamp: now, Category: "mcp", Detail: "user-mem0", InputTokens: 500, OutputTokens: 200},
		{Timestamp: now, Category: "mcp", Detail: "user-mem0", InputTokens: 300, OutputTokens: 100},
		{Timestamp: now, Category: "mcp", Detail: "user-exa", InputTokens: 400, OutputTokens: 150},
		{Timestamp: now, Hook: "afterModelResponse", Action: "log", InputTokens: 1000, OutputTokens: 500},
	}

	summary := tokenusage.Aggregate(records, time.Time{}, time.Time{})

	if len(summary.Breakdown) != 3 {
		t.Fatalf("Breakdown entries=%d, want 3", len(summary.Breakdown))
	}
	if summary.Breakdown[0].TotalTokens < summary.Breakdown[1].TotalTokens {
		t.Error("breakdown should be sorted by total tokens descending")
	}
}

func TestDashboard_LoadGlobPattern(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Now().UTC()

	for _, name := range []string{"agentrace-01.ndjson", "agentrace-02.ndjson"} {
		writeNDJSON(t, filepath.Join(dir, name), []tokenusage.Record{
			{Timestamp: now, Category: "mcp", Detail: "test", InputTokens: 100, OutputTokens: 50},
		})
	}

	records, err := tokenusage.LoadGlob(filepath.Join(dir, "agentrace-*.ndjson"))
	if err != nil {
		t.Fatalf("LoadGlob: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("loaded %d records from 2 files, want 2", len(records))
	}
}
