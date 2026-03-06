package health

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nfsarch33/cursor-tools/internal/config"
)

// BuildAllSuites creates all 19 health check suites.
func BuildAllSuites(p config.Paths) []*Suite {
	return []*Suite{
		suiteL0Rules(p),
		suiteL1Pepper(p),
		suiteL2GlobalKB(p),
		suiteSkillsRegistry(p),
		suiteCrossFileConsistency(p),
		suiteGitSync(p),
		suiteHooksSubagentsCommandsMCP(p),
		suiteMemoryRouting(p),
		suiteCrossMachineSync(p),
		suiteProgrammaticCounts(p),
		suiteHookUnitTests(p),
		suiteLogFileIntegrity(p),
		suiteAutomationPipeline(p),
		suiteSkillvetEDR(p),
		suiteGlobalCursorConfig(p),
		suiteRaceConditionPrevention(p),
		suiteDataIntegrity(p),
		suiteGitHookIntegrity(p),
		suiteSelfImprovementPipeline(p),
	}
}

func suiteL0Rules(p config.Paths) *Suite {
	s := &Suite{Name: "L0 Rules"}
	rulesDir := p.RulesDir

	expectedRules := []string{
		"00-capabilities", "engineering-standards", "self-improvement",
		"subagents", "template.rules", "zendesk-workspace.rules",
	}
	s.AssertFileExists("Rules directory exists", rulesDir)

	entries, err := os.ReadDir(rulesDir)
	if err == nil {
		s.Assert("Rules dir has files", len(entries) >= 6, "expected >= 6 rule files")
		nameSet := make(map[string]bool)
		for _, e := range entries {
			nameSet[strings.TrimSuffix(e.Name(), ".md")] = true
			nameSet[e.Name()] = true
		}
		for _, expected := range expectedRules {
			found := nameSet[expected] || nameSet[expected+".md"]
			s.Assert("Rule: "+expected, found, "not found in rules/")
		}
	} else {
		for _, expected := range expectedRules {
			s.Fail("Rule: "+expected, "rules dir not readable")
		}
	}

	for _, rule := range expectedRules[:3] {
		path := filepath.Join(rulesDir, rule+".md")
		if _, err := os.Stat(path); err == nil {
			data, _ := os.ReadFile(path)
			s.Assert(rule+" is non-empty", len(data) > 50, "file too small")
		}
	}

	credPatterns := []string{`ATATT3x`, `glpat-`, `ghp_`, `sk-proj-`, `sk-ant-`, `AKIA`}
	entries2, _ := os.ReadDir(rulesDir)
	for _, e := range entries2 {
		data, err := os.ReadFile(filepath.Join(rulesDir, e.Name()))
		if err != nil {
			continue
		}
		content := string(data)
		leaked := false
		for _, pat := range credPatterns {
			if strings.Contains(content, pat) {
				leaked = true
				break
			}
		}
		s.Assert("No credentials in "+e.Name(), !leaked, "credential pattern found")
	}

	return s
}

func suiteL1Pepper(p config.Paths) *Suite {
	s := &Suite{Name: "L1 Pepper"}
	gmDir := p.GlobalMemoriesDir()

	s.AssertFileExists("global-memories dir exists", gmDir)

	pepperFiles := []string{
		"daily-startup-prompt.md", "skills-index.md",
		"mcp-index-and-selection-sop.md", "one-person-company-progress.md",
	}
	for _, f := range pepperFiles {
		s.AssertFileExists("Pepper: "+f, filepath.Join(gmDir, f))
	}

	s.AssertFileContains("daily prompt has memory system", filepath.Join(gmDir, "daily-startup-prompt.md"), "Memory System")
	s.AssertFileContains("skills index has count", filepath.Join(gmDir, "skills-index.md"), "unique skills")

	credPatterns2 := []string{`ATATT3x`, `glpat-`, `ghp_`, `sk-proj-`, `sk-ant-`, `AKIA`}
	for _, f := range pepperFiles {
		path := filepath.Join(gmDir, f)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		leaked := false
		for _, pat := range credPatterns2 {
			if strings.Contains(string(data), pat) {
				leaked = true
			}
		}
		s.Assert("No credentials in "+f, !leaked, "credential pattern found")
	}

	return s
}

