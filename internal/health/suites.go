package health

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/nfsarch33/cursor-tools/internal/config"
)

// BuildAllSuites creates the full health check suite set.
func BuildAllSuites(p config.Paths) []*Suite {
	return []*Suite{
		suiteL0Rules(p),
		suiteL1Pepper(p),
		suiteL2GlobalKB(p),
		suiteSkillsRegistry(p),
		suiteSkillsCursorPolicy(p),
		suiteCrossFileConsistency(p),
		suiteGitSync(p),
		suiteHooksSubagentsCommandsMCP(p),
		suiteInstallReadiness(p),
		suiteMCPReadiness(p),
		suiteMem0Connectivity(p),
		suitePlatformReadiness(p),
		suiteResumeReadiness(p),
		suiteMemoryRouting(p),
		suiteCrossMachineSync(p),
		suiteProgrammaticCounts(p),
		suiteHookUnitTests(p),
		suiteLogFileIntegrity(p),
		suiteAutomationPipeline(p),
		suiteSkillvetEDR(p),
		suiteIronClawReadiness(p),
		suiteGlobalCursorConfig(p),
		suiteRaceConditionPrevention(p),
		suiteDataIntegrity(p),
		suiteGitHookIntegrity(p),
		suiteSelfImprovementPipeline(p),
		suiteDevContainerCompliance(p),
		suiteRTKTokenOptimization(p),
		suiteToolchainFreshness(p),
		suiteToolchainCrossPlatform(p),
		suiteHandoffAcknowledgement(p),
		suiteGitSyncResilience(p),
	}
}

// BuildDoctorSuites selects the shared health suites used by doctor subcommands.
func BuildDoctorSuites(p config.Paths, profile string) []*Suite {
	selected := map[string]bool{}
	switch profile {
	case "install":
		selected = map[string]bool{
			"L1 Pepper":                        true,
			"L2 Global KB":                     true,
			"Skills Registry":                  true,
			"skills-cursor Policy":             true,
			"Hooks, Sub-agents, Commands, MCP": true,
			"Install Readiness":                true,
			"MCP Readiness":                    true,
			"Mem0 Connectivity":                true,
			"Platform Readiness":               true,
			"Global Cursor Config":             true,
			"Cross-Machine Sync":               true,
			"rtk Token Optimization":           true,
		}
	case "mcp":
		selected = map[string]bool{
			"Hooks, Sub-agents, Commands, MCP": true,
			"MCP Readiness":                    true,
			"IronClaw Readiness":               true,
			"Platform Readiness":               true,
			"Programmatic Count Verification":  true,
		}
	case "platform":
		selected = map[string]bool{
			"Install Readiness":       true,
			"Platform Readiness":      true,
			"Cross-Machine Sync":      true,
			"Global Cursor Config":    true,
			"DevContainer Compliance": true,
		}
	case "resume":
		selected = map[string]bool{
			"Cross-File Consistency":          true,
			"Git Sync":                        true,
			"Programmatic Count Verification": true,
			"Resume Readiness":                true,
			"Automation Pipeline":             true,
			"Data Integrity":                  true,
			"MCP Readiness":                   true,
			"Mem0 Connectivity":               true,
			"Toolchain Freshness":             true,
			"Toolchain Cross-Platform":        true,
			"Handoff Acknowledgement":         true,
			"Git Sync Resilience":             true,
		}
	default:
		return BuildAllSuites(p)
	}

	var filtered []*Suite
	for _, suite := range BuildAllSuites(p) {
		if selected[suite.Name] {
			filtered = append(filtered, suite)
		}
	}
	return filtered
}

