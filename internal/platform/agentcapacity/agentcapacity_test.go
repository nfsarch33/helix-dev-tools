package agentcapacity

import (
	"testing"
	"time"
)

func TestRegisterAndClaim(t *testing.T) {
	p := NewPlanner()
	p.Register("cursor", "cursor", 3)
	if err := p.Claim("cursor"); err != nil {
		t.Fatal(err)
	}
	avail := p.Available()
	if len(avail) != 1 || avail[0].ActiveTasks != 1 {
		t.Errorf("expected 1 active task, got %v", avail)
	}
}

func TestClaimAtCapacity(t *testing.T) {
	p := NewPlanner()
	p.Register("worker", "codex", 2)
	p.Claim("worker")
	p.Claim("worker")
	if err := p.Claim("worker"); err == nil {
		t.Fatal("should fail at capacity")
	}
}

func TestClaimUnregistered(t *testing.T) {
	p := NewPlanner()
	if err := p.Claim("ghost"); err == nil {
		t.Fatal("should fail for unregistered agent")
	}
}

func TestRelease(t *testing.T) {
	p := NewPlanner()
	p.Register("a", "cursor", 5)
	p.Claim("a")
	p.Claim("a")
	if err := p.Release("a"); err != nil {
		t.Fatal(err)
	}
	avail := p.Available()
	if avail[0].ActiveTasks != 1 {
		t.Errorf("expected 1 after release, got %d", avail[0].ActiveTasks)
	}
}

func TestReleaseEmpty(t *testing.T) {
	p := NewPlanner()
	p.Register("a", "cursor", 3)
	if err := p.Release("a"); err == nil {
		t.Fatal("should fail releasing with no active tasks")
	}
}

func TestLeastLoaded(t *testing.T) {
	p := NewPlanner()
	p.Register("heavy", "cursor", 3)
	p.Register("light", "codex", 3)
	p.Claim("heavy")
	p.Claim("heavy")
	p.Claim("light")

	id, err := p.LeastLoaded()
	if err != nil {
		t.Fatal(err)
	}
	if id != "light" {
		t.Errorf("expected light, got %s", id)
	}
}

func TestLeastLoadedEmpty(t *testing.T) {
	p := NewPlanner()
	_, err := p.LeastLoaded()
	if err == nil {
		t.Fatal("should fail with no agents")
	}
}

func TestStaleDetection(t *testing.T) {
	p := NewPlanner()
	p.Register("active", "cursor", 3)
	p.Register("stale", "codex", 3)
	p.mu.Lock()
	p.agents["stale"].LastActivity = time.Now().Add(-2 * time.Hour)
	p.mu.Unlock()

	stale := p.Stale(1 * time.Hour)
	if len(stale) != 1 || stale[0] != "stale" {
		t.Errorf("expected [stale], got %v", stale)
	}
}

func TestUtilization(t *testing.T) {
	p := NewPlanner()
	p.Register("a", "cursor", 4)
	p.Register("b", "codex", 4)
	p.Claim("a")
	p.Claim("a")
	util := p.Utilization()
	if util < 0.24 || util > 0.26 {
		t.Errorf("expected ~0.25, got %f", util)
	}
}

func TestUtilizationEmpty(t *testing.T) {
	p := NewPlanner()
	if p.Utilization() != 0 {
		t.Error("empty planner should have 0 utilization")
	}
}