func suiteL2GlobalKB(p config.Paths) *Suite {
	s := &Suite{Name: "L2 Global KB"}
	s.AssertFileExists("global-kb dir exists", p.GlobalKB)
	s.AssertFileExists("sop/ dir exists", p.SOPDir())
	s.AssertFileExists("cursor-config/ dir exists", p.CursorConfigDir())
	s.AssertFileExists("architecture/ dir exists", filepath.Join(p.GlobalKB, "architecture"))
	s.AssertFileExists("learnings/ dir exists", p.GlobalLearningsDir())
	s.AssertFileExists("global-memories/ dir exists", p.GlobalMemoriesDir())
	return s
}

func suiteSkillsRegistry(p config.Paths) *Suite {
	s := &Suite{Name: "Skills Registry"}

	s.AssertFileExists("skills dir exists", p.SkillsDir)
	s.AssertFileExists("agents-skills dir exists", p.AgentsSkillsDir)

	cursorCount := countDirsWithFile(p.SkillsDir, "SKILL.md", map[string]bool{"00-index": true})
	agentsCount := countDirsWithFile(p.AgentsSkillsDir, "SKILL.md", nil)
	total := cursorCount + agentsCount

	s.Assert("cursor skills >= 25", cursorCount >= 25, itoa(cursorCount))
	s.Assert("agents skills >= 3", agentsCount >= 3, itoa(agentsCount))
	s.Assert("total skills >= 30", total >= 30, itoa(total))

	indexPath := filepath.Join(p.GlobalMemoriesDir(), "skills-index.md")
	s.AssertFileExists("skills-index.md exists", indexPath)

	metaIndexPath := filepath.Join(p.SkillsDir, "00-index", "SKILL.md")
	s.AssertFileExists("00-index SKILL.md exists", metaIndexPath)

	coreSkills := []string{
		"self-improving-agent", "automation-workflows", "systematic-debugging",
		"go-clean-architecture", "go-security-review", "multi-search-engine",
		"memory-system", "session-handoff", "find-skills", "skillvet",
	}
	for _, skill := range coreSkills {
		path := filepath.Join(p.SkillsDir, skill, "SKILL.md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			path = filepath.Join(p.AgentsSkillsDir, skill, "SKILL.md")
		}
		s.AssertFileExists("Skill: "+skill, path)
	}

	return s
}

func suiteCrossFileConsistency(p config.Paths) *Suite {
	s := &Suite{Name: "Cross-File Consistency"}

	dailyPrompt := filepath.Join(p.GlobalMemoriesDir(), "daily-startup-prompt.md")
	skillsIndex := filepath.Join(p.GlobalMemoriesDir(), "skills-index.md")

	cursorCount := countDirsWithFile(p.SkillsDir, "SKILL.md", map[string]bool{"00-index": true})
	agentsCount := countDirsWithFile(p.AgentsSkillsDir, "SKILL.md", nil)
	total := cursorCount + agentsCount
	totalStr := itoa(total)

	s.AssertFileContains("daily prompt has skill count", dailyPrompt, totalStr+" unique skills")
	s.AssertFileContains("skills index has count", skillsIndex, totalStr+" unique skills")

	s.AssertFileContains("daily prompt mentions hooks", dailyPrompt, "hooks")
	s.AssertFileContains("daily prompt mentions sub-agents", dailyPrompt, "Sub-agents")
	s.AssertFileContains("daily prompt mentions slash commands", dailyPrompt, "commands")
	s.AssertFileContains("daily prompt mentions unified repo", dailyPrompt, "unified")
	s.AssertFileContains("daily prompt mentions bootstrap", dailyPrompt, "bootstrap")
	s.AssertFileContains("daily prompt mentions Context Mode", dailyPrompt, "Context Mode")
	s.AssertFileContains("skills index mentions cursor path", skillsIndex, ".cursor/skills")
	s.AssertFileContains("skills index mentions agents path", skillsIndex, ".agents/skills")

	return s
}

