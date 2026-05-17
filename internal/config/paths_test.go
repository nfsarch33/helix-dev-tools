package config_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/helix-dev-tools/internal/config"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

var _ = Describe("Paths", func() {
	Describe("DefaultPaths", func() {
		It("returns paths based on HOME", func() {
			p := config.DefaultPaths()
			home := os.Getenv("HOME")
			Expect(p.Home).To(Equal(home))
			Expect(p.GlobalKB).To(ContainSubstring("Code/global-kb"))
			Expect(p.Memo).To(Equal(p.GlobalKB))
			Expect(p.HooksDir).To(ContainSubstring(".cursor/hooks"))
			Expect(p.SkillsDir).To(ContainSubstring(".cursor/skills"))
		})

		It("respects GLOBAL_KB env override", func() {
			os.Setenv("GLOBAL_KB", "/custom/path")
			defer os.Unsetenv("GLOBAL_KB")

			p := config.DefaultPaths()
			Expect(p.GlobalKB).To(Equal("/custom/path"))
		})

		It("ignores MEMO env override and keeps global-kb as the durable root", func() {
			os.Setenv("MEMO", "/custom/memo")
			defer os.Unsetenv("MEMO")

			p := config.DefaultPaths()
			Expect(p.Memo).To(Equal(p.GlobalKB))
		})

		It("populates all directory fields", func() {
			p := config.DefaultPaths()
			Expect(p.AgentsDir).To(ContainSubstring(".claude/agents"))
			Expect(p.AgentsSkillsDir).To(ContainSubstring(".agents/skills"))
			Expect(p.CommandsDir).To(ContainSubstring(".cursor/commands"))
			Expect(p.RulesDir).To(ContainSubstring(".cursor/rules"))
			Expect(p.BinDir).To(ContainSubstring("bin"))
		})

		It("falls back to tilde when HOME is empty", func() {
			orig := os.Getenv("HOME")
			os.Unsetenv("HOME")
			defer os.Setenv("HOME", orig)

			p := config.DefaultPaths()
			Expect(p.Home).To(Equal("~"))
		})
	})

	Describe("derived paths", func() {
		It("computes CursorConfigDir correctly", func() {
			p := config.DefaultPaths()
			Expect(p.CursorConfigDir()).To(ContainSubstring("cursor-config"))
			Expect(p.CursorConfigDir()).To(Equal(filepath.Join(p.GlobalKB, "cursor-config")))
		})

		It("computes GlobalMemoriesDir correctly", func() {
			p := config.DefaultPaths()
			Expect(p.GlobalMemoriesDir()).To(ContainSubstring("global-memories"))
			Expect(p.GlobalMemoriesDir()).To(Equal(filepath.Join(p.GlobalKB, "global-memories")))
		})

		It("computes GlobalLearningsDir correctly", func() {
			p := config.DefaultPaths()
			Expect(p.GlobalLearningsDir()).To(ContainSubstring("learnings"))
			Expect(p.GlobalLearningsDir()).To(Equal(filepath.Join(p.GlobalKB, "learnings")))
		})

		It("computes SOPDir correctly", func() {
			p := config.DefaultPaths()
			Expect(p.SOPDir()).To(ContainSubstring("sop"))
			Expect(p.SOPDir()).To(Equal(filepath.Join(p.GlobalKB, "sop")))
		})

		It("computes LogFile correctly", func() {
			p := config.DefaultPaths()
			Expect(p.LogFile("guard-shell")).To(ContainSubstring("guard-shell.log"))
			Expect(p.LogFile("mcp-audit")).To(HaveSuffix("mcp-audit.log"))
		})

		It("computes CursorMCPConfig correctly", func() {
			p := config.DefaultPaths()
			Expect(p.CursorMCPConfig()).To(HaveSuffix(".cursor/mcp.json"))
		})

		It("computes HooksJSONPath correctly", func() {
			p := config.DefaultPaths()
			Expect(p.HooksJSONPath()).To(HaveSuffix(".cursor/hooks.json"))
		})

		It("computes SkillsCursorDir correctly", func() {
			p := config.DefaultPaths()
			Expect(p.SkillsCursorDir()).To(HaveSuffix(".cursor/skills-cursor"))
		})

		It("computes LockDir correctly", func() {
			p := config.DefaultPaths()
			lockDir := p.LockDir("housekeeping")
			Expect(lockDir).To(ContainSubstring(".housekeeping.lock"))
			Expect(lockDir).To(Equal(filepath.Join(p.HooksDir, ".housekeeping.lock")))
		})

		It("computes LockFile correctly", func() {
			p := config.DefaultPaths()
			lockFile := p.LockFile("promote")
			Expect(lockFile).To(ContainSubstring(".promote.lock"))
			Expect(lockFile).To(Equal(filepath.Join(p.HooksDir, ".promote.lock")))
		})

		It("keeps global-kb derived paths even when MEMO is set", func() {
			os.Setenv("MEMO", "/tmp/test-memo")
			defer os.Unsetenv("MEMO")

			p := config.DefaultPaths()
			Expect(p.Memo).To(Equal(p.GlobalKB))
			Expect(p.GlobalMemoriesDir()).To(Equal(filepath.Join(p.GlobalKB, "global-memories")))
			Expect(p.GlobalLearningsDir()).To(Equal(filepath.Join(p.GlobalKB, "learnings")))
		})

		It("returns a recognised platform profile", func() {
			p := config.DefaultPaths()
			Expect([]string{"macos", "wsl", "linux"}).To(ContainElement(p.PlatformProfile()))
		})
	})
})
