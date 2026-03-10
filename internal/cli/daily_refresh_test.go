package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

var _ = Describe("syncRepoMemories", func() {
	var srcDir, dstDir string

	BeforeEach(func() {
		srcDir = GinkgoT().TempDir()
		dstDir = GinkgoT().TempDir()
	})

	It("copies new .md files from src to dst", func() {
		Expect(os.WriteFile(filepath.Join(srcDir, "readme.md"), []byte("# Hello"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(srcDir, "notes.md"), []byte("# Notes"), 0o644)).To(Succeed())

		c := syncRepoMemories(srcDir, dstDir)
		Expect(c.added).To(Equal(2))
		Expect(c.updated).To(Equal(0))
		Expect(c.skipped).To(Equal(0))

		data, err := os.ReadFile(filepath.Join(dstDir, "readme.md"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("# Hello"))
	})

	It("skips unchanged files", func() {
		content := []byte("same content")
		Expect(os.WriteFile(filepath.Join(srcDir, "same.md"), content, 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dstDir, "same.md"), content, 0o644)).To(Succeed())

		c := syncRepoMemories(srcDir, dstDir)
		Expect(c.skipped).To(Equal(1))
		Expect(c.added).To(Equal(0))
		Expect(c.updated).To(Equal(0))
	})

	It("updates changed files and creates backups", func() {
		Expect(os.WriteFile(filepath.Join(srcDir, "doc.md"), []byte("new content"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dstDir, "doc.md"), []byte("old content"), 0o644)).To(Succeed())

		c := syncRepoMemories(srcDir, dstDir)
		Expect(c.updated).To(Equal(1))

		data, err := os.ReadFile(filepath.Join(dstDir, "doc.md"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("new content"))

		entries, err := os.ReadDir(dstDir)
		Expect(err).NotTo(HaveOccurred())
		backupFound := false
		for _, e := range entries {
			if len(e.Name()) > len("doc.md.bak.") {
				backupFound = true
			}
		}
		Expect(backupFound).To(BeTrue(), "backup file should exist")
	})

	It("ignores non-.md files", func() {
		Expect(os.WriteFile(filepath.Join(srcDir, "script.sh"), []byte("#!/bin/bash"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(srcDir, "data.json"), []byte("{}"), 0o644)).To(Succeed())

		c := syncRepoMemories(srcDir, dstDir)
		Expect(c.added).To(Equal(0))
	})

	It("ignores directories", func() {
		Expect(os.MkdirAll(filepath.Join(srcDir, "subdir"), 0o755)).To(Succeed())
		c := syncRepoMemories(srcDir, dstDir)
		Expect(c.added).To(Equal(0))
	})

	It("handles empty source directory", func() {
		c := syncRepoMemories(srcDir, dstDir)
		Expect(c.added).To(Equal(0))
		Expect(c.updated).To(Equal(0))
		Expect(c.skipped).To(Equal(0))
	})

	It("handles missing source directory", func() {
		c := syncRepoMemories("/nonexistent/path", dstDir)
		Expect(c.added).To(Equal(0))
	})
})

var _ = Describe("isDir", func() {
	It("returns true for existing directories", func() {
		dir := GinkgoT().TempDir()
		Expect(isDir(dir)).To(BeTrue())
	})

	It("returns false for files", func() {
		dir := GinkgoT().TempDir()
		f := filepath.Join(dir, "file.txt")
		Expect(os.WriteFile(f, []byte("data"), 0o644)).To(Succeed())
		Expect(isDir(f)).To(BeFalse())
	})

	It("returns false for nonexistent paths", func() {
		Expect(isDir("/nonexistent/path/abc123")).To(BeFalse())
	})
})

var _ = Describe("daily refresher helpers", func() {
	var tmpDir string
	var oldHome string

	BeforeEach(func() {
		tmpDir = GinkgoT().TempDir()
		oldHome = os.Getenv("HOME")
		Expect(os.Setenv("HOME", tmpDir)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.Setenv("HOME", oldHome)).To(Succeed())
	})

	It("sets GIT_SSH_COMMAND when a key exists", func() {
		sshDir := filepath.Join(tmpDir, ".ssh")
		Expect(os.MkdirAll(sshDir, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(sshDir, "agtc"), []byte("key"), 0o600)).To(Succeed())

		d := &dailyRefresher{
			paths: config.DefaultPaths(),
			out:   clilog.NewPrefixed("[test]"),
		}
		d.setSSHCommand()
		Expect(os.Getenv("GIT_SSH_COMMAND")).To(ContainSubstring(filepath.Join(sshDir, "agtc")))
	})

	It("writes a metrics report when events exist", func() {
		p := config.DefaultPaths()
		Expect(os.MkdirAll(p.HooksDir, 0o755)).To(Succeed())
		Expect(os.MkdirAll(p.GlobalMemoriesDir(), 0o755)).To(Succeed())
		Expect(metrics.Record(p.MetricsFile(), metrics.Event{
			Timestamp: nowUTC().Add(-1 * time.Hour),
			Hook:      "guard-shell",
			Action:    "allow",
			Category:  "shell",
			LatencyMs: 1,
		})).To(Succeed())

		d := &dailyRefresher{
			paths: p,
			out:   clilog.NewPrefixed("[test]"),
		}
		d.stepMetricsReport()

		report := filepath.Join(p.GlobalMemoriesDir(), "system-performance.md")
		Expect(report).To(BeAnExistingFile())
		data, err := os.ReadFile(report)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("System Performance Report"))
	})

	It("syncs a file in dry-run mode without writing", func() {
		srcDir := filepath.Join(tmpDir, "src")
		dstDir := filepath.Join(tmpDir, "dst")
		Expect(os.MkdirAll(srcDir, 0o755)).To(Succeed())
		Expect(os.MkdirAll(dstDir, 0o755)).To(Succeed())
		src := filepath.Join(srcDir, "rule.md")
		dst := filepath.Join(dstDir, "rule.md")
		Expect(os.WriteFile(src, []byte("new"), 0o644)).To(Succeed())
		Expect(os.WriteFile(dst, []byte("old"), 0o644)).To(Succeed())

		d := &dailyRefresher{
			paths:  config.DefaultPaths(),
			dryRun: true,
			out:    clilog.NewPrefixed("[test]"),
		}
		d.syncFile("rule", src, dst)

		data, err := os.ReadFile(dst)
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(string(data))).To(Equal("old"))
	})
})

func nowUTC() time.Time {
	return time.Now().UTC()
}

func TestDailyRefreshStepsAndRunner(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldGlobalKB := os.Getenv("GLOBAL_KB")
	oldMemo := os.Getenv("MEMO")
	oldWorkspaceRules := os.Getenv("WORKSPACE_RULES_PATH")
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
		_ = os.Setenv("WORKSPACE_RULES_PATH", oldWorkspaceRules)
	}()

	binDir := t.TempDir()
	gitLog := filepath.Join(binDir, "git.log")
	restorePath := prependPath(t, binDir)
	defer restorePath()
	writeExecutable(t, binDir, "git", "#!/bin/sh\necho \"$@\" >> \""+gitLog+"\"\ncase \"$*\" in\n  *\"status --porcelain\"*) echo \" M drift.go\" ;;\n  *) exit 0 ;;\nesac\n")

	p := config.DefaultPaths()
	for _, dir := range []string{
		filepath.Join(home, ".cursor"),
		p.GlobalMemoriesDir(),
		p.ToolsDir(),
		p.HooksDir,
		filepath.Join(p.GlobalKB, ".git"),
		filepath.Join(p.GlobalKB, "cursor-config", "rules"),
		filepath.Join(p.Memo, "skills", "demo-skill"),
		filepath.Join(home, "repo-a"),
		filepath.Join(home, "workspace-rules"),
		p.SkillsDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, ".cursor", "mcp.json"), []byte(`{"mcpServers":{"perplexity":{"command":"npx","args":["-y","@perplexity-ai/mcp-server"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.ToolsDir(), "repos-to-sync.txt"), []byte(filepath.Join(home, "repo-a")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "repo-a", "notes.md"), []byte("repo memory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := metrics.Record(p.MetricsFile(), metrics.Event{
		Timestamp: nowUTC().Add(-1 * time.Hour),
		Hook:      "guard-shell",
		Action:    "allow",
		Category:  "shell",
		LatencyMs: 5,
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.GlobalKB, "cursor-config", "rules", "zendesk-workspace.rules"), []byte("rule"), 0o644); err != nil {
		t.Fatal(err)
	}
	ruleTarget := filepath.Join(home, "workspace-rules", "target.rules")
	if err := os.Setenv("WORKSPACE_RULES_PATH", ruleTarget+":zendesk-workspace.rules"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Memo, "skills", "demo-skill", "SKILL.md"), []byte("# Demo"), 0o644); err != nil {
		t.Fatal(err)
	}

	d := &dailyRefresher{
		paths: p,
		out:   clilog.NewPrefixed("[test]"),
	}
	d.stepMCPIndex()
	d.stepRepoMemories()
	d.stepMetricsReport()
	d.stepGitSync()
	d.stepSkillsSync()

	for _, wantFile := range []string{
		filepath.Join(p.GlobalMemoriesDir(), "mcp-index-and-selection-sop.md"),
		filepath.Join(p.GlobalMemoriesDir(), "notes.md"),
		filepath.Join(p.GlobalMemoriesDir(), "system-performance.md"),
		ruleTarget,
		filepath.Join(p.SkillsDir, "demo-skill", "SKILL.md"),
	} {
		if _, err := os.Stat(wantFile); err != nil {
			t.Fatalf("expected file %s: %v", wantFile, err)
		}
	}

	gitData, err := os.ReadFile(gitLog)
	if err != nil {
		t.Fatal(err)
	}
	gitText := string(gitData)
	for _, want := range []string{"-C " + p.GlobalKB + " pull --rebase --autostash origin main", "-C " + p.GlobalKB + " add -A", "-C " + p.GlobalKB + " commit -m auto: daily sync", "-C " + p.GlobalKB + " push origin main"} {
		if !strings.Contains(gitText, want) {
			t.Fatalf("git log missing %q in %q", want, gitText)
		}
	}

	oldDryRun := dailyRefreshDryRun
	defer func() { dailyRefreshDryRun = oldDryRun }()
	dailyRefreshDryRun = true
	if err := runDailyRefresh(nil, nil); err != nil {
		t.Fatalf("runDailyRefresh() error = %v", err)
	}
}

func TestDailyRefreshErrorAndMissingBranches(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldGlobalKB := os.Getenv("GLOBAL_KB")
	oldMemo := os.Getenv("MEMO")
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
	for _, dir := range []string{filepath.Join(home, ".cursor"), p.GlobalMemoriesDir(), p.HooksDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	d := &dailyRefresher{
		paths: p,
		out:   clilog.NewPrefixed("[test]"),
	}
	d.stepMCPIndex()
	d.stepMetricsReport()
	d.stepGitSync()
	d.stepSkillsSync()

	if err := os.WriteFile(filepath.Join(home, ".cursor", "mcp.json"), []byte("{bad json}"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldDryRun := dailyRefreshDryRun
	defer func() { dailyRefreshDryRun = oldDryRun }()
	dailyRefreshDryRun = false
	if err := runDailyRefresh(nil, nil); err == nil {
		t.Fatal("runDailyRefresh() expected error when MCP config is invalid")
	}
}
