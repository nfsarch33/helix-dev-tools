package mem0cache

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestDedup_DuplicateDetection(t *testing.T) {
	dir := t.TempDir()
	d, err := NewDedup(DedupConfig{DBPath: filepath.Join(dir, "test.db")})
	if err != nil {
		t.Fatalf("open dedup: %v", err)
	}
	defer d.Close()

	if d.IsDuplicate("hello world") {
		t.Fatal("should not be duplicate on first check")
	}

	if err := d.Mark("hello world"); err != nil {
		t.Fatalf("mark: %v", err)
	}

	if !d.IsDuplicate("hello world") {
		t.Fatal("should be duplicate after mark")
	}

	if d.IsDuplicate("different content") {
		t.Fatal("different content should not be duplicate")
	}
}

func TestDedup_PersistenceAcrossRestarts(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "persist.db")

	d1, err := NewDedup(DedupConfig{DBPath: dbPath})
	if err != nil {
		t.Fatalf("open d1: %v", err)
	}
	if err := d1.Mark("persistent content"); err != nil {
		t.Fatalf("mark d1: %v", err)
	}
	d1.Close()

	d2, err := NewDedup(DedupConfig{DBPath: dbPath})
	if err != nil {
		t.Fatalf("open d2: %v", err)
	}
	defer d2.Close()

	if !d2.IsDuplicate("persistent content") {
		t.Fatal("content should be duplicate after reopen")
	}
}

func TestDedup_ConcurrentMarks(t *testing.T) {
	dir := t.TempDir()
	d, err := NewDedup(DedupConfig{DBPath: filepath.Join(dir, "concurrent.db")})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			content := "content-" + string(rune('A'+n))
			_ = d.Mark(content)
			d.IsDuplicate(content)
		}(i)
	}
	wg.Wait()

	stats := d.Stats()
	if stats.TotalEntries == 0 {
		t.Fatal("expected entries after concurrent marks")
	}
}

func TestDedup_Stats(t *testing.T) {
	dir := t.TempDir()
	d, err := NewDedup(DedupConfig{DBPath: filepath.Join(dir, "stats.db")})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	d.IsDuplicate("a")
	_ = d.Mark("a")
	d.IsDuplicate("a")

	stats := d.Stats()
	if stats.Hits != 1 || stats.Misses != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if stats.TotalEntries != 1 {
		t.Fatalf("expected 1 entry, got %d", stats.TotalEntries)
	}
}

func TestDedup_CreatesDirIfMissing(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "c", "test.db")

	d, err := NewDedup(DedupConfig{DBPath: nested})
	if err != nil {
		t.Fatalf("open with nested path: %v", err)
	}
	defer d.Close()

	if _, err := os.Stat(nested); err != nil {
		t.Fatalf("db file not created: %v", err)
	}
}
