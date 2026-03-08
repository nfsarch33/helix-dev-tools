package cli

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("countSkillDirs", func() {
	var base string

	BeforeEach(func() {
		base = GinkgoT().TempDir()
	})

	It("counts directories that contain SKILL.md", func() {
		for _, name := range []string{"skill-a", "skill-b", "skill-c"} {
			dir := filepath.Join(base, name)
			Expect(os.MkdirAll(dir, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill"), 0o644)).To(Succeed())
		}
		Expect(countSkillDirs(base, nil)).To(Equal(3))
	})

	It("excludes dirs without SKILL.md", func() {
		Expect(os.MkdirAll(filepath.Join(base, "no-skill"), 0o755)).To(Succeed())
		Expect(countSkillDirs(base, nil)).To(Equal(0))
	})

	It("excludes hidden directories", func() {
		dir := filepath.Join(base, ".hidden")
		Expect(os.MkdirAll(dir, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill"), 0o644)).To(Succeed())
		Expect(countSkillDirs(base, nil)).To(Equal(0))
	})

	It("respects exclude map", func() {
		for _, name := range []string{"keep", "exclude-me"} {
			dir := filepath.Join(base, name)
			Expect(os.MkdirAll(dir, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# Skill"), 0o644)).To(Succeed())
		}
		Expect(countSkillDirs(base, map[string]bool{"exclude-me": true})).To(Equal(1))
	})

	It("returns 0 for nonexistent directory", func() {
		Expect(countSkillDirs("/nonexistent/path", nil)).To(Equal(0))
	})
})

var _ = Describe("countFiles", func() {
	var dir string

	BeforeEach(func() {
		dir = GinkgoT().TempDir()
	})

	It("counts files with the given extension", func() {
		for _, name := range []string{"a.md", "b.md", "c.txt", "d.md"} {
			Expect(os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644)).To(Succeed())
		}
		Expect(countFiles(dir, ".md")).To(Equal(3))
		Expect(countFiles(dir, ".txt")).To(Equal(1))
	})

	It("does not count directories", func() {
		Expect(os.MkdirAll(filepath.Join(dir, "subdir.md"), 0o755)).To(Succeed())
		Expect(countFiles(dir, ".md")).To(Equal(0))
	})

	It("returns 0 for empty directory", func() {
		Expect(countFiles(dir, ".md")).To(Equal(0))
	})

	It("returns 0 for nonexistent directory", func() {
		Expect(countFiles("/nonexistent/path", ".md")).To(Equal(0))
	})
})
