package learnings_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/cursor-tools/internal/learnings"
)

func TestLearnings(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Learnings Suite")
}

var _ = Describe("ParsePatterns", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "learnings-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("parses a valid PATTERNS.md table", func() {
		content := `# Global Semantic Patterns

| ID | Pattern | Confidence | Applications | Category | Created | Projects |
|----|---------|------------|--------------|----------|---------|---------|
| pat-001 | Always run skillvet before installing | 1.0 | 5+ | security | 2026-03-05 | global |
| pat-002 | Adapt concepts from external skills | 0.95 | 3 | security | 2026-03-05 | global |
`
		f := filepath.Join(tmpDir, "PATTERNS.md")
		Expect(os.WriteFile(f, []byte(content), 0o644)).To(Succeed())

		pats, err := learnings.ParsePatterns(f)
		Expect(err).NotTo(HaveOccurred())
		Expect(pats).To(HaveLen(2))
		Expect(pats["pat-001"].Confidence).To(Equal(1.0))
		Expect(pats["pat-001"].Applications).To(Equal(5))
		Expect(pats["pat-001"].ApplicationsRaw).To(Equal("5+"))
		Expect(pats["pat-001"].Category).To(Equal("security"))
		Expect(pats["pat-002"].Applications).To(Equal(3))
	})

	It("returns empty map for non-existent file", func() {
		pats, err := learnings.ParsePatterns("/nonexistent/PATTERNS.md")
		Expect(err).NotTo(HaveOccurred())
		Expect(pats).To(BeEmpty())
	})
})

var _ = Describe("ParseEntries", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "entries-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("parses entry sections", func() {
		content := `# Errors

## [2026-03-05] Category: sync
Description of the error.

## [2026-03-06] Category: hooks
Another error description.
`
		f := filepath.Join(tmpDir, "ERRORS.md")
		Expect(os.WriteFile(f, []byte(content), 0o644)).To(Succeed())

		entries, err := learnings.ParseEntries(f)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(HaveLen(2))
		Expect(entries[0].Date).To(Equal("2026-03-05"))
		Expect(entries[0].Category).To(Equal("sync"))
		Expect(entries[1].Category).To(Equal("hooks"))
	})

	It("returns nil for non-existent file", func() {
		entries, err := learnings.ParseEntries("/nonexistent/ERRORS.md")
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeNil())
	})
})

var _ = Describe("GenerateL1Digest", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "digest-test-*")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.MkdirAll(filepath.Join(tmpDir, "learnings", "episodes"), 0o755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(tmpDir, "pepper"), 0o755)).To(Succeed())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("generates digest from qualifying patterns", func() {
		content := `# Patterns
| ID | Pattern | Confidence | Applications | Category | Created | Projects |
|----|---------|------------|--------------|----------|---------|---------|
| pat-001 | Test pattern one | 1.0 | 5+ | security | 2026-03-05 | global |
| pat-002 | Test pattern two | 0.8 | 2 | testing | 2026-03-05 | global |
| pat-003 | Test pattern three | 0.9 | 4 | arch | 2026-03-05 | global |
`
		Expect(os.WriteFile(filepath.Join(tmpDir, "learnings", "PATTERNS.md"), []byte(content), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(tmpDir, "learnings", "episodes", "2026-03-05-test.md"), []byte("# Test"), 0o644)).To(Succeed())

		count := learnings.GenerateL1Digest(
			filepath.Join(tmpDir, "learnings"),
			filepath.Join(tmpDir, "pepper"),
			false,
		)
		Expect(count).To(Equal(2)) // pat-001 (5+) and pat-003 (4) qualify with >= 3

		digestPath := filepath.Join(tmpDir, "pepper", "learnings-digest.md")
		Expect(digestPath).To(BeAnExistingFile())
		data, _ := os.ReadFile(digestPath)
		Expect(string(data)).To(ContainSubstring("pat-001"))
		Expect(string(data)).To(ContainSubstring("pat-003"))
		Expect(string(data)).NotTo(ContainSubstring("pat-002"))
		Expect(string(data)).To(ContainSubstring("Recent Episodes"))
	})

	It("returns count only in dry-run mode", func() {
		content := `# Patterns
| ID | Pattern | Confidence | Applications | Category | Created | Projects |
|----|---------|------------|--------------|----------|---------|---------|
| pat-001 | Test | 1.0 | 5+ | security | 2026-03-05 | global |
`
		Expect(os.WriteFile(filepath.Join(tmpDir, "learnings", "PATTERNS.md"), []byte(content), 0o644)).To(Succeed())

		count := learnings.GenerateL1Digest(
			filepath.Join(tmpDir, "learnings"),
			filepath.Join(tmpDir, "pepper"),
			true,
		)
		Expect(count).To(Equal(1))
		digestPath := filepath.Join(tmpDir, "pepper", "learnings-digest.md")
		Expect(digestPath).NotTo(BeAnExistingFile())
	})
})

var _ = Describe("GenerateL2SOP", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "sop-test-*")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.MkdirAll(filepath.Join(tmpDir, "learnings"), 0o755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(tmpDir, "sop"), 0o755)).To(Succeed())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("generates SOP from patterns with 5+ apps", func() {
		content := `# Patterns
| ID | Pattern | Confidence | Applications | Category | Created | Projects |
|----|---------|------------|--------------|----------|---------|---------|
| pat-001 | Test pattern one | 1.0 | 5+ | security | 2026-03-05 | global |
| pat-002 | Test pattern two | 0.8 | 3 | testing | 2026-03-05 | global |
`
		Expect(os.WriteFile(filepath.Join(tmpDir, "learnings", "PATTERNS.md"), []byte(content), 0o644)).To(Succeed())

		count := learnings.GenerateL2SOP(
			filepath.Join(tmpDir, "learnings"),
			filepath.Join(tmpDir, "sop"),
			false,
		)
		Expect(count).To(Equal(1)) // Only pat-001 qualifies with 5+

		sopPath := filepath.Join(tmpDir, "sop", "learned-patterns.md")
		Expect(sopPath).To(BeAnExistingFile())
		data, _ := os.ReadFile(sopPath)
		Expect(string(data)).To(ContainSubstring("pat-001"))
		Expect(string(data)).NotTo(ContainSubstring("pat-002"))
	})
})

var _ = Describe("PromoteWorkspace", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "workspace-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("returns zero counts when no .learnings dir exists", func() {
		result := learnings.PromoteWorkspace(tmpDir, filepath.Join(tmpDir, "global"), false)
		Expect(result.Entries).To(Equal(0))
		Expect(result.Patterns).To(Equal(0))
		Expect(result.Episodes).To(Equal(0))
	})

	It("promotes episodes from project to global", func() {
		projectEp := filepath.Join(tmpDir, "project", ".learnings", "episodes")
		globalEp := filepath.Join(tmpDir, "global", "episodes")
		Expect(os.MkdirAll(projectEp, 0o755)).To(Succeed())
		Expect(os.MkdirAll(globalEp, 0o755)).To(Succeed())

		Expect(os.WriteFile(filepath.Join(projectEp, "2026-03-07-test.md"), []byte("# Test Episode"), 0o644)).To(Succeed())

		result := learnings.PromoteWorkspace(filepath.Join(tmpDir, "project"), filepath.Join(tmpDir, "global"), false)
		Expect(result.Episodes).To(Equal(1))
		Expect(filepath.Join(globalEp, "2026-03-07-test.md")).To(BeAnExistingFile())
	})
})
