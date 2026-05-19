package selfimprove

import (
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	e := New(Config{AgentID: "cursor-parent"})
	if e == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestObserve(t *testing.T) {
	e := New(Config{AgentID: "test"})
	e.Observe(Observation{
		Kind:    "test-pass-rate",
		Value:   0.95,
		Context: "v6310 sprint",
	})
	obs := e.Observations()
	if len(obs) != 1 {
		t.Fatalf("got %d observations, want 1", len(obs))
	}
}

func TestReflect(t *testing.T) {
	e := New(Config{AgentID: "test"})
	e.Observe(Observation{Kind: "velocity", Value: 6.0, Context: "packages/hour"})
	e.Observe(Observation{Kind: "pass-rate", Value: 1.0, Context: "100%"})

	insight := e.Reflect()
	if insight.PatternCount == 0 {
		t.Error("expected at least one pattern from reflection")
	}
}

func TestPromotePattern(t *testing.T) {
	e := New(Config{AgentID: "test"})
	p := Pattern{
		ID:        "pat-200",
		Name:      "warm-context-go-tdd",
		Insight:   "5min per package in warm context",
		CreatedAt: time.Now(),
	}
	e.PromotePattern(p)
	patterns := e.Patterns()
	if len(patterns) != 1 {
		t.Fatalf("got %d patterns, want 1", len(patterns))
	}
	if patterns[0].ID != "pat-200" {
		t.Errorf("got pattern ID %q, want pat-200", patterns[0].ID)
	}
}

func TestGenerateCapsule(t *testing.T) {
	e := New(Config{AgentID: "cursor-parent"})
	e.Observe(Observation{Kind: "sprint-complete", Value: 1.0, Context: "v6310"})
	e.PromotePattern(Pattern{ID: "pat-200", Name: "test-pattern", Insight: "test"})

	capsule := e.GenerateCapsule("v6310")
	if capsule.SprintID != "v6310" {
		t.Errorf("capsule sprint %q, want v6310", capsule.SprintID)
	}
	if len(capsule.Patterns) != 1 {
		t.Errorf("got %d patterns in capsule, want 1", len(capsule.Patterns))
	}
}
