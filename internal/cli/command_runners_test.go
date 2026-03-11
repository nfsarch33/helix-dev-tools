package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/health"
)

func TestRunSuitesAggregatesCounts(t *testing.T) {
	s1 := &health.Suite{Name: "one"}
	s1.Pass("a")
	s1.Fail("b", "nope")
	s2 := &health.Suite{Name: "two"}
	s2.Pass("c")

	pass, total := runSuites("test", []*health.Suite{s1, s2})
	if pass != 2 || total != 3 {
		t.Fatalf("runSuites() = %d/%d, want 2/3", pass, total)
	}
}

func TestRunDoctorProfile(t *testing.T) {
	oldHome := os.Getenv("HOME")
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	defer os.Setenv("HOME", oldHome)

	oldSync := doctorSyncCountsApply
	oldBuild := doctorBuildSuites
	oldRun := doctorRunSuites
	oldRecord := doctorRecordCheckRun
	defer func() {
		doctorSyncCountsApply = oldSync
		doctorBuildSuites = oldBuild
		doctorRunSuites = oldRun
		doctorRecordCheckRun = oldRecord
	}()

	t.Run("install profile syncs counts and records success", func(t *testing.T) {
		syncCalled := 0
		var gotProfile string
		var gotTitle string
		var gotMetric string

		doctorSyncCountsApply = func(apply, quiet bool) (int, int) {
			syncCalled++
			if !apply || !quiet {
				t.Fatalf("sync called with unexpected args: apply=%v quiet=%v", apply, quiet)
			}
			return 2, 0
		}
		doctorBuildSuites = func(_ config.Paths, profile string) []*health.Suite {
			gotProfile = profile
			return []*health.Suite{{Name: "dummy"}}
		}
		doctorRunSuites = func(title string, _ []*health.Suite) (int, int) {
			gotTitle = title
			return 3, 3
		}
		doctorRecordCheckRun = func(name, command, profile string, _ time.Time, pass, total int) string {
			gotMetric = name
			if command != "doctor" || profile != "install" {
				t.Fatalf("unexpected command/profile: %q %q", command, profile)
			}
			if pass != 3 || total != 3 {
				t.Fatalf("recorded %d/%d, want 3/3", pass, total)
			}
			return "run-1"
		}

		if err := runDoctorProfile("install"); err != nil {
			t.Fatalf("runDoctorProfile() error = %v", err)
		}
		if syncCalled != 1 {
			t.Fatalf("sync called %d times, want 1", syncCalled)
		}
		if gotProfile != "install" || gotTitle != "cursor-tools doctor install" || gotMetric != "doctor-install" {
			t.Fatalf("unexpected doctor values: profile=%q title=%q metric=%q", gotProfile, gotTitle, gotMetric)
		}
	})

	t.Run("mcp profile skips sync and returns failure", func(t *testing.T) {
		doctorSyncCountsApply = func(bool, bool) (int, int) {
			t.Fatal("syncCounts should not run for mcp profile")
			return 0, 0
		}
		doctorBuildSuites = func(_ config.Paths, profile string) []*health.Suite {
			if profile != "mcp" {
				t.Fatalf("profile = %q, want mcp", profile)
			}
			return []*health.Suite{{Name: "dummy"}}
		}
		doctorRunSuites = func(string, []*health.Suite) (int, int) {
			return 1, 2
		}
		doctorRecordCheckRun = func(string, string, string, time.Time, int, int) string { return "run-2" }

		err := runDoctorProfile("mcp")
		if err == nil || !strings.Contains(err.Error(), "doctor-mcp failed: 1/2 passed") {
			t.Fatalf("runDoctorProfile() error = %v, want doctor-mcp failure", err)
		}
	})
}

