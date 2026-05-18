package skillindex

import "testing"

func TestRegister_Lookup(t *testing.T) {
	idx := NewIndex()
	idx.Register(SkillEntry{ID: "code-review-pro", Title: "Code Review Pro", Category: "review", HasSKILLMD: true})
	e, ok := idx.Lookup("code-review-pro")
	if !ok {
		t.Fatal("expected to find skill")
	}
	if e.Title != "Code Review Pro" {
		t.Errorf("wrong title: %s", e.Title)
	}
}

func TestLookup_NotFound(t *testing.T) {
	idx := NewIndex()
	_, ok := idx.Lookup("missing")
	if ok {
		t.Error("expected false for missing skill")
	}
}

func TestAll_SortedByID(t *testing.T) {
	idx := NewIndex()
	idx.Register(SkillEntry{ID: "zzz"})
	idx.Register(SkillEntry{ID: "aaa"})
	all := idx.All()
	if len(all) != 2 || all[0].ID != "aaa" {
		t.Errorf("expected sorted by ID, got %v", all)
	}
}

func TestByCategory(t *testing.T) {
	idx := NewIndex()
	idx.Register(SkillEntry{ID: "cr", Category: "review"})
	idx.Register(SkillEntry{ID: "ghci", Category: "ci"})
	idx.Register(SkillEntry{ID: "pr", Category: "review"})
	reviews := idx.ByCategory("review")
	if len(reviews) != 2 {
		t.Errorf("expected 2 review skills, got %d", len(reviews))
	}
}

func TestStaleEntries(t *testing.T) {
	idx := NewIndex()
	idx.Register(SkillEntry{ID: "good", HasSKILLMD: true})
	idx.Register(SkillEntry{ID: "stale", HasSKILLMD: false})
	stale := idx.StaleEntries()
	if len(stale) != 1 || stale[0] != "stale" {
		t.Errorf("expected [stale], got %v", stale)
	}
}

func TestRemove_Found(t *testing.T) {
	idx := NewIndex()
	idx.Register(SkillEntry{ID: "removeme"})
	if err := idx.Remove("removeme"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	_, ok := idx.Lookup("removeme")
	if ok {
		t.Error("skill still present after removal")
	}
}

func TestRemove_NotFound(t *testing.T) {
	idx := NewIndex()
	if err := idx.Remove("missing"); err == nil {
		t.Error("expected error for missing skill")
	}
}
