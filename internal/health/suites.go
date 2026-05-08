// runx-public-repo-gate: allow-file personal_path_id — identity gate detects literal personal-stack identifiers, so the strings must remain in source

package health

import (
	"context"
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
	"github.com/nfsarch33/cursor-tools/internal/metrics"
)

type suiteBuilder func(config.Paths) *Suite

type suiteSpec struct {
	name    string
	builder suiteBuilder
}

var suiteCatalog = []suiteSpec{
	{name: "L0 Rules", builder: suiteL0Rules},
	{name: "L1 Startup Indexes", builder: suiteL1StartupIndexes},
	{name: "L2 Global KB", builder: suiteL2GlobalKB},
	{name: "Skills Registry", builder: suiteSkillsRegistry},
	{name: "skills-cursor Policy", builder: suiteSkillsCursorPolicy},
	{name: "Cross-File Consistency", builder: suiteCrossFileConsistency},
	{name: "Git Sync", builder: suiteGitSync},
	{name: "Hooks, Sub-agents, Commands, MCP", builder: suiteHooksSubagentsCommandsMCP},
	{name: "Install Readiness", builder: suiteInstallReadiness},
	{name: "MCP Readiness", builder: suiteMCPReadiness},
	{name: "Mem0 Connectivity", builder: suiteMem0Connectivity},
	{name: "Platform Readiness", builder: suitePlatformReadiness},
	{name: "Resume Readiness", builder: suiteResumeReadiness},
	{name: "Memory Routing", builder: suiteMemoryRouting},
	{name: "Memory Evidence", builder: suiteMemoryEvidence},
	{name: "Cross-Machine Sync", builder: suiteCrossMachineSync},
	{name: "Programmatic Count Verification", builder: suiteProgrammaticCounts},
	{name: "Hook Unit Tests", builder: suiteHookUnitTests},
	{name: "Log File Integrity", builder: suiteLogFileIntegrity},
	{name: "Automation Pipeline", builder: suiteAutomationPipeline},
	{name: "Skillvet EDR-Safety", builder: suiteSkillvetEDR},
	{name: "IronClaw Readiness", builder: suiteIronClawReadiness},
	{name: "Global Cursor Config", builder: suiteGlobalCursorConfig},
	{name: "Race Condition Prevention", builder: suiteRaceConditionPrevention},
	{name: "Data Integrity", builder: suiteDataIntegrity},
	{name: "Git Hook Integrity", builder: suiteGitHookIntegrity},
	{name: "Self-Improvement Pipeline", builder: suiteSelfImprovementPipeline},
	{name: "DevContainer Compliance", builder: suiteDevContainerCompliance},
	{name: "rtk Token Optimization", builder: suiteRTKTokenOptimization},
	{name: "Toolchain Freshness", builder: suiteToolchainFreshness},
	{name: "Toolchain Cross-Platform", builder: suiteToolchainCrossPlatform},
	{name: "Handoff Acknowledgement", builder: suiteHandoffAcknowledgement},
	{name: "Git Sync Resilience", builder: suiteGitSyncResilience},
	{name: "Dependency Readiness", builder: suiteDependencyReadiness},
	{name: "Coordination Signals", builder: suiteCoordinationSignals},
	{name: "Agent Stack Health", builder: suiteAgentStackHealth},
	{name: "DRL EvoLoop Observability", builder: suiteDRLEvoLoopObservability},
	{name: "EvoLoop Cycle Freshness", builder: suiteStaleCycleAge},
	{name: "Pre-Push Readiness", builder: suitePrePushReadiness},
}

var suiteCatalogByName = func() map[string]suiteSpec {
	byName := make(map[string]suiteSpec, len(suiteCatalog))
	for _, spec := range suiteCatalog {
		byName[spec.name] = spec
	}
	return byName
}()

func buildSuiteList(p config.Paths, names []string) []*Suite {
	suites := make([]*Suite, 0, len(names))
	for _, name := range names {
		spec, ok := suiteCatalogByName[name]
		if !ok {
			continue
		}
		started := time.Now()
		suite := spec.builder(p)
		if suite == nil {
			suite = &Suite{Name: name}
		}
		if strings.TrimSpace(suite.Name) == "" {
			suite.Name = name
		}
		suite.DurationMs = time.Since(started).Milliseconds()
		suites = append(suites, suite)
	}
	return suites
}

