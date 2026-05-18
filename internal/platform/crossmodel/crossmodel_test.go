package crossmodel

import "testing"

func TestAdd_Best_HighestScore(t *testing.T) {
	c := NewComparison("task1")
	c.Add(EvalResult{ModelID: "haiku", Score: 0.70, LatencyMS: 100, CostUSD: 0.01})
	c.Add(EvalResult{ModelID: "sonnet", Score: 0.85, LatencyMS: 300, CostUSD: 0.05})
	c.Add(EvalResult{ModelID: "opus", Score: 0.92, LatencyMS: 800, CostUSD: 0.20})

	best, ok := c.Best()
	if !ok {
		t.Fatal("expected Best to return a result")
	}
	if best.ModelID != "opus" {
		t.Errorf("expected opus as best, got %s", best.ModelID)
	}
}

func TestBest_TieBreakByLatency(t *testing.T) {
	c := NewComparison("task2")
	c.Add(EvalResult{ModelID: "slow", Score: 0.80, LatencyMS: 500})
	c.Add(EvalResult{ModelID: "fast", Score: 0.80, LatencyMS: 100})

	best, _ := c.Best()
	if best.ModelID != "fast" {
		t.Errorf("expected fast as tie-break winner, got %s", best.ModelID)
	}
}

func TestBest_Empty(t *testing.T) {
	c := NewComparison("empty")
	_, ok := c.Best()
	if ok {
		t.Error("expected false for empty comparison")
	}
}

func TestRecommend_CheapestWithinThreshold(t *testing.T) {
	c := NewComparison("task3")
	c.Add(EvalResult{ModelID: "haiku", Score: 0.80, CostUSD: 0.01})
	c.Add(EvalResult{ModelID: "sonnet", Score: 0.88, CostUSD: 0.05})
	c.Add(EvalResult{ModelID: "opus", Score: 0.92, CostUSD: 0.20})

	// threshold=0.15 means floor=0.92-0.15=0.77 -> all 3 qualify; cheapest=haiku
	rec, ok := c.Recommend(0.15)
	if !ok {
		t.Fatal("expected recommendation")
	}
	if rec.ModelID != "haiku" {
		t.Errorf("expected haiku as cheapest within threshold, got %s", rec.ModelID)
	}
}

func TestRecommend_EmptyComparison(t *testing.T) {
	c := NewComparison("empty")
	_, ok := c.Recommend(0.1)
	if ok {
		t.Error("expected false for empty comparison")
	}
}