func suiteGitSync(p config.Paths) *Suite {
	s := &Suite{Name: "Git Sync"}
	gitDir := filepath.Join(p.GlobalKB, ".git")
	s.AssertFileExists(".git exists for global-kb", gitDir)

	remotes, _ := gitOutput(p.GlobalKB, "remote", "-v")
	s.Assert("origin remote exists", strings.Contains(remotes, "origin"), "no origin remote")
	s.Assert("remote points to cursor-global-kb", strings.Contains(remotes, "cursor-global-kb"), "wrong remote URL")

	email, _ := gitOutput(p.GlobalKB, "config", "user.email")
	s.Assert("git identity is personal", strings.Contains(email, "jaslian@gmail.com"), "expected jaslian@gmail.com, got "+email)

	allowMain, _ := gitOutput(p.GlobalKB, "config", "hooks.allowMainPush")
	s.Assert("allowMainPush is true", strings.Contains(allowMain, "true"), "expected true")

	status, _ := gitOutput(p.GlobalKB, "status", "--porcelain")
	s.Assert("repo is clean", strings.TrimSpace(status) == "", "uncommitted changes: "+status)

	hooksPath, _ := gitOutput("", "config", "--global", "core.hooksPath")
	s.Assert("core.hooksPath set", strings.TrimSpace(hooksPath) != "", "not set")

	return s
}

func suiteHooksSubagentsCommandsMCP(p config.Paths) *Suite {
	s := &Suite{Name: "Hooks, Sub-agents, Commands, MCP"}

	hookFiles := []string{"guard-shell.sh", "sanitize-read.sh", "guard-mcp.sh", "post-edit.sh", "housekeeping.sh"}
	for _, h := range hookFiles {
		s.AssertFileExists("Hook: "+h, filepath.Join(p.HooksDir, h))
	}

	s.AssertSymlink("hooks.json is symlink", filepath.Join(p.Home, ".cursor", "hooks.json"))

	agentFiles := []string{"go-architect.md", "go-tester.md", "flutter-architect.md", "flutter-implementer.md", "agent-orchestrator.md", "memory-ops.md"}
	for _, a := range agentFiles {
		s.AssertFileExists("Agent: "+a, filepath.Join(p.AgentsDir, a))
	}

	s.AssertDirMinCount("Commands >= 5", p.CommandsDir, 5, ".md")

	mcpIndex := filepath.Join(p.GlobalMemoriesDir(), "mcp-index-and-selection-sop.md")
	s.AssertFileExists("MCP index exists", mcpIndex)
	s.AssertFileContains("MCP index has servers", mcpIndex, "MCP")
	s.AssertFileNotContains("No credentials in MCP index", mcpIndex, "ATATT3x")

	return s
}

func suiteMemoryRouting(p config.Paths) *Suite {
	s := &Suite{Name: "Memory Routing"}

	s.AssertFileExists("rules dir (L0)", p.RulesDir)
	s.AssertFileExists("global-memories dir (L1)", p.GlobalMemoriesDir())
	s.AssertFileExists("sop dir (L2)", p.SOPDir())
	s.AssertFileExists("learnings dir", p.GlobalLearningsDir())

	dailyPrompt := filepath.Join(p.GlobalMemoriesDir(), "daily-startup-prompt.md")
	s.AssertFileContains("routing mentions L0", dailyPrompt, "L0")
	s.AssertFileContains("routing mentions L1", dailyPrompt, "L1")
	s.AssertFileContains("routing mentions L2", dailyPrompt, "L2")
	s.AssertFileContains("routing mentions memo", dailyPrompt, "memo")

	selfImproveSkill := filepath.Join(p.SkillsDir, "self-improving-agent", "SKILL.md")
	s.AssertFileExists("self-improving-agent skill exists", selfImproveSkill)
	s.AssertFileContains("skill has memory architecture", selfImproveSkill, "Memory")
	s.AssertFileContains("skill has promotion rules", selfImproveSkill, "Promote")

	return s
}

func suiteCrossMachineSync(p config.Paths) *Suite {
	s := &Suite{Name: "Cross-Machine Sync"}

	s.AssertFileExists("bootstrap.sh exists", filepath.Join(p.CursorConfigDir(), "bootstrap.sh"))

	bootstrapPath := filepath.Join(p.CursorConfigDir(), "bootstrap.sh")
	s.AssertFileNotContains("bootstrap has no /opt/homebrew", bootstrapPath, "/opt/homebrew")

	hooksDir := filepath.Join(p.CursorConfigDir(), "hooks")
	hookFiles, _ := os.ReadDir(hooksDir)
	for _, h := range hookFiles {
		if !strings.HasSuffix(h.Name(), ".sh") {
			continue
		}
		path := filepath.Join(hooksDir, h.Name())
		s.AssertFileNotContains("No /opt/homebrew in "+h.Name(), path, "/opt/homebrew")
	}

	s.AssertFileExists("SSH key exists", filepath.Join(p.Home, ".ssh", "agtc"))

	return s
}

