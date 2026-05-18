package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCheckHelixBinaryPresence verifies that checkHelixBinary returns true when
// either helix-dev-tools or cursor-tools (symlink) is present.
func TestCheckHelixBinaryPresence(t *testing.T) {
	dir := t.TempDir()

	// Neither binary present -- expect false.
	if got := checkHelixBinary(dir); got {
		t.Error("expected false when neither binary is present")
	}

	// helix-dev-tools present -- expect true.
	helixPath := filepath.Join(dir, "helix-dev-tools")
	if err := os.WriteFile(helixPath, []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := checkHelixBinary(dir); !got {
		t.Error("expected true when helix-dev-tools is present")
	}
	os.Remove(helixPath)

	// cursor-tools symlink present -- expect true (backward-compat burn-in).
	cursorPath := filepath.Join(dir, "cursor-tools")
	if err := os.WriteFile(cursorPath, []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := checkHelixBinary(dir); !got {
		t.Error("expected true when cursor-tools is present (symlink burn-in)")
	}
}

// TestCheckHelixBinary_SymlinkResolves verifies that a symlink from
// cursor-tools to helix-dev-tools is treated as present.
func TestCheckHelixBinary_SymlinkResolves(t *testing.T) {
	dir := t.TempDir()

	helixPath := filepath.Join(dir, "helix-dev-tools")
	if err := os.WriteFile(helixPath, []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}

	symlinkPath := filepath.Join(dir, "cursor-tools")
	if err := os.Symlink(helixPath, symlinkPath); err != nil {
		t.Fatal(err)
	}

	if got := checkHelixBinary(dir); !got {
		t.Error("expected true with cursor-tools symlink resolving to helix-dev-tools")
	}
}
