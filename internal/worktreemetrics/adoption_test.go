package worktreemetrics

import "testing"

func TestAdoption_RollingWindowAtLeast80Pct(t *testing.T) {
	sessions := []Session{
		{ID: "a", Parallel: true, Worktree: true},
		{ID: "b", Parallel: true, Worktree: true},
		{ID: "c", Parallel: true, Worktree: true},
		{ID: "d", Parallel: true, Worktree: true},
		{ID: "e", Parallel: true, Worktree: false},
		{ID: "f", Parallel: false, Worktree: false},
	}

	result := Adoption(sessions, 0.80)
	if result.ParallelSessions != 5 || result.WorktreeSessions != 4 {
		t.Fatalf("counts = %#v", result)
	}
	if result.Rate != 0.80 {
		t.Fatalf("Rate = %.2f, want 0.80", result.Rate)
	}
	if result.BelowThreshold {
		t.Fatal("80% should meet threshold")
	}
}

func TestParseRunxWorktreeList(t *testing.T) {
	raw := "/Users/agent/runs/worktrees/runx/test-v308\tbranch=test/v308\nno runx worktrees\n"
	sessions := ParseRunxWorktreeList(raw)
	if len(sessions) != 1 || !sessions[0].Worktree || !sessions[0].Parallel {
		t.Fatalf("ParseRunxWorktreeList = %#v", sessions)
	}
}
