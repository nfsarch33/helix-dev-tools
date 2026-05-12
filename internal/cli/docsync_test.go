package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
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

func TestDocsCheckAliasReportsReleaseChecklistDrift(t *testing.T) {
	root := t.TempDir()
	repoRoot := filepath.Join(root, "repo")
	mustWriteDocsyncFile(t, repoRoot, "VERSION", "7.5.1\n")
	mustWriteDocsyncFile(t, repoRoot, "README.md", "# Demo\n\nCurrent release: **v7.5.1**.\n")
	mustWriteDocsyncFile(t, repoRoot, "CHANGELOG.md", "## 7.5.1\n")
	mustWriteDocsyncFile(t, repoRoot, "docs/release-checklist.md", "# v6.6.0 Release Checklist\n")

	cfg := filepath.Join(root, "runx.yaml")
	if err := os.WriteFile(cfg, []byte("repos:\n  demo:\n    path: "+repoRoot+"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldConfig := docsCheckConfig
	oldAliases := docsCheckRepoAliases
	oldPublic := docsyncRequirePublicFiles
	t.Cleanup(func() {
		docsCheckConfig = oldConfig
		docsCheckRepoAliases = oldAliases
		docsyncRequirePublicFiles = oldPublic
	})
	docsCheckConfig = cfg
	docsCheckRepoAliases = []string{"demo"}
	docsyncRequirePublicFiles = false

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	err := runDocsCheckAliases(cmd)
	if err == nil {
		t.Fatal("expected release checklist drift error")
	}
	if !bytes.Contains(out.Bytes(), []byte("RELEASE_CHECKLIST_VERSION")) {
		t.Fatalf("docs-check output missing release checklist finding:\n%s", out.String())
	}
}

func mustWriteDocsyncFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}
