package healthagg

import "testing"

func TestAggregate_AllHealthy(t *testing.T) {
	a := NewAggregator()
	components := []ComponentState{
		{Name: "mem0", Healthy: true},
		{Name: "vllm", Healthy: true},
	}
	r := a.Aggregate(components)
	if r.Score != 100 {
		t.Errorf("expected score 100, got %d", r.Score)
	}
	if r.Healthy != 2 {
		t.Errorf("expected 2 healthy, got %d", r.Healthy)
	}
}

func TestAggregate_AllUnhealthy(t *testing.T) {
	a := NewAggregator()
	components := []ComponentState{
		{Name: "mem0", Healthy: false},
		{Name: "vllm", Healthy: false},
	}
	r := a.Aggregate(components)
	if r.Score != 0 {
		t.Errorf("expected score 0, got %d", r.Score)
	}
}

func TestAggregate_HalfHealthy(t *testing.T) {
	a := NewAggregator()
	components := []ComponentState{
		{Name: "mem0", Healthy: true},
		{Name: "vllm", Healthy: false},
	}
	r := a.Aggregate(components)
	if r.Score != 50 {
		t.Errorf("expected score 50, got %d", r.Score)
	}
}

func TestAggregate_WeightedScore(t *testing.T) {
	a := NewAggregator()
	components := []ComponentState{
		{Name: "critical", Healthy: true, Weight: 3},
		{Name: "minor", Healthy: false, Weight: 1},
	}
	r := a.Aggregate(components)
	// healthy weight=3, total=4, score=75
	if r.Score != 75 {
		t.Errorf("expected score 75, got %d", r.Score)
	}
}

func TestAggregate_EmptyComponents(t *testing.T) {
	a := NewAggregator()
	r := a.Aggregate(nil)
	if r.Score != 0 {
		t.Errorf("expected score 0 for empty, got %d", r.Score)
	}
}

func TestTrend_Improving(t *testing.T) {
	a := NewAggregator()
	// First: 50% healthy
	a.Aggregate([]ComponentState{{Name: "a", Healthy: true}, {Name: "b", Healthy: false}})
	// Second: all healthy (improves >5 points)
	r := a.Aggregate([]ComponentState{{Name: "a", Healthy: true}, {Name: "b", Healthy: true}})
	if r.Trend != "improving" {
		t.Errorf("expected improving, got %s", r.Trend)
	}
}

func TestTrend_Degrading(t *testing.T) {
	a := NewAggregator()
	a.Aggregate([]ComponentState{{Name: "a", Healthy: true}, {Name: "b", Healthy: true}})
	r := a.Aggregate([]ComponentState{{Name: "a", Healthy: true}, {Name: "b", Healthy: false}})
	if r.Trend != "degrading" {
		t.Errorf("expected degrading, got %s", r.Trend)
	}
}

func TestLastN(t *testing.T) {
	a := NewAggregator()
	for i := 0; i < 5; i++ {
		a.Aggregate([]ComponentState{{Name: "x", Healthy: true}})
	}
	last3 := a.LastN(3)
	if len(last3) != 3 {
		t.Errorf("expected 3, got %d", len(last3))
	}
}
