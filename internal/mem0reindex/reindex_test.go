package mem0reindex

import (
	"testing"
	"time"
)

func TestNewReindexer(t *testing.T) {
	r := New(Config{
		Mem0URL:    "http://localhost:8080",
		Mem0APIKey: "test-key",
		BatchSize:  10,
		Timeout:    90 * time.Second,
	})
	if r == nil {
		t.Fatal("expected non-nil reindexer")
	}
}

func TestPlanReindex(t *testing.T) {
	r := New(Config{Mem0URL: "http://localhost:8080", BatchSize: 5})
	memories := []Memory{
		{ID: "m1", Text: "hello", HasVector: false},
		{ID: "m2", Text: "world", HasVector: true},
		{ID: "m3", Text: "test", HasVector: false},
	}
	plan := r.Plan(memories)
	if plan.TotalMemories != 3 {
		t.Errorf("got total %d, want 3", plan.TotalMemories)
	}
	if plan.NeedReindex != 2 {
		t.Errorf("got need reindex %d, want 2", plan.NeedReindex)
	}
	if plan.AlreadyIndexed != 1 {
		t.Errorf("got already indexed %d, want 1", plan.AlreadyIndexed)
	}
}

func TestBatchSplit(t *testing.T) {
	r := New(Config{BatchSize: 2})
	memories := []Memory{
		{ID: "m1", Text: "a", HasVector: false},
		{ID: "m2", Text: "b", HasVector: false},
		{ID: "m3", Text: "c", HasVector: false},
		{ID: "m4", Text: "d", HasVector: false},
		{ID: "m5", Text: "e", HasVector: false},
	}
	batches := r.SplitBatches(memories)
	if len(batches) != 3 {
		t.Errorf("got %d batches, want 3 (5 items / batch size 2)", len(batches))
	}
	if len(batches[2]) != 1 {
		t.Errorf("last batch has %d items, want 1", len(batches[2]))
	}
}

func TestReindexResult(t *testing.T) {
	r := New(Config{BatchSize: 10})
	result := &ReindexResult{}
	result.RecordSuccess("m1", 2*time.Second)
	result.RecordSuccess("m2", 3*time.Second)
	result.RecordFailure("m3", "timeout")

	if result.SuccessCount != 2 {
		t.Errorf("got %d successes, want 2", result.SuccessCount)
	}
	if result.FailCount != 1 {
		t.Errorf("got %d failures, want 1", result.FailCount)
	}
	if result.TotalDuration() < 5*time.Second {
		t.Errorf("total duration %v, want >= 5s", result.TotalDuration())
	}
	_ = r
}

func TestFilterNeedsReindex(t *testing.T) {
	r := New(Config{})
	memories := []Memory{
		{ID: "m1", HasVector: false},
		{ID: "m2", HasVector: true},
		{ID: "m3", HasVector: false},
		{ID: "m4", HasVector: true},
	}
	filtered := r.FilterNeedsReindex(memories)
	if len(filtered) != 2 {
		t.Fatalf("got %d, want 2", len(filtered))
	}
	if filtered[0].ID != "m1" || filtered[1].ID != "m3" {
		t.Error("wrong items filtered")
	}
}