func suiteProgrammaticCounts(p config.Paths) *Suite {
	s := &Suite{Name: "Programmatic Count Verification"}

	cursorCount := countDirsWithFile(p.SkillsDir, "SKILL.md", map[string]bool{"00-index": true})
	agentsCount := countDirsWithFile(p.AgentsSkillsDir, "SKILL.md", nil)
	total := cursorCount + agentsCount

	s.Assert("cursor skills > 0", cursorCount > 0, itoa(cursorCount))
	s.Assert("agents skills > 0", agentsCount > 0, itoa(agentsCount))
	s.Assert("total matches sum", total == cursorCount+agentsCount, "mismatch")

	hookCount := countFilesWithExt(p.HooksDir, ".sh")
	s.Assert("hooks = 5", hookCount == 5, itoa(hookCount))

	agentCount := countFilesWithExt(p.AgentsDir, ".md")
	s.Assert("agents = 6", agentCount == 6, itoa(agentCount))

	cmdCount := countFilesWithExt(p.CommandsDir, ".md")
	s.Assert("commands = 5", cmdCount == 5, itoa(cmdCount))

	return s
}

func suiteHookUnitTests(p config.Paths) *Suite {
	s := &Suite{Name: "Hook Unit Tests"}
	testFile := filepath.Join(p.CursorConfigDir(), "hooks", "test_hooks.py")
	s.AssertFileExists("test_hooks.py exists", testFile)
	s.AssertFileContains("has guard-shell tests", testFile, "guard-shell")
	s.AssertFileContains("has sanitize-read tests", testFile, "sanitize-read")
	s.AssertFileContains("has housekeeping tests", testFile, "housekeeping")
	return s
}

func suiteLogFileIntegrity(p config.Paths) *Suite {
	s := &Suite{Name: "Log File Integrity"}
	logNames := []string{"guard-shell", "sanitize-read", "mcp-audit", "post-edit", "housekeeping"}

	for _, name := range logNames {
		logPath := filepath.Join(p.HooksDir, name+".log")
		if _, err := os.Stat(logPath); err == nil {
			s.Pass("Log exists: " + name)
			data, _ := os.ReadFile(logPath)
			hasTimestamp := regexp.MustCompile(`\[\d{4}-\d{2}-\d{2}T`).Match(data)
			s.Assert("Timestamps in "+name+".log", hasTimestamp, "no ISO-8601 timestamps found")
		} else {
			s.Pass("Log not yet created: " + name + " (OK)")
			s.Pass("Timestamp check skipped: " + name)
		}
	}

	hookFiles := []string{"guard-shell.sh", "sanitize-read.sh", "guard-mcp.sh", "post-edit.sh", "housekeeping.sh"}
	for _, h := range hookFiles {
		path := filepath.Join(p.CursorConfigDir(), "hooks", h)
		s.AssertFileContains(h+" writes to log", path, ".log")
	}

	housekeepingPath := filepath.Join(p.CursorConfigDir(), "hooks", "housekeeping.sh")
	s.AssertFileContains("housekeeping rotates logs", housekeepingPath, "rotate")
	s.AssertFileContains("housekeeping has max_bytes", housekeepingPath, "max_bytes")
	s.AssertFileContains("housekeeping rotates mcp-audit", housekeepingPath, "mcp-audit")

	return s
}

func suiteAutomationPipeline(p config.Paths) *Suite {
	s := &Suite{Name: "Automation Pipeline"}

	hooksJSON := filepath.Join(p.CursorConfigDir(), "hooks.json")
	s.AssertFileExists("hooks.json exists", hooksJSON)
	s.AssertFileContains("has beforeShellExecution", hooksJSON, "beforeShellExecution")
	s.AssertFileContains("has beforeReadFile", hooksJSON, "beforeReadFile")
	s.AssertFileContains("has beforeMCPExecution", hooksJSON, "beforeMCPExecution")
	s.AssertFileContains("has afterFileEdit", hooksJSON, "afterFileEdit")
	s.AssertFileContains("has stop", hooksJSON, "stop")

	postEditPath := filepath.Join(p.CursorConfigDir(), "hooks", "post-edit.sh")
	s.AssertFileContains("post-edit calls sync-counts", postEditPath, "sync-counts")
	s.AssertFileContains("post-edit formats go", postEditPath, "gofmt")
	s.AssertFileContains("post-edit formats dart", postEditPath, "dart")
	s.AssertFileContains("post-edit formats python", postEditPath, "ruff")

	housekeepingPath := filepath.Join(p.CursorConfigDir(), "hooks", "housekeeping.sh")
	s.AssertFileContains("housekeeping does git sync", housekeepingPath, "sync_repo")

	return s
}

