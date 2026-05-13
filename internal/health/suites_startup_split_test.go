package health

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nfsarch33/cursor-tools/internal/config"
)

func TestCrossFileConsistencyUsesSplitStartupPromptSources(t *testing.T) {
	p := splitStartupFixture(t)

	writeFile(t, filepath.Join(p.GlobalMemoriesDir(), "daily-startup-prompt.md"), "# Daily Startup Prompt\n\nSee `daily-startup-static.md` for canonical procedures.\n")
	writeFile(t, filepath.Join(p.GlobalMemoriesDir(), "daily-startup-static.md"), strings.Join([]string{
		"# Daily Startup Prompt — Static Procedures",
		"Memory System Snapshot",
		"Skills: (2 unique skills, 10 L0 rules)",
		"Slash commands: 6 in `~/.cursor/commands/`",
		"hooks",
		"Sub-agents",
		"commands",
		"unified",
		"bootstrap",
		"Context Mode",
	}, "\n"))
	writeFile(t, filepath.Join(p.GlobalMemoriesDir(), "skills-index.md"), "2 unique skills\n.cursor/skills\n.agents/skills\n")

	suite := suiteCrossFileConsistency(p)

	assertSuiteResultPassed(t, suite, "daily prompt has skill count")
	assertSuiteResultPassed(t, suite, "daily prompt mentions hooks")
	assertSuiteResultPassed(t, suite, "daily prompt mentions sub-agents")
	assertSuiteResultPassed(t, suite, "daily prompt mentions slash commands")
	assertSuiteResultPassed(t, suite, "daily prompt mentions unified repo")
	assertSuiteResultPassed(t, suite, "daily prompt mentions bootstrap")
	assertSuiteResultPassed(t, suite, "daily prompt mentions Context Mode")
}

func TestResumeReadinessUsesSplitStartupPromptSources(t *testing.T) {
	p := splitStartupFixture(t)

	writeFile(t, filepath.Join(p.GlobalMemoriesDir(), "daily-startup-prompt.md"), "# Daily Startup Prompt\n\nSee `daily-startup-static.md` first.\n")
	writeFile(t, filepath.Join(p.GlobalMemoriesDir(), "daily-startup-static.md"), "# Daily Startup Prompt — Static Procedures\n")
	writeFile(t, filepath.Join(p.GlobalMemoriesDir(), "daily-startup-sot.md"), "Current source-of-truth notes:\n- session-handoff review is mandatory before resume.\n")
	writeFile(t, filepath.Join(p.GlobalMemoriesDir(), "session-handoff-2026-05-14-macos.md"), "## Summary\n")
	writeFile(t, filepath.Join(p.GlobalMemoriesDir(), "mcp-index-and-selection-sop.md"), "# MCP Index\n")

	suite := suiteResumeReadiness(p)

	assertSuiteResultPassed(t, suite, "daily prompt references session handoff")
	assertSuiteResultPassed(t, suite, "git status executes")
}

func TestCoordinationSignalsUsesSplitStartupPromptSources(t *testing.T) {
	p := splitStartupFixture(t)

	writeFile(t, filepath.Join(p.GlobalMemoriesDir(), "daily-startup-prompt.md"), "# Daily Startup Prompt\n\nSee `daily-startup-static.md` first.\n")
	writeFile(t, filepath.Join(p.GlobalMemoriesDir(), "daily-startup-static.md"), "Phase 0\n- run cursor-tools signal list before execution.\n")
	writeFile(t, filepath.Join(p.SkillsDir, "memory-system", "SKILL.md"), "cursor-coordination\n")
	writeFile(t, filepath.Join(p.SOPDir(), "multi-cursor-sync-protocol.md"), "cursor-coordination\n")
	writeExecutable(t, filepath.Join(p.BinDir, "cursor-tools"), "#!/bin/sh\nexit 0\n")

	suite := suiteCoordinationSignals(p)

	assertSuiteResultPassed(t, suite, "daily prompt references signal list")
	assertSuiteResultPassed(t, suite, "signal list reachable")
}

func splitStartupFixture(t *testing.T) config.Paths {
	t.Helper()

	base := t.TempDir()
	p := config.Paths{
		Home:            base,
		GlobalKB:        filepath.Join(base, "global-kb"),
		SkillsDir:       filepath.Join(base, ".cursor", "skills"),
		AgentsDir:       filepath.Join(base, ".claude", "agents"),
		AgentsSkillsDir: filepath.Join(base, ".agents", "skills"),
		CommandsDir:     filepath.Join(base, ".cursor", "commands"),
		RulesDir:        filepath.Join(base, ".cursor", "rules"),
		BinDir:          filepath.Join(base, "bin"),
	}

	for _, dir := range []string{
		p.GlobalMemoriesDir(),
		p.SOPDir(),
		p.SkillsDir,
		p.AgentsSkillsDir,
		p.AgentsDir,
		p.CommandsDir,
		p.RulesDir,
		p.BinDir,
		filepath.Join(p.SkillsDir, "memory-system"),
		filepath.Join(p.SkillsDir, "go"),
		filepath.Join(p.AgentsSkillsDir, "testing"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	writeFile(t, filepath.Join(p.SkillsDir, "go", "SKILL.md"), "# go\n")
	writeFile(t, filepath.Join(p.AgentsSkillsDir, "testing", "SKILL.md"), "# testing\n")

	for i := 1; i <= 6; i++ {
		writeFile(t, filepath.Join(p.AgentsDir, "agent-"+itoa(i)+".md"), "agent\n")
		writeFile(t, filepath.Join(p.CommandsDir, "command-"+itoa(i)+".md"), "command\n")
	}

	cmd := exec.Command("git", "init", p.GlobalKB)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v (%s)", p.GlobalKB, err, string(out))
	}

	return p
}

func writeExecutable(t *testing.T, path, body string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func assertSuiteResultPassed(t *testing.T, suite *Suite, name string) {
	t.Helper()

	for _, result := range suite.Results {
		if result.Name != name {
			continue
		}
		if !result.Passed {
			t.Fatalf("%s failed: %s", name, result.Detail)
		}
		return
	}
	t.Fatalf("result %q not found", name)
}
