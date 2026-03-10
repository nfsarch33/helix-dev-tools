package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/clilog"
	"github.com/nfsarch33/cursor-tools/internal/config"
	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

var dailyRefreshDryRun bool
var dailyRefreshLoadMetrics = metrics.LoadAll
var dailyRefreshVerify = runDailyRefreshVerification

var dailyRefreshCmd = &cobra.Command{
	Use:   "daily-refresh",
	Short: "Daily Pepper automation: MCP index, repo memories, git sync, skills sync",
	Long:  "Runs the four-step daily refresh pipeline previously handled by daily_refresh.sh.",
	RunE:  runDailyRefresh,
}

func init() {
	dailyRefreshCmd.Flags().BoolVar(&dailyRefreshDryRun, "dry-run", false, "Show what would happen without making changes")
}

const dailyLogPrefix = "[pepper-daily]"

type dailyRefresher struct {
	paths  config.Paths
	dryRun bool
	out    *clilog.Prefixed
}

func (d *dailyRefresher) setSSHCommand() {
	keyPath := d.paths.SSHKeyPath()
	if _, err := os.Stat(keyPath); err == nil {
		os.Setenv("GIT_SSH_COMMAND", fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", keyPath))
	}
}

func (d *dailyRefresher) stepMCPIndex() {
	d.out.Info("step 1/8: MCP index")
	mcpJSON := filepath.Join(d.paths.Home, ".cursor", "mcp.json")
	outPath := filepath.Join(d.paths.GlobalMemoriesDir(), "mcp-index-and-selection-sop.md")

	if _, err := os.Stat(mcpJSON); err != nil {
		d.out.Warn("~/.cursor/mcp.json not found")
		return
	}
	if d.dryRun {
		d.out.Info("[dry-run] would refresh MCP index: %s", outPath)
		return
	}
	updated, err := refreshMCPIndex(mcpJSON, outPath)
	if err != nil {
		d.out.Error("MCP index refresh failed: %s", err.Error())
		return
	}
	if updated {
		d.out.Info("MCP index: updated")
	} else {
		d.out.Info("MCP index: no changes")
	}
}

func (d *dailyRefresher) stepRepoMemories() {
	d.out.Info("step 2/8: repo memories")
	listFile := filepath.Join(d.paths.ToolsDir(), "repos-to-sync.txt")
	f, err := os.Open(listFile)
	if err != nil {
		return
	}
	defer f.Close()

	dstDir := d.paths.GlobalMemoriesDir()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !isDir(line) {
			continue
		}
		if d.dryRun {
			d.out.Info("[dry-run] would sync repo: %s", line)
			continue
		}
		counts := syncRepoMemories(line, dstDir)
		d.out.Info("repo %s: added=%d updated=%d skipped=%d",
			filepath.Base(line), counts.added, counts.updated, counts.skipped)
	}
}

type syncCounts struct {
	added   int
	updated int
	skipped int
}

// syncRepoMemories copies *.md files from srcDir to dstDir, backing up changed files.
func syncRepoMemories(srcDir, dstDir string) syncCounts {
	var c syncCounts
	_ = os.MkdirAll(dstDir, 0o755)

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return c
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		srcPath := filepath.Join(srcDir, e.Name())
		dstPath := filepath.Join(dstDir, e.Name())

		srcData, err := os.ReadFile(srcPath)
		if err != nil {
			continue
		}

		dstData, err := os.ReadFile(dstPath)
		if err == nil {
			if bytes.Equal(srcData, dstData) {
				c.skipped++
				continue
			}
			ts := time.Now().Format("20060102-150405")
			backupPath := filepath.Join(dstDir, e.Name()+".bak."+ts)
			_ = os.WriteFile(backupPath, dstData, 0o644) // #nosec G703 -- path from trusted config
			c.updated++
		} else {
			c.added++
		}

		_ = os.WriteFile(dstPath, srcData, 0o644) // #nosec G703 -- path from trusted config
	}

	return c
}