func suiteSkillvetEDR(p config.Paths) *Suite {
	s := &Suite{Name: "Skillvet EDR-Safety"}

	skillvetDir := filepath.Join(p.SkillsDir, "skillvet")
	s.AssertFileExists("skillvet skill exists", filepath.Join(skillvetDir, "SKILL.md"))

	findSkillsDir := filepath.Join(p.SkillsDir, "find-skills")
	s.AssertFileExists("find-skills skill exists", filepath.Join(findSkillsDir, "SKILL.md"))

	entries, _ := os.ReadDir(p.SkillsDir)
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || e.Name() == "00-index" {
			continue
		}
		skillPath := filepath.Join(p.SkillsDir, e.Name(), "SKILL.md")
		if _, err := os.Stat(skillPath); err == nil {
			data, _ := os.ReadFile(skillPath)
			hasFrontmatter := strings.Contains(string(data), "---")
			s.Assert(e.Name()+" has frontmatter", hasFrontmatter, "missing YAML frontmatter")
		}
	}

	return s
}

func suiteGlobalCursorConfig(p config.Paths) *Suite {
	s := &Suite{Name: "Global Cursor Config"}

	s.AssertSymlink("skills is symlink", p.SkillsDir)
	s.AssertSymlink("rules is symlink", p.RulesDir)
	s.AssertSymlink("commands is symlink", p.CommandsDir)
	s.AssertSymlink("agents is symlink", p.AgentsDir)
	s.AssertSymlink("agents-skills is symlink", p.AgentsSkillsDir)

	s.AssertSymlink("hooks.json is symlink", filepath.Join(p.Home, ".cursor", "hooks.json"))

	hookFiles := []string{"guard-shell.sh", "sanitize-read.sh", "guard-mcp.sh", "post-edit.sh", "housekeeping.sh"}
	for _, h := range hookFiles {
		s.AssertSymlink("hook "+h+" is symlink", filepath.Join(p.HooksDir, h))
	}

	binFiles := []string{"cursor-safe", "sync-counts.py", "system-health-check.py", "promote-learnings.py"}
	for _, b := range binFiles {
		s.AssertSymlink("bin/"+b+" is symlink", filepath.Join(p.BinDir, b))
	}

	bootstrapPath := filepath.Join(p.CursorConfigDir(), "bootstrap.sh")
	s.AssertFileExists("bootstrap.sh exists", bootstrapPath)
	s.AssertFileContains("bootstrap has skills", bootstrapPath, "skills")
	s.AssertFileContains("bootstrap has rules", bootstrapPath, "rules")
	s.AssertFileContains("bootstrap has hooks", bootstrapPath, "hooks")
	s.AssertFileContains("bootstrap has memo", bootstrapPath, "memo")

	return s
}

