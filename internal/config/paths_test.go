package config_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/cursor-tools/internal/config"
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
			Expect(p.Memo).To(ContainSubstring("memo"))
			Expect(p.HooksDir).To(ContainSubstring(".cursor/hooks"))
			Expect(p.SkillsDir).To(ContainSubstring(".cursor/skills"))
		})

		It("respects GLOBAL_KB env override", func() {
			os.Setenv("GLOBAL_KB", "/custom/path")
			defer os.Unsetenv("GLOBAL_KB")

			p := config.DefaultPaths()
			Expect(p.GlobalKB).To(Equal("/custom/path"))
		})
	})

	Describe("derived paths", func() {
		It("computes CursorConfigDir correctly", func() {
			p := config.DefaultPaths()
			Expect(p.CursorConfigDir()).To(ContainSubstring("cursor-config"))
		})

		It("computes GlobalMemoriesDir correctly", func() {
			p := config.DefaultPaths()
			Expect(p.GlobalMemoriesDir()).To(ContainSubstring("global-memories"))
		})

		It("computes LogFile correctly", func() {
			p := config.DefaultPaths()
			Expect(p.LogFile("guard-shell")).To(ContainSubstring("guard-shell.log"))
		})
	})
})