func (d *dailyRefresher) stepMetricsReport() {
	d.out.Info("step 4/8: metrics report")
	metricsPath := d.paths.MetricsFile()
	outPath := filepath.Join(d.paths.GlobalMemoriesDir(), "system-performance.md")

	events, err := dailyRefreshLoadMetrics(metricsPath)
	if err != nil {
		d.out.Warn("metrics load failed: %s", err.Error())
		return
	}

	if len(events) == 0 {
		d.out.Info("metrics: no data yet")
		return
	}

	if d.dryRun {
		d.out.Info("[dry-run] would generate metrics report: %s", outPath)
		return
	}

	since := time.Now().UTC().Add(-7 * 24 * time.Hour)
	summary := metrics.Summarise(events, since)
	md := summary.Markdown()

	if err := os.WriteFile(outPath, []byte(md), 0o644); err != nil {
		d.out.Error("metrics report write failed: %s", err.Error())
		return
	}
	d.out.Info("metrics: report updated (%d events)", summary.TotalEvents)
}

func (d *dailyRefresher) stepGitSync() {
	d.out.Info("step 7/8: git unified-memory")
	repoPath := d.paths.GlobalKB
	if !isDir(repoPath + "/.git") {
		d.out.Warn("unified-memory repo not found")
		return
	}
	d.setSSHCommand()

	rulesSrc := filepath.Join(repoPath, "cursor-config", "rules")
	if isDir(rulesSrc) {
		d.syncWorkspaceRules(rulesSrc)
	}

	if d.dryRun {
		d.out.Info("[dry-run] would git sync: %s", repoPath)
		return
	}

	if err := gitCmd(repoPath, "pull", "--rebase", "--autostash", "origin", "main"); err != nil {
		d.out.Warn("unified-memory: pull failed (offline?)")
	}

	if hasChanges(repoPath) {
		gitCmd(repoPath, "add", "-A")
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		commitMsg := fmt.Sprintf("auto: daily sync %s [%s]", time.Now().Format("2006-01-02"), hostname)
		if err := gitCmd(repoPath, "commit", "-m", commitMsg); err != nil {
			d.out.Warn("unified-memory: commit failed")
		} else {
			d.out.Info("unified-memory: committed and pushed")
		}
		if err := gitCmd(repoPath, "push", "origin", "main"); err != nil {
			d.out.Warn("unified-memory: push failed (offline?)")
		}
	} else {
		d.out.Info("unified-memory: clean")
	}
}

func (d *dailyRefresher) syncWorkspaceRules(rulesSrc string) {
	type ruleTarget struct {
		targetFile string
		srcName    string
	}

	var targets []ruleTarget

	zendesk := filepath.Join(d.paths.Home, "Code", "zendesk", ".cursor", "rules")
	if isDir(filepath.Dir(zendesk)) {
		targets = append(targets, ruleTarget{
			targetFile: zendesk,
			srcName:    "zendesk-workspace.rules",
		})
	}

	if envPath := os.Getenv("WORKSPACE_RULES_PATH"); envPath != "" {
		parts := strings.SplitN(envPath, ":", 2)
		if len(parts) == 2 {
			targets = append(targets, ruleTarget{targetFile: parts[0], srcName: parts[1]})
		}
	}

	for _, t := range targets {
		srcFile := filepath.Join(rulesSrc, t.srcName)
		if _, err := os.Stat(srcFile); err != nil {
			continue
		}
		if !isDir(filepath.Dir(t.targetFile)) {
			continue
		}
		d.syncFile("rules/"+t.srcName, srcFile, t.targetFile)
	}
}

func (d *dailyRefresher) syncFile(label, src, dst string) {
	srcData, err := os.ReadFile(src)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(dst), 0o755)

	dstData, err := os.ReadFile(dst)
	if err == nil && bytes.Equal(srcData, dstData) {
		d.out.Info("%s: up to date", label)
		return
	}

	if d.dryRun {
		d.out.Info("[dry-run] would sync %s", label)
		return
	}
	if err := os.WriteFile(dst, srcData, 0o644); err != nil { // #nosec G703 -- path from trusted config
		d.out.Warn("%s: write failed", label)
	} else {
		d.out.Info("%s: synced", label)
	}
}

