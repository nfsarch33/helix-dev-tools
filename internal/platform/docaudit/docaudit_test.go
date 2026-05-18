package docaudit

import (
	"testing"
	"time"
)

func TestIsStale_FreshDoc(t *testing.T) {
	now := time.Now()
	e := DocEntry{Path: "README.md", LastUpdated: now.Add(-5 * 24 * time.Hour), MaxAgeDays: 30}
	if e.IsStale(now) {
		t.Error("expected doc to be fresh (5 days old, max 30)")
	}
}

func TestIsStale_StaleDoc(t *testing.T) {
	now := time.Now()
	e := DocEntry{Path: "SOP.md", LastUpdated: now.Add(-35 * 24 * time.Hour), MaxAgeDays: 30}
	if !e.IsStale(now) {
		t.Error("expected doc to be stale (35 days old, max 30)")
	}
}

func TestIsStale_ZeroMaxAge_NotStale(t *testing.T) {
	now := time.Now()
	e := DocEntry{Path: "pinned.md", LastUpdated: now.Add(-365 * 24 * time.Hour), MaxAgeDays: 0}
	if e.IsStale(now) {
		t.Error("expected doc with MaxAgeDays=0 to never be stale")
	}
}

func TestAuditor_Run(t *testing.T) {
	now := time.Now()
	a := NewAuditor(now)
	a.Register(DocEntry{Path: "fresh.md", LastUpdated: now.Add(-1 * 24 * time.Hour), MaxAgeDays: 30})
	a.Register(DocEntry{Path: "stale.md", LastUpdated: now.Add(-60 * 24 * time.Hour), MaxAgeDays: 30})
	results := a.Run()
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Status != DocFresh {
		t.Errorf("expected fresh.md to be fresh, got %s", results[0].Status)
	}
	if results[1].Status != DocStale {
		t.Errorf("expected stale.md to be stale, got %s", results[1].Status)
	}
}

func TestAuditor_StaleCount(t *testing.T) {
	now := time.Now()
	a := NewAuditor(now)
	a.Register(DocEntry{Path: "ok.md", LastUpdated: now, MaxAgeDays: 30})
	a.Register(DocEntry{Path: "old1.md", LastUpdated: now.Add(-60 * 24 * time.Hour), MaxAgeDays: 30})
	a.Register(DocEntry{Path: "old2.md", LastUpdated: now.Add(-90 * 24 * time.Hour), MaxAgeDays: 30})
	if a.StaleCount() != 2 {
		t.Errorf("expected 2 stale docs, got %d", a.StaleCount())
	}
}