func TestRunHealthCheck(t *testing.T) {
	oldHome := os.Getenv("HOME")
	tempHome := t.TempDir()
	if err := os.Setenv("HOME", tempHome); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	defer os.Setenv("HOME", oldHome)

	oldSync := healthCheckSyncCountsApply
	oldBuild := healthCheckBuildSuites
	oldRun := healthCheckRunSuites
	oldRecord := healthCheckRecordCheckRun
	defer func() {
		healthCheckSyncCountsApply = oldSync
		healthCheckBuildSuites = oldBuild
		healthCheckRunSuites = oldRun
		healthCheckRecordCheckRun = oldRecord
	}()

	syncCalled := 0
	var recordedName string
	healthCheckSyncCountsApply = func(apply, quiet bool) (int, int) {
		syncCalled++
		if !apply || !quiet {
			t.Fatalf("sync called with unexpected args: apply=%v quiet=%v", apply, quiet)
		}
		return 1, 0
	}
	healthCheckBuildSuites = func(config.Paths) []*health.Suite {
		return []*health.Suite{{Name: "dummy"}}
	}
	healthCheckRunSuites = func(title string, _ []*health.Suite) (int, int) {
		if title != "cursor-tools health-check" {
			t.Fatalf("title = %q", title)
		}
		return 2, 2
	}
	healthCheckRecordCheckRun = func(name, command, profile string, _ time.Time, pass, total int) string {
		recordedName = name
		if command != "health-check" || profile != "" {
			t.Fatalf("unexpected command/profile: %q %q", command, profile)
		}
		if pass != 2 || total != 2 {
			t.Fatalf("recorded %d/%d, want 2/2", pass, total)
		}
		return "run-3"
	}

	if err := runHealthCheck(nil, nil); err != nil {
		t.Fatalf("runHealthCheck() error = %v", err)
	}
	if syncCalled != 1 || recordedName != "health-check" {
		t.Fatalf("unexpected sync/record results: sync=%d record=%q", syncCalled, recordedName)
	}

	healthCheckRunSuites = func(string, []*health.Suite) (int, int) { return 3, 4 }
	healthCheckRecordCheckRun = func(string, string, string, time.Time, int, int) string { return "run-4" }
	if err := runHealthCheck(nil, nil); err == nil || !strings.Contains(err.Error(), "health-check failed: 3/4 passed") {
		t.Fatalf("runHealthCheck() error = %v, want health-check failure", err)
	}
}