// BuildAllSuites creates the full health check suite set.
func BuildAllSuites(p config.Paths) []*Suite {
	names := make([]string, 0, len(suiteCatalog))
	for _, spec := range suiteCatalog {
		names = append(names, spec.name)
	}
	return buildSuiteList(p, names)
}

// BuildDoctorSuites selects the shared health suites used by doctor subcommands.
func BuildDoctorSuites(p config.Paths, profile string) []*Suite {
	var names []string
	switch profile {
	case "install":
		names = []string{
			"L1 Startup Indexes",
			"L2 Global KB",
			"Skills Registry",
			"skills-cursor Policy",
			"Hooks, Sub-agents, Commands, MCP",
			"Install Readiness",
			"MCP Readiness",
			"Mem0 Connectivity",
			"Platform Readiness",
			"Global Cursor Config",
			"Cross-Machine Sync",
			"rtk Token Optimization",
		}
	case "mcp":
		names = []string{
			"Hooks, Sub-agents, Commands, MCP",
			"MCP Readiness",
			"Mem0 Connectivity",
			"IronClaw Readiness",
			"Platform Readiness",
			"Programmatic Count Verification",
		}
	case "platform":
		names = []string{
			"Install Readiness",
			"Platform Readiness",
			"Cross-Machine Sync",
			"Global Cursor Config",
			"DevContainer Compliance",
		}
	case "deps":
		names = []string{
			"Dependency Readiness",
			"Platform Readiness",
		}
	case "resume":
		names = []string{
			"Cross-File Consistency",
			"Git Sync",
			"Programmatic Count Verification",
			"Resume Readiness",
			"Automation Pipeline",
			"Data Integrity",
			"MCP Readiness",
			"Mem0 Connectivity",
			"Coordination Signals",
			"Toolchain Freshness",
			"Toolchain Cross-Platform",
			"Handoff Acknowledgement",
			"Git Sync Resilience",
			"EvoLoop Cycle Freshness",
			"Pre-Push Readiness",
		}
	case "stack":
		names = []string{
			"IronClaw Readiness",
			"Agent Stack Health",
			"MCP Readiness",
			"Mem0 Connectivity",
			"Coordination Signals",
			"Platform Readiness",
		}
	case "drl":
		names = []string{
			"DRL EvoLoop Observability",
			"EvoLoop Cycle Freshness",
			"Mem0 Connectivity",
			"Coordination Signals",
			"Self-Improvement Pipeline",
		}
	default:
		return BuildAllSuites(p)
	}
	return buildSuiteList(p, names)
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

func suiteL1StartupIndexes(p config.Paths) *Suite {
	s := &Suite{Name: "L1 Startup Indexes"}
	gmDir := p.GlobalMemoriesDir()

	s.AssertFileExists("global-memories dir exists", gmDir)

	pepperFiles := []string{
		"daily-startup-prompt.md", "skills-index.md",
		"mcp-index-and-selection-sop.md", "one-person-company-progress.md",
	}
	for _, f := range pepperFiles {
		s.AssertFileExists("Startup index: "+f, filepath.Join(gmDir, f))
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

// assertHooksJSONInstallState checks %USERPROFILE%\.cursor\hooks.json (or $HOME/.cursor on Unix).
// On Windows a regular file with absolute cursor-tools.exe paths is expected; elsewhere a symlink
// into the KB template is typical.
func assertHooksJSONInstallState(p config.Paths, s *Suite) {
	live := p.HooksJSONPath()
	if p.PlatformProfile() == "windows" {
		data, err := os.ReadFile(live)
		if err != nil {
			s.Fail("hooks.json readable", err.Error())
		} else {
			ok, detail := windowsHooksJSONPolicy(string(data))
			s.Assert("hooks.json uses Windows-native cursor-tools path", ok, detail)
		}
	} else {
		s.AssertSymlink("hooks.json is symlink", live)
	}
}

func suiteHooksSubagentsCommandsMCP(p config.Paths) *Suite {
	s := &Suite{Name: "Hooks, Sub-agents, Commands, MCP"}

	binaryPath := filepath.Join(p.BinDir, "cursor-tools")
	s.AssertFileExists("cursor-tools binary exists", binaryPath)

	templateHooks := filepath.Join(p.CursorConfigDir(), "hooks.json")
	s.AssertFileContains("hooks.json routes guard-shell to Go", templateHooks, "cursor-tools hook guard-shell")
	s.AssertFileContains("hooks.json routes sanitize-read to Go", templateHooks, "cursor-tools hook sanitize-read")
	s.AssertFileContains("hooks.json routes guard-mcp to Go", templateHooks, "cursor-tools hook guard-mcp")
	s.AssertFileContains("hooks.json routes post-edit to Go", templateHooks, "cursor-tools hook post-edit")
	s.AssertFileContains("hooks.json routes housekeeping to Go", templateHooks, "cursor-tools hook housekeeping")

	assertHooksJSONInstallState(p, s)

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
	assertHooksJSONInstallState(p, s)
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

	// github-official omitted: WSL fleet uses git-mcp-server + gh; Windows native omits PAT-heavy Docker GitHub MCP per daily-startup.
	requiredServers := []string{
		"mem0", "context-mode", "context7", "git-mcp-server",
		"duckduckgo", "fetch",
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
		// Remote HTTP/SSE MCP (e.g. exa) has no local command — Cursor connects via url.
		if strings.TrimSpace(spec.URL) != "" {
			s.Pass("MCP remote URL: " + name)
			continue
		}
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
	s.Assert("platform profile recognised", profile == "macos" || profile == "wsl" || profile == "linux" || profile == "windows", profile)

	switch profile {
	case "macos":
		s.Assert("GOOS is darwin", runtime.GOOS == "darwin", runtime.GOOS)
		s.AssertFileExists("macOS MCP extras doc exists", filepath.Join(p.CursorConfigDir(), "mcp-templates", "mcp-config-macos-extras.md"))
	case "wsl":
		s.Assert("WSL detected", true, "")
		s.Assert("home path looks Linux", strings.HasPrefix(p.Home, "/home/"), p.Home)
		s.AssertFileExists("WSL MCP extras doc exists", filepath.Join(p.CursorConfigDir(), "mcp-templates", "mcp-config-wsl-extras.md"))
	case "windows":
		s.Assert("GOOS is windows", runtime.GOOS == "windows", runtime.GOOS)
		winHome := strings.HasPrefix(p.Home, `C:\Users\`) || strings.HasPrefix(p.Home, `c:\users\`) ||
			strings.Contains(strings.ToLower(p.Home), `\users\`)
		s.Assert("home path looks Windows", winHome || p.Home != "", p.Home)
		s.AssertFileExists("Windows PowerShell Cursor onboarding SOP exists", filepath.Join(p.SOPDir(), "windows-powershell-cursor-onboarding-template.md"))
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
	s.AssertFileContains("memory-task records context-mode outcomes", memoryTask, "context-mode:ctx_search")
	s.AssertFileContains("memory-task records git kb outcomes", memoryTask, "--memory-layer git_kb")

	selfImproveRule := filepath.Join(p.RulesDir, "self-improvement.md")
	s.AssertFileExists("self-improvement rule exists", selfImproveRule)
	s.AssertFileContains("self-improvement promotes to Mem0", selfImproveRule, "promote to Mem0")
	s.AssertFileNotContains("self-improvement no longer uses memo learnings as primary shared store", selfImproveRule, "~/memo/learnings/")

	capabilitiesRule := filepath.Join(p.RulesDir, "00-capabilities.md")
	s.AssertFileExists("00-capabilities rule exists", capabilitiesRule)
	s.AssertFileContains("00-capabilities uses Mem0 as L1", capabilitiesRule, "Shared hot memory")
	s.AssertFileNotContains("00-capabilities no longer routes L1 to Pepper", capabilitiesRule, "via Pepper")

	templateRule := filepath.Join(p.RulesDir, "template.rules")
	s.AssertFileExists("template rule exists", templateRule)
	s.AssertFileNotContains("template rule no longer routes add to memory to Pepper", templateRule, "put procedures/checklists in Pepper")

	zendeskRule := filepath.Join(p.RulesDir, "zendesk-workspace.rules")
	s.AssertFileExists("zendesk workspace rule exists", zendeskRule)
	s.AssertFileNotContains("zendesk rule no longer calls Pepper source of truth", zendeskRule, "Pepper Memory Bank")
	s.AssertFileNotContains("zendesk rule no longer updates Pepper", zendeskRule, "UPDATE Pepper")

	contextModeSkill := filepath.Join(p.SkillsDir, "context-mode", "SKILL.md")
	s.AssertFileExists("context-mode skill exists", contextModeSkill)
	s.AssertFileContains("context-mode skill uses ctx_search", contextModeSkill, "ctx_search")
	s.AssertFileContains("context-mode skill tracks outcomes", contextModeSkill, "--memory-layer context_mode")

	mcpIndex := filepath.Join(p.GlobalMemoriesDir(), "mcp-index-and-selection-sop.md")
	s.AssertFileContains("mcp index marks allPepper disabled", mcpIndex, "### allPepper-memory-bank")
	s.AssertFileContains("mcp index marks allPepper legacy", mcpIndex, "legacy fallback only")

	return s
}

func suiteMemoryEvidence(p config.Paths) *Suite {
	s := &Suite{Name: "Memory Evidence"}
	logsDir := filepath.Join(p.Home, "logs")
	parityExport := filepath.Join(logsDir, "memory-parity.md")
	metricsExport := filepath.Join(logsDir, "memory-metrics.md")

	s.AssertFileExists("logs dir exists", logsDir)
	s.AssertFileExists("memory parity export exists", parityExport)
	s.AssertFileExists("memory metrics export exists", metricsExport)
	s.AssertFileContains("memory parity export proves parity", parityExport, "Parity proven: `true`")
	s.AssertFileContains("memory parity export has zero missing entries", parityExport, "Missing manifest entries: 0")
	s.AssertFileContains("memory metrics export includes memory KPI section", metricsExport, "## Memory Layer KPIs")
	s.AssertFileContains("memory metrics export includes coverage column", metricsExport, "Coverage")

	for _, item := range []struct {
		label string
		path  string
	}{
		{label: "memory parity export is fresh", path: parityExport},
		{label: "memory metrics export is fresh", path: metricsExport},
	} {
		info, err := os.Stat(item.path)
		if err != nil {
			s.Fail(item.label, "missing file: "+item.path)
			continue
		}
		s.Assert(item.label, info.ModTime().After(time.Now().UTC().Add(-35*24*time.Hour)),
			fmt.Sprintf("stale file (%s) — run 'cursor-tools memory-routine'", info.ModTime().Format("2006-01-02 15:04 UTC")))
	}

	events, err := metrics.LoadAll(p.MetricsFile())
	if err != nil {
		s.Fail("metrics history loads", err.Error())
		return s
	}
	s.Pass("metrics history loads")

	summary := metrics.Summarise(events, time.Now().UTC().Add(-30*24*time.Hour))
	for _, layer := range summary.MemoryLayers {
		switch layer.Layer {
		case metrics.MemoryLayerMem0, metrics.MemoryLayerContextMode:
			attempts := layer.Searches + layer.Reads
			if attempts == 0 {
				s.Pass(layer.Layer + " coverage not applicable (no retrieval attempts)")
				continue
			}
			coverage, ok := layer.OutcomeCoverage()
			s.Assert(layer.Layer+" has observed outcomes", ok && layer.Observed > 0,
				"retrieval attempts exist without tracked outcomes — run memory validation or improve automatic tracking")
			s.Assert(layer.Layer+" outcome coverage is at least 50%", ok && coverage >= 50,
				fmt.Sprintf("coverage=%.1f%% observed=%d attempts=%d", coverage, layer.Observed, attempts))
		case metrics.MemoryLayerGitKB:
			attempts := layer.Searches + layer.Reads
			if attempts == 0 {
				continue
			}
			coverage, ok := layer.OutcomeCoverage()
			s.Assert(layer.Layer+" outcome coverage is at least 90%", ok && coverage >= 90,
				fmt.Sprintf("coverage=%.1f%% observed=%d attempts=%d — git_kb reads should auto-infer hit", coverage, layer.Observed, attempts))
		}
	}

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

	out, err := runCombinedOutput(2*time.Minute, binaryPath, "selftest")
	output := string(out)
	s.Assert("selftest runs without error", err == nil, "selftest failed: "+output)
	s.Assert("selftest has guard-shell tests", strings.Contains(output, "guard-shell"), "missing guard-shell tests")
	s.Assert("selftest has sanitize-read tests", strings.Contains(output, "sanitize-read"), "missing sanitize-read tests")
	s.Assert("selftest has guard-mcp tests", strings.Contains(output, "guard-mcp"), "missing guard-mcp tests")

	return s
}

func suiteLogFileIntegrity(p config.Paths) *Suite {
	s := &Suite{Name: "Log File Integrity"}
	logNames := []string{"guard-shell", "sanitize-read", "mcp-audit", "post-edit", "housekeeping", "checks"}

	for _, name := range logNames {
		logPath := filepath.Join(p.HooksDir, name+".log")
		if _, err := os.Stat(logPath); err == nil {
			s.Pass("Log exists: " + name)
			data, _ := os.ReadFile(logPath)
			if len(strings.TrimSpace(string(data))) == 0 {
				s.Pass("Empty current log after rotation: " + name)
				s.Pass("Timestamp check skipped: " + name)
				continue
			}
			hasStructuredTimestamp := regexp.MustCompile(`"ts":"\d{4}-\d{2}-\d{2}T`).Match(data)
			hasLegacyTimestamp := regexp.MustCompile(`\[\d{4}-\d{2}-\d{2}T`).Match(data)
			s.Assert("timestamps in "+name+".log", hasStructuredTimestamp || hasLegacyTimestamp, "no legacy or structured timestamp found")
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

	assertHooksJSONInstallState(p, s)

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

func runCombinedOutput(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func gitOutput(repoPath string, args ...string) (string, error) {
	var fullArgs []string
	if repoPath != "" {
		fullArgs = append([]string{"-C", repoPath}, args...)
	} else {
		fullArgs = args
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	out, err := cmd.Output()
	return string(out), err
}

// rtkBinaryInstalled returns true if rtk is on PATH or present in the user's bin dir
// or configured BinDir. Tests often use an isolated HOME without PATH inheritance to ~/bin;
// checking standard install locations keeps health checks and unit tests aligned.
func rtkBinaryInstalled(p config.Paths) bool {
	if _, err := exec.LookPath("rtk"); err == nil {
		return true
	}
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("rtk.exe"); err == nil {
			return true
		}
	}
	candidates := []string{
		filepath.Join(p.BinDir, "rtk"),
		filepath.Join(p.BinDir, "rtk.exe"),
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, "bin", "rtk"),
			filepath.Join(home, "bin", "rtk.exe"),
		)
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return true
		}
	}
	return false
}

func suiteRTKTokenOptimization(p config.Paths) *Suite {
	s := &Suite{Name: "rtk Token Optimization"}

	s.Assert("rtk binary installed", rtkBinaryInstalled(p),
		"rtk not found on PATH or in ~/bin or tools bindir; install per https://github.com/rtk-ai/rtk/releases")

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
	URL      string            `json:"url"`
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
		branchStatus, _ := gitOutput(repoPath, "status", "--short", "--branch")
		trackedAndSynced := strings.Contains(branchStatus, "...") &&
			!strings.Contains(branchStatus, "ahead ") &&
			!strings.Contains(branchStatus, "behind ")
		s.Assert("last push was successful or branch is synced with upstream",
			strings.Contains(content, "result: success") || trackedAndSynced,
			"last push failed or deferred and branch is not synced with upstream -- check "+pushStateFile)
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

	_, err := runCombinedOutput(30*time.Second, binPath, "version")
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

func suiteDependencyReadiness(p config.Paths) *Suite {
	s := &Suite{Name: "Dependency Readiness"}

	type dependencyCheck struct {
		name        string
		versionArgs []string
		required    bool
	}

	checks := []dependencyCheck{
		{name: "git", versionArgs: []string{"--version"}, required: true},
		{name: "go", versionArgs: []string{"version"}, required: true},
		{name: "gh", versionArgs: []string{"--version"}, required: true},
		{name: "ssh", versionArgs: []string{"-V"}, required: true},
		{name: "node", versionArgs: []string{"--version"}, required: true},
		{name: "npm", versionArgs: []string{"--version"}, required: true},
		{name: "python3", versionArgs: []string{"--version"}, required: true},
		{name: "uv", versionArgs: []string{"--version"}, required: true},
		{name: "docker", versionArgs: []string{"--version"}, required: true},
		{name: "jq", versionArgs: []string{"--version"}, required: true},
		{name: "curl", versionArgs: []string{"--version"}, required: true},
		{name: "make", versionArgs: []string{"--version"}, required: true},
		{name: "rtk", versionArgs: []string{"--version"}, required: true},
		{name: "kubectl", versionArgs: []string{"version", "--client", "--output=yaml"}, required: false},
		{name: "terraform", versionArgs: []string{"version"}, required: false},
		{name: "helm", versionArgs: []string{"version"}, required: false},
	}

	for _, check := range checks {
		path, err := exec.LookPath(check.name)
		if err != nil {
			if check.required {
				s.Fail(check.name+" on PATH", check.name+" not found")
			} else {
				s.Pass(check.name + " optional (not installed)")
			}
			continue
		}

		version := strings.TrimSpace(firstLine(commandVersion(path, check.versionArgs...)))
		assertionName := check.name + " available"
		if version != "" {
			assertionName += " (" + version + ")"
		}
		s.Pass(assertionName)

		if check.name == "go" {
			s.Assert("go >= 1.24", goVersionAtLeast(version, 1, 24), "upgrade Go to 1.24+")
		}
		if check.name == "docker" {
			composeVersion := strings.TrimSpace(firstLine(commandVersion(path, "compose", "version")))
			s.Assert("docker compose plugin available", composeVersion != "", "docker compose version failed")
		}
	}

	if p.PlatformProfile() == "macos" {
		s.Pass("nvidia-smi optional on macOS")
	} else {
		path, err := exec.LookPath("nvidia-smi")
		s.Assert("nvidia-smi on PATH", err == nil, "nvidia-smi not found")
		if err == nil {
			version := strings.TrimSpace(firstLine(commandVersion(path, "--version")))
			assertionName := "nvidia-smi responds"
			if version != "" {
				assertionName += " (" + version + ")"
			}
			s.Pass(assertionName)
		}
	}

	return s
}

func commandVersion(name string, args ...string) string {
	output, err := runCombinedOutput(5*time.Second, name, args...)
	if err != nil {
		return ""
	}
	return string(output)
}

func firstLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func goVersionAtLeast(version string, major, minor int) bool {
	matches := regexp.MustCompile(`go(\d+)\.(\d+)`).FindStringSubmatch(version)
	if len(matches) != 3 {
		return false
	}
	var gotMajor, gotMinor int
	_, err := fmt.Sscanf(matches[1]+"."+matches[2], "%d.%d", &gotMajor, &gotMinor)
	if err != nil {
		return false
	}
	if gotMajor != major {
		return gotMajor > major
	}
	return gotMinor >= minor
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

// suiteCoordinationSignals checks that the Mem0-based cross-machine coordination
// infrastructure is in place: the skill documents mention cursor-coordination,
// the signal CLI subcommand is built, and the daily prompt references signals.
func suiteCoordinationSignals(p config.Paths) *Suite {
	s := &Suite{Name: "Coordination Signals"}

	memorySkill := filepath.Join(p.SkillsDir, "memory-system", "SKILL.md")
	s.AssertFileExists("memory-system skill exists", memorySkill)
	s.AssertFileContains("memory-system mentions coordination", memorySkill, "cursor-coordination")

	syncProtocol := filepath.Join(p.SOPDir(), "multi-cursor-sync-protocol.md")
	s.AssertFileExists("multi-cursor-sync-protocol exists", syncProtocol)
	s.AssertFileContains("sync protocol mentions Mem0 coordination", syncProtocol, "cursor-coordination")

	dailyPrompt := filepath.Join(p.GlobalMemoriesDir(), "daily-startup-prompt.md")
	s.AssertFileContains("daily prompt references signal list", dailyPrompt, "signal list")

	signalBin := filepath.Join(p.BinDir, "cursor-tools")
	s.AssertFileExists("cursor-tools binary exists", signalBin)

	// Live check: run signal list and report pending tasks
	pendingCount, listErr := probeCoordinationSignals(signalBin)
	s.Assert("signal list reachable", listErr == nil,
		fmt.Sprintf("cursor-tools signal list: %v", listErr))
	if listErr == nil && pendingCount > 0 {
		s.Assert("no pending cross-machine tasks", false,
			fmt.Sprintf("%d pending task(s) -- run: cursor-tools signal list", pendingCount))
	}

	return s
}

// suiteAgentStackHealth shells out to agent-doctor all --json if the binary exists
// and merges results into the cursor-tools health report.
func suiteAgentStackHealth(p config.Paths) *Suite {
	s := &Suite{Name: "Agent Stack Health"}

	agentDoctorBin := filepath.Join(p.Home, "ai-agent-business-stack", "go", "bin", "agent-doctor")
	if _, err := os.Stat(agentDoctorBin); err != nil {
		altPath, lookErr := exec.LookPath("agent-doctor")
		if lookErr != nil {
			s.Pass("agent-doctor not installed (optional — build with: cd ~/ai-agent-business-stack/go && go build -o bin/agent-doctor ./cmd/agent-doctor/)")
			return s
		}
		agentDoctorBin = altPath
	}
	s.Pass("agent-doctor binary found: " + agentDoctorBin)

	out, err := runCombinedOutput(20*time.Second, agentDoctorBin, "all", "--json")
	if err != nil && len(out) == 0 {
		s.Fail("agent-doctor all --json executes", "error: "+err.Error())
		return s
	}
	s.Pass("agent-doctor all --json executes")

	var report struct {
		Suites []struct {
			Name   string `json:"name"`
			Checks []struct {
				Name    string `json:"name"`
				Status  string `json:"status"`
				Message string `json:"message"`
			} `json:"checks"`
		} `json:"suites"`
		Overall string `json:"overall"`
	}
	if jsonErr := json.Unmarshal(out, &report); jsonErr != nil {
		s.Fail("agent-doctor JSON parses", jsonErr.Error())
		return s
	}
	s.Pass("agent-doctor JSON parses")

	for _, suite := range report.Suites {
		for _, check := range suite.Checks {
			passed := check.Status == "pass" || check.Status == "ok"
			detail := check.Message
			if !passed && detail == "" {
				detail = "status: " + check.Status
			}
			s.Assert(suite.Name+"/"+check.Name, passed, detail)
		}
	}

	s.Assert("agent stack overall healthy", report.Overall == "healthy",
		"overall: "+report.Overall)

	return s
}

// probeCoordinationSignals runs cursor-tools signal list and counts pending tasks.
// Returns -1 on error.
func probeCoordinationSignals(binPath string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, "signal", "list", "--json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback: signal list without --json (counts "pending" lines)
		cmd2 := exec.CommandContext(ctx, binPath, "signal", "list")
		out2, err2 := cmd2.CombinedOutput()
		if err2 != nil {
			return -1, fmt.Errorf("signal list failed: %w", err2)
		}
		count := 0
		for _, line := range strings.Split(string(out2), "\n") {
			if strings.Contains(line, "task-dispatch") {
				count++
			}
		}
		return count, nil
	}
	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "task-dispatch") {
			count++
		}
	}
	return count, nil
}

// suitePrePushReadiness checks that formatting tools are configured for
// Zendesk repos and that the pre-push skill covers TypeScript repos.
func suitePrePushReadiness(p config.Paths) *Suite {
	s := &Suite{Name: "Pre-Push Readiness"}

	prePushSkill := filepath.Join(p.SkillsDir, "pre-push", "SKILL.md")
	if _, err := os.Stat(prePushSkill); err == nil {
		data, _ := os.ReadFile(prePushSkill)
		content := string(data)
		s.Assert("pre-push skill covers prettier", strings.Contains(content, "prettier"), "pre-push SKILL.md missing prettier step for TS repos")
		s.Assert("pre-push skill covers gofmt", strings.Contains(content, "gofmt"), "pre-push SKILL.md missing gofmt step for Go repos")
	} else {
		s.Pass("pre-push skill (workspace-specific, not in global path)")
	}

	evidenceRule := filepath.Join(p.RulesDir, "evidence-based-development.mdc")
	if _, err := os.Stat(evidenceRule); err == nil {
		data, _ := os.ReadFile(evidenceRule)
		content := string(data)
		s.Assert("evidence rule has CI verification", strings.Contains(content, "CI Status Claims"), "evidence-based-development.mdc missing CI Status Claims section")
		s.Assert("evidence rule has rebase strategy", strings.Contains(content, "Monorepo Rebase Strategy"), "evidence-based-development.mdc missing Monorepo Rebase Strategy section")
	} else {
		s.Pass("evidence-based-development rule (workspace-specific)")
	}

	prDiscipline := filepath.Join(p.RulesDir, "zendesk-pr-discipline.mdc")
	if _, err := os.Stat(prDiscipline); err == nil {
		data, _ := os.ReadFile(prDiscipline)
		content := string(data)
		s.Assert("PR discipline has pre-push formatting", strings.Contains(content, "Pre-Push Formatting"), "zendesk-pr-discipline.mdc missing Pre-Push Formatting section")
	} else {
		s.Pass("PR discipline rule (workspace-specific)")
	}

	return s
}
