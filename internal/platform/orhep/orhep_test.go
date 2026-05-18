package orhep

import "testing"

func TestCapsule_IsComplete_AllPhases(t *testing.T) {
	c := Capsule{ID: "v6200-v6299"}
	c.Add(PhaseObserve, "collected 50 sprint data points")
	c.Add(PhaseReflect, "identified 3 recurring issues")
	c.Add(PhaseHeal, "fixed mutex deadlock pattern")
	c.Add(PhaseEvolve, "promoted 5 new patterns")
	c.Add(PhasePromote, "written to Mem0 and Git KB")
	if !c.IsComplete() {
		t.Error("expected capsule to be complete with all 5 phases")
	}
}

func TestCapsule_IsComplete_MissingPhase(t *testing.T) {
	c := Capsule{ID: "partial"}
	c.Add(PhaseObserve, "observed")
	c.Add(PhaseReflect, "reflected")
	// missing Heal, Evolve, Promote
	if c.IsComplete() {
		t.Error("expected capsule to not be complete with missing phases")
	}
}

func TestCapsule_Promote(t *testing.T) {
	c := Capsule{ID: "cap1"}
	c.Promote()
	if !c.Promoted {
		t.Error("expected Promoted to be true after Promote()")
	}
	if c.ClosedAt.IsZero() {
		t.Error("expected ClosedAt to be set after Promote()")
	}
}

func TestStore_Save_Get(t *testing.T) {
	s := NewStore()
	cap := Capsule{ID: "cycle1", SprintRange: "v6200-v6299"}
	s.Save(cap)
	got, ok := s.Get("cycle1")
	if !ok {
		t.Fatal("expected to find capsule")
	}
	if got.SprintRange != "v6200-v6299" {
		t.Errorf("wrong sprint range: %s", got.SprintRange)
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	s := NewStore()
	_, ok := s.Get("missing")
	if ok {
		t.Error("expected false for missing capsule")
	}
}

func TestStore_Promoted_FiltersList(t *testing.T) {
	s := NewStore()
	c1 := Capsule{ID: "c1"}
	c1.Promote()
	c2 := Capsule{ID: "c2"} // not promoted
	s.Save(c1)
	s.Save(c2)
	promoted := s.Promoted()
	if len(promoted) != 1 || promoted[0].ID != "c1" {
		t.Errorf("expected [c1] promoted, got %v", promoted)
	}
}
