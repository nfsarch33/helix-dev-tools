package onboarding

import "testing"

func TestNewChecklist_HasFourSteps(t *testing.T) {
	c := NewChecklist()
	steps := c.Steps()
	if len(steps) != 4 {
		t.Errorf("expected 4 steps, got %d", len(steps))
	}
	for _, s := range steps {
		if s.Status != StepPending {
			t.Errorf("expected step %s to be pending, got %s", s.ID, s.Status)
		}
	}
}

func TestComplete_MarksStep(t *testing.T) {
	c := NewChecklist()
	ok := c.Complete("store")
	if !ok {
		t.Fatal("expected Complete to return true")
	}
	done, _ := c.Progress()
	if done != 1 {
		t.Errorf("expected 1 done, got %d", done)
	}
}

func TestComplete_UnknownID_ReturnsFalse(t *testing.T) {
	c := NewChecklist()
	ok := c.Complete("nonexistent")
	if ok {
		t.Error("expected false for unknown step ID")
	}
}

func TestSkip_MarksStep(t *testing.T) {
	c := NewChecklist()
	ok := c.Skip("domain")
	if !ok {
		t.Fatal("expected Skip to return true")
	}
	steps := c.Steps()
	for _, s := range steps {
		if s.ID == "domain" && s.Status != StepSkipped {
			t.Errorf("expected domain to be skipped, got %s", s.Status)
		}
	}
}

func TestProgress_Counts(t *testing.T) {
	c := NewChecklist()
	c.Complete("store")
	c.Complete("product")
	done, total := c.Progress()
	if done != 2 {
		t.Errorf("expected 2 done, got %d", done)
	}
	if total != 4 {
		t.Errorf("expected 4 total, got %d", total)
	}
}

func TestIsFinished_AllComplete(t *testing.T) {
	c := NewChecklist()
	for _, s := range c.Steps() {
		c.Complete(s.ID)
	}
	if !c.IsFinished() {
		t.Error("expected IsFinished to be true")
	}
}

func TestIsFinished_MixedCompleteAndSkipped(t *testing.T) {
	c := NewChecklist()
	c.Complete("store")
	c.Complete("product")
	c.Skip("payment")
	c.Skip("domain")
	if !c.IsFinished() {
		t.Error("expected IsFinished with all complete/skipped")
	}
}

func TestIsFinished_NotDone(t *testing.T) {
	c := NewChecklist()
	c.Complete("store")
	if c.IsFinished() {
		t.Error("expected IsFinished to be false when steps remain")
	}
}
