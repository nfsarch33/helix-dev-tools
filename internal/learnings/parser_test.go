package learnings_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/helix-dev-tools/internal/learnings"
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

	It("returns empty map for file with no pattern lines", func() {
		f := filepath.Join(tmpDir, "PATTERNS.md")
		Expect(os.WriteFile(f, []byte("# Just a header\nNo table here.\n"), 0o644)).To(Succeed())

		pats, err := learnings.ParsePatterns(f)
		Expect(err).NotTo(HaveOccurred())
		Expect(pats).To(BeEmpty())
	})

	It("handles patterns with non-numeric applications", func() {
		content := `| ID | Pattern | Confidence | Applications | Category | Created | Projects |
|----|---------|------------|--------------|----------|---------|---------|
| pat-001 | Test | 0.5 | many | testing | 2026-01-01 | global |
`
		f := filepath.Join(tmpDir, "PATTERNS.md")
		Expect(os.WriteFile(f, []byte(content), 0o644)).To(Succeed())

		pats, err := learnings.ParsePatterns(f)
		Expect(err).NotTo(HaveOccurred())
		Expect(pats).To(HaveLen(1))
		Expect(pats["pat-001"].Applications).To(Equal(0))
		Expect(pats["pat-001"].ApplicationsRaw).To(Equal("many"))
	})

	It("captures Description and RawLine", func() {
		content := `| pat-010 | After gem version bumps verify CI | 0.80 | 2 | ci | 2026-03-05 | zendesk |
`
		f := filepath.Join(tmpDir, "PATTERNS.md")
		Expect(os.WriteFile(f, []byte(content), 0o644)).To(Succeed())

		pats, err := learnings.ParsePatterns(f)
		Expect(err).NotTo(HaveOccurred())
		Expect(pats["pat-010"].Description).To(Equal("After gem version bumps verify CI"))
		Expect(pats["pat-010"].RawLine).To(ContainSubstring("pat-010"))
		Expect(pats["pat-010"].Created).To(Equal("2026-03-05"))
	})
})

