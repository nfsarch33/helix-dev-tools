// runx-public-repo-gate: allow-file personal_path_id,fleet_host_alias — tests assert detection of literal personal-stack identifiers (gate test fixtures)

package cli

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("classifyLearningsPath", func() {
	It("detects workspace .learnings/ edits", func() {
		action, ws := classifyLearningsPath("/home/testuser/agentic-ai-research/.learnings/ERRORS.md")
		Expect(action).To(Equal(learningsLocal))
		Expect(ws).To(Equal("/home/testuser/agentic-ai-research"))
	})

	It("does not treat the retired ~/memo path as canonical learnings", func() {
		action, _ := classifyLearningsPath("/home/testuser/memo/learnings/PATTERNS.md")
		Expect(action).To(Equal(learningsNone))
	})

	It("detects global learnings via ~/Code/global-kb/ real path", func() {
		action, _ := classifyLearningsPath("/home/testuser/Code/global-kb/learnings/PATTERNS.md")
		Expect(action).To(Equal(learningsGlobal))
	})

	It("detects global learnings via ~/Code/global-kb/ episodes", func() {
		action, _ := classifyLearningsPath("/home/user/Code/global-kb/learnings/episodes/2026-03-07-test.md")
		Expect(action).To(Equal(learningsGlobal))
	})

	It("returns learningsNone for unrelated files", func() {
		action, _ := classifyLearningsPath("/home/testuser/Code/global-kb/sop/engineering.md")
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
