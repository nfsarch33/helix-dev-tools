package cicd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMirror_Push_NoRepo(t *testing.T) {
	m := NewMirror()
	result := m.Push(context.Background(), "/nonexistent/path", "https://example.com/repo.git", "main")
	if result.Success {
		t.Error("expected failure for nonexistent repo")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestMirror_AddRemote_NewRemote(t *testing.T) {
	dir := t.TempDir()
	gitInit := exec.Command("git", "init", dir)
	if out, err := gitInit.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}

	m := NewMirror()
	err := m.AddRemote(context.Background(), dir, "gitlab", "https://gitlab.example.com/group/repo.git")
	if err != nil {
		t.Fatalf("AddRemote error: %v", err)
	}

	checkCmd := exec.Command("git", "-C", dir, "remote", "get-url", "gitlab")
	out, err := checkCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("get-url failed: %v", err)
	}
	got := filepath.Clean(string(out))
	if got == "" {
		t.Error("remote URL should be set")
	}
}

func TestMirror_AddRemote_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	gitInit := exec.Command("git", "init", dir)
	gitInit.Run()
	addCmd := exec.Command("git", "-C", dir, "remote", "add", "gitlab", "https://old.example.com/repo.git")
	addCmd.Run()

	m := NewMirror()
	err := m.AddRemote(context.Background(), dir, "gitlab", "https://new.example.com/repo.git")
	if err != nil {
		t.Errorf("should succeed for existing remote: %v", err)
	}
}

func TestMirror_Push_EmptyRepo(t *testing.T) {
	dir := t.TempDir()
	gitInit := exec.Command("git", "init", dir)
	gitInit.Run()

	dummyFile := filepath.Join(dir, "README.md")
	os.WriteFile(dummyFile, []byte("# test"), 0o644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()

	m := NewMirror()
	result := m.Push(context.Background(), dir, "https://nonexistent.example.com/repo.git", "main")
	if result.Success {
		t.Error("expected failure for unreachable remote")
	}
}