var _ = Describe("FormatPatternLine", func() {
	It("formats a pattern as a markdown table row", func() {
		p := learnings.Pattern{
			ID:              "pat-001",
			Description:     "Test pattern",
			Confidence:      0.95,
			ApplicationsRaw: "5+",
			Category:        "security",
			Created:         "2026-03-05",
		}
		line := learnings.FormatPatternLine(p)
		Expect(line).To(ContainSubstring("pat-001"))
		Expect(line).To(ContainSubstring("Test pattern"))
		Expect(line).To(ContainSubstring("0.95"))
		Expect(line).To(ContainSubstring("5+"))
		Expect(line).To(ContainSubstring("security"))
		Expect(line).To(ContainSubstring("global"))
	})

	It("formats zero-confidence pattern", func() {
		p := learnings.Pattern{
			ID:              "pat-999",
			Description:     "Unvalidated",
			Confidence:      0.0,
			ApplicationsRaw: "0",
			Category:        "test",
			Created:         "2026-01-01",
		}
		line := learnings.FormatPatternLine(p)
		Expect(line).To(ContainSubstring("0.00"))
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

	It("returns empty for file with no entry sections", func() {
		f := filepath.Join(tmpDir, "EMPTY.md")
		Expect(os.WriteFile(f, []byte("# Just a title\nNo sections.\n"), 0o644)).To(Succeed())

		entries, err := learnings.ParseEntries(f)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	})

	It("sets fingerprint on entries", func() {
		content := `## [2026-03-05] Category: sync
Short description.
`
		f := filepath.Join(tmpDir, "TEST.md")
		Expect(os.WriteFile(f, []byte(content), 0o644)).To(Succeed())

		entries, err := learnings.ParseEntries(f)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(HaveLen(1))
		Expect(entries[0].Fingerprint).NotTo(BeEmpty())
		Expect(entries[0].Content).To(ContainSubstring("Short description"))
	})

	It("handles entries with non-matching section format", func() {
		content := `## [2026-03-05] Category: sync
Valid entry.

## No date or category
This section won't parse as an entry.
`
		f := filepath.Join(tmpDir, "MIXED.md")
		Expect(os.WriteFile(f, []byte(content), 0o644)).To(Succeed())

		entries, err := learnings.ParseEntries(f)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(HaveLen(1))
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
		Expect(count).To(Equal(2))

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

	It("returns 0 when no patterns qualify", func() {
		content := `# Patterns
| ID | Pattern | Confidence | Applications | Category | Created | Projects |
|----|---------|------------|--------------|----------|---------|---------|
| pat-001 | Low apps | 0.5 | 1 | testing | 2026-03-05 | global |
`
		Expect(os.WriteFile(filepath.Join(tmpDir, "learnings", "PATTERNS.md"), []byte(content), 0o644)).To(Succeed())

		count := learnings.GenerateL1Digest(
			filepath.Join(tmpDir, "learnings"),
			filepath.Join(tmpDir, "pepper"),
			false,
		)
		Expect(count).To(Equal(0))
	})

	It("returns 0 when PATTERNS.md is missing", func() {
		count := learnings.GenerateL1Digest(
			filepath.Join(tmpDir, "learnings"),
			filepath.Join(tmpDir, "pepper"),
			false,
		)
		Expect(count).To(Equal(0))
	})

	It("generates digest without episodes dir", func() {
		os.RemoveAll(filepath.Join(tmpDir, "learnings", "episodes"))

		content := `# Patterns
| ID | Pattern | Confidence | Applications | Category | Created | Projects |
|----|---------|------------|--------------|----------|---------|---------|
| pat-001 | Test | 1.0 | 5+ | security | 2026-03-05 | global |
`
		Expect(os.WriteFile(filepath.Join(tmpDir, "learnings", "PATTERNS.md"), []byte(content), 0o644)).To(Succeed())

		count := learnings.GenerateL1Digest(
			filepath.Join(tmpDir, "learnings"),
			filepath.Join(tmpDir, "pepper"),
			false,
		)
		Expect(count).To(Equal(1))
		data, _ := os.ReadFile(filepath.Join(tmpDir, "pepper", "learnings-digest.md"))
		Expect(string(data)).NotTo(ContainSubstring("Recent Episodes"))
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
		Expect(count).To(Equal(1))

		sopPath := filepath.Join(tmpDir, "sop", "learned-patterns.md")
		Expect(sopPath).To(BeAnExistingFile())
		data, _ := os.ReadFile(sopPath)
		Expect(string(data)).To(ContainSubstring("pat-001"))
		Expect(string(data)).NotTo(ContainSubstring("pat-002"))
	})

	It("returns 0 when no patterns qualify for SOP", func() {
		content := `| ID | Pattern | Confidence | Applications | Category | Created | Projects |
|----|---------|------------|--------------|----------|---------|---------|
| pat-001 | Low | 0.5 | 2 | testing | 2026-03-05 | global |
`
		Expect(os.WriteFile(filepath.Join(tmpDir, "learnings", "PATTERNS.md"), []byte(content), 0o644)).To(Succeed())

		count := learnings.GenerateL2SOP(
			filepath.Join(tmpDir, "learnings"),
			filepath.Join(tmpDir, "sop"),
			false,
		)
		Expect(count).To(Equal(0))
	})

	It("returns count in dry-run mode without writing", func() {
		content := `| ID | Pattern | Confidence | Applications | Category | Created | Projects |
|----|---------|------------|--------------|----------|---------|---------|
| pat-001 | Test | 1.0 | 7 | arch | 2026-03-05 | global |
`
		Expect(os.WriteFile(filepath.Join(tmpDir, "learnings", "PATTERNS.md"), []byte(content), 0o644)).To(Succeed())

		count := learnings.GenerateL2SOP(
			filepath.Join(tmpDir, "learnings"),
			filepath.Join(tmpDir, "sop"),
			true,
		)
		Expect(count).To(Equal(1))
		Expect(filepath.Join(tmpDir, "sop", "learned-patterns.md")).NotTo(BeAnExistingFile())
	})

	It("groups multiple patterns by category", func() {
		content := `| ID | Pattern | Confidence | Applications | Category | Created | Projects |
|----|---------|------------|--------------|----------|---------|---------|
| pat-001 | Sec one | 1.0 | 6 | security | 2026-03-05 | global |
| pat-002 | Sec two | 0.9 | 8 | security | 2026-03-05 | global |
| pat-003 | Arch one | 0.95 | 5+ | architecture | 2026-03-05 | global |
`
		Expect(os.WriteFile(filepath.Join(tmpDir, "learnings", "PATTERNS.md"), []byte(content), 0o644)).To(Succeed())

		count := learnings.GenerateL2SOP(
			filepath.Join(tmpDir, "learnings"),
			filepath.Join(tmpDir, "sop"),
			false,
		)
		Expect(count).To(Equal(3))

		data, _ := os.ReadFile(filepath.Join(tmpDir, "sop", "learned-patterns.md"))
		content = string(data)
		Expect(content).To(ContainSubstring("## Architecture"))
		Expect(content).To(ContainSubstring("## Security"))
	})

	It("returns 0 when PATTERNS.md is missing", func() {
		count := learnings.GenerateL2SOP(
			filepath.Join(tmpDir, "learnings"),
			filepath.Join(tmpDir, "sop"),
			false,
		)
		Expect(count).To(Equal(0))
	})
})

var _ = Describe("PromoteResults", func() {
	It("computes Total correctly", func() {
		r := learnings.PromoteResults{
			Promoted: learnings.WorkspaceResult{Entries: 2, Patterns: 1, Episodes: 3},
			L1Digest: 4,
			L2SOP:    2,
		}
		Expect(r.Total()).To(Equal(12))
	})

	It("returns 0 for empty results", func() {
		r := learnings.PromoteResults{}
		Expect(r.Total()).To(Equal(0))
	})

	It("generates summary string", func() {
		r := learnings.PromoteResults{
			Promoted: learnings.WorkspaceResult{Entries: 2, Patterns: 1},
			L1Digest: 5,
		}
		s := r.Summary()
		Expect(s).To(ContainSubstring("2 entries"))
		Expect(s).To(ContainSubstring("1 patterns"))
		Expect(s).To(ContainSubstring("L1 digest (5 patterns)"))
	})

	It("returns empty summary for zero results", func() {
		r := learnings.PromoteResults{}
		Expect(r.Summary()).To(BeEmpty())
	})

	It("includes L2 SOP in summary", func() {
		r := learnings.PromoteResults{L2SOP: 3}
		Expect(r.Summary()).To(ContainSubstring("L2 SOP (3 patterns)"))
	})

	It("includes episodes in summary", func() {
		r := learnings.PromoteResults{Promoted: learnings.WorkspaceResult{Episodes: 2}}
		Expect(r.Summary()).To(ContainSubstring("2 episodes"))
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

	It("skips episodes that already exist in global", func() {
		projectEp := filepath.Join(tmpDir, "project", ".learnings", "episodes")
		globalEp := filepath.Join(tmpDir, "global", "episodes")
		Expect(os.MkdirAll(projectEp, 0o755)).To(Succeed())
		Expect(os.MkdirAll(globalEp, 0o755)).To(Succeed())

		Expect(os.WriteFile(filepath.Join(projectEp, "2026-03-07-test.md"), []byte("# New"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(globalEp, "2026-03-07-test.md"), []byte("# Existing"), 0o644)).To(Succeed())

		result := learnings.PromoteWorkspace(filepath.Join(tmpDir, "project"), filepath.Join(tmpDir, "global"), false)
		Expect(result.Episodes).To(Equal(0))
	})

	It("skips non-md files in episodes", func() {
		projectEp := filepath.Join(tmpDir, "project", ".learnings", "episodes")
		globalEp := filepath.Join(tmpDir, "global", "episodes")
		Expect(os.MkdirAll(projectEp, 0o755)).To(Succeed())
		Expect(os.MkdirAll(globalEp, 0o755)).To(Succeed())

		Expect(os.WriteFile(filepath.Join(projectEp, "notes.txt"), []byte("text"), 0o644)).To(Succeed())

		result := learnings.PromoteWorkspace(filepath.Join(tmpDir, "project"), filepath.Join(tmpDir, "global"), false)
		Expect(result.Episodes).To(Equal(0))
	})

	It("counts episodes in dry-run without copying", func() {
		projectEp := filepath.Join(tmpDir, "project", ".learnings", "episodes")
		Expect(os.MkdirAll(projectEp, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(projectEp, "2026-03-07-test.md"), []byte("# Test"), 0o644)).To(Succeed())

		result := learnings.PromoteWorkspace(filepath.Join(tmpDir, "project"), filepath.Join(tmpDir, "global"), true)
		Expect(result.Episodes).To(Equal(1))
	})

	It("merges entries from project to global", func() {
		projectDir := filepath.Join(tmpDir, "project", ".learnings")
		globalDir := filepath.Join(tmpDir, "global")
		Expect(os.MkdirAll(projectDir, 0o755)).To(Succeed())
		Expect(os.MkdirAll(globalDir, 0o755)).To(Succeed())

		srcContent := "## [2026-03-07] Category: sync\nNew project error.\n"
		tgtContent := "# Errors\n\n## [2026-03-05] Category: hooks\nExisting error.\n"
		Expect(os.WriteFile(filepath.Join(projectDir, "ERRORS.md"), []byte(srcContent), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(globalDir, "ERRORS.md"), []byte(tgtContent), 0o644)).To(Succeed())

		result := learnings.PromoteWorkspace(filepath.Join(tmpDir, "project"), globalDir, false)
		Expect(result.Entries).To(Equal(1))

		data, _ := os.ReadFile(filepath.Join(globalDir, "ERRORS.md"))
		Expect(string(data)).To(ContainSubstring("2026-03-07"))
		Expect(string(data)).To(ContainSubstring("2026-03-05"))
	})

	It("merges patterns with updated application counts", func() {
		projectDir := filepath.Join(tmpDir, "project", ".learnings")
		globalDir := filepath.Join(tmpDir, "global")
		Expect(os.MkdirAll(projectDir, 0o755)).To(Succeed())
		Expect(os.MkdirAll(globalDir, 0o755)).To(Succeed())

		srcContent := `| ID | Pattern | Confidence | Applications | Category | Created | Projects |
|----|---------|------------|--------------|----------|---------|---------|
| pat-001 | Updated pattern | 1.0 | 10 | security | 2026-03-05 | global |
| pat-099 | Brand new | 0.9 | 1 | new | 2026-03-07 | global |
`
		tgtContent := `# Patterns

| ID | Pattern | Confidence | Applications | Category | Created | Projects |
|----|---------|------------|--------------|----------|---------|---------|
| pat-001 | Old pattern | 1.0 | 5+ | security | 2026-03-05 | global |
`
		Expect(os.WriteFile(filepath.Join(projectDir, "PATTERNS.md"), []byte(srcContent), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(globalDir, "PATTERNS.md"), []byte(tgtContent), 0o644)).To(Succeed())

		result := learnings.PromoteWorkspace(filepath.Join(tmpDir, "project"), globalDir, false)
		Expect(result.Patterns).To(Equal(2))
	})
})