func (d *dailyRefresher) stepSkillsSync() {
	d.out.Info("step 8/8: skills")
	skillsSrc := filepath.Join(d.paths.Memo, "skills")
	skillsDst := d.paths.SkillsDir

	if !isDir(skillsSrc) {
		return
	}

	entries, err := os.ReadDir(skillsSrc)
	if err != nil {
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		srcSkill := filepath.Join(skillsSrc, name, "SKILL.md")
		if _, err := os.Stat(srcSkill); err != nil {
			continue
		}

		dstDir := filepath.Join(skillsDst, name)
		dstSkill := filepath.Join(dstDir, "SKILL.md")

		srcData, err := os.ReadFile(srcSkill)
		if err != nil {
			continue
		}

		dstData, _ := os.ReadFile(dstSkill)
		if bytes.Equal(srcData, dstData) {
			d.out.Info("skill %s: up to date", name)
			continue
		}

		if d.dryRun {
			d.out.Info("[dry-run] would sync skill: %s", name)
			continue
		}

		_ = os.MkdirAll(dstDir, 0o755)

		mdFiles, _ := filepath.Glob(filepath.Join(skillsSrc, name, "*.md"))
		for _, md := range mdFiles {
			data, err := os.ReadFile(md)
			if err != nil {
				continue
			}
			dst := filepath.Join(dstDir, filepath.Base(md))
			_ = os.WriteFile(dst, data, 0o644) // #nosec G703 -- path from trusted config
		}
		d.out.Info("skill %s: synced", name)
	}
}

func (d *dailyRefresher) stepSessionHandoff() {
	d.out.Info("step 5/8: session handoff")
	if err := generateSessionHandoff(d.paths, d.out, false, d.dryRun); err != nil {
		d.out.Error("session handoff failed: %s", err.Error())
	}
}

func (d *dailyRefresher) stepPrePullHandoffReview() {
	d.out.Info("step 6/8: pre-pull handoff review")
	if d.dryRun {
		d.out.Info("[dry-run] would fetch origin and check for remote handoffs")
		return
	}
	found, err := previewRemoteHandoffs(d.paths, d.out)
	if err != nil {
		d.out.Warn("handoff review error: %s", err.Error())
		return
	}
	if !found {
		d.out.Info("no new handoffs from other machines")
	}
}

func runDailyRefresh(_ *cobra.Command, _ []string) error {
	p := config.DefaultPaths()
	out := clilog.NewPrefixed(dailyLogPrefix)
	d := &dailyRefresher{
		paths:  p,
		dryRun: dailyRefreshDryRun,
		out:    out,
	}

	if d.dryRun {
		d.out.Info("DRY-RUN MODE: no changes will be made")
	}

	d.stepMCPIndex()
	d.stepRepoMemories()
	if err := dailyRefreshVerify(d); err != nil {
		d.out.Error("verification failed: %s", err.Error())
	}
	d.stepMetricsReport()
	d.stepSessionHandoff()
	d.stepPrePullHandoffReview()
	d.stepGitSync()
	d.stepSkillsSync()

	if out.Errors() > 0 {
		d.out.Info("done with %d error(s)", out.Errors())
		return fmt.Errorf("%d error(s) during daily refresh", out.Errors())
	}
	d.out.Info("done")
	return nil
}

func runDailyRefreshVerification(d *dailyRefresher) error {
	d.out.Info("step 3/8: verification")

	if d.dryRun {
		d.out.Info("[dry-run] would run: doctor resume, doctor mcp, health-check, selftest, IronClaw smoke")
		return nil
	}

	commands := [][]string{
		{"doctor", "resume"},
		{"doctor", "mcp"},
		{"health-check"},
		{"selftest"},
	}
	for _, args := range commands {
		if err := dailyRefreshRunSelfCommand(d.paths, args...); err != nil {
			return err
		}
	}

	smokeScript := os.Getenv("IRONCLAW_MCP_SMOKE_SCRIPT")
	if smokeScript == "" {
		smokeScript = "/mnt/f/onedrive/repo/biz-stack/ironclaw-mcp/scripts/smoke-test.sh"
	}
	if _, err := os.Stat(smokeScript); err == nil {
		if err := dailyRefreshRunSmoke(smokeScript); err != nil {
			return err
		}
	} else {
		d.out.Warn("IronClaw smoke script not found: %s", smokeScript)
	}
	return nil
}

func dailyRefreshRunSelfCommand(paths config.Paths, args ...string) error {
	binPath := filepath.Join(paths.BinDir, "cursor-tools")
	if _, err := os.Stat(binPath); err != nil {
		exe, exeErr := os.Executable()
		if exeErr != nil {
			return nil
		}
		base := filepath.Base(exe)
		if base != "cursor-tools" && !strings.HasPrefix(base, "cursor-tools") {
			return nil
		}
		binPath = exe
	}
	cmd := exec.Command(binPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func dailyRefreshRunSmoke(scriptPath string) error {
	cmd := exec.Command(scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(),
		"SMOKE_REQUIRE_ROUTER=true",
		"SMOKE_STATEFUL_TOOL=ironclaw_chat",
	)
	return cmd.Run()
}
