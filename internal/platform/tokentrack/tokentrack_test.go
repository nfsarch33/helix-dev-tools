package tokentrack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func tmpTracker(t *testing.T) *Tracker {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "token-usage.ndjson")
	tr, err := NewTracker(path)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

func TestRecordWritesNDJSON(t *testing.T) {
	tr := tmpTracker(t)
	rec := UsageRecord{
		Timestamp:    time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC),
		Model:        "MiniMax-M2.7-highspeed",
		Provider:     "minimax",
		InputTokens:  100,
		OutputTokens: 50,
		RequestID:    "req-1",
		AgentID:      "agent-1",
	}
	if err := tr.Record(rec); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(tr.path)
	var got UsageRecord
	if err := json.Unmarshal(data[:len(data)-1], &got); err != nil {
		t.Fatal(err)
	}
	if got.Model != "MiniMax-M2.7-highspeed" {
		t.Errorf("model: %s", got.Model)
	}
}

func TestRecordComputesCost(t *testing.T) {
	tr := tmpTracker(t)
	rec := UsageRecord{
		Timestamp:    time.Now(),
		Model:        "MiniMax-M2.7-highspeed",
		Provider:     "minimax",
		InputTokens:  1000,
		OutputTokens: 1000,
	}
	if err := tr.Record(rec); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(tr.path)
	var got UsageRecord
	json.Unmarshal(data[:len(data)-1], &got)
	expected := 1000*0.001/1000 + 1000*0.002/1000
	if got.Cost < expected-0.0001 || got.Cost > expected+0.0001 {
		t.Errorf("cost: got %f, want ~%f", got.Cost, expected)
	}
}

func TestDailySummary(t *testing.T) {
	tr := tmpTracker(t)
	day := time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		tr.Record(UsageRecord{
			Timestamp:    day.Add(time.Duration(i) * time.Hour),
			Model:        "MiniMax-M2.7-highspeed",
			Provider:     "minimax",
			InputTokens:  100,
			OutputTokens: 50,
		})
	}
	tr.Record(UsageRecord{
		Timestamp:    day.AddDate(0, 0, 1),
		Model:        "MiniMax-M2.7-highspeed",
		Provider:     "minimax",
		InputTokens:  999,
		OutputTokens: 999,
	})

	summary, err := tr.DailySummary(day)
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalRequests != 3 {
		t.Errorf("requests: %d", summary.TotalRequests)
	}
	if summary.TotalInputTokens != 300 {
		t.Errorf("input tokens: %d", summary.TotalInputTokens)
	}
	if summary.TotalOutputTokens != 150 {
		t.Errorf("output tokens: %d", summary.TotalOutputTokens)
	}
}

func TestTotalSince(t *testing.T) {
	tr := tmpTracker(t)
	now := time.Now().UTC()
	tr.Record(UsageRecord{
		Timestamp:    now.Add(-2 * time.Hour),
		Model:        "model-a",
		Provider:     "prov-a",
		InputTokens:  200,
		OutputTokens: 100,
	})
	tr.Record(UsageRecord{
		Timestamp:    now.Add(-1 * time.Hour),
		Model:        "model-b",
		Provider:     "prov-b",
		InputTokens:  300,
		OutputTokens: 200,
	})

	summary, err := tr.TotalSince(now.Add(-3 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalRequests != 2 {
		t.Errorf("requests: %d", summary.TotalRequests)
	}
	if summary.TotalInputTokens != 500 {
		t.Errorf("input: %d", summary.TotalInputTokens)
	}
}

func TestTotalSinceExcludesOld(t *testing.T) {
	tr := tmpTracker(t)
	now := time.Now().UTC()
	tr.Record(UsageRecord{
		Timestamp:    now.Add(-5 * time.Hour),
		Model:        "old",
		Provider:     "x",
		InputTokens:  9999,
		OutputTokens: 9999,
	})
	tr.Record(UsageRecord{
		Timestamp:    now.Add(-1 * time.Hour),
		Model:        "new",
		Provider:     "x",
		InputTokens:  10,
		OutputTokens: 5,
	})

	summary, err := tr.TotalSince(now.Add(-2 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalRequests != 1 {
		t.Errorf("requests: %d", summary.TotalRequests)
	}
	if summary.TotalInputTokens != 10 {
		t.Errorf("input: %d", summary.TotalInputTokens)
	}
}

func TestEmptyFileSummary(t *testing.T) {
	tr := tmpTracker(t)
	summary, err := tr.DailySummary(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if summary.TotalRequests != 0 {
		t.Errorf("expected 0 requests, got %d", summary.TotalRequests)
	}
}

func TestModelBreakdown(t *testing.T) {
	tr := tmpTracker(t)
	now := time.Now().UTC()
	tr.Record(UsageRecord{Timestamp: now, Model: "model-a", Provider: "x", InputTokens: 100, OutputTokens: 50})
	tr.Record(UsageRecord{Timestamp: now, Model: "model-b", Provider: "x", InputTokens: 200, OutputTokens: 100})
	tr.Record(UsageRecord{Timestamp: now, Model: "model-a", Provider: "x", InputTokens: 50, OutputTokens: 25})

	summary, err := tr.TotalSince(now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.ByModel) != 2 {
		t.Errorf("expected 2 models, got %d", len(summary.ByModel))
	}
	if summary.ByModel["model-a"].Requests != 2 {
		t.Errorf("model-a requests: %d", summary.ByModel["model-a"].Requests)
	}
}

func TestCustomCostRate(t *testing.T) {
	tr := tmpTracker(t)
	tr.SetCostRate("custom-model", 0.01, 0.03)
	rec := UsageRecord{
		Timestamp:    time.Now(),
		Model:        "custom-model",
		Provider:     "custom",
		InputTokens:  1000,
		OutputTokens: 1000,
	}
	if err := tr.Record(rec); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(tr.path)
	var got UsageRecord
	json.Unmarshal(data[:len(data)-1], &got)
	expected := 1000*0.01/1000 + 1000*0.03/1000
	if got.Cost < expected-0.0001 || got.Cost > expected+0.0001 {
		t.Errorf("cost: got %f, want ~%f", got.Cost, expected)
	}
}
