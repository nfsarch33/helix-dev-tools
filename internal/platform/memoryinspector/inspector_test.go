package memoryinspector

import (
	"testing"
	"time"
)

func TestStoreAndRetrieve(t *testing.T) {
	store := NewMemoryStore()

	entry := MemoryEntry{
		ID:        "mem-001",
		AppID:     "cursor-global-kb",
		UserID:    "testuser",
		Content:   "vLLM Qwen3.5-4B is stable on gpu-host-1 port 8000",
		Category:  CatOperational,
		CreatedAt: time.Now(),
	}

	err := store.Add(entry)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, err := store.Get("mem-001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Content != entry.Content {
		t.Errorf("content mismatch")
	}
	if got.Category != CatOperational {
		t.Errorf("got category %q", got.Category)
	}
}

func TestGetNotFound(t *testing.T) {
	store := NewMemoryStore()
	_, err := store.Get("missing")
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestAddDuplicate(t *testing.T) {
	store := NewMemoryStore()
	store.Add(MemoryEntry{ID: "x", AppID: "test"})
	err := store.Add(MemoryEntry{ID: "x", AppID: "test"})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestSearchByAppID(t *testing.T) {
	store := NewMemoryStore()
	store.Add(MemoryEntry{ID: "1", AppID: "cursor-global-kb", Content: "hello"})
	store.Add(MemoryEntry{ID: "2", AppID: "cursor-coordination", Content: "world"})
	store.Add(MemoryEntry{ID: "3", AppID: "cursor-global-kb", Content: "foo"})

	results := store.SearchByAppID("cursor-global-kb")
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}
}

func TestSearchByContent(t *testing.T) {
	store := NewMemoryStore()
	store.Add(MemoryEntry{ID: "1", Content: "vLLM is running on port 8000"})
	store.Add(MemoryEntry{ID: "2", Content: "K3s cluster has 2 nodes"})
	store.Add(MemoryEntry{ID: "3", Content: "vLLM scale-up failed"})

	results := store.SearchByContent("vLLM")
	if len(results) != 2 {
		t.Errorf("expected 2, got %d", len(results))
	}

	results = store.SearchByContent("k3s")
	if len(results) != 1 {
		t.Errorf("expected 1, got %d", len(results))
	}
}

func TestListByCategory(t *testing.T) {
	store := NewMemoryStore()
	store.Add(MemoryEntry{ID: "1", Category: CatOperational})
	store.Add(MemoryEntry{ID: "2", Category: CatPattern})
	store.Add(MemoryEntry{ID: "3", Category: CatOperational})
	store.Add(MemoryEntry{ID: "4", Category: CatDecision})

	ops := store.ListByCategory(CatOperational)
	if len(ops) != 2 {
		t.Errorf("expected 2 operational, got %d", len(ops))
	}

	patterns := store.ListByCategory(CatPattern)
	if len(patterns) != 1 {
		t.Errorf("expected 1 pattern, got %d", len(patterns))
	}
}

func TestDelete(t *testing.T) {
	store := NewMemoryStore()
	store.Add(MemoryEntry{ID: "del-me"})

	err := store.Delete("del-me")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = store.Get("del-me")
	if err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestDeleteNotFound(t *testing.T) {
	store := NewMemoryStore()
	err := store.Delete("ghost")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAll(t *testing.T) {
	store := NewMemoryStore()
	store.Add(MemoryEntry{ID: "a"})
	store.Add(MemoryEntry{ID: "b"})
	store.Add(MemoryEntry{ID: "c"})

	all := store.All()
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}
}

func TestStats(t *testing.T) {
	store := NewMemoryStore()
	store.Add(MemoryEntry{ID: "1", AppID: "cursor-global-kb", Category: CatOperational})
	store.Add(MemoryEntry{ID: "2", AppID: "cursor-global-kb", Category: CatPattern})
	store.Add(MemoryEntry{ID: "3", AppID: "cursor-coordination", Category: CatOperational})

	stats := store.Stats()
	if stats.Total != 3 {
		t.Errorf("expected total 3, got %d", stats.Total)
	}
	if stats.ByApp["cursor-global-kb"] != 2 {
		t.Errorf("expected 2 for cursor-global-kb, got %d", stats.ByApp["cursor-global-kb"])
	}
	if stats.ByCategory[CatOperational] != 2 {
		t.Errorf("expected 2 operational, got %d", stats.ByCategory[CatOperational])
	}
}

func TestRecentEntries(t *testing.T) {
	store := NewMemoryStore()

	now := time.Now()
	store.Add(MemoryEntry{ID: "old", CreatedAt: now.Add(-48 * time.Hour)})
	store.Add(MemoryEntry{ID: "recent1", CreatedAt: now.Add(-1 * time.Hour)})
	store.Add(MemoryEntry{ID: "recent2", CreatedAt: now.Add(-30 * time.Minute)})

	recent := store.Recent(24 * time.Hour)
	if len(recent) != 2 {
		t.Errorf("expected 2 recent, got %d", len(recent))
	}
}
