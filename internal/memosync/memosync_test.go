package memosync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSyncCopiesNewFiles(t *testing.T) {
	gkRoot, memoRoot := setupDirs(t)
	writeFile(t, filepath.Join(gkRoot, "sop", "example.md"), "# SOP Example\n")
	writeFile(t, filepath.Join(gkRoot, "adrs", "ADR-001.md"), "# ADR 001\n")

	res, err := Sync(gkRoot, memoRoot)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(res.Copied) != 2 {
		t.Fatalf("copied %d files, want 2", len(res.Copied))
	}

	assertFileContent(t, filepath.Join(memoRoot, "sop", "example.md"), "# SOP Example\n")
	assertFileContent(t, filepath.Join(memoRoot, "adrs", "ADR-001.md"), "# ADR 001\n")
}

func TestSyncSkipsUnchangedFiles(t *testing.T) {
	gkRoot, memoRoot := setupDirs(t)
	content := "unchanged content\n"
	writeFile(t, filepath.Join(gkRoot, "sop", "stable.md"), content)
	writeFile(t, filepath.Join(memoRoot, "sop", "stable.md"), content)

	res, err := Sync(gkRoot, memoRoot)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if res.Skipped != 1 {
		t.Fatalf("skipped %d, want 1", res.Skipped)
	}
	if len(res.Copied) != 0 {
		t.Fatalf("copied %d files, want 0", len(res.Copied))
	}
}

func TestSyncCopiesChangedFiles(t *testing.T) {
	gkRoot, memoRoot := setupDirs(t)
	writeFile(t, filepath.Join(gkRoot, "sop", "updated.md"), "version 2\n")
	writeFile(t, filepath.Join(memoRoot, "sop", "updated.md"), "version 1\n")

	res, err := Sync(gkRoot, memoRoot)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(res.Copied) != 1 {
		t.Fatalf("copied %d, want 1", len(res.Copied))
	}
	assertFileContent(t, filepath.Join(memoRoot, "sop", "updated.md"), "version 2\n")
}

func TestSyncHandlesNestedDirs(t *testing.T) {
	gkRoot, memoRoot := setupDirs(t)
	nested := filepath.Join(gkRoot, "cursor-config", "rules", "deep.mdc")
	writeFile(t, nested, "rule content\n")

	res, err := Sync(gkRoot, memoRoot)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(res.Copied) != 1 {
		t.Fatalf("copied %d, want 1", len(res.Copied))
	}
	assertFileContent(t, filepath.Join(memoRoot, "config", "rules", "deep.mdc"), "rule content\n")
}

func TestSyncSkipsMissingSrcDirs(t *testing.T) {
	gkRoot, memoRoot := setupDirs(t)

	res, err := Sync(gkRoot, memoRoot)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(res.Copied) != 0 {
		t.Fatalf("copied %d, want 0 (no source dirs)", len(res.Copied))
	}
	if len(res.Errors) != 0 {
		t.Fatalf("errors %d, want 0", len(res.Errors))
	}
}

func TestSyncRejectsInvalidRoots(t *testing.T) {
	tmp := t.TempDir()
	_, err := Sync(filepath.Join(tmp, "nonexistent"), tmp)
	if err == nil {
		t.Fatal("expected error for missing global-kb root")
	}
	_, err = Sync(tmp, filepath.Join(tmp, "nonexistent"))
	if err == nil {
		t.Fatal("expected error for missing memo root")
	}
}

func TestCommitMessageFormat(t *testing.T) {
	msg := CommitMessage()
	if !strings.HasPrefix(msg, "sync: ") {
		t.Fatalf("commit message %q missing 'sync: ' prefix", msg)
	}
	ts := strings.TrimPrefix(msg, "sync: ")
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Fatalf("commit message timestamp %q not RFC3339: %v", ts, err)
	}
}

func TestUpdateReadme(t *testing.T) {
	memoRoot := t.TempDir()
	readmePath := filepath.Join(memoRoot, "README.md")
	writeFile(t, readmePath, "# Memo\n\nLast sync: (not yet synced)\n")

	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	modified, err := UpdateReadme(memoRoot, now)
	if err != nil {
		t.Fatalf("UpdateReadme: %v", err)
	}
	if !modified {
		t.Fatal("expected modified=true")
	}

	data, _ := os.ReadFile(readmePath)
	if !strings.Contains(string(data), "Last sync: 2026-05-26T12:00:00Z") {
		t.Fatalf("README missing updated timestamp: %s", data)
	}
}

func TestUpdateReadmeNoOp(t *testing.T) {
	memoRoot := t.TempDir()
	modified, err := UpdateReadme(memoRoot, time.Now())
	if err != nil {
		t.Fatalf("UpdateReadme: %v", err)
	}
	if modified {
		t.Fatal("expected modified=false when README is missing")
	}
}

func TestSyncAllDirMappings(t *testing.T) {
	gkRoot, memoRoot := setupDirs(t)
	for srcSub := range SyncDirs {
		writeFile(t, filepath.Join(gkRoot, srcSub, "test.md"), "content for "+srcSub+"\n")
	}

	res, err := Sync(gkRoot, memoRoot)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if len(res.Copied) != len(SyncDirs) {
		t.Fatalf("copied %d, want %d (one per dir mapping)", len(res.Copied), len(SyncDirs))
	}
	for _, dstSub := range SyncDirs {
		path := filepath.Join(memoRoot, dstSub, "test.md")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file at %s: %v", path, err)
		}
	}
}

// helpers

func setupDirs(t *testing.T) (gkRoot, memoRoot string) {
	t.Helper()
	tmp := t.TempDir()
	gkRoot = filepath.Join(tmp, "global-kb")
	memoRoot = filepath.Join(tmp, "memo")
	for _, sub := range []string{gkRoot, memoRoot} {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}
	for _, sub := range SyncDirs {
		if err := os.MkdirAll(filepath.Join(memoRoot, sub), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}
	return
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertFileContent(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != expected {
		t.Fatalf("file %s: got %q, want %q", path, data, expected)
	}
}
