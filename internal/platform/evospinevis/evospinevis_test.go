package evospinevis

import "testing"

func TestRecord_AllEvents(t *testing.T) {
	tl := NewTimeline()
	tl.Record(PatternEvent{PatternID: "p1", Action: "discovered", Impact: 0.15})
	tl.Record(PatternEvent{PatternID: "p2", Action: "discovered", Impact: 0.20})
	events := tl.AllEvents()
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestActivePatterns_DiscoveredNotRetired(t *testing.T) {
	tl := NewTimeline()
	tl.Record(PatternEvent{PatternID: "p1", Action: "discovered"})
	tl.Record(PatternEvent{PatternID: "p2", Action: "discovered"})
	tl.Record(PatternEvent{PatternID: "p1", Action: "retired"})
	active := tl.ActivePatterns()
	if len(active) != 1 || active[0] != "p2" {
		t.Errorf("expected [p2] active, got %v", active)
	}
}

func TestActivePatterns_Empty(t *testing.T) {
	tl := NewTimeline()
	if len(tl.ActivePatterns()) != 0 {
		t.Error("expected no active patterns for empty timeline")
	}
}

func TestAverageImpact(t *testing.T) {
	tl := NewTimeline()
	tl.Record(PatternEvent{PatternID: "p1", Action: "discovered", Impact: 0.10})
	tl.Record(PatternEvent{PatternID: "p2", Action: "discovered", Impact: 0.30})
	tl.Record(PatternEvent{PatternID: "p1", Action: "retired"}) // retired events don't count
	avg := tl.AverageImpact()
	// average of 0.10 and 0.30 = 0.20
	if avg < 0.19 || avg > 0.21 {
		t.Errorf("expected avg impact ~0.20, got %f", avg)
	}
}

func TestAverageImpact_Empty(t *testing.T) {
	tl := NewTimeline()
	if tl.AverageImpact() != 0 {
		t.Error("expected 0 average impact for empty timeline")
	}
}
