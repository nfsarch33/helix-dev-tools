package programclose

import "testing"

func TestNew_HasSixMilestones(t *testing.T) {
	pc := New("v6200-v6299", "v6200-v6299")
	if len(pc.Milestones()) != 6 {
		t.Errorf("expected 6 milestones, got %d", len(pc.Milestones()))
	}
}

func TestComplete_MarksMilestone(t *testing.T) {
	pc := New("test", "v1-v10")
	ok := pc.Complete("qa-sweep")
	if !ok {
		t.Fatal("expected Complete to return true")
	}
	done, _ := pc.Progress()
	if done != 1 {
		t.Errorf("expected 1 done, got %d", done)
	}
}

func TestComplete_UnknownID(t *testing.T) {
	pc := New("test", "v1-v10")
	ok := pc.Complete("nonexistent")
	if ok {
		t.Error("expected false for unknown milestone ID")
	}
}

func TestProgress_Counts(t *testing.T) {
	pc := New("test", "v1-v10")
	pc.Complete("qa-sweep")
	pc.Complete("doc-audit")
	done, total := pc.Progress()
	if done != 2 {
		t.Errorf("expected 2 done, got %d", done)
	}
	if total != 6 {
		t.Errorf("expected 6 total, got %d", total)
	}
}

func TestIsClosed_AllComplete(t *testing.T) {
	pc := New("test", "v1-v10")
	for _, m := range pc.Milestones() {
		pc.Complete(m.ID)
	}
	if !pc.IsClosed() {
		t.Error("expected programme to be closed when all milestones done")
	}
}

func TestIsClosed_NotDone(t *testing.T) {
	pc := New("test", "v1-v10")
	pc.Complete("qa-sweep")
	if pc.IsClosed() {
		t.Error("expected not closed when milestones remain")
	}
}

func TestMilestone_CompletedAt_Set(t *testing.T) {
	pc := New("test", "v1-v10")
	pc.Complete("retro")
	for _, m := range pc.Milestones() {
		if m.ID == "retro" {
			if m.CompletedAt.IsZero() {
				t.Error("expected CompletedAt to be set")
			}
		}
	}
}