func suiteRaceConditionPrevention(p config.Paths) *Suite {
	s := &Suite{Name: "Race Condition Prevention"}

	housekeepingPath := filepath.Join(p.CursorConfigDir(), "hooks", "housekeeping.sh")
	s.AssertFileContains("housekeeping has lockdir", housekeepingPath, "LOCKDIR")
	s.AssertFileContains("housekeeping has mkdir lock", housekeepingPath, "mkdir")
	s.AssertFileContains("housekeeping has pid file", housekeepingPath, "pid")
	s.AssertFileContains("housekeeping has stale detection", housekeepingPath, "STALE_SECONDS")
	s.AssertFileContains("housekeeping has lock_age", housekeepingPath, "lock_age")
	s.AssertFileContains("housekeeping has release_lock", housekeepingPath, "release_lock")
	s.AssertFileContains("housekeeping has acquire_lock", housekeepingPath, "acquire_lock")
	s.AssertFileContains("housekeeping has trap EXIT", housekeepingPath, "trap")

	syncCountsPath := filepath.Join(p.CursorConfigDir(), "bin", "sync-counts.py")
	s.AssertFileContains("sync-counts has fcntl", syncCountsPath, "fcntl")
	s.AssertFileContains("sync-counts has LOCK_EX", syncCountsPath, "LOCK_EX")

	promotePath := filepath.Join(p.CursorConfigDir(), "bin", "promote-learnings.py")
	s.AssertFileContains("promote has fcntl", promotePath, "fcntl")
	s.AssertFileContains("promote has locked_write", promotePath, "locked_write")

	lockDir := filepath.Join(p.HooksDir, ".housekeeping.lock")
	info, err := os.Stat(lockDir)
	s.Assert("No stale housekeeping lock", err != nil || !info.IsDir(), "stale lock found")

	housekeepingData, _ := os.ReadFile(housekeepingPath)
	content := string(housekeepingData)
	s.Assert("housekeeping has stat -f (macOS)", strings.Contains(content, "stat -f"), "missing macOS stat")
	s.Assert("housekeeping has stat -c (Linux)", strings.Contains(content, "stat -c"), "missing Linux stat")

	s.Assert("No /opt/homebrew in housekeeping", !strings.Contains(content, "/opt/homebrew"), "hardcoded macOS path")

	guardShellPath := filepath.Join(p.CursorConfigDir(), "hooks", "guard-shell.sh")
	guardShellData, _ := os.ReadFile(guardShellPath)
	s.Assert("guard-shell silences subprocess stderr", strings.Contains(string(guardShellData), "2>/dev/null"), "missing stderr suppression")

	s.AssertFileContains("housekeeping silences git stdout", housekeepingPath, ">/dev/null")

	prePushPath := filepath.Join(p.CursorConfigDir(), "git-hooks", "pre-push")
	s.AssertFileContains("pre-push has allowMainPush", prePushPath, "allowMainPush")

	commitMsgPath := filepath.Join(p.CursorConfigDir(), "git-hooks", "commit-msg")
	s.AssertFileContains("commit-msg has conventional format", commitMsgPath, "conventional")

	return s
}

func suiteDataIntegrity(p config.Paths) *Suite {
	s := &Suite{Name: "Data Integrity"}

	memoLink := filepath.Join(p.Home, "memo")
	info, err := os.Lstat(memoLink)
	s.Assert("~/memo is symlink", err == nil && info.Mode()&os.ModeSymlink != 0, "not a symlink")

	target, err := os.Readlink(memoLink)
	s.Assert("~/memo -> global-kb", err == nil && strings.Contains(target, "global-kb"), "wrong target: "+target)

	remotes, _ := gitOutput(p.GlobalKB, "remote", "-v")
	remoteCount := 0
	for _, line := range strings.Split(remotes, "\n") {
		if strings.TrimSpace(line) != "" {
			remoteCount++
		}
	}
	s.Assert("Single git remote (2 lines: fetch+push)", remoteCount == 2, itoa(remoteCount)+" lines")

	s.Assert("Remote is cursor-global-kb", strings.Contains(remotes, "cursor-global-kb"), "wrong remote")

	s.AssertFileExists("global learnings PATTERNS.md", filepath.Join(p.GlobalLearningsDir(), "PATTERNS.md"))
	s.AssertFileExists("global learnings episodes/", filepath.Join(p.GlobalLearningsDir(), "episodes"))

	return s
}

