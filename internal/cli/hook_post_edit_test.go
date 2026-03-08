package cli

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("classifyLearningsPath", func() {
	It("detects workspace .learnings/ edits", func() {
		action, ws := classifyLearningsPath("/Users/jason.lian/agentic-ai-research/.learnings/ERRORS.md")
		Expect(action).To(Equal(learningsLocal))
		Expect(ws).To(Equal("/Users/jason.lian/agentic-ai-research"))
	})

	It("detects global learnings via ~/memo/ symlink path", func() {
		action, _ := classifyLearningsPath("/Users/jason.lian/memo/learnings/PATTERNS.md")
		Expect(action).To(Equal(learningsGlobal))
	})

	It("detects global learnings via ~/Code/global-kb/ real path", func() {
		action, _ := classifyLearningsPath("/Users/jason.lian/Code/global-kb/learnings/PATTERNS.md")
		Expect(action).To(Equal(learningsGlobal))
	})

	It("detects global learnings via ~/Code/global-kb/ episodes", func() {
		action, _ := classifyLearningsPath("/home/user/Code/global-kb/learnings/episodes/2026-03-07-test.md")
		Expect(action).To(Equal(learningsGlobal))
	})

	It("returns learningsNone for unrelated files", func() {
		action, _ := classifyLearningsPath("/Users/jason.lian/Code/global-kb/sop/engineering.md")
		Expect(action).To(Equal(learningsNone))
	})

	It("returns learningsNone for empty path", func() {
		action, _ := classifyLearningsPath("")
		Expect(action).To(Equal(learningsNone))
	})

	It("prefers workspace .learnings/ over global path", func() {
		action, ws := classifyLearningsPath("/home/user/projects/myapp/.learnings/LEARNINGS.md")
		Expect(action).To(Equal(learningsLocal))
		Expect(ws).To(Equal("/home/user/projects/myapp"))
	})
})
