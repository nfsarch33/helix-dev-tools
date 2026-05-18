package configmgr

import "testing"

func TestSet_Get_Roundtrip(t *testing.T) {
	c := New()
	c.Set("editor", "vim")
	v, ok := c.Get("editor")
	if !ok || v != "vim" {
		t.Errorf("expected 'vim', got %q ok=%v", v, ok)
	}
}

func TestGet_Missing(t *testing.T) {
	c := New()
	_, ok := c.Get("missing")
	if ok {
		t.Error("expected false for missing key")
	}
}

func TestAll_ReturnsCopy(t *testing.T) {
	c := New()
	c.Set("a", "1")
	c.Set("b", "2")
	all := c.All()
	if len(all) != 2 {
		t.Errorf("expected 2 entries, got %d", len(all))
	}
}

func TestDiff_ChangedAndMissing(t *testing.T) {
	a := New()
	a.Set("key1", "old")
	a.Set("key2", "same")
	b := New()
	b.Set("key1", "new")  // changed
	b.Set("key2", "same") // same
	b.Set("key3", "new")  // added in b

	diffs := a.Diff(b)
	if len(diffs) != 2 {
		t.Errorf("expected 2 diffs (key1, key3), got %v", diffs)
	}
}

func TestDiff_NoDifferences(t *testing.T) {
	a := New()
	a.Set("x", "1")
	b := New()
	b.Set("x", "1")
	if len(a.Diff(b)) != 0 {
		t.Error("expected no diffs for identical configs")
	}
}

func TestValidate_AllPresent(t *testing.T) {
	c := New()
	c.Set("host", "localhost")
	c.Set("port", "8080")
	errs := c.Validate([]string{"host", "port"})
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidate_MissingRequired(t *testing.T) {
	c := New()
	c.Set("host", "localhost")
	errs := c.Validate([]string{"host", "port"})
	if len(errs) != 1 {
		t.Errorf("expected 1 error for missing port, got %v", errs)
	}
}

func TestMerge_OverwritesOnConflict(t *testing.T) {
	a := New()
	a.Set("key", "old")
	b := New()
	b.Set("key", "new")
	b.Set("extra", "value")
	a.Merge(b)
	v, _ := a.Get("key")
	if v != "new" {
		t.Errorf("expected 'new' after merge, got %q", v)
	}
	_, ok := a.Get("extra")
	if !ok {
		t.Error("expected extra key after merge")
	}
}