func suiteGitHookIntegrity(p config.Paths) *Suite {
	s := &Suite{Name: "Git Hook Integrity"}

	gitHooksDir := filepath.Join(p.CursorConfigDir(), "git-hooks")
	s.AssertFileExists("git-hooks dir exists", gitHooksDir)

	commitMsgPath := filepath.Join(gitHooksDir, "commit-msg")
	s.AssertFileExists("commit-msg hook exists", commitMsgPath)
	s.AssertFileContains("commit-msg checks AI attribution", commitMsgPath, "ai")
	s.AssertFileContains("commit-msg checks conventional format", commitMsgPath, "conventional")

	prePushPath := filepath.Join(gitHooksDir, "pre-push")
	s.AssertFileExists("pre-push hook exists", prePushPath)
	s.AssertFileContains("pre-push blocks main", prePushPath, "main")
	s.AssertFileContains("pre-push blocks master", prePushPath, "master")
	s.AssertFileContains("pre-push has allowMainPush", prePushPath, "allowMainPush")

	hooksPath, _ := gitOutput("", "config", "--global", "core.hooksPath")
	s.Assert("core.hooksPath is set", strings.TrimSpace(hooksPath) != "", "not set")
	s.Assert("core.hooksPath points to cursor-config", strings.Contains(hooksPath, "cursor-config/git-hooks"), "wrong path: "+hooksPath)

	allowMain, _ := gitOutput(p.GlobalKB, "config", "hooks.allowMainPush")
	s.Assert("global-kb allows main push", strings.Contains(allowMain, "true"), "not true")

	email, _ := gitOutput(p.GlobalKB, "config", "user.email")
	s.Assert("git email is personal", strings.Contains(email, "jaslian"), "expected personal email")
	s.Assert("git email not zendesk", !strings.Contains(email, "zendesk"), "zendesk email on personal repo")

	return s
}

func suiteSelfImprovementPipeline(p config.Paths) *Suite {
	s := &Suite{Name: "Self-Improvement Pipeline"}

	promotePath := filepath.Join(p.CursorConfigDir(), "bin", "promote-learnings.py")
	s.AssertFileExists("promote-learnings.py exists", promotePath)

	s.AssertFileExists("global PATTERNS.md", filepath.Join(p.GlobalLearningsDir(), "PATTERNS.md"))
	s.AssertFileExists("global ERRORS.md", filepath.Join(p.GlobalLearningsDir(), "ERRORS.md"))
	s.AssertFileExists("global LEARNINGS.md", filepath.Join(p.GlobalLearningsDir(), "LEARNINGS.md"))
	s.AssertFileExists("global FEATURE_REQUESTS.md", filepath.Join(p.GlobalLearningsDir(), "FEATURE_REQUESTS.md"))
	s.AssertFileExists("global episodes/", filepath.Join(p.GlobalLearningsDir(), "episodes"))

	s.AssertDirMinCount("episodes >= 3", filepath.Join(p.GlobalLearningsDir(), "episodes"), 3, ".md")

	patternsPath := filepath.Join(p.GlobalLearningsDir(), "PATTERNS.md")
	s.AssertFileContains("PATTERNS has table header", patternsPath, "| ID |")
	s.AssertFileContains("PATTERNS has entries", patternsPath, "pat-")

	postEditPath := filepath.Join(p.CursorConfigDir(), "hooks", "post-edit.sh")
	s.AssertFileContains("post-edit calls promote-learnings", postEditPath, "promote-learnings")
	s.AssertFileContains("post-edit detects .learnings/", postEditPath, ".learnings")

	housekeepingPath := filepath.Join(p.CursorConfigDir(), "hooks", "housekeeping.sh")
	s.AssertFileContains("housekeeping calls promote-learnings", housekeepingPath, "promote-learnings")

	selfImpSkill := filepath.Join(p.SkillsDir, "self-improving-agent", "SKILL.md")
	s.AssertFileContains("skill has abstraction rules", selfImpSkill, "abstraction")
	s.AssertFileContains("skill references promotion pipeline", selfImpSkill, "promote")

	binSymlink := filepath.Join(p.BinDir, "promote-learnings.py")
	s.AssertSymlink("~/bin/promote-learnings.py is symlink", binSymlink)

	bootstrapPath := filepath.Join(p.CursorConfigDir(), "bootstrap.sh")
	s.AssertFileContains("bootstrap includes promote-learnings", bootstrapPath, "promote-learnings")

	return s
}

// --- helpers ---

func countDirsWithFile(dir, filename string, exclude map[string]bool) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if exclude != nil && exclude[e.Name()] {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, e.Name(), filename)); err == nil {
			count++
		}
	}
	return count
}

func countFilesWithExt(dir, ext string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ext) {
			count++
		}
	}
	return count
}

func gitOutput(repoPath string, args ...string) (string, error) {
	var fullArgs []string
	if repoPath != "" {
		fullArgs = append([]string{"-C", repoPath}, args...)
	} else {
		fullArgs = args
	}
	cmd := exec.Command("git", fullArgs...)
	out, err := cmd.Output()
	return string(out), err
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
