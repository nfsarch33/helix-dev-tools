package experiment

import "testing"

func TestAdd_Get_Roundtrip(t *testing.T) {
	r := NewRegistry()
	r.Add(Experiment{ID: "exp1", Hypothesis: "better cache", Status: StatusPending})
	e, ok := r.Get("exp1")
	if !ok {
		t.Fatal("expected to find exp1")
	}
	if e.Hypothesis != "better cache" {
		t.Errorf("wrong hypothesis: %s", e.Hypothesis)
	}
}

func TestGet_NotFound(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get("missing")
	if ok {
		t.Error("expected false for missing ID")
	}
}

func TestComplete_RecordsOutcome(t *testing.T) {
	r := NewRegistry()
	r.Add(Experiment{ID: "exp1", Status: StatusPending})
	ok := r.Complete("exp1", Outcome{MetricBefore: 0.70, MetricAfter: 0.85, Kept: true})
	if !ok {
		t.Fatal("expected Complete to return true")
	}
	e, _ := r.Get("exp1")
	if e.Status != StatusComplete {
		t.Errorf("expected StatusComplete, got %s", e.Status)
	}
	if e.Outcome == nil || !e.Outcome.Kept {
		t.Error("expected outcome with Kept=true")
	}
}

func TestComplete_NotFound(t *testing.T) {
	r := NewRegistry()
	ok := r.Complete("missing", Outcome{})
	if ok {
		t.Error("expected false for unknown ID")
	}
}

func TestOutcome_Delta(t *testing.T) {
	o := Outcome{MetricBefore: 0.7, MetricAfter: 0.85}
	if d := o.Delta(); d < 0.14 || d > 0.16 {
		t.Errorf("expected delta ~0.15, got %f", d)
	}
}

func TestKeptCount(t *testing.T) {
	r := NewRegistry()
	r.Add(Experiment{ID: "e1"})
	r.Add(Experiment{ID: "e2"})
	r.Add(Experiment{ID: "e3"})
	r.Complete("e1", Outcome{Kept: true})
	r.Complete("e2", Outcome{Kept: false})
	r.Complete("e3", Outcome{Kept: true})
	if r.KeptCount() != 2 {
		t.Errorf("expected 2 kept, got %d", r.KeptCount())
	}
}

func TestAll_ReturnsCopy(t *testing.T) {
	r := NewRegistry()
	r.Add(Experiment{ID: "e1"})
	r.Add(Experiment{ID: "e2"})
	all := r.All()
	if len(all) != 2 {
		t.Errorf("expected 2 experiments, got %d", len(all))
	}
}