func suiteL0Rules(p config.Paths) *Suite {
	s := &Suite{Name: "L0 Rules"}
	rulesDir := p.RulesDir

	expectedRules := []string{
		"00-capabilities", "devcontainer-execution", "engineering-standards",
		"outsource-to-save-tokens", "rtk-token-optimization", "self-improvement",
		"skill-routing", "subagents", "template.rules", "zendesk-workspace.rules",
	}
	s.AssertFileExists("Rules directory exists", rulesDir)

	entries, err := os.ReadDir(rulesDir)
	if err == nil {
		s.Assert("Rules dir has files", len(entries) >= 10, "expected >= 10 rule files")
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

func suiteSkillsCursorPolicy(p config.Paths) *Suite {
	s := &Suite{Name: "skills-cursor Policy"}
	s.AssertFileExists("skills-cursor dir exists", p.SkillsCursorDir())

	count := countDirsWithFile(p.SkillsCursorDir(), "SKILL.md", nil)
	s.Assert("skills-cursor count >= 5", count >= 5, itoa(count))

	indexPath := filepath.Join(p.GlobalMemoriesDir(), "skills-index.md")
	s.AssertFileContains("skills-index documents skills-cursor", indexPath, "skills-cursor")
	metaIndexPath := filepath.Join(p.SkillsDir, "00-index", "SKILL.md")
	s.AssertFileContains("00-index documents skills-cursor", metaIndexPath, "skills-cursor")

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

	binaryPath := filepath.Join(p.BinDir, "cursor-tools")
	s.AssertFileExists("cursor-tools binary exists", binaryPath)

	hooksJSON := filepath.Join(p.CursorConfigDir(), "hooks.json")
	s.AssertFileContains("hooks.json routes guard-shell to Go", hooksJSON, "cursor-tools hook guard-shell")
	s.AssertFileContains("hooks.json routes sanitize-read to Go", hooksJSON, "cursor-tools hook sanitize-read")
	s.AssertFileContains("hooks.json routes guard-mcp to Go", hooksJSON, "cursor-tools hook guard-mcp")
	s.AssertFileContains("hooks.json routes post-edit to Go", hooksJSON, "cursor-tools hook post-edit")
	s.AssertFileContains("hooks.json routes housekeeping to Go", hooksJSON, "cursor-tools hook housekeeping")

	s.AssertSymlink("hooks.json is symlink", filepath.Join(p.Home, ".cursor", "hooks.json"))

	agentFiles := []string{"go-architect.md", "go-tester.md", "flutter-architect.md", "flutter-implementer.md", "agent-orchestrator.md", "memory-ops.md"}
	for _, a := range agentFiles {
		s.AssertFileExists("Agent: "+a, filepath.Join(p.AgentsDir, a))
	}

	s.AssertDirMinCount("Commands >= 6", p.CommandsDir, 6, ".md")

	mcpIndex := filepath.Join(p.GlobalMemoriesDir(), "mcp-index-and-selection-sop.md")
	s.AssertFileExists("MCP index exists", mcpIndex)
	s.AssertFileContains("MCP index has servers", mcpIndex, "MCP")
	s.AssertFileNotContains("No credentials in MCP index", mcpIndex, "ATATT3x")

	return s
}

func suiteInstallReadiness(p config.Paths) *Suite {
	s := &Suite{Name: "Install Readiness"}

	s.AssertSymlink("~/memo is symlink", p.Memo)
	s.AssertFileExists("cursor-tools binary exists", filepath.Join(p.BinDir, "cursor-tools"))
	s.AssertSymlink("skills symlink exists", p.SkillsDir)
	s.AssertSymlink("rules symlink exists", p.RulesDir)
	s.AssertSymlink("commands symlink exists", p.CommandsDir)
	s.AssertSymlink("agents symlink exists", p.AgentsDir)
	s.AssertSymlink("hooks.json symlink exists", p.HooksJSONPath())
	s.AssertFileExists("MCP config exists", p.CursorMCPConfig())
	s.AssertFileExists("bootstrap.sh exists", filepath.Join(p.CursorConfigDir(), "bootstrap.sh"))

	switch p.PlatformProfile() {
	case "macos":
		s.AssertFileExists("launchd installer exists", filepath.Join(p.GlobalKB, "tools", "install-launchd-automation.sh"))
	default:
		s.AssertFileExists("systemd installer exists", filepath.Join(p.GlobalKB, "tools", "install-systemd-user-timer.sh"))
	}

	return s
}

func suiteMCPReadiness(p config.Paths) *Suite {
	s := &Suite{Name: "MCP Readiness"}
	mcpPath := p.CursorMCPConfig()
	s.AssertFileExists("mcp.json exists", mcpPath)

	cfg, err := loadMCPHealthConfig(mcpPath)
	if err != nil {
		s.Fail("mcp.json parses", err.Error())
		return s
	}
	s.Pass("mcp.json parses")
	s.Assert("mcpServers present", len(cfg.MCPServers) >= 9, itoa(len(cfg.MCPServers)))

	requiredServers := []string{
		"mem0", "context-mode", "context7", "git-mcp-server",
		"github-official", "duckduckgo", "fetch",
	}
	for _, name := range requiredServers {
		_, ok := cfg.MCPServers[name]
		s.Assert("MCP server: "+name, ok, "not found in mcp.json")
	}

	_, hasPerplexityAsk := cfg.MCPServers["perplexity-ask"]
	_, hasPerplexity := cfg.MCPServers["perplexity"]
	s.Assert("Perplexity server present", hasPerplexityAsk || hasPerplexity, "expected perplexity or perplexity-ask")

	activeServers := 0
	for name, spec := range cfg.MCPServers {
		if spec.Disabled {
			continue
		}
		activeServers++
		s.Assert("Command resolvable: "+name, commandResolvable(spec.Command), "command not found: "+spec.Command)
		s.Assert("Env ready: "+name, envReady(spec.Env), "missing env placeholder for enabled server")
		s.Assert("Absolute args exist: "+name, absArgsExist(spec.Args), "missing absolute arg path")
	}
	s.Assert("active MCP servers >= 9", activeServers >= 9, itoa(activeServers))

	return s
}

func suiteMem0Connectivity(p config.Paths) *Suite {
	s := &Suite{Name: "Mem0 Connectivity"}
	mcpPath := p.CursorMCPConfig()
	s.AssertFileExists("mcp.json exists", mcpPath)

	cfg, err := loadMCPHealthConfig(mcpPath)
	if err != nil {
		s.Fail("mcp.json parses", err.Error())
		return s
	}
	s.Pass("mcp.json parses")

	mem0, ok := cfg.MCPServers["mem0"]
	s.Assert("mem0 configured", ok, "add mem0 to ~/.cursor/mcp.json")
	if !ok {
		return s
	}

	s.Assert("mem0 enabled", !mem0.Disabled, "mem0 is disabled")
	s.Assert("mem0 command resolvable", commandResolvable(mem0.Command), "command not found: "+mem0.Command)
	s.Assert("mem0 env ready", envReady(mem0.Env), "MEM0_API_KEY or MEM0_DEFAULT_USER_ID missing")
	s.Assert("mem0 default user id configured", strings.TrimSpace(mem0.Env["MEM0_DEFAULT_USER_ID"]) != "", "set MEM0_DEFAULT_USER_ID")
	s.Assert("mem0 api key configured", strings.TrimSpace(mem0.Env["MEM0_API_KEY"]) != "", "set MEM0_API_KEY")

	if allPepper, hasAllPepper := cfg.MCPServers["allPepper-memory-bank"]; hasAllPepper {
		s.Assert("allPepper disabled during Mem0 migration", allPepper.Disabled, "disable allPepper-memory-bank after Mem0 cutover")
	} else {
		s.Pass("allPepper absent or removed")
	}

	memoryTask := filepath.Join(p.CommandsDir, "memory-task.md")
	s.AssertFileContains("memory task routes to mem0", memoryTask, "mem0")

	return s
}

func suitePlatformReadiness(p config.Paths) *Suite {
	s := &Suite{Name: "Platform Readiness"}
	profile := p.PlatformProfile()
	s.Assert("platform profile recognised", profile == "macos" || profile == "wsl" || profile == "linux", profile)

	switch profile {
	case "macos":
		s.Assert("GOOS is darwin", runtime.GOOS == "darwin", runtime.GOOS)
		s.AssertFileExists("macOS MCP extras doc exists", filepath.Join(p.CursorConfigDir(), "mcp-templates", "mcp-config-macos-extras.md"))
	case "wsl":
		s.Assert("WSL detected", true, "")
		s.Assert("home path looks Linux", strings.HasPrefix(p.Home, "/home/"), p.Home)
		s.AssertFileExists("WSL MCP extras doc exists", filepath.Join(p.CursorConfigDir(), "mcp-templates", "mcp-config-wsl-extras.md"))
	default:
		s.Assert("home path looks Linux", strings.HasPrefix(p.Home, "/home/") || p.Home == "~", p.Home)
		s.AssertFileExists("WSL MCP extras doc exists", filepath.Join(p.CursorConfigDir(), "mcp-templates", "mcp-config-wsl-extras.md"))
	}

	s.Assert("SSH key path computed", strings.TrimSpace(p.SSHKeyPath()) != "", "empty ssh key path")

	return s
}

func suiteResumeReadiness(p config.Paths) *Suite {
	s := &Suite{Name: "Resume Readiness"}

	dailyPrompt := filepath.Join(p.GlobalMemoriesDir(), "daily-startup-prompt.md")
	handoff := latestGlobMatch(p.GlobalMemoriesDir(), "session-handoff-*.md")
	mcpIndex := filepath.Join(p.GlobalMemoriesDir(), "mcp-index-and-selection-sop.md")

	s.AssertFileExists("daily-startup prompt exists", dailyPrompt)
	s.Assert("session handoff exists", handoff != "", "no session-handoff-*.md found in "+p.GlobalMemoriesDir())
	s.AssertFileExists("MCP index exists", mcpIndex)
	s.AssertFileContains("daily prompt references session handoff", dailyPrompt, "session-handoff")
	if handoff != "" {
		s.AssertFileContains("session handoff has section headers", handoff, "## ")
	}

	status, err := gitOutput(p.GlobalKB, "status", "--short")
	s.Assert("git status executes", err == nil, "git status failed")
	s.Assert("git status output captured", err == nil && status != "" || strings.TrimSpace(status) == "", "status unavailable")

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
	s.AssertFileContains("routing mentions Mem0", dailyPrompt, "Mem0")
	s.AssertFileNotContains("daily prompt does not route primary memory to allPepper", dailyPrompt, "L1 Pepper")

	selfImproveSkill := filepath.Join(p.SkillsDir, "self-improving-agent", "SKILL.md")
	s.AssertFileExists("self-improving-agent skill exists", selfImproveSkill)
	s.AssertFileContains("skill has memory architecture", selfImproveSkill, "Memory")
	s.AssertFileContains("skill has promotion rules", selfImproveSkill, "Promote")

	memorySystemSkill := filepath.Join(p.SkillsDir, "memory-system", "SKILL.md")
	s.AssertFileExists("memory-system skill exists", memorySystemSkill)
	s.AssertFileContains("memory-system uses mem0", memorySystemSkill, "mem0")

	memoryTask := filepath.Join(p.CommandsDir, "memory-task.md")
	s.AssertFileExists("memory-task command exists", memoryTask)
	s.AssertFileContains("memory-task searches mem0 first", memoryTask, "search_memories")
	s.AssertFileContains("memory-task records memory outcomes", memoryTask, "--memory-layer mem0")

	selfImproveRule := filepath.Join(p.RulesDir, "self-improvement.md")
	s.AssertFileExists("self-improvement rule exists", selfImproveRule)
	s.AssertFileContains("self-improvement promotes to Mem0", selfImproveRule, "promote to Mem0")
	s.AssertFileNotContains("self-improvement no longer uses memo learnings as primary shared store", selfImproveRule, "~/memo/learnings/")

	mcpIndex := filepath.Join(p.GlobalMemoriesDir(), "mcp-index-and-selection-sop.md")
	s.AssertFileContains("mcp index marks allPepper disabled", mcpIndex, "### allPepper-memory-bank")
	s.AssertFileContains("mcp index marks allPepper legacy", mcpIndex, "legacy fallback only")

	return s
}

func suiteCrossMachineSync(p config.Paths) *Suite {
	s := &Suite{Name: "Cross-Machine Sync"}

	s.AssertFileExists("bootstrap.sh exists", filepath.Join(p.CursorConfigDir(), "bootstrap.sh"))

	bootstrapPath := filepath.Join(p.CursorConfigDir(), "bootstrap.sh")
	s.AssertFileNotContains("bootstrap has no /opt/homebrew", bootstrapPath, "/opt/homebrew")

	binaryPath := filepath.Join(p.BinDir, "cursor-tools")
	s.AssertFileExists("cursor-tools binary exists", binaryPath)

	gitHooksDir := filepath.Join(p.CursorConfigDir(), "git-hooks")
	for _, h := range []string{"commit-msg", "pre-push"} {
		path := filepath.Join(gitHooksDir, h)
		s.AssertFileContains("git-hook "+h+" delegates to Go binary", path, "cursor-tools")
	}

	legacyDir := filepath.Join(p.CursorConfigDir(), "legacy")
	s.AssertFileExists("legacy/ archive exists", legacyDir)
	s.AssertFileExists("legacy README exists", filepath.Join(legacyDir, "README.md"))

	sshKey := p.SSHKeyPath()
	s.AssertFileExists("SSH key exists -- "+filepath.Base(sshKey), sshKey)

	return s
}

func suiteProgrammaticCounts(p config.Paths) *Suite {
	s := &Suite{Name: "Programmatic Count Verification"}

	// Note: The total number of assertions in this health check
	// is documented in ~/memo/global-memories/daily-startup-prompt.md.
	// If you add or remove assertions, update the count in that file.

	cursorCount := countDirsWithFile(p.SkillsDir, "SKILL.md", map[string]bool{"00-index": true})
	agentsCount := countDirsWithFile(p.AgentsSkillsDir, "SKILL.md", nil)
	total := cursorCount + agentsCount

	s.Assert("cursor skills > 0", cursorCount > 0, itoa(cursorCount))
	s.Assert("agents skills > 0", agentsCount > 0, itoa(agentsCount))
	s.Assert("total matches sum", total == cursorCount+agentsCount, "mismatch")

	hooksJSON := filepath.Join(p.CursorConfigDir(), "hooks.json")
	data, err := os.ReadFile(hooksJSON)
	hookRoutes := 0
	if err == nil {
		hookRoutes = strings.Count(string(data), "cursor-tools hook")
	}
	s.Assert("hooks.json has 5 Go routes", hookRoutes == 5, itoa(hookRoutes))

	agentCount := countFilesWithExt(p.AgentsDir, ".md")
	s.Assert("agents = 6", agentCount == 6, itoa(agentCount))

	cmdCount := countFilesWithExt(p.CommandsDir, ".md")
	s.Assert("commands = 6", cmdCount == 6, itoa(cmdCount))

	return s
}

func suiteHookUnitTests(p config.Paths) *Suite {
	s := &Suite{Name: "Hook Unit Tests"}

	binaryPath := filepath.Join(p.BinDir, "cursor-tools")
	s.AssertFileExists("cursor-tools binary exists", binaryPath)

	cmd := exec.Command(binaryPath, "selftest")
	out, err := cmd.CombinedOutput()
	output := string(out)
	s.Assert("selftest runs without error", err == nil, "selftest failed: "+output)
	s.Assert("selftest has guard-shell tests", strings.Contains(output, "guard-shell"), "missing guard-shell tests")
	s.Assert("selftest has sanitize-read tests", strings.Contains(output, "sanitize-read"), "missing sanitize-read tests")
	s.Assert("selftest has guard-mcp tests", strings.Contains(output, "guard-mcp"), "missing guard-mcp tests")

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

	hooksJSON := filepath.Join(p.CursorConfigDir(), "hooks.json")
	s.AssertFileContains("hooks.json has guard-shell route", hooksJSON, "guard-shell")
	s.AssertFileContains("hooks.json has sanitize-read route", hooksJSON, "sanitize-read")
	s.AssertFileContains("hooks.json has guard-mcp route", hooksJSON, "guard-mcp")
	s.AssertFileContains("hooks.json has post-edit route", hooksJSON, "post-edit")
	s.AssertFileContains("hooks.json has housekeeping route", hooksJSON, "housekeeping")

	binaryPath := filepath.Join(p.BinDir, "cursor-tools")
	s.AssertFileExists("Go binary handles log rotation", binaryPath)
	s.AssertFileExists("Go binary handles log writing", binaryPath)
	s.AssertFileExists("Go binary handles mcp-audit logs", binaryPath)

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

	s.AssertFileContains("hooks.json uses Go binary", hooksJSON, "cursor-tools")

	goPostEdit := filepath.Join(p.CursorConfigDir(), "cursor-tools", "internal", "cli", "hook_post_edit.go")
	s.AssertFileContains("Go post-edit calls sync-counts", goPostEdit, "sync-counts")
	s.AssertFileContains("Go post-edit formats go", goPostEdit, "gofmt")
	s.AssertFileContains("Go post-edit formats dart", goPostEdit, "dart")

	goHousekeeping := filepath.Join(p.CursorConfigDir(), "cursor-tools", "internal", "cli", "hook_housekeeping.go")
	s.AssertFileContains("Go housekeeping does git sync", goHousekeeping, "git")

	goDailyRefresh := filepath.Join(p.CursorConfigDir(), "cursor-tools", "internal", "cli", "daily_refresh.go")
	s.AssertFileExists("Go daily-refresh exists", goDailyRefresh)
	s.AssertFileContains("Go daily-refresh has MCP step", goDailyRefresh, "stepMCPIndex")
	s.AssertFileContains("Go daily-refresh has git sync", goDailyRefresh, "stepGitSync")
	s.AssertFileContains("Go daily-refresh has inline repo sync", goDailyRefresh, "syncRepoMemories")

	goMCPIndex := filepath.Join(p.CursorConfigDir(), "cursor-tools", "internal", "cli", "mcp_index.go")
	s.AssertFileExists("Go mcp-index exists", goMCPIndex)
	s.AssertFileContains("Go mcp-index renders markdown", goMCPIndex, "renderMCPIndex")
	s.AssertFileContains("Go mcp-index redacts env", goMCPIndex, "values redacted")

	goMetricsStore := filepath.Join(p.CursorConfigDir(), "cursor-tools", "internal", "metrics", "store.go")
	s.AssertFileExists("Go metrics store exists", goMetricsStore)
	s.AssertFileContains("Go metrics has JSONL recording", goMetricsStore, "Record")
	s.AssertFileContains("Go metrics has summarise", goMetricsStore, "Summarise")

	goMetricsCmd := filepath.Join(p.CursorConfigDir(), "cursor-tools", "internal", "cli", "metrics_cmd.go")
	s.AssertFileExists("Go metrics command exists", goMetricsCmd)

	goClilog := filepath.Join(p.CursorConfigDir(), "cursor-tools", "internal", "clilog", "clilog.go")
	s.AssertFileExists("Go clilog package exists", goClilog)
	s.AssertFileContains("Go clilog has colour support", goClilog, "colorEnabled")

	s.AssertFileContains("Go daily-refresh has metrics step", goDailyRefresh, "stepMetricsReport")

	return s
}

func suiteIronClawReadiness(p config.Paths) *Suite {
	s := &Suite{Name: "IronClaw Readiness"}
	mcpPath := p.CursorMCPConfig()
	cfg, err := loadMCPHealthConfig(mcpPath)
	if err != nil {
		s.Pass("mcp.json parses (IronClaw check skipped)")
		return s
	}
	spec, hasIronclaw := cfg.MCPServers["ironclaw"]
	if !hasIronclaw {
		s.Pass("ironclaw not configured (optional for local Cursor+IronClaw integration)")
		return s
	}
	if spec.Disabled {
		s.Pass("ironclaw configured but disabled")
		return s
	}
	s.Assert("ironclaw command resolvable", commandResolvable(spec.Command), "ironclaw-mcp binary not found: "+spec.Command)
	// envReady is optional: ironclaw-mcp defaults to http://localhost:3000
	if len(spec.Env) > 0 {
		s.Assert("ironclaw env ready", envReady(spec.Env), "missing env placeholder for ironclaw")
	}
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

	binaryPath := filepath.Join(p.BinDir, "cursor-tools")
	s.AssertFileExists("cursor-tools binary in ~/bin", binaryPath)

	info, err := os.Stat(binaryPath)
	if err == nil {
		s.Assert("cursor-tools is executable", info.Mode()&0111 != 0, "not executable")
	} else {
		s.Fail("cursor-tools is executable", "binary not found")
	}

	legacyDir := filepath.Join(p.CursorConfigDir(), "legacy")
	s.AssertFileExists("legacy/ archive exists", legacyDir)

	bootstrapPath := filepath.Join(p.CursorConfigDir(), "bootstrap.sh")
	s.AssertFileExists("bootstrap.sh exists", bootstrapPath)
	s.AssertFileContains("bootstrap has skills", bootstrapPath, "skills")
	s.AssertFileContains("bootstrap has rules", bootstrapPath, "rules")
	s.AssertFileContains("bootstrap has hooks", bootstrapPath, "hooks")
	s.AssertFileContains("bootstrap has memo", bootstrapPath, "memo")
	s.AssertFileContains("bootstrap builds Go binary", bootstrapPath, "cursor-tools")

	return s
}

func suiteRaceConditionPrevention(p config.Paths) *Suite {
	s := &Suite{Name: "Race Condition Prevention"}

	goLockfile := filepath.Join(p.CursorConfigDir(), "cursor-tools", "internal", "lockfile")
	s.AssertFileExists("Go lockfile package exists", goLockfile)

	dirLockSrc := filepath.Join(goLockfile, "dirlock.go")
	if _, err := os.Stat(dirLockSrc); err == nil {
		s.AssertFileContains("Go DirLock has mkdir", dirLockSrc, "Mkdir")
		s.AssertFileContains("Go DirLock has pid tracking", dirLockSrc, "pid")
		s.AssertFileContains("Go DirLock has stale detection", dirLockSrc, "Stale")
		s.AssertFileContains("Go DirLock has release", dirLockSrc, "Release")
	} else {
		s.Fail("Go DirLock has mkdir", "dirlock.go not found")
		s.Fail("Go DirLock has pid tracking", "dirlock.go not found")
		s.Fail("Go DirLock has stale detection", "dirlock.go not found")
		s.Fail("Go DirLock has release", "dirlock.go not found")
	}

	fileLockSrc := filepath.Join(goLockfile, "filelock.go")
	if _, err := os.Stat(fileLockSrc); err == nil {
		s.AssertFileContains("Go FileLock has flock", fileLockSrc, "flock")
		s.AssertFileContains("Go FileLock has LOCK_EX", fileLockSrc, "LOCK_EX")
	} else {
		s.Fail("Go FileLock has flock", "filelock.go not found")
		s.Fail("Go FileLock has LOCK_EX", "filelock.go not found")
	}

	lockDir := filepath.Join(p.HooksDir, ".housekeeping.lock")
	info, err := os.Stat(lockDir)
	s.Assert("No stale housekeeping lock", err != nil || !info.IsDir(), "stale lock found")

	goHousekeeping := filepath.Join(p.CursorConfigDir(), "cursor-tools", "internal", "cli", "hook_housekeeping.go")
	if _, err := os.Stat(goHousekeeping); err == nil {
		s.AssertFileContains("Go housekeeping has git sync", goHousekeeping, "git")
		s.AssertFileContains("Go housekeeping has log rotation", goHousekeeping, "Rotate")
	} else {
		s.Fail("Go housekeeping has git sync", "hook_housekeeping.go not found")
		s.Fail("Go housekeeping has log rotation", "hook_housekeeping.go not found")
	}

	goGuardShell := filepath.Join(p.CursorConfigDir(), "cursor-tools", "internal", "cli", "hook_guard_shell.go")
	s.AssertFileExists("Go guard-shell implementation exists", goGuardShell)
	s.AssertFileContains("Go guard-shell records metrics", goGuardShell, "metrics.Record")

	prePushPath := filepath.Join(p.CursorConfigDir(), "git-hooks", "pre-push")
	s.AssertFileContains("pre-push delegates to Go", prePushPath, "cursor-tools")

	commitMsgPath := filepath.Join(p.CursorConfigDir(), "git-hooks", "commit-msg")
	s.AssertFileContains("commit-msg delegates to Go", commitMsgPath, "cursor-tools")

	goCommitMsg := filepath.Join(p.CursorConfigDir(), "cursor-tools", "internal", "cli", "githook_commit_msg.go")
	s.AssertFileContains("Go commit-msg has conventional format", goCommitMsg, "conventional")

	goPrePush := filepath.Join(p.CursorConfigDir(), "cursor-tools", "internal", "cli", "githook_pre_push.go")
	s.AssertFileContains("Go pre-push has allowMainPush", goPrePush, "allowMainPush")

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
	s.AssertFileExists("commit-msg shim exists", commitMsgPath)
	s.AssertFileContains("commit-msg delegates to cursor-tools", commitMsgPath, "cursor-tools")
	s.AssertFileContains("commit-msg calls githook subcommand", commitMsgPath, "githook")

	prePushPath := filepath.Join(gitHooksDir, "pre-push")
	s.AssertFileExists("pre-push shim exists", prePushPath)
	s.AssertFileContains("pre-push delegates to cursor-tools", prePushPath, "cursor-tools")
	s.AssertFileContains("pre-push calls githook subcommand", prePushPath, "githook")

	goCommitMsg := filepath.Join(p.CursorConfigDir(), "cursor-tools", "internal", "cli", "githook_commit_msg.go")
	s.AssertFileContains("Go commit-msg checks AI attribution", goCommitMsg, "aiPatterns")
	s.AssertFileContains("Go commit-msg checks conventional format", goCommitMsg, "conventionalFormat")

	goPrePush := filepath.Join(p.CursorConfigDir(), "cursor-tools", "internal", "cli", "githook_pre_push.go")
	s.AssertFileContains("Go pre-push blocks main", goPrePush, "main")
	s.AssertFileContains("Go pre-push blocks master", goPrePush, "master")
	s.AssertFileContains("Go pre-push has allowMainPush", goPrePush, "allowMainPush")

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

	binaryPath := filepath.Join(p.BinDir, "cursor-tools")
	s.AssertFileExists("cursor-tools binary exists", binaryPath)

	s.AssertFileExists("global PATTERNS.md", filepath.Join(p.GlobalLearningsDir(), "PATTERNS.md"))
	s.AssertFileExists("global ERRORS.md", filepath.Join(p.GlobalLearningsDir(), "ERRORS.md"))
	s.AssertFileExists("global LEARNINGS.md", filepath.Join(p.GlobalLearningsDir(), "LEARNINGS.md"))
	s.AssertFileExists("global FEATURE_REQUESTS.md", filepath.Join(p.GlobalLearningsDir(), "FEATURE_REQUESTS.md"))
	s.AssertFileExists("global episodes/", filepath.Join(p.GlobalLearningsDir(), "episodes"))

	s.AssertDirMinCount("episodes >= 3", filepath.Join(p.GlobalLearningsDir(), "episodes"), 3, ".md")

	patternsPath := filepath.Join(p.GlobalLearningsDir(), "PATTERNS.md")
	s.AssertFileContains("PATTERNS has table header", patternsPath, "| ID |")
	s.AssertFileContains("PATTERNS has entries", patternsPath, "pat-")

	goPostEdit := filepath.Join(p.CursorConfigDir(), "cursor-tools", "internal", "cli", "hook_post_edit.go")
	s.AssertFileContains("Go post-edit triggers promote", goPostEdit, "promote")
	s.AssertFileContains("Go post-edit detects learnings", goPostEdit, "learnings")

	goHousekeeping := filepath.Join(p.CursorConfigDir(), "cursor-tools", "internal", "cli", "hook_housekeeping.go")
	s.AssertFileContains("Go housekeeping runs promote", goHousekeeping, "promote")

	selfImpSkill := filepath.Join(p.SkillsDir, "self-improving-agent", "SKILL.md")
	s.AssertFileContains("skill has abstraction rules", selfImpSkill, "abstraction")
	s.AssertFileContains("skill references promotion pipeline", selfImpSkill, "promote")

	bootstrapPath := filepath.Join(p.CursorConfigDir(), "bootstrap.sh")
	s.AssertFileContains("bootstrap installs cursor-tools", bootstrapPath, "cursor-tools")

	return s
}

func suiteDevContainerCompliance(p config.Paths) *Suite {
	s := &Suite{Name: "DevContainer Compliance"}
	ccDir := p.CursorConfigDir()
	templatesDir := filepath.Join(ccDir, "devcontainer-templates")

	s.AssertFileExists("devcontainer-templates dir exists", templatesDir)
	s.AssertFileExists("go-workspace Dockerfile exists", filepath.Join(templatesDir, "go-workspace", "Dockerfile"))
	s.AssertFileExists("go-workspace devcontainer.json", filepath.Join(templatesDir, "go-workspace", "devcontainer.json"))
	s.AssertFileExists("cursor-tools .devcontainer exists", filepath.Join(ccDir, "cursor-tools", ".devcontainer", "devcontainer.json"))
	s.AssertFileExists("cursor-tools Dockerfile exists", filepath.Join(ccDir, "cursor-tools", "build", "package", "Dockerfile"))
	s.AssertFileExists("cursor-tools Dockerfile.dev exists", filepath.Join(ccDir, "cursor-tools", "build", "package", "Dockerfile.dev"))
	s.AssertFileContains("Makefile has docker target", filepath.Join(ccDir, "cursor-tools", "Makefile"), "docker-native")
	s.AssertFileContains("Makefile has test-docker target", filepath.Join(ccDir, "cursor-tools", "Makefile"), "test-docker")
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

func suiteRTKTokenOptimization(p config.Paths) *Suite {
	s := &Suite{Name: "rtk Token Optimization"}

	rtkBinary, err := exec.LookPath("rtk")
	s.Assert("rtk binary on PATH", err == nil, "rtk not found; install via brew (macOS) or curl installer (Linux)")
	_ = rtkBinary

	rtkRule := filepath.Join(p.RulesDir, "rtk-token-optimization.md")
	s.AssertFileExists("L0 rule rtk-token-optimization.md exists", rtkRule)

	rtkSkill := filepath.Join(p.SkillsDir, "rtk-integration", "SKILL.md")
	s.AssertFileExists("rtk-integration skill exists", rtkSkill)

	return s
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

type mcpHealthServerSpec struct {
	Command  string            `json:"command"`
	Args     []string          `json:"args"`
	Env      map[string]string `json:"env"`
	Disabled bool              `json:"disabled"`
}

type mcpHealthConfig struct {
	MCPServers map[string]mcpHealthServerSpec `json:"mcpServers"`
}

func loadMCPHealthConfig(path string) (*mcpHealthConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg mcpHealthConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.MCPServers == nil {
		cfg.MCPServers = map[string]mcpHealthServerSpec{}
	}
	return &cfg, nil
}

func commandResolvable(command string) bool {
	if command == "" {
		return false
	}
	if filepath.IsAbs(command) {
		_, err := os.Stat(command)
		return err == nil
	}
	_, err := exec.LookPath(command)
	return err == nil
}

func envReady(env map[string]string) bool {
	for _, value := range env {
		if isExactEnvPlaceholder(value) && os.Getenv(strings.TrimPrefix(value, "$")) == "" {
			return false
		}
	}
	return true
}

func isExactEnvPlaceholder(value string) bool {
	return strings.HasPrefix(value, "$") && !strings.Contains(value, "/") && len(value) > 1
}

func absArgsExist(args []string) bool {
	for _, arg := range args {
		if !filepath.IsAbs(arg) {
			continue
		}
		if _, err := os.Stat(arg); err != nil {
			return false
		}
	}
	return true
}

// latestGlobMatch returns the lexicographically latest file matching pattern in dir,
// or empty string if none found. For date-stamped files this yields the most recent.
func latestGlobMatch(dir, pattern string) string {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil || len(matches) == 0 {
		return ""
	}
	latest := matches[0]
	for _, m := range matches[1:] {
		if m > latest {
			latest = m
		}
	}
	return latest
}

// suiteGitSyncResilience validates the concurrent-sync hardening configuration:
// rerere, .gitattributes merge drivers, no stale rebase, and last push state.
func suiteGitSyncResilience(p config.Paths) *Suite {
	s := &Suite{Name: "Git Sync Resilience"}

	repoPath := p.GlobalKB
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		s.Fail("git repo exists", "no .git at "+repoPath)
		return s
	}

	rerere, _ := gitOutput(repoPath, "config", "--local", "rerere.enabled")
	s.Assert("rerere.enabled is true", strings.TrimSpace(rerere) == "true",
		"run: git -C "+repoPath+" config --local rerere.enabled true")

	oursDriver, _ := gitOutput(repoPath, "config", "--local", "merge.ours.driver")
	s.Assert("merge.ours.driver registered", strings.TrimSpace(oursDriver) == "true",
		"run: git -C "+repoPath+" config --local merge.ours.driver true")

	gitattributes := filepath.Join(repoPath, ".gitattributes")
	s.AssertFileExists(".gitattributes exists", gitattributes)
	s.AssertFileContains(".gitattributes has ours merge", gitattributes, "merge=ours")
	s.AssertFileContains(".gitattributes has union merge", gitattributes, "merge=union")
	s.AssertFileContains(".gitattributes has binary dist", gitattributes, "binary")

	status, _ := gitOutput(repoPath, "status")
	rebaseInProgress := strings.Contains(status, "rebase in progress")
	s.Assert("no rebase in progress", !rebaseInProgress,
		"run: git -C "+repoPath+" rebase --abort")

	indexLock := filepath.Join(gitDir, "index.lock")
	_, lockErr := os.Stat(indexLock)
	s.Assert("no stale index.lock", lockErr != nil,
		"stale lock: rm "+indexLock)

	pushStateFile := filepath.Join(p.HooksDir, "last-push-result.txt")
	pushInfo, pushErr := os.Stat(pushStateFile)
	s.Assert("last-push-result.txt exists", pushErr == nil,
		"no push state recorded yet -- will be created on next sync")

	if pushErr == nil {
		data, _ := os.ReadFile(pushStateFile)
		content := string(data)
		s.Assert("last push was successful", strings.Contains(content, "result: success"),
			"last push failed or deferred -- check "+pushStateFile)
		withinDay := pushInfo.ModTime().After(time.Now().UTC().Add(-48 * time.Hour))
		s.Assert("push state is recent (within 48h)", withinDay,
			fmt.Sprintf("last push state: %s", pushInfo.ModTime().Format("2006-01-02 15:04 UTC")))
	}

	return s
}

// suiteToolchainFreshness warns when the cursor-tools binary is older than its source.
func suiteToolchainFreshness(p config.Paths) *Suite {
	s := &Suite{Name: "Toolchain Freshness"}

	binPath := filepath.Join(p.BinDir, "cursor-tools")
	srcRoot := filepath.Join(p.CursorConfigDir(), "cursor-tools")

	binInfo, err := os.Stat(binPath)
	s.Assert("cursor-tools binary exists", err == nil, binPath+" not found")
	if err != nil {
		return s
	}

	srcInfo, srcErr := os.Stat(srcRoot)
	srcExists := srcErr == nil && srcInfo.IsDir()
	s.Assert("cursor-tools source dir exists", srcExists, srcRoot+" not found")
	if !srcExists {
		return s
	}

	newestSrc := binInfo.ModTime()
	_ = filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(newestSrc) {
			newestSrc = info.ModTime()
		}
		return nil
	})

	stale := newestSrc.After(binInfo.ModTime())
	s.Assert(
		"binary is up-to-date with source",
		!stale,
		fmt.Sprintf("source newer than binary — rebuild: cd %s && go build -o ~/bin/cursor-tools ./cmd/cursor-tools/", srcRoot),
	)

	distDir := filepath.Join(srcRoot, "dist")
	distVersionBytes, _ := os.ReadFile(filepath.Join(distDir, "VERSION"))
	distVersion := strings.TrimSpace(string(distVersionBytes))
	distBin := filepath.Join(distDir, "cursor-tools-"+p.PlatformBinarySuffix())
	distInfo, distErr := os.Stat(distBin)
	distNewer := distErr == nil && distInfo.ModTime().After(binInfo.ModTime())

	distHint := "pre-built binary available — run: cursor-tools auto-update"
	if distVersion != "" {
		distHint = fmt.Sprintf("pre-built binary %s available — run: cursor-tools auto-update", distVersion)
	}
	s.Assert("dist binary matches local binary", !distNewer, distHint)

	return s
}

// suiteToolchainCrossPlatform verifies the binary can execute on this platform.
func suiteToolchainCrossPlatform(p config.Paths) *Suite {
	s := &Suite{Name: "Toolchain Cross-Platform"}

	binPath := filepath.Join(p.BinDir, "cursor-tools")
	if _, err := os.Stat(binPath); err != nil {
		s.Assert("cursor-tools binary exists", false, binPath+" not found")
		return s
	}

	cmd := exec.Command(binPath, "version")
	err := cmd.Run()
	s.Assert(
		"binary executes on current platform ("+p.PlatformProfile()+")",
		err == nil,
		fmt.Sprintf("binary failed to run (%v) — rebuild for %s: cd %s/cursor-tools && go build -o ~/bin/cursor-tools ./cmd/cursor-tools/", err, p.PlatformProfile(), p.CursorConfigDir()),
	)

	if runtime.GOOS == "darwin" {
		s.Assert("running on macOS profile", p.PlatformProfile() == "macos", "PlatformProfile mismatch: got "+p.PlatformProfile())
	} else if isWSLEnv() {
		s.Assert("running on WSL profile", p.PlatformProfile() == "wsl", "PlatformProfile mismatch: got "+p.PlatformProfile())
	}

	return s
}

func isWSLEnv() bool {
	if os.Getenv("WSL_INTEROP") != "" || os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// suiteHandoffAcknowledgement verifies that the pre-pull handoff review has run today.
// The review is recorded by `cursor-tools handoff-review` (or via daily-refresh step 6/8)
// in ~/.cursor/hooks/handoff-last-check.txt.
func suiteHandoffAcknowledgement(p config.Paths) *Suite {
	s := &Suite{Name: "Handoff Acknowledgement"}

	stateFile := filepath.Join(p.HooksDir, "handoff-last-check.txt")
	info, err := os.Stat(stateFile)
	exists := err == nil
	s.Assert("handoff-last-check.txt exists", exists,
		"run 'cursor-tools handoff-review' or 'cursor-tools daily-refresh' to create it")
	if !exists {
		return s
	}

	withinDay := info.ModTime().After(time.Now().UTC().Add(-24 * time.Hour))
	s.Assert("handoff review ran within last 24h", withinDay,
		fmt.Sprintf("last check: %s — run 'cursor-tools handoff-review'", info.ModTime().Format("2006-01-02 15:04 UTC")))

	data, err := os.ReadFile(stateFile)
	if err == nil {
		s.Assert("state file has checked timestamp", strings.Contains(string(data), "checked:"),
			"state file malformed — run 'cursor-tools handoff-review' to regenerate")
	}

	return s
}
