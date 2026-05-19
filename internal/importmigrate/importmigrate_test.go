package importmigrate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/importmigrate"
)

// fixture builds a temporary directory tree with .go files for migration tests.
func fixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", full, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	return root
}

func TestMigrate_BasicSubstitution(t *testing.T) {
	root := fixture(t, map[string]string{
		"cmd/main.go": `package main

import "github.com/nfsarch33/ai-agent-business-stack/go/internal/config"
`,
		"internal/config/config.go": `package config

import "github.com/nfsarch33/ai-agent-business-stack/go/internal/common"
`,
	})

	result, err := importmigrate.Migrate(importmigrate.Config{
		Root:      root,
		OldPrefix: "github.com/nfsarch33/ai-agent-business-stack/go",
		NewPrefix: "github.com/nfsarch33/helixon-ec/go",
		DryRun:    false,
	})
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if result.FilesChanged != 2 {
		t.Errorf("FilesChanged = %d, want 2", result.FilesChanged)
	}
	if result.FilesScanned < 2 {
		t.Errorf("FilesScanned = %d, want >= 2", result.FilesScanned)
	}
	if result.SubstitutionsTotal < 2 {
		t.Errorf("SubstitutionsTotal = %d, want >= 2", result.SubstitutionsTotal)
	}

	got, err := os.ReadFile(filepath.Join(root, "cmd/main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "ai-agent-business-stack") {
		t.Errorf("old prefix still present in cmd/main.go: %s", got)
	}
	if !strings.Contains(string(got), "github.com/nfsarch33/helixon-ec/go") {
		t.Errorf("new prefix not found in cmd/main.go: %s", got)
	}
}

func TestMigrate_DryRun_NoFileChanges(t *testing.T) {
	original := `package main

import "github.com/nfsarch33/ai-agent-business-stack/go/internal/config"
`
	root := fixture(t, map[string]string{"main.go": original})

	result, err := importmigrate.Migrate(importmigrate.Config{
		Root:      root,
		OldPrefix: "github.com/nfsarch33/ai-agent-business-stack/go",
		NewPrefix: "github.com/nfsarch33/helixon-ec/go",
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("Migrate dry-run: %v", err)
	}

	if result.FilesChanged != 1 {
		t.Errorf("DryRun FilesChanged = %d, want 1 (would change)", result.FilesChanged)
	}

	// File must be unchanged on disk.
	got, _ := os.ReadFile(filepath.Join(root, "main.go"))
	if string(got) != original {
		t.Errorf("DryRun mutated the file: got\n%s", got)
	}
}

func TestMigrate_SkipsNonGoFiles(t *testing.T) {
	root := fixture(t, map[string]string{
		"README.md": "github.com/nfsarch33/ai-agent-business-stack/go",
		"doc.go":    `package doc // no imports`,
	})

	result, err := importmigrate.Migrate(importmigrate.Config{
		Root:      root,
		OldPrefix: "github.com/nfsarch33/ai-agent-business-stack/go",
		NewPrefix: "github.com/nfsarch33/helixon-ec/go",
		DryRun:    false,
	})
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if result.FilesChanged != 0 {
		t.Errorf("FilesChanged = %d, want 0 (non-go files skipped)", result.FilesChanged)
	}
}

func TestMigrate_SkipsVendorAndGit(t *testing.T) {
	root := fixture(t, map[string]string{
		"vendor/lib/lib.go":       `import "github.com/nfsarch33/ai-agent-business-stack/go/x"`,
		".git/config":             `url = github.com/nfsarch33/ai-agent-business-stack/go`,
		"internal/pkg/pkg.go":    `import "github.com/nfsarch33/ai-agent-business-stack/go/internal/pkg"`,
	})

	result, err := importmigrate.Migrate(importmigrate.Config{
		Root:      root,
		OldPrefix: "github.com/nfsarch33/ai-agent-business-stack/go",
		NewPrefix: "github.com/nfsarch33/helixon-ec/go",
		DryRun:    false,
	})
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	if result.FilesChanged != 1 {
		t.Errorf("FilesChanged = %d, want 1 (only internal/pkg/pkg.go)", result.FilesChanged)
	}

	// vendor file must be untouched.
	got, _ := os.ReadFile(filepath.Join(root, "vendor/lib/lib.go"))
	if !strings.Contains(string(got), "ai-agent-business-stack") {
		t.Errorf("vendor file was mutated")
	}
}

func TestMigrate_PreservesGoModReplaceDirective(t *testing.T) {
	// go.mod has a replace directive -- Migrate must NOT rewrite it (go.mod is
	// not a .go file, so the walker skips it; test confirms this explicitly).
	gomod := `module github.com/nfsarch33/ai-agent-business-stack/go

go 1.25.6

replace github.com/cloudwego/eino v0.8.13 => ../../eino
`
	root := fixture(t, map[string]string{
		"go.mod":        gomod,
		"main.go":       `import "github.com/nfsarch33/ai-agent-business-stack/go/internal/x"`,
	})

	_, err := importmigrate.Migrate(importmigrate.Config{
		Root:      root,
		OldPrefix: "github.com/nfsarch33/ai-agent-business-stack/go",
		NewPrefix: "github.com/nfsarch33/helixon-ec/go",
		DryRun:    false,
	})
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(root, "go.mod"))
	if string(got) != gomod {
		t.Errorf("go.mod was mutated:\n%s", got)
	}
}

func TestMigrate_ConcurrentSafety(t *testing.T) {
	// Large fixture to exercise concurrent writes under race detector.
	files := make(map[string]string, 50)
	for i := 0; i < 50; i++ {
		files[filepath.Join("pkg", string(rune('a'+i%26)), "f.go")] =
			`import "github.com/nfsarch33/ai-agent-business-stack/go/internal/x"`
	}
	root := fixture(t, files)

	result, err := importmigrate.Migrate(importmigrate.Config{
		Root:        root,
		OldPrefix:   "github.com/nfsarch33/ai-agent-business-stack/go",
		NewPrefix:   "github.com/nfsarch33/helixon-ec/go",
		DryRun:      false,
		Concurrency: 8,
	})
	if err != nil {
		t.Fatalf("Migrate concurrent: %v", err)
	}
	if result.FilesChanged < 1 {
		t.Errorf("expected files changed > 0")
	}
}

func TestMigrateResult_Summary(t *testing.T) {
	r := importmigrate.Result{
		FilesScanned:       100,
		FilesChanged:       13,
		SubstitutionsTotal: 42,
	}
	s := r.Summary()
	if !strings.Contains(s, "13") || !strings.Contains(s, "42") {
		t.Errorf("Summary() missing counts: %s", s)
	}
}
