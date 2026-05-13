package cli

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("bootstrap helpers", func() {
	Describe("runBootstrap", func() {
		It("succeeds in dry-run mode on a fresh temp home", func() {
			base := GinkgoT().TempDir()
			oldHome := os.Getenv("HOME")
			oldGlobalKB := os.Getenv("GLOBAL_KB")
			defer os.Setenv("HOME", oldHome)
			defer os.Setenv("GLOBAL_KB", oldGlobalKB)

			Expect(os.Setenv("HOME", base)).To(Succeed())
			Expect(os.Setenv("GLOBAL_KB", filepath.Join(base, "Code", "global-kb"))).To(Succeed())

			bootstrapDryRun = true
			defer func() { bootstrapDryRun = false }()

			Expect(runBootstrap(nil, nil)).To(Succeed())
		})

		It("does not recreate the retired ~/memo path", func() {
			base := GinkgoT().TempDir()
			oldHome := os.Getenv("HOME")
			oldGlobalKB := os.Getenv("GLOBAL_KB")
			defer os.Setenv("HOME", oldHome)
			defer os.Setenv("GLOBAL_KB", oldGlobalKB)

			Expect(os.Setenv("HOME", base)).To(Succeed())
			Expect(os.Setenv("GLOBAL_KB", filepath.Join(base, "Code", "global-kb"))).To(Succeed())

			Expect(runBootstrap(nil, nil)).To(Succeed())
			_, err := os.Lstat(filepath.Join(base, "memo"))
			Expect(os.IsNotExist(err)).To(BeTrue())
		})

		It("leaves an existing legacy ~/memo directory untouched", func() {
			base := GinkgoT().TempDir()
			oldHome := os.Getenv("HOME")
			oldGlobalKB := os.Getenv("GLOBAL_KB")
			defer os.Setenv("HOME", oldHome)
			defer os.Setenv("GLOBAL_KB", oldGlobalKB)

			Expect(os.Setenv("HOME", base)).To(Succeed())
			Expect(os.Setenv("GLOBAL_KB", filepath.Join(base, "Code", "global-kb"))).To(Succeed())

			legacyMemo := filepath.Join(base, "memo")
			Expect(os.MkdirAll(legacyMemo, 0o755)).To(Succeed())

			Expect(runBootstrap(nil, nil)).To(Succeed())

			info, err := os.Lstat(legacyMemo)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.IsDir()).To(BeTrue())
		})
	})

	Describe("safeSymlink", func() {
		It("creates a symlink when destination is missing", func() {
			base := GinkgoT().TempDir()
			target := filepath.Join(base, "target")
			link := filepath.Join(base, "link")
			Expect(os.MkdirAll(target, 0o755)).To(Succeed())

			safeSymlink(link, target, false)

			info, err := os.Lstat(link)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode() & os.ModeSymlink).NotTo(Equal(os.FileMode(0)))
			resolved, err := os.Readlink(link)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved).To(Equal(target))
		})

		It("backs up an existing directory before linking", func() {
			base := GinkgoT().TempDir()
			target := filepath.Join(base, "target")
			link := filepath.Join(base, "link")
			Expect(os.MkdirAll(target, 0o755)).To(Succeed())
			Expect(os.MkdirAll(link, 0o755)).To(Succeed())

			safeSymlink(link, target, false)

			Expect(filepath.Join(base, "link.bak")).To(BeADirectory())
			info, err := os.Lstat(link)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode() & os.ModeSymlink).NotTo(Equal(os.FileMode(0)))
		})

		It("does not modify the filesystem in dry-run mode", func() {
			base := GinkgoT().TempDir()
			target := filepath.Join(base, "target")
			link := filepath.Join(base, "link")
			Expect(os.MkdirAll(target, 0o755)).To(Succeed())

			safeSymlink(link, target, true)

			_, err := os.Lstat(link)
			Expect(os.IsNotExist(err)).To(BeTrue())
		})
	})

	Describe("chmodDir", func() {
		It("updates file modes for all direct entries", func() {
			base := GinkgoT().TempDir()
			f1 := filepath.Join(base, "a.sh")
			f2 := filepath.Join(base, "b.sh")
			Expect(os.WriteFile(f1, []byte("echo a"), 0o644)).To(Succeed())
			Expect(os.WriteFile(f2, []byte("echo b"), 0o644)).To(Succeed())

			chmodDir(base, 0o755)

			info1, err := os.Stat(f1)
			Expect(err).NotTo(HaveOccurred())
			info2, err := os.Stat(f2)
			Expect(err).NotTo(HaveOccurred())
			Expect(info1.Mode().Perm()).To(Equal(os.FileMode(0o755)))
			Expect(info2.Mode().Perm()).To(Equal(os.FileMode(0o755)))
		})
	})
})
