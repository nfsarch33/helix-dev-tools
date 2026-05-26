package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoMemoSyncCopiesAndCommits(t *testing.T) {
	gkRoot, mRoot := setupMemoTestDirs(t)
	writeMemoFile(t, filepath.Join(gkRoot, "sop", "deploy.md"), "# Deploy\n")

	var gitCalls [][]string
	oldRunner := gitRunner
	gitRunner = func(dir string, args ...string) error {
		gitCalls = append(gitCalls, append([]string{dir}, args...))
		return nil
	}
	defer func() { gitRunner = oldRunner }()

	var buf bytes.Buffer
	if err := doMemoSync(&buf, gkRoot, mRoot); err != nil {
		t.Fatalf("doMemoSync: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[COPIED] sop/deploy.md") {
		t.Errorf("output missing copied file: %s", output)
	}
	if !strings.Contains(output, "committed") {
		t.Errorf("output missing commit confirmation: %s", output)
	}
	if len(gitCalls) != 2 {
		t.Fatalf("expected 2 git calls, got %d", len(gitCalls))
	}
	if gitCalls[0][1] != "add" || gitCalls[0][2] != "-A" {
		t.Errorf("first git call = %v, want [add -A]", gitCalls[0][1:])
	}
	if gitCalls[1][1] != "commit" {
		t.Errorf("second git call = %v, want commit", gitCalls[1][1:])
	}
}

func TestDoMemoSyncNoChanges(t *testing.T) {
	gkRoot, mRoot := setupMemoTestDirs(t)

	oldRunner := gitRunner
	gitRunner = func(string, ...string) error { return nil }
	defer func() { gitRunner = oldRunner }()

	var buf bytes.Buffer
	if err := doMemoSync(&buf, gkRoot, mRoot); err != nil {
		t.Fatalf("doMemoSync: %v", err)
	}
	if !strings.Contains(buf.String(), "no changes detected") {
		t.Errorf("expected no-changes message, got: %s", buf.String())
	}
}

func TestDoMemoSyncUpdatesReadme(t *testing.T) {
	gkRoot, mRoot := setupMemoTestDirs(t)
	writeMemoFile(t, filepath.Join(gkRoot, "sop", "test.md"), "content\n")
	writeMemoFile(t, filepath.Join(mRoot, "README.md"), "# Memo\n\nLast sync: (not yet synced)\n")

	oldRunner := gitRunner
	gitRunner = func(string, ...string) error { return nil }
	defer func() { gitRunner = oldRunner }()

	var buf bytes.Buffer
	if err := doMemoSync(&buf, gkRoot, mRoot); err != nil {
		t.Fatalf("doMemoSync: %v", err)
	}
	if !strings.Contains(buf.String(), "[UPDATED] README.md") {
		t.Errorf("expected README update message, got: %s", buf.String())
	}
	data, _ := os.ReadFile(filepath.Join(mRoot, "README.md"))
	if strings.Contains(string(data), "(not yet synced)") {
		t.Error("README still contains placeholder timestamp")
	}
}

func TestDoMemoSyncInvalidRoot(t *testing.T) {
	tmp := t.TempDir()
	var buf bytes.Buffer
	err := doMemoSync(&buf, filepath.Join(tmp, "missing"), tmp)
	if err == nil {
		t.Fatal("expected error for invalid global-kb root")
	}
}

func TestMemoSyncMultipleDirs(t *testing.T) {
	gkRoot, mRoot := setupMemoTestDirs(t)
	writeMemoFile(t, filepath.Join(gkRoot, "sop", "a.md"), "sop\n")
	writeMemoFile(t, filepath.Join(gkRoot, "adrs", "b.md"), "adr\n")
	writeMemoFile(t, filepath.Join(gkRoot, "cursor-config", "c.yaml"), "config\n")
	writeMemoFile(t, filepath.Join(gkRoot, "session-handoffs", "d.md"), "handoff\n")

	oldRunner := gitRunner
	gitRunner = func(string, ...string) error { return nil }
	defer func() { gitRunner = oldRunner }()

	var buf bytes.Buffer
	if err := doMemoSync(&buf, gkRoot, mRoot); err != nil {
		t.Fatalf("doMemoSync: %v", err)
	}
	output := buf.String()
	for _, expected := range []string{"sop/a.md", "adrs/b.md", "config/c.yaml", "handoffs/d.md"} {
		if !strings.Contains(output, expected) {
			t.Errorf("output missing %q: %s", expected, output)
		}
	}
}

func TestMemoCommandRegistered(t *testing.T) {
	if memoCmd.Use != "memo" {
		t.Fatalf("memoCmd.Use = %q, want memo", memoCmd.Use)
	}
	found := false
	for _, sub := range memoCmd.Commands() {
		if sub.Use == "sync" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("sync subcommand not registered under memo")
	}
}

func setupMemoTestDirs(t *testing.T) (gkRoot, mRoot string) {
	t.Helper()
	tmp := t.TempDir()
	gkRoot = filepath.Join(tmp, "global-kb")
	mRoot = filepath.Join(tmp, "memo")
	for _, d := range []string{gkRoot, mRoot, filepath.Join(mRoot, "sop"), filepath.Join(mRoot, "adrs"), filepath.Join(mRoot, "config"), filepath.Join(mRoot, "handoffs")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	return
}

func writeMemoFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
