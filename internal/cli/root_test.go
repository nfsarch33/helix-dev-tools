package cli

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("root helpers", func() {
	It("updates the version string", func() {
		old := version
		defer func() { version = old }()
		SetVersion("v1.2.3")
		Expect(version).To(Equal("v1.2.3"))
	})

	It("truncates long strings for selftest output", func() {
		Expect(truncate("abcdef", 3)).To(Equal("abc..."))
		Expect(truncate("abc", 10)).To(Equal("abc"))
	})

	It("registers auto-update on the root command", func() {
		names := []string{}
		for _, cmd := range rootCmd.Commands() {
			names = append(names, cmd.Name())
		}
		Expect(names).To(ContainElement("auto-update"))
	})

	It("keeps the global-kb hook command surface registered", func() {
		rootNames := []string{}
		for _, cmd := range rootCmd.Commands() {
			rootNames = append(rootNames, cmd.Name())
		}

		hookNames := []string{}
		for _, cmd := range hookCmd.Commands() {
			hookNames = append(hookNames, cmd.Name())
		}

		gitHookNames := []string{}
		for _, cmd := range githookCmd.Commands() {
			gitHookNames = append(gitHookNames, cmd.Name())
		}

		Expect(rootNames).To(ContainElements("hook", "githook", "mcp-index", "doctor", "mem0-outbox", "docsync", "docs-check"))
		Expect(hookNames).To(ContainElements("guard-shell", "sanitize-read", "guard-mcp", "post-edit", "housekeeping"))
		Expect(gitHookNames).To(ContainElements("commit-msg", "pre-push"))
	})
})
