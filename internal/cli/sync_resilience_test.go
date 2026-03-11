package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSafeRebase_Success(t *testing.T) {
	binDir := t.TempDir()
	gitLog := filepath.Join(binDir, "git.log")
	restorePath := prependPath(t, binDir)
	defer restorePath()
	writeExecutable(t, binDir, "git", "#!/bin/sh\necho \"$@\" >> \""+gitLog+"\"\nexit 0\n")

	repo := t.TempDir()
	if err := safeRebase(repo); err != nil {
		t.Fatalf("safeRebase() returned error on success: %v", err)
	}
	data, _ := os.ReadFile(gitLog)
	if !strings.Contains(string(data), "pull --rebase origin main") {
		t.Fatalf("expected pull --rebase in git log: %s", string(data))
	}
}

func TestSafeRebase_ConflictAborts(t *testing.T) {
	binDir := t.TempDir()
	gitLog := filepath.Join(binDir, "git.log")
	restorePath := prependPath(t, binDir)
	defer restorePath()
	writeExecutable(t, binDir, "git", "#!/bin/sh\necho \"$@\" >> \""+gitLog+"\"\ncase \"$*\" in\n  *\"pull --rebase\"*) exit 1 ;;\n  *status*) echo \"rebase in progress\" ;;\n  *\"diff --name-only\"*) echo \"conflicted.go\" ;;\n  *) exit 0 ;;\nesac\n")

	repo := t.TempDir()
	err := safeRebase(repo)
	if err == nil {
		t.Fatal("safeRebase() should return error on conflict")
	}
	if !strings.Contains(err.Error(), "conflicted.go") {
		t.Fatalf("error should mention conflicted file, got: %v", err)
	}

	data, _ := os.ReadFile(gitLog)
	if !strings.Contains(string(data), "rebase --abort") {
		t.Fatal("expected rebase --abort after conflict")
	}
}

func TestSafeRebase_PullFailNoRebase(t *testing.T) {
	binDir := t.TempDir()
	restorePath := prependPath(t, binDir)
	defer restorePath()
	writeExecutable(t, binDir, "git", "#!/bin/sh\ncase \"$*\" in\n  *\"pull --rebase\"*) exit 1 ;;\n  *status*) echo \"nothing to commit\" ;;\n  *) exit 0 ;;\nesac\n")

	repo := t.TempDir()
	err := safeRebase(repo)
	if err == nil {
		t.Fatal("safeRebase() should return error when pull fails")
	}
	if !strings.Contains(err.Error(), "offline or non-fast-forward") {
		t.Fatalf("expected offline/non-ff error, got: %v", err)
	}
}

func TestPushWithRetry_FirstAttemptSuccess(t *testing.T) {
	binDir := t.TempDir()
	restorePath := prependPath(t, binDir)
	defer restorePath()
	writeExecutable(t, binDir, "git", "#!/bin/sh\nexit 0\n")

	repo := t.TempDir()
	result := pushWithRetry(repo, 3)
	if result.Err != nil {
		t.Fatalf("pushWithRetry() error = %v", result.Err)
	}
	if result.Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", result.Attempts)
	}
}

func TestPushWithRetry_RetriesOnRejection(t *testing.T) {
	binDir := t.TempDir()
	counterFile := filepath.Join(binDir, "push_count")
	restorePath := prependPath(t, binDir)
	defer restorePath()

	writeExecutable(t, binDir, "git", `#!/bin/sh
case "$*" in
  *"push origin main"*)
    count=$(cat "`+counterFile+`" 2>/dev/null || echo 0)
    count=$((count + 1))
    echo $count > "`+counterFile+`"
    if [ "$count" -lt 2 ]; then exit 1; fi
    exit 0
    ;;
  *) exit 0 ;;
esac
`)

	repo := t.TempDir()
	result := pushWithRetry(repo, 3)
	if result.Err != nil {
		t.Fatalf("pushWithRetry() should succeed on retry, got: %v", result.Err)
	}
	if result.Attempts < 2 {
		t.Fatalf("expected >= 2 attempts, got %d", result.Attempts)
	}
}

func TestPushWithRetry_AllAttemptsFail(t *testing.T) {
	binDir := t.TempDir()
	restorePath := prependPath(t, binDir)
	defer restorePath()

	writeExecutable(t, binDir, "git", "#!/bin/sh\ncase \"$*\" in\n  *push*) exit 1 ;;\n  *status*) echo \"nothing\" ;;\n  *) exit 0 ;;\nesac\n")

	repo := t.TempDir()
	result := pushWithRetry(repo, 2)
	if result.Err == nil {
		t.Fatal("pushWithRetry() should fail after all attempts exhausted")
	}
	if result.Attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", result.Attempts)
	}
}

func TestWritePushState_Success(t *testing.T) {
	dir := t.TempDir()
	writePushState(dir, pushResult{Attempts: 1})

	data, err := os.ReadFile(filepath.Join(dir, "last-push-result.txt"))
	if err != nil {
		t.Fatalf("state file not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "result: success") {
		t.Fatalf("expected 'result: success', got: %s", content)
	}
	if !strings.Contains(content, "attempts: 1") {
		t.Fatalf("expected 'attempts: 1', got: %s", content)
	}
}

func TestWritePushState_Failure(t *testing.T) {
	dir := t.TempDir()
	writePushState(dir, pushResult{
		Attempts:    3,
		Err:         os.ErrPermission,
		Conflicting: "file.go",
	})

	data, err := os.ReadFile(filepath.Join(dir, "last-push-result.txt"))
	if err != nil {
		t.Fatalf("state file not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "result: failed") {
		t.Fatalf("expected 'result: failed', got: %s", content)
	}
	if !strings.Contains(content, "conflicting: file.go") {
		t.Fatalf("expected conflicting files, got: %s", content)
	}
}

func TestEnsureGitSyncConfig(t *testing.T) {
	binDir := t.TempDir()
	gitLog := filepath.Join(binDir, "git.log")
	restorePath := prependPath(t, binDir)
	defer restorePath()
	writeExecutable(t, binDir, "git", "#!/bin/sh\necho \"$@\" >> \""+gitLog+"\"\nexit 0\n")

	repo := t.TempDir()
	ensureGitSyncConfig(repo)

	data, err := os.ReadFile(gitLog)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{
		"config --local rerere.enabled true",
		"config --local rerere.autoupdate true",
		"config --local merge.ours.driver true",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("git log missing %q in: %s", want, content)
		}
	}
}

func TestPlatformSuffix(t *testing.T) {
	suffix := platformSuffix()
	switch runtime.GOOS {
	case "darwin":
		if suffix != "macos" {
			t.Fatalf("expected 'macos' on darwin, got %q", suffix)
		}
	case "linux":
		if suffix != "wsl" && suffix != "linux" {
			t.Fatalf("expected 'wsl' or 'linux' on Linux, got %q", suffix)
		}
	default:
		if suffix == "" {
			t.Fatal("platformSuffix() returned empty string")
		}
	}
}

func TestGitCmdOutput(t *testing.T) {
	binDir := t.TempDir()
	restorePath := prependPath(t, binDir)
	defer restorePath()
	writeExecutable(t, binDir, "git", "#!/bin/sh\necho \"hello world\"\n")

	repo := t.TempDir()
	out, err := gitCmdOutput(repo, "test")
	if err != nil {
		t.Fatalf("gitCmdOutput() error = %v", err)
	}
	if out != "hello world" {
		t.Fatalf("expected 'hello world', got %q", out)
	}
}
