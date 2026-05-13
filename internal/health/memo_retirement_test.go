package health

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyMemoRetired_PassesWhenPathAbsent(t *testing.T) {
	home := t.TempDir()

	ok, detail := legacyMemoRetired(home)
	if !ok {
		t.Fatalf("legacyMemoRetired(%q) = false, detail=%q", home, detail)
	}
	if detail != "" {
		t.Fatalf("legacyMemoRetired(%q) detail = %q, want empty", home, detail)
	}
}

func TestLegacyMemoRetired_FailsForSymlink(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(home, "Code", "global-kb")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(home, "memo")); err != nil {
		t.Fatal(err)
	}

	ok, detail := legacyMemoRetired(home)
	if ok {
		t.Fatalf("legacyMemoRetired(%q) = true, want false", home)
	}
	if detail == "" {
		t.Fatal("legacyMemoRetired() detail = empty, want explanation")
	}
}

func TestLegacyMemoRetired_FailsForDirectory(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "memo"), 0o755); err != nil {
		t.Fatal(err)
	}

	ok, detail := legacyMemoRetired(home)
	if ok {
		t.Fatalf("legacyMemoRetired(%q) = true, want false", home)
	}
	if detail == "" {
		t.Fatal("legacyMemoRetired() detail = empty, want explanation")
	}
}
