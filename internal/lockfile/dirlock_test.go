package lockfile_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/nfsarch33/cursor-tools/internal/lockfile"
)

func TestLockfile(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lockfile Suite")
}

var _ = Describe("DirLock", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "lockfile-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("Acquire", func() {
		It("creates the lock directory", func() {
			lockPath := filepath.Join(tmpDir, ".test.lock")
			lock := lockfile.NewDirLock(lockPath)
			Expect(lock.Acquire()).To(Succeed())
			defer lock.Release()

			Expect(lockPath).To(BeADirectory())
			Expect(filepath.Join(lockPath, "pid")).To(BeAnExistingFile())
		})

		It("fails when lock directory exists without PID file and is not stale", func() {
			lockPath := filepath.Join(tmpDir, ".test.lock")
			Expect(os.Mkdir(lockPath, 0o755)).To(Succeed())
			// No PID file, directory just created (not stale)
			lock := lockfile.NewDirLock(lockPath).WithStaleTimeout(999 * time.Hour)
			err := lock.Acquire()
			// mkdir fails because dir exists; no PID file so isHeldByDeadProcess
			// returns false; not stale so no reclaim; returns error
			Expect(err).To(HaveOccurred())
		})

		It("reclaims stale lock", func() {
			lockPath := filepath.Join(tmpDir, ".test.lock")
			Expect(os.Mkdir(lockPath, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(lockPath, "pid"), []byte("999999"), 0o644)).To(Succeed())

			lock := lockfile.NewDirLock(lockPath).WithStaleTimeout(0 * time.Second)
			Expect(lock.Acquire()).To(Succeed())
			defer lock.Release()
		})
	})

	Describe("Release", func() {
		It("removes lock directory", func() {
			lockPath := filepath.Join(tmpDir, ".test.lock")
			lock := lockfile.NewDirLock(lockPath)
			Expect(lock.Acquire()).To(Succeed())
			lock.Release()

			Expect(lockPath).NotTo(BeADirectory())
		})
	})
})

var _ = Describe("FileLock", func() {
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "filelock-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("LockedWrite", func() {
		It("writes file content under lock", func() {
			lockPath := filepath.Join(tmpDir, ".write.lock")
			targetPath := filepath.Join(tmpDir, "output.txt")
			Expect(lockfile.LockedWrite(lockPath, targetPath, "hello")).To(Succeed())

			data, err := os.ReadFile(targetPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("hello"))
		})

		It("creates parent directories for target", func() {
			lockPath := filepath.Join(tmpDir, ".write.lock")
			targetPath := filepath.Join(tmpDir, "sub", "dir", "output.txt")
			Expect(lockfile.LockedWrite(lockPath, targetPath, "nested")).To(Succeed())
			Expect(targetPath).To(BeAnExistingFile())
		})
	})
})
