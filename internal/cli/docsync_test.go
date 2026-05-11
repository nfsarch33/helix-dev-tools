package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRunxRepoPathUsesAliasOnlyConfig(t *testing.T) {
	root := t.TempDir()
	cfg := filepath.Join(root, "runx.yaml")
	repoRoot := filepath.Join(root, "repo")
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.WriteFile(cfg, []byte("repos:\n  demo:\n    path: "+repoRoot+"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got, err := resolveRunxRepoPath(cfg, "demo")
	if err != nil {
		t.Fatalf("resolveRunxRepoPath: %v", err)
	}
	if got != repoRoot {
		t.Fatalf("path = %q, want configured path", got)
	}
}

func TestDisplayRepoDoesNotExposeResolvedPath(t *testing.T) {
	if got := displayRepo(); got != "." && got != "selected repo" {
		t.Fatalf("displayRepo = %q, want redacted label", got)
	}
}
