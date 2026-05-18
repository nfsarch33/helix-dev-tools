package patterndisc

import "testing"

func TestRecord_Discover_Frequency(t *testing.T) {
	d := NewDiscoverer()
	seq := []string{"mem0:search", "edit", "test"}
	for i := 0; i < 5; i++ {
		d.Record(seq)
	}
	d.Record([]string{"read", "edit"}) // only 1 occurrence

	patterns := d.Discover(3)
	if len(patterns) != 1 {
		t.Fatalf("expected 1 pattern above threshold, got %d", len(patterns))
	}
	if patterns[0].Occurrences != 5 {
		t.Errorf("expected 5 occurrences, got %d", patterns[0].Occurrences)
	}
}

func TestDiscover_Confidence(t *testing.T) {
	d := NewDiscoverer()
	for i := 0; i < 3; i++ {
		d.Record([]string{"a", "b"})
	}
	d.Record([]string{"c"})

	patterns := d.Discover(1)
	for _, p := range patterns {
		if p.ID == "a->b" {
			// 3 out of 4 total = 0.75
			if p.Confidence < 0.74 || p.Confidence > 0.76 {
				t.Errorf("expected confidence ~0.75, got %f", p.Confidence)
			}
		}
	}
}

func TestDiscover_Empty(t *testing.T) {
	d := NewDiscoverer()
	patterns := d.Discover(1)
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns from empty discoverer, got %d", len(patterns))
	}
}

func TestRecord_EmptySequence(t *testing.T) {
	d := NewDiscoverer()
	d.Record(nil)
	d.Record([]string{})
	patterns := d.Discover(1)
	if len(patterns) != 0 {
		t.Errorf("expected no patterns from empty sequences, got %d", len(patterns))
	}
}

func TestMarkAntiPattern(t *testing.T) {
	d := NewDiscoverer()
	d.Record([]string{"bad", "step"})
	patterns := d.Discover(1)
	id := patterns[0].ID
	patterns = MarkAntiPattern(patterns, id)
	if !patterns[0].IsAntiPattern {
		t.Error("expected IsAntiPattern to be true after marking")
	}
}

func TestDiscover_SortedByOccurrences(t *testing.T) {
	d := NewDiscoverer()
	for i := 0; i < 2; i++ {
		d.Record([]string{"rare"})
	}
	for i := 0; i < 10; i++ {
		d.Record([]string{"common"})
	}
	patterns := d.Discover(1)
	if len(patterns) < 2 {
		t.Fatal("expected at least 2 patterns")
	}
	if patterns[0].Occurrences < patterns[1].Occurrences {
		t.Error("expected patterns sorted by occurrences descending")
	}
}
