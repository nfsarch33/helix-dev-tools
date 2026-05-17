package tokenusage_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nfsarch33/helix-dev-tools/internal/tokenusage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func writeNDJSON(t *testing.T, path string, records []tokenusage.Record) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	for _, r := range records {
		data, err := json.Marshal(r)
		require.NoError(t, err)
		_, err = f.Write(append(data, '\n'))
		require.NoError(t, err)
	}
}

func TestLoadRecords_Empty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.ndjson")
	require.NoError(t, os.WriteFile(path, nil, 0o644))

	records, err := tokenusage.LoadRecords(path)
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestLoadRecords_NonExistent(t *testing.T) {
	records, err := tokenusage.LoadRecords("/tmp/nonexistent-test-file.ndjson")
	require.NoError(t, err)
	assert.Nil(t, records)
}

func TestLoadRecords_ValidData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.ndjson")

	now := time.Now().UTC().Truncate(time.Second)
	records := []tokenusage.Record{
		{Timestamp: now, Hook: "guard-shell", Action: "allow", BytesIn: 100, BytesOut: 50},
		{Timestamp: now, Category: "mcp", Detail: "mem0:search", InputTokens: 200, OutputTokens: 80},
		{Timestamp: now, Category: "skill", Detail: "go-clean-arch", InputTokens: 500, OutputTokens: 300},
	}
	writeNDJSON(t, path, records)

	loaded, err := tokenusage.LoadRecords(path)
	require.NoError(t, err)
	assert.Len(t, loaded, 3)
	assert.Equal(t, "guard-shell", loaded[0].Hook)
	assert.Equal(t, 200, loaded[1].InputTokens)
}