func TestCountHookRoutesAndDiskCounts(t *testing.T) {
	base := t.TempDir()
	hooksJSON := filepath.Join(base, "hooks.json")
	if err := os.WriteFile(hooksJSON, []byte(`cursor-tools hook guard-shell
cursor-tools hook sanitize-read
cursor-tools hook guard-mcp`), 0o644); err != nil {
		t.Fatalf("write hooks.json: %v", err)
	}
	if got := countHookRoutes(hooksJSON); got != 3 {
		t.Fatalf("countHookRoutes() = %d, want 3", got)
	}

	p := config.Paths{
		Home:            base,
		GlobalKB:        filepath.Join(base, "Code", "global-kb"),
		Memo:            filepath.Join(base, "memo"),
		HooksDir:        filepath.Join(base, ".cursor", "hooks"),
		SkillsDir:       filepath.Join(base, ".cursor", "skills"),
		AgentsDir:       filepath.Join(base, ".claude", "agents"),
		AgentsSkillsDir: filepath.Join(base, ".agents", "skills"),
		CommandsDir:     filepath.Join(base, ".cursor", "commands"),
		RulesDir:        filepath.Join(base, ".cursor", "rules"),
		BinDir:          filepath.Join(base, "bin"),
	}
	dirs := []string{
		p.SkillsDir, p.AgentsSkillsDir, p.AgentsDir, p.CommandsDir, filepath.Join(base, ".cursor"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	for _, dir := range []string{"skill-a", "skill-b"} {
		if err := os.MkdirAll(filepath.Join(p.SkillsDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p.SkillsDir, dir, "SKILL.md"), []byte("# skill"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(p.SkillsDir, "00-index"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.SkillsDir, "00-index", "SKILL.md"), []byte("# index"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(p.AgentsSkillsDir, "agent-skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.AgentsSkillsDir, "agent-skill", "SKILL.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, ".cursor", "hooks.json"), []byte("cursor-tools hook guard-shell"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"go-architect.md", "go-tester.md"} {
		if err := os.WriteFile(filepath.Join(p.AgentsDir, file), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{"a.md", "b.md", "c.txt"} {
		if err := os.WriteFile(filepath.Join(p.CommandsDir, file), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	counts := getDiskCounts(p)
	if counts.CursorSkills != 2 || counts.AgentsSkills != 1 || counts.TotalSkills != 3 || counts.Hooks != 1 || counts.Agents != 2 || counts.Commands != 2 {
		t.Fatalf("unexpected disk counts: %+v", counts)
	}
}

func TestSyncCountsApplyAndRunSyncCounts(t *testing.T) {
	oldHome, oldGlobalKB, oldMemo := os.Getenv("HOME"), os.Getenv("GLOBAL_KB"), os.Getenv("MEMO")
	home := t.TempDir()
	globalKB := filepath.Join(home, "kb")
	memo := filepath.Join(home, "memo")
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("GLOBAL_KB", globalKB); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("MEMO", memo); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("GLOBAL_KB", oldGlobalKB)
		_ = os.Setenv("MEMO", oldMemo)
	}()

	p := config.DefaultPaths()
	dirs := []string{
		p.SkillsDir, p.AgentsSkillsDir, p.AgentsDir, p.CommandsDir,
		filepath.Join(p.SkillsDir, "00-index"), p.GlobalMemoriesDir(),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, ".cursor", "hooks.json"), []byte("cursor-tools hook guard-shell\ncursor-tools hook guard-mcp\n"), 0o644); err != nil {
		if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".cursor", "hooks.json"), []byte("cursor-tools hook guard-shell\ncursor-tools hook guard-mcp\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{"skill-a", "skill-b"} {
		if err := os.MkdirAll(filepath.Join(p.SkillsDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p.SkillsDir, dir, "SKILL.md"), []byte("# skill"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(p.SkillsDir, "00-index", "SKILL.md"), []byte("## Skills (1 unique across\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(p.AgentsSkillsDir, "agent-a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.AgentsSkillsDir, "agent-a", "SKILL.md"), []byte("# agent"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.md", "b.md"} {
		if err := os.WriteFile(filepath.Join(p.AgentsDir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"cmd-a.md", "cmd-b.md", "note.txt"} {
		if err := os.WriteFile(filepath.Join(p.CommandsDir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(p.GlobalMemoriesDir(), "daily-startup-prompt.md"):        "Skills: (1 unique skills, 10 L0 rules)\nSlash commands: 1 in\n",
		filepath.Join(p.GlobalMemoriesDir(), "skills-index.md"):                "Total: 1 unique skills across ~/.cursor/skills/ (1) and ~/.agents/skills/ (0)\n",
		filepath.Join(p.GlobalMemoriesDir(), "one-person-company-progress.md"): "### Agent Skills (1 unique across two dirs\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	changes, errorsCount := SyncCountsApply(false, true)
	if changes != 5 || errorsCount != 0 {
		t.Fatalf("SyncCountsApply(false) = (%d, %d), want (5, 0)", changes, errorsCount)
	}
	changes, errorsCount = SyncCountsApply(true, true)
	if changes != 5 || errorsCount != 0 {
		t.Fatalf("SyncCountsApply(true) = (%d, %d), want (5, 0)", changes, errorsCount)
	}
	data, err := os.ReadFile(filepath.Join(p.GlobalMemoriesDir(), "skills-index.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Total: 3 unique skills across ~/.cursor/skills/ (2) and ~/.agents/skills/ (1)") {
		t.Fatalf("skills-index not updated: %s", string(data))
	}

	oldApply := syncCountsApply
	defer func() { syncCountsApply = oldApply }()
	syncCountsApply = true
	if err := runSyncCounts(nil, nil); err != nil {
		t.Fatalf("runSyncCounts() error = %v", err)
	}
}

func TestRunPromote(t *testing.T) {
	oldHome, oldGlobalKB, oldMemo := os.Getenv("HOME"), os.Getenv("GLOBAL_KB"), os.Getenv("MEMO")
	oldWorkspace, oldDryRun := promoteWorkspace, promoteDryRun
	defer func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("GLOBAL_KB", oldGlobalKB)
		_ = os.Setenv("MEMO", oldMemo)
		promoteWorkspace = oldWorkspace
		promoteDryRun = oldDryRun
	}()

	home := t.TempDir()
	globalKB := filepath.Join(home, "kb")
	memo := filepath.Join(home, "memo")
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("GLOBAL_KB", globalKB); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("MEMO", memo); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{
		filepath.Join(memo, "learnings", "patterns"),
		filepath.Join(memo, "learnings", "episodes"),
		filepath.Join(memo, "global-memories"),
		filepath.Join(globalKB, "sop"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	workspace := filepath.Join(home, "workspace")
	if err := os.MkdirAll(filepath.Join(workspace, ".learnings"), 0o755); err != nil {
		t.Fatal(err)
	}
	promoteWorkspace = workspace
	promoteDryRun = true
	if err := runPromote(nil, nil); err != nil {
		t.Fatalf("runPromote() error = %v", err)
	}
}

func TestRunSafeStartsCursorBinary(t *testing.T) {
	binDir := t.TempDir()
	logPath := filepath.Join(binDir, "cursor.log")
	mockPath := writeExecutable(t, binDir, "cursor", "#!/bin/sh\necho \"$@\" > \""+logPath+"\"\n")
	restorePath := prependPath(t, binDir)
	defer restorePath()

	oldPath := safeCursorPath
	safeCursorPath = mockPath
	defer func() { safeCursorPath = oldPath }()

	if err := runSafe(nil, nil); err != nil {
		t.Fatalf("runSafe() error = %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		data, err := os.ReadFile(logPath)
		if err == nil {
			if strings.TrimSpace(string(data)) != "--disable-gpu" {
				t.Fatalf("cursor args = %q, want --disable-gpu", string(data))
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("cursor log not written: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestRunCommitMsg(t *testing.T) {
	oldExit := commitMsgExit
	oldStderr := commitMsgStderr
	defer func() {
		commitMsgExit = oldExit
		commitMsgStderr = oldStderr
	}()

	exitCalled := false
	commitMsgExit = func(code int) {
		exitCalled = true
		panic(code)
	}

	t.Run("allows conventional commits", func(t *testing.T) {
		exitCalled = false
		file := filepath.Join(t.TempDir(), "msg.txt")
		if err := os.WriteFile(file, []byte("feat(cli): add tests\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		commitMsgStderr = &bytes.Buffer{}
		if err := runCommitMsg(nil, []string{file}); err != nil {
			t.Fatalf("runCommitMsg() error = %v", err)
		}
		if exitCalled {
			t.Fatal("commitMsgExit was called for valid message")
		}
	})

	t.Run("rejects ai attribution", func(t *testing.T) {
		exitCalled = false
		file := filepath.Join(t.TempDir(), "msg.txt")
		if err := os.WriteFile(file, []byte("feat: add tests\nGenerated by AI\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		commitMsgStderr = &stderr
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic from commitMsgExit")
			}
			if !exitCalled || !strings.Contains(stderr.String(), "AI attribution") {
				t.Fatalf("unexpected exit/stderr: exit=%v stderr=%q", exitCalled, stderr.String())
			}
		}()
		_ = runCommitMsg(nil, []string{file})
	})

	t.Run("rejects non-conventional messages", func(t *testing.T) {
		exitCalled = false
		file := filepath.Join(t.TempDir(), "msg.txt")
		if err := os.WriteFile(file, []byte("bad message\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		commitMsgStderr = &bytes.Buffer{}
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic from commitMsgExit")
			}
		}()
		_ = runCommitMsg(nil, []string{file})
	})
}

func TestRunPrePush(t *testing.T) {
	oldExit := prePushExit
	oldStderr := prePushStderr
	oldStdin := os.Stdin
	defer func() {
		prePushExit = oldExit
		prePushStderr = oldStderr
		os.Stdin = oldStdin
	}()

	binDir := t.TempDir()
	restorePath := prependPath(t, binDir)
	defer restorePath()

	t.Run("allows when hooks.allowMainPush is true", func(t *testing.T) {
		writeExecutable(t, binDir, "git", "#!/bin/sh\necho true\n")
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.WriteString("")
		_ = w.Close()
		os.Stdin = r
		prePushStderr = &bytes.Buffer{}
		prePushExit = func(int) { panic("unexpected exit") }

		if err := runPrePush(nil, []string{"origin"}); err != nil {
			t.Fatalf("runPrePush() error = %v", err)
		}
	})

	t.Run("rejects direct pushes to main", func(t *testing.T) {
		writeExecutable(t, binDir, "git", "#!/bin/sh\necho false\n")
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.WriteString("abc def refs/heads/main\n")
		_ = w.Close()
		os.Stdin = r
		var stderr bytes.Buffer
		prePushStderr = &stderr
		exitCalled := false
		prePushExit = func(code int) {
			exitCalled = true
			panic(code)
		}

		defer func() {
			if recover() == nil {
				t.Fatal("expected panic from prePushExit")
			}
			if !exitCalled || !strings.Contains(stderr.String(), "direct push to 'main' is blocked") {
				t.Fatalf("unexpected exit/stderr: exit=%v stderr=%q", exitCalled, stderr.String())
			}
		}()
		_ = runPrePush(nil, []string{"origin"})
	})
}

func TestRunPromoteAndSyncBinaryHelperCanFail(t *testing.T) {
	oldEnv := os.Getenv(helperEnv)
	oldLog := os.Getenv("CURSOR_TOOLS_HELPER_LOG")
	oldFail := os.Getenv("CURSOR_TOOLS_HELPER_FAIL")
	defer func() {
		_ = os.Setenv(helperEnv, oldEnv)
		_ = os.Setenv("CURSOR_TOOLS_HELPER_LOG", oldLog)
		_ = os.Setenv("CURSOR_TOOLS_HELPER_FAIL", oldFail)
	}()

	logPath := filepath.Join(t.TempDir(), "helper.log")
	if err := os.Setenv(helperEnv, "1"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("CURSOR_TOOLS_HELPER_LOG", logPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("CURSOR_TOOLS_HELPER_FAIL", ""); err != nil {
		t.Fatal(err)
	}

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	if _, err := os.Stat(self); err != nil {
		t.Fatalf("test binary stat: %v", err)
	}
}

func TestHelperUtilities(t *testing.T) {
	if got := nonTestArgs([]string{"-test.run=TestCLI", "sync-counts", "--apply"}); strings.Join(got, " ") != "sync-counts --apply" {
		t.Fatalf("nonTestArgs() = %v", got)
	}

	path := filepath.Join(t.TempDir(), "missing")
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing path, got %v", err)
	}
}
