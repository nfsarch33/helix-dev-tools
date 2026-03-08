package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nfsarch33/cursor-tools/internal/config"
)

var dailyRefreshDryRun bool

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
	errors int
}

func (d *dailyRefresher) log(msg string) {
	fmt.Printf("%s %s\n", dailyLogPrefix, msg)
}

func (d *dailyRefresher) warn(msg string) {
	fmt.Fprintf(os.Stderr, "%s WARN: %s\n", dailyLogPrefix, msg)
}

func (d *dailyRefresher) fail(msg string) {
	fmt.Fprintf(os.Stderr, "%s ERROR: %s\n", dailyLogPrefix, msg)
	d.errors++
}

func (d *dailyRefresher) setSSHCommand() {
	keyPath := d.paths.SSHKeyPath()
	if _, err := os.Stat(keyPath); err == nil {
		os.Setenv("GIT_SSH_COMMAND", fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", keyPath))
	}
}

func (d *dailyRefresher) stepMCPIndex() {
	d.log("step 1/4: MCP index")
	mcpJSON := filepath.Join(d.paths.Home, ".cursor", "mcp.json")
	outPath := filepath.Join(d.paths.GlobalMemoriesDir(), "mcp-index-and-selection-sop.md")

	if _, err := os.Stat(mcpJSON); err != nil {
		d.warn("~/.cursor/mcp.json not found")
		return
	}
	if d.dryRun {
		d.log("[dry-run] would refresh MCP index: " + outPath)
		return
	}
	updated, err := refreshMCPIndex(mcpJSON, outPath)
	if err != nil {
		d.fail("MCP index refresh failed: " + err.Error())
		return
	}
	if updated {
		d.log("MCP index: updated")
	} else {
		d.log("MCP index: no changes")
	}
}

func (d *dailyRefresher) stepRepoMemories() {
	d.log("step 2/4: repo memories")
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
			d.log("[dry-run] would sync repo: " + line)
			continue
		}
		counts := syncRepoMemories(line, dstDir)
		d.log(fmt.Sprintf("repo %s: added=%d updated=%d skipped=%d",
			filepath.Base(line), counts.added, counts.updated, counts.skipped))
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
			_ = os.WriteFile(backupPath, dstData, 0o644)
			c.updated++
		} else {
			c.added++
		}

		_ = os.WriteFile(dstPath, srcData, 0o644)
	}

	return c
}

func (d *dailyRefresher) stepGitSync() {
	d.log("step 3/4: git unified-memory")
	repoPath := d.paths.GlobalKB
	if !isDir(repoPath + "/.git") {
		d.warn("unified-memory repo not found")
		return
	}
	d.setSSHCommand()

	rulesSrc := filepath.Join(repoPath, "cursor-config", "rules")
	if isDir(rulesSrc) {
		d.syncWorkspaceRules(rulesSrc)
	}

	if d.dryRun {
		d.log("[dry-run] would git sync: " + repoPath)
		return
	}

	if err := gitCmd(repoPath, "pull", "--rebase", "--autostash", "origin", "main"); err != nil {
		d.warn("unified-memory: pull failed (offline?)")
	}

	if hasChanges(repoPath) {
		gitCmd(repoPath, "add", "-A")
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		commitMsg := fmt.Sprintf("auto: daily sync %s [%s]", time.Now().Format("2006-01-02"), hostname)
		if err := gitCmd(repoPath, "commit", "-m", commitMsg); err != nil {
			d.warn("unified-memory: commit failed")
		} else {
			d.log("unified-memory: committed and pushed")
		}
		if err := gitCmd(repoPath, "push", "origin", "main"); err != nil {
			d.warn("unified-memory: push failed (offline?)")
		}
	} else {
		d.log("unified-memory: clean")
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
		d.log(label + ": up to date")
		return
	}

	if d.dryRun {
		d.log("[dry-run] would sync " + label)
		return
	}
	if err := os.WriteFile(dst, srcData, 0o644); err != nil {
		d.warn(label + ": write failed")
	} else {
		d.log(label + ": synced")
	}
}

func (d *dailyRefresher) stepSkillsSync() {
	d.log("step 4/4: skills")
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
			d.log("skill " + name + ": up to date")
			continue
		}

		if d.dryRun {
			d.log("[dry-run] would sync skill: " + name)
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
			_ = os.WriteFile(dst, data, 0o644)
		}
		d.log("skill " + name + ": synced")
	}
}

func runDailyRefresh(_ *cobra.Command, _ []string) error {
	p := config.DefaultPaths()
	d := &dailyRefresher{
		paths:  p,
		dryRun: dailyRefreshDryRun,
	}

	if d.dryRun {
		d.log("DRY-RUN MODE: no changes will be made")
	}

	d.stepMCPIndex()
	d.stepRepoMemories()
	d.stepGitSync()
	d.stepSkillsSync()

	if d.errors > 0 {
		d.log(fmt.Sprintf("done with %d error(s)", d.errors))
		return fmt.Errorf("%d error(s) during daily refresh", d.errors)
	}
	d.log("done")
	return nil
}
