package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nfsarch33/helix-dev-tools/internal/config"
	"github.com/nfsarch33/helix-dev-tools/internal/hookio"
	"github.com/nfsarch33/helix-dev-tools/internal/logger"
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
	writeExecutable(t, binDir, "git", "#!/bin/sh\necho \"$@\" >> \""+gitLog+"\"\ncase \"$*\" in\n  *\"status --porcelain\"*) echo \" M changed.go\" ;;\n  *\"pull --rebase\"*) exit 1 ;;\n  *\"status\"*) echo \"nothing to commit\" ;;\n  *) exit 0 ;;\nesac\n")

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
	for _, want := range []string{
		"-C " + p.GlobalKB + " config --local rerere.enabled true",
		"-C " + p.GlobalKB + " add -A",
		"-C " + p.GlobalKB + " commit -m auto: session sync",
		"-C " + p.GlobalKB + " pull --rebase origin main",
		"-C " + p.GlobalKB + " push origin main",
		"-C " + p.GlobalKB + " fetch origin --quiet",
		"-C " + p.GlobalKB + " merge --ff-only origin/main",
	} {
		if !strings.Contains(gitText, want) {
			t.Fatalf("git log missing %q in %q", want, gitText)
		}
	}

	pushState := filepath.Join(p.HooksDir, "last-push-result.txt")
	if _, err := os.Stat(pushState); err != nil {
		t.Fatalf("last-push-result.txt not created: %v", err)
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
	logText := string(logData)
	if !strings.Contains(logText, `"msg":"stop event received"`) || !strings.Contains(logText, `"status":"completed"`) {
		t.Fatalf("housekeeping log missing completed event: %q", string(logData))
	}

	helperData, err := os.ReadFile(filepath.Join(home, "helper.log"))
	if err != nil {
		t.Fatal(err)
	}
	helperText := string(helperData)
	resourceIdx := strings.Index(helperText, "resource-probe-once")
	handoffIdx := strings.Index(helperText, "session-handoff")
	if resourceIdx == -1 {
		t.Fatalf("helper log missing resource-probe-once in %q", helperText)
	}
	if handoffIdx == -1 {
		t.Fatalf("helper log missing session-handoff in %q", helperText)
	}
	if resourceIdx > handoffIdx {
		t.Fatalf("resource-probe-once should run before session-handoff: %q", helperText)
	}
}

func TestCleanCoordinationSignals_CalledOnCompleted(t *testing.T) {
	cleanCalled := false
	origClean := cleanCoordinationSignalsFn
	cleanCoordinationSignalsFn = func(_ config.Paths, _ *logger.Logger) {
		cleanCalled = true
	}
	defer func() { cleanCoordinationSignalsFn = origClean }()

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
	restorePath := prependPath(t, binDir)
	defer restorePath()
	writeExecutable(t, binDir, "git", "#!/bin/sh\nexit 0\n")

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

	_, err := h.Handle(context.Background(), &hookio.Input{Status: "completed"})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !cleanCalled {
		t.Error("cleanCoordinationSignals was not called on 'completed' status")
	}
}

func TestCleanCoordinationSignals_SkippedOnNonCompleted(t *testing.T) {
	cleanCalled := false
	origClean := cleanCoordinationSignalsFn
	cleanCoordinationSignalsFn = func(_ config.Paths, _ *logger.Logger) {
		cleanCalled = true
	}
	defer func() { cleanCoordinationSignalsFn = origClean }()

	oldHome := os.Getenv("HOME")
	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	defer os.Setenv("HOME", oldHome)

	binDir := t.TempDir()
	restorePath := prependPath(t, binDir)
	defer restorePath()
	writeExecutable(t, binDir, "git", "#!/bin/sh\nexit 0\n")

	p := config.DefaultPaths()
	os.MkdirAll(p.HooksDir, 0o755)
	os.MkdirAll(filepath.Join(p.GlobalKB, ".git"), 0o755)

	h := &housekeepingHandler{
		log:   logger.New(filepath.Join(home, "housekeeping.log")),
		paths: p,
	}

	_, err := h.Handle(context.Background(), &hookio.Input{Status: "pending"})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if cleanCalled {
		t.Error("cleanCoordinationSignals should NOT be called on 'pending' status")
	}
}

func TestMaybeFleetPreflight_SkippedWithoutEnv(t *testing.T) {
	old := fleetPreflightHTTPGet
	called := false
	fleetPreflightHTTPGet = func(_ context.Context, _ *http.Client, _ string) (int, error) {
		called = true
		return 200, nil
	}
	defer func() { fleetPreflightHTTPGet = old }()

	oldPref := os.Getenv("CURSOR_TOOLS_FLEET_PREFLIGHT")
	_ = os.Unsetenv("CURSOR_TOOLS_FLEET_PREFLIGHT")
	defer func() { _ = os.Setenv("CURSOR_TOOLS_FLEET_PREFLIGHT", oldPref) }()

	home := t.TempDir()
	h := &housekeepingHandler{
		log:   logger.New(filepath.Join(home, "housekeeping.log")),
		paths: config.DefaultPaths(),
	}
	h.maybeFleetPreflight()
	if called {
		t.Fatal("fleet preflight should not run without CURSOR_TOOLS_FLEET_PREFLIGHT=1")
	}
}

func TestMaybeFleetPreflight_OK(t *testing.T) {
	old := fleetPreflightHTTPGet
	fleetPreflightHTTPGet = func(_ context.Context, _ *http.Client, _ string) (int, error) {
		return 200, nil
	}
	defer func() { fleetPreflightHTTPGet = old }()

	oldPref := os.Getenv("CURSOR_TOOLS_FLEET_PREFLIGHT")
	_ = os.Setenv("CURSOR_TOOLS_FLEET_PREFLIGHT", "1")
	defer func() { _ = os.Setenv("CURSOR_TOOLS_FLEET_PREFLIGHT", oldPref) }()

	home := t.TempDir()
	h := &housekeepingHandler{
		log:   logger.New(filepath.Join(home, "housekeeping.log")),
		paths: config.DefaultPaths(),
	}
	h.maybeFleetPreflight()
	data, err := os.ReadFile(filepath.Join(home, "housekeeping.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "fleet preflight: ok drl-service") || !strings.Contains(string(data), "fleet preflight: ok prometheus") {
		t.Fatalf("expected ok lines, got %q", string(data))
	}
}

func TestMaybeFleetPreflight_WarnOnError(t *testing.T) {
	old := fleetPreflightHTTPGet
	fleetPreflightHTTPGet = func(_ context.Context, _ *http.Client, _ string) (int, error) {
		return 0, fmt.Errorf("refused")
	}
	defer func() { fleetPreflightHTTPGet = old }()

	oldPref := os.Getenv("CURSOR_TOOLS_FLEET_PREFLIGHT")
	_ = os.Setenv("CURSOR_TOOLS_FLEET_PREFLIGHT", "1")
	defer func() { _ = os.Setenv("CURSOR_TOOLS_FLEET_PREFLIGHT", oldPref) }()

	home := t.TempDir()
	h := &housekeepingHandler{
		log:   logger.New(filepath.Join(home, "housekeeping.log")),
		paths: config.DefaultPaths(),
	}
	h.maybeFleetPreflight()
	data, err := os.ReadFile(filepath.Join(home, "housekeeping.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "fleet preflight: drl-service unreachable") {
		t.Fatalf("expected warn, got %q", string(data))
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
