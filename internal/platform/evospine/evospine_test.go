package evospine

import "testing"

func TestRecord_Velocity(t *testing.T) {
	tr := NewTracker()
	tr.Record(PatternEntry{PatternsNew: 5})
	tr.Record(PatternEntry{PatternsNew: 3})
	tr.Record(PatternEntry{PatternsNew: 4})
	// velocity = (5+3+4)/3 = 4.0
	v := tr.Velocity()
	if v < 3.9 || v > 4.1 {
		t.Errorf("expected velocity ~4.0, got %f", v)
	}
}

func TestConvergence_Insufficient(t *testing.T) {
	tr := NewTracker()
	if tr.Convergence() != StatusInsufficient {
		t.Error("expected insufficient with no data")
	}
	tr.Record(PatternEntry{PatternsNew: 5})
	if tr.Convergence() != StatusInsufficient {
		t.Error("expected insufficient with 1 observation")
	}
}

func TestConvergence_Converging(t *testing.T) {
	tr := NewTracker()
	tr.Record(PatternEntry{PatternsNew: 10})
	tr.Record(PatternEntry{PatternsNew: 4}) // fewer new patterns = converging
	if tr.Convergence() != StatusConverging {
		t.Errorf("expected converging, got %s", tr.Convergence())
	}
}

func TestConvergence_Diverging(t *testing.T) {
	tr := NewTracker()
	tr.Record(PatternEntry{PatternsNew: 2})
	tr.Record(PatternEntry{PatternsNew: 8})
	if tr.Convergence() != StatusDiverging {
		t.Errorf("expected diverging, got %s", tr.Convergence())
	}
}

func TestConvergence_Stable(t *testing.T) {
	tr := NewTracker()
	tr.Record(PatternEntry{PatternsNew: 3})
	tr.Record(PatternEntry{PatternsNew: 3})
	if tr.Convergence() != StatusStable {
		t.Errorf("expected stable, got %s", tr.Convergence())
	}
}

func TestLastN_ReturnsSlice(t *testing.T) {
	tr := NewTracker()
	for i := 0; i < 5; i++ {
		tr.Record(PatternEntry{PatternsNew: i})
	}
	last3 := tr.LastN(3)
	if len(last3) != 3 {
		t.Errorf("expected 3 entries, got %d", len(last3))
	}
	// Last 3 should be PatternsNew = 2, 3, 4
	if last3[0].PatternsNew != 2 {
		t.Errorf("expected PatternsNew=2 first, got %d", last3[0].PatternsNew)
	}
}

func TestVelocity_Empty(t *testing.T) {
	tr := NewTracker()
	if tr.Velocity() != 0 {
		t.Error("expected velocity=0 for empty tracker")
	}
}
