package importaudit_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/importaudit"
)

// TestScanDirectory_FindsOldModulePath verifies that ScanDirectory correctly
// identifies Go files containing the old module import prefix.
func TestScanDirectory_FindsOldModulePath(t *testing.T) {
	dir := t.TempDir()

	// A file importing the old path.
	old := `package main
import "github.com/nfsarch33/ai-agent-business-stack/go/internal/something"
func main() {}`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := importaudit.ScanDirectory(dir, "github.com/nfsarch33/ai-agent-business-stack/go")
	if err != nil {
		t.Fatal(err)
	}
	if result.OldPathCount == 0 {
		t.Fatalf("expected at least 1 old-path import, got 0; files: %v", result.Files)
	}
}

// TestScanDirectory_CleanFileNotReported verifies that files without the old
// module path produce zero results.
func TestScanDirectory_CleanFileNotReported(t *testing.T) {
	dir := t.TempDir()

	clean := `package main
import "github.com/nfsarch33/helixon-ec/go/internal/something"
func main() {}`
	if err := os.WriteFile(filepath.Join(dir, "clean.go"), []byte(clean), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := importaudit.ScanDirectory(dir, "github.com/nfsarch33/ai-agent-business-stack/go")
	if err != nil {
		t.Fatal(err)
	}
	if result.OldPathCount != 0 {
		t.Fatalf("expected 0 old-path imports, got %d", result.OldPathCount)
	}
}

// TestScanDirectory_CountsFiles verifies FileCount includes scanned .go files.
func TestScanDirectory_CountsFiles(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"a.go", "b.go", "c.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("package p\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := importaudit.ScanDirectory(dir, "github.com/nfsarch33/ai-agent-business-stack/go")
	if err != nil {
		t.Fatal(err)
	}
	if result.FileCount < 3 {
		t.Errorf("expected FileCount >= 3, got %d", result.FileCount)
	}
}

// TestAuditResult_HasOldPaths verifies HasOldPaths() predicate.
func TestAuditResult_HasOldPaths(t *testing.T) {
	clean := importaudit.AuditResult{OldPathCount: 0}
	if clean.HasOldPaths() {
		t.Error("expected HasOldPaths()=false for zero old paths")
	}

	dirty := importaudit.AuditResult{OldPathCount: 5}
	if !dirty.HasOldPaths() {
		t.Error("expected HasOldPaths()=true for non-zero old paths")
	}
}
