package worktree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureGitignore_CreatesEntry(t *testing.T) {
	tmp := t.TempDir()
	gitignorePath := filepath.Join(tmp, ".gitignore")

	if err := EnsureGitignore(tmp, ".worktrees/"); err != nil {
		t.Fatalf("EnsureGitignore: %v", err)
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}

	if got := string(data); got != ".worktrees/\n" {
		t.Errorf("expected '.worktrees/\\n', got %q", got)
	}
}

func TestEnsureGitignore_SkipsIfPresent(t *testing.T) {
	tmp := t.TempDir()
	gitignorePath := filepath.Join(tmp, ".gitignore")
	initial := "node_modules/\n.worktrees/\n"
	if err := os.WriteFile(gitignorePath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGitignore(tmp, ".worktrees/"); err != nil {
		t.Fatalf("EnsureGitignore: %v", err)
	}

	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatal(err)
	}

	if got := string(data); got != initial {
		t.Errorf("expected file unchanged, got %q", got)
	}
}

func TestCopyEnvFiles(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	envFiles := []string{".env", ".env.local", ".env.test"}
	for _, name := range envFiles {
		if err := os.WriteFile(filepath.Join(src, name), []byte("KEY=val-"+name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Also create a non-env file that should NOT be copied.
	if err := os.WriteFile(filepath.Join(src, "README.md"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	copied, err := CopyEnvFiles(src, dst)
	if err != nil {
		t.Fatalf("CopyEnvFiles: %v", err)
	}

	if len(copied) != len(envFiles) {
		t.Errorf("expected %d copied files, got %d", len(envFiles), len(copied))
	}

	for _, name := range envFiles {
		data, readErr := os.ReadFile(filepath.Join(dst, name))
		if readErr != nil {
			t.Errorf("expected %s in dst: %v", name, readErr)
			continue
		}
		if string(data) != "KEY=val-"+name {
			t.Errorf("%s: expected 'KEY=val-%s', got %q", name, name, string(data))
		}
	}

	if _, err := os.Stat(filepath.Join(dst, "README.md")); !os.IsNotExist(err) {
		t.Error("README.md should NOT have been copied")
	}
}

func TestCopyEnvFiles_NoEnvFiles(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	copied, err := CopyEnvFiles(src, dst)
	if err != nil {
		t.Fatalf("CopyEnvFiles: %v", err)
	}
	if len(copied) != 0 {
		t.Errorf("expected 0 copied files, got %d", len(copied))
	}
}

func TestParseWorktreeList(t *testing.T) {
	output := `/home/dev/project abc1234 [main]
/home/dev/.worktrees/feature-auth def5678 [feature-auth]
/home/dev/.worktrees/write-tests  ghi9012 [write-tests]
`
	entries := ParseWorktreeList(output)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	tests := []struct {
		idx    int
		path   string
		branch string
	}{
		{0, "/home/dev/project", "main"},
		{1, "/home/dev/.worktrees/feature-auth", "feature-auth"},
		{2, "/home/dev/.worktrees/write-tests", "write-tests"},
	}
	for _, tc := range tests {
		e := entries[tc.idx]
		if e.Path != tc.path {
			t.Errorf("[%d] path: expected %q, got %q", tc.idx, tc.path, e.Path)
		}
		if e.Branch != tc.branch {
			t.Errorf("[%d] branch: expected %q, got %q", tc.idx, tc.branch, e.Branch)
		}
	}
}

func TestParseWorktreeList_Empty(t *testing.T) {
	entries := ParseWorktreeList("")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}