func TestLoadRecords_SkipsMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mixed.ndjson")

	content := `{"ts":"2026-05-13T00:00:00Z","hook":"test","bytes_in":10}
not valid json
{"ts":"2026-05-13T00:01:00Z","hook":"test2","bytes_out":20}
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	loaded, err := tokenusage.LoadRecords(path)
	require.NoError(t, err)
	assert.Len(t, loaded, 2)
}

func TestLoadGlob(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)

	writeNDJSON(t, filepath.Join(dir, "agentrace-2026-05-12.ndjson"), []tokenusage.Record{
		{Timestamp: now, Hook: "h1", BytesIn: 10},
	})
	writeNDJSON(t, filepath.Join(dir, "agentrace-2026-05-13.ndjson"), []tokenusage.Record{
		{Timestamp: now, Hook: "h2", BytesIn: 20},
		{Timestamp: now, Hook: "h3", BytesIn: 30},
	})

	records, err := tokenusage.LoadGlob(filepath.Join(dir, "agentrace*.ndjson"))
	require.NoError(t, err)
	assert.Len(t, records, 3)
}

func TestAggregate_Basic(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	records := []tokenusage.Record{
		{Timestamp: now, Category: "mcp", Detail: "mem0:search", InputTokens: 100, OutputTokens: 50},
		{Timestamp: now, Category: "mcp", Detail: "mem0:search", InputTokens: 200, OutputTokens: 80},
		{Timestamp: now, Category: "mcp", Detail: "exa:search", InputTokens: 300, OutputTokens: 120},
		{Timestamp: now, Hook: "guard-shell", Action: "allow", BytesIn: 500, BytesOut: 200},
	}

	summary := tokenusage.Aggregate(records, time.Time{}, time.Time{})
	assert.Equal(t, 4, summary.TotalCalls)
	assert.Equal(t, 600, summary.TotalInput)
	assert.Equal(t, 250, summary.TotalOutput)
	assert.Equal(t, 850, summary.TotalTokens)
	assert.Equal(t, int64(500), summary.TotalBytesIn)
	assert.Equal(t, int64(200), summary.TotalBytesOut)
	assert.Len(t, summary.Breakdown, 3)

	// Breakdown sorted by TotalTokens descending: mem0=430, exa=420, guard-shell=0(bytes only)
	assert.Equal(t, "mcp:mem0:search", summary.Breakdown[0].Key)
	assert.Equal(t, 430, summary.Breakdown[0].TotalTokens)
	assert.Equal(t, "mcp:exa:search", summary.Breakdown[1].Key)
	assert.Equal(t, 420, summary.Breakdown[1].TotalTokens)
}

func TestAggregate_TimeFilter(t *testing.T) {
	base := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)
	records := []tokenusage.Record{
		{Timestamp: base.Add(-2 * time.Hour), Category: "old", Detail: "x", InputTokens: 100},
		{Timestamp: base, Category: "current", Detail: "y", InputTokens: 200},
		{Timestamp: base.Add(time.Hour), Category: "current", Detail: "z", InputTokens: 300},
		{Timestamp: base.Add(3 * time.Hour), Category: "future", Detail: "f", InputTokens: 400},
	}

	summary := tokenusage.Aggregate(records, base, base.Add(2*time.Hour))
	assert.Equal(t, 2, summary.TotalCalls)
	assert.Equal(t, 500, summary.TotalInput)
}

func TestAggregate_SkipsZeroTokenRecords(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	records := []tokenusage.Record{
		{Timestamp: now, Hook: "test", Action: "allow"},
		{Timestamp: now, Hook: "test2", Action: "deny", BytesIn: 100},
	}

	summary := tokenusage.Aggregate(records, time.Time{}, time.Time{})
	assert.Equal(t, 1, summary.TotalCalls)
}

func TestAggregate_CostAggregation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	records := []tokenusage.Record{
		{Timestamp: now, Category: "llm", Detail: "claude", InputTokens: 1000, Cost: 0.01},
		{Timestamp: now, Category: "llm", Detail: "claude", InputTokens: 2000, Cost: 0.02},
		{Timestamp: now, Category: "llm", Detail: "gpt4", InputTokens: 500, Cost: 0.005},
	}

	summary := tokenusage.Aggregate(records, time.Time{}, time.Time{})
	assert.InDelta(t, 0.035, summary.TotalCost, 0.0001)
	assert.Len(t, summary.Breakdown, 2)
}

func TestAggregate_CacheTracking(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	records := []tokenusage.Record{
		{Timestamp: now, Category: "llm", Detail: "claude", InputTokens: 1000, CacheRead: 800, CacheWrite: 200},
		{Timestamp: now, Category: "llm", Detail: "claude", InputTokens: 500, CacheRead: 400},
	}

	summary := tokenusage.Aggregate(records, time.Time{}, time.Time{})
	assert.Equal(t, 2, summary.TotalCalls)
	assert.Equal(t, 1200, summary.Breakdown[0].CacheRead)
	assert.Equal(t, 200, summary.Breakdown[0].CacheWrite)
}

func TestFormatTable_NoData(t *testing.T) {
	summary := &tokenusage.Summary{
		Since: time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 5, 13, 23, 59, 0, 0, time.UTC),
	}
	output := tokenusage.FormatTable(summary)
	assert.Contains(t, output, "No token usage data found")
}

func TestFormatTable_WithData(t *testing.T) {
	summary := &tokenusage.Summary{
		Since:       time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC),
		Until:       time.Date(2026, 5, 13, 23, 59, 0, 0, time.UTC),
		TotalCalls:  10,
		TotalInput:  5000,
		TotalOutput: 2000,
		TotalTokens: 7000,
		Breakdown: []tokenusage.ToolBreakdown{
			{Key: "mcp:mem0:search", Calls: 5, InputTokens: 3000, OutputTokens: 1000, TotalTokens: 4000},
			{Key: "skill:go-clean-arch", Calls: 5, InputTokens: 2000, OutputTokens: 1000, TotalTokens: 3000},
		},
	}
	output := tokenusage.FormatTable(summary)
	assert.Contains(t, output, "mcp:mem0:search")
	assert.Contains(t, output, "skill:go-clean-arch")
	assert.Contains(t, output, "7000")
}

func TestFormatTable_TruncatesLongKeys(t *testing.T) {
	summary := &tokenusage.Summary{
		Since:       time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC),
		Until:       time.Date(2026, 5, 13, 23, 59, 0, 0, time.UTC),
		TotalCalls:  1,
		TotalTokens: 100,
		Breakdown: []tokenusage.ToolBreakdown{
			{Key: "this-is-a-very-long-key-that-should-be-truncated-for-display", Calls: 1, TotalTokens: 100},
		},
	}
	output := tokenusage.FormatTable(summary)
	assert.Contains(t, output, "...")
}

func TestDefaultLogPattern(t *testing.T) {
	pattern := tokenusage.DefaultLogPattern()
	assert.Contains(t, pattern, "agentrace")
	assert.Contains(t, pattern, ".ndjson")
}
