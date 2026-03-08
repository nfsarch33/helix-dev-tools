package cli

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("syncRepoMemories", func() {
	var srcDir, dstDir string

	BeforeEach(func() {
		srcDir = GinkgoT().TempDir()
		dstDir = GinkgoT().TempDir()
	})

	It("copies new .md files from src to dst", func() {
		Expect(os.WriteFile(filepath.Join(srcDir, "readme.md"), []byte("# Hello"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(srcDir, "notes.md"), []byte("# Notes"), 0o644)).To(Succeed())

		c := syncRepoMemories(srcDir, dstDir)
		Expect(c.added).To(Equal(2))
		Expect(c.updated).To(Equal(0))
		Expect(c.skipped).To(Equal(0))

		data, err := os.ReadFile(filepath.Join(dstDir, "readme.md"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("# Hello"))
	})

	It("skips unchanged files", func() {
		content := []byte("same content")
		Expect(os.WriteFile(filepath.Join(srcDir, "same.md"), content, 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dstDir, "same.md"), content, 0o644)).To(Succeed())

		c := syncRepoMemories(srcDir, dstDir)
		Expect(c.skipped).To(Equal(1))
		Expect(c.added).To(Equal(0))
		Expect(c.updated).To(Equal(0))
	})

	It("updates changed files and creates backups", func() {
		Expect(os.WriteFile(filepath.Join(srcDir, "doc.md"), []byte("new content"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dstDir, "doc.md"), []byte("old content"), 0o644)).To(Succeed())

		c := syncRepoMemories(srcDir, dstDir)
		Expect(c.updated).To(Equal(1))

		data, err := os.ReadFile(filepath.Join(dstDir, "doc.md"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("new content"))

		entries, err := os.ReadDir(dstDir)
		Expect(err).NotTo(HaveOccurred())
		backupFound := false
		for _, e := range entries {
			if len(e.Name()) > len("doc.md.bak.") {
				backupFound = true
			}
		}
		Expect(backupFound).To(BeTrue(), "backup file should exist")
	})

	It("ignores non-.md files", func() {
		Expect(os.WriteFile(filepath.Join(srcDir, "script.sh"), []byte("#!/bin/bash"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(srcDir, "data.json"), []byte("{}"), 0o644)).To(Succeed())

		c := syncRepoMemories(srcDir, dstDir)
		Expect(c.added).To(Equal(0))
	})

	It("ignores directories", func() {
		Expect(os.MkdirAll(filepath.Join(srcDir, "subdir"), 0o755)).To(Succeed())
		c := syncRepoMemories(srcDir, dstDir)
		Expect(c.added).To(Equal(0))
	})

	It("handles empty source directory", func() {
		c := syncRepoMemories(srcDir, dstDir)
		Expect(c.added).To(Equal(0))
		Expect(c.updated).To(Equal(0))
		Expect(c.skipped).To(Equal(0))
	})

	It("handles missing source directory", func() {
		c := syncRepoMemories("/nonexistent/path", dstDir)
		Expect(c.added).To(Equal(0))
	})
})

var _ = Describe("isDir", func() {
	It("returns true for existing directories", func() {
		dir := GinkgoT().TempDir()
		Expect(isDir(dir)).To(BeTrue())
	})

	It("returns false for files", func() {
		dir := GinkgoT().TempDir()
		f := filepath.Join(dir, "file.txt")
		Expect(os.WriteFile(f, []byte("data"), 0o644)).To(Succeed())
		Expect(isDir(f)).To(BeFalse())
	})

	It("returns false for nonexistent paths", func() {
		Expect(isDir("/nonexistent/path/abc123")).To(BeFalse())
	})
})
