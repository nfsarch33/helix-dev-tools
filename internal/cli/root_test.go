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
})
