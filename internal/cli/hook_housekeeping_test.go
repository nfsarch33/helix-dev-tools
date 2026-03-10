package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/hookio"
	"github.com/nfsarch33/cursor-tools/internal/logger"
)

func TestHousekeepingHelpers(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldWorkspace := os.Getenv("CURSOR_WORKSPACE")
	oldHelper := os.Getenv(helperEnv)
	oldHelperLog := os.Getenv("CURSOR_TOOLS_HELPER_LOG")
	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv(helperEnv, "1"); err != nil {
		t.Fatal(err)
	}
	helperLogPath := filepath.Join(home, "helper.log")
	if err := os.Setenv("CURSOR_TOOLS_HELPER_LOG", helperLogPath); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("CURSOR_WORKSPACE", oldWorkspace)
		_ = os.Setenv(helperEnv, oldHelper)
		_ = os.Setenv("CURSOR_TOOLS_HELPER_LOG", oldHelperLog)
	}()

	binDir := t.TempDir()
	gitLog := filepath.Join(binDir, "git.log")
	restorePath := prependPath(t, binDir)
	defer restorePath()
	writeExecutable(t, binDir, "git", "#!/bin/sh\necho \"$@\" >> \""+gitLog+"\"\ncase \"$*\" in\n  *\"status --porcelain\"*) echo \" M file.go\" ;;\n  *\"fetch\"*) exit 0 ;;\n  *\"merge\"*) exit 0 ;;\n  *\"pull --rebase\"*) exit 1 ;;\n  *) exit 0 ;;\nesac\n")

	p := config.DefaultPaths()
	for _, dir := range []string{p.HooksDir, filepath.Join(p.GlobalKB, ".git"), filepath.Join(home, ".ssh")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, ".ssh", "agtc"), []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := &housekeepingHandler{
		log:   logger.New(filepath.Join(home, "housekeeping.log")),
		paths: p,
	}

	h.setSSHCommand()
	if !strings.Contains(os.Getenv("GIT_SSH_COMMAND"), filepath.Join(home, ".ssh", "agtc")) {
		t.Fatalf("GIT_SSH_COMMAND not set correctly: %q", os.Getenv("GIT_SSH_COMMAND"))
	}

	if !hasChanges(p.GlobalKB) {
		t.Fatal("hasChanges() = false, want true")
	}
	if err := gitCmd(p.GlobalKB, "status", "--porcelain"); err != nil {
		t.Fatalf("gitCmd() error = %v", err)
	}

	h.runSyncCounts()

	workspace := filepath.Join(home, "workspace")
	if err := os.MkdirAll(filepath.Join(workspace, ".learnings"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("CURSOR_WORKSPACE", workspace); err != nil {
		t.Fatal(err)
	}
	h.runPromoteLearnings()
	_ = os.Setenv("CURSOR_WORKSPACE", "")
	h.runPromoteLearnings()

	helperData, err := os.ReadFile(helperLogPath)
	if err != nil {
		t.Fatal(err)
	}
	helperText := string(helperData)
	for _, want := range []string{"sync-counts --apply", "promote --workspace " + workspace, "promote "} {
		if !strings.Contains(helperText, want) {
			t.Fatalf("helper log missing %q in %q", want, helperText)
		}
	}
}

func TestHousekeepingSyncRepoPullRepoAndHandle(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldHelper := os.Getenv(helperEnv)
	oldHelperLog := os.Getenv("CURSOR_TOOLS_HELPER_LOG")
	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv(helperEnv, "1"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("CURSOR_TOOLS_HELPER_LOG", filepath.Join(home, "helper.log")); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv(helperEnv, oldHelper)
		_ = os.Setenv("CURSOR_TOOLS_HELPER_LOG", oldHelperLog)
	}()

	binDir := t.TempDir()
	gitLog := filepath.Join(binDir, "git.log")
	restorePath := prependPath(t, binDir)
	defer restorePath()
	writeExecutable(t, binDir, "git", "#!/bin/sh\necho \"$@\" >> \""+gitLog+"\"\ncase \"$*\" in\n  *\"status --porcelain\"*) echo \" M changed.go\" ;;\n  *\"pull --rebase\"*) exit 1 ;;\n  *) exit 0 ;;\nesac\n")

	p := config.DefaultPaths()
	for _, dir := range []string{p.HooksDir, filepath.Join(p.GlobalKB, ".git")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	h := &housekeepingHandler{
		log:   logger.New(filepath.Join(home, "housekeeping.log")),
		paths: p,
	}

	h.syncRepo()
	h.pullRepo()

	gitData, err := os.ReadFile(gitLog)
	if err != nil {
		t.Fatal(err)
	}
	gitText := string(gitData)
	for _, want := range []string{"-C " + p.GlobalKB + " add -A", "-C " + p.GlobalKB + " commit -m auto: session sync", "-C " + p.GlobalKB + " pull --rebase origin main", "-C " + p.GlobalKB + " pull --ff-only origin main", "-C " + p.GlobalKB + " push", "-C " + p.GlobalKB + " fetch origin --quiet", "-C " + p.GlobalKB + " merge --ff-only origin/main"} {
		if !strings.Contains(gitText, want) {
			t.Fatalf("git log missing %q in %q", want, gitText)
		}
	}

	resp, err := h.Handle(context.Background(), &hookio.Input{Status: "completed"})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if resp.Permission != "" {
		t.Fatalf("Handle() permission = %q, want empty", resp.Permission)
	}

	logData, err := os.ReadFile(filepath.Join(home, "housekeeping.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logData), "stop event: status=completed") {
		t.Fatalf("housekeeping log missing completed event: %q", string(logData))
	}
}

func TestRunHousekeepingWritesEmptyResponseOnBadInput(t *testing.T) {
	oldHome := os.Getenv("HOME")
	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv("HOME", oldHome)

	inFile := filepath.Join(home, "input.txt")
	outFile := filepath.Join(home, "output.json")
	if err := os.WriteFile(inFile, []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(inFile)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(outFile)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	if err := runHousekeeping(in, out); err != nil {
		t.Fatalf("runHousekeeping() error = %v", err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "{}") {
		t.Fatalf("runHousekeeping() output = %q, want empty hook response", string(data))
	}
}
