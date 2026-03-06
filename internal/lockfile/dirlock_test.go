package lockfile_test

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
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

	Describe("NewDirLock", func() {
		It("creates a lock with default stale timeout", func() {
			lock := lockfile.NewDirLock(filepath.Join(tmpDir, ".test.lock"))
			Expect(lock).NotTo(BeNil())
		})
	})

	Describe("WithStaleTimeout", func() {
		It("returns the same lock for chaining", func() {
			lock := lockfile.NewDirLock(filepath.Join(tmpDir, ".test.lock"))
			lock2 := lock.WithStaleTimeout(10 * time.Second)
			Expect(lock2).To(BeIdenticalTo(lock))
		})
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

		It("writes current PID to the PID file", func() {
			lockPath := filepath.Join(tmpDir, ".test.lock")
			lock := lockfile.NewDirLock(lockPath)
			Expect(lock.Acquire()).To(Succeed())
			defer lock.Release()

			data, err := os.ReadFile(filepath.Join(lockPath, "pid"))
			Expect(err).NotTo(HaveOccurred())
			pid, err := strconv.Atoi(string(data))
			Expect(err).NotTo(HaveOccurred())
			Expect(pid).To(Equal(os.Getpid()))
		})

		It("fails when lock directory exists without PID file and is not stale", func() {
			lockPath := filepath.Join(tmpDir, ".test.lock")
			Expect(os.Mkdir(lockPath, 0o755)).To(Succeed())
			lock := lockfile.NewDirLock(lockPath).WithStaleTimeout(999 * time.Hour)
			err := lock.Acquire()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("lock held"))
		})

		It("reclaims stale lock", func() {
			lockPath := filepath.Join(tmpDir, ".test.lock")
			Expect(os.Mkdir(lockPath, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(lockPath, "pid"), []byte("999999"), 0o644)).To(Succeed())

			lock := lockfile.NewDirLock(lockPath).WithStaleTimeout(0 * time.Second)
			Expect(lock.Acquire()).To(Succeed())
			defer lock.Release()
		})

		It("reclaims lock held by dead process", func() {
			lockPath := filepath.Join(tmpDir, ".test.lock")
			Expect(os.Mkdir(lockPath, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(lockPath, "pid"), []byte("99999999"), 0o644)).To(Succeed())

			lock := lockfile.NewDirLock(lockPath).WithStaleTimeout(999 * time.Hour)
			Expect(lock.Acquire()).To(Succeed())
			defer lock.Release()
		})

		It("does not reclaim lock with invalid PID content", func() {
			lockPath := filepath.Join(tmpDir, ".test.lock")
			Expect(os.Mkdir(lockPath, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(lockPath, "pid"), []byte("not-a-number"), 0o644)).To(Succeed())

			lock := lockfile.NewDirLock(lockPath).WithStaleTimeout(999 * time.Hour)
			err := lock.Acquire()
			Expect(err).To(HaveOccurred())
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

		It("is safe to call on non-existent lock", func() {
			lockPath := filepath.Join(tmpDir, ".never-acquired.lock")
			lock := lockfile.NewDirLock(lockPath)
			Expect(func() { lock.Release() }).NotTo(Panic())
		})

		It("can acquire after release", func() {
			lockPath := filepath.Join(tmpDir, ".test.lock")
			lock := lockfile.NewDirLock(lockPath)
			Expect(lock.Acquire()).To(Succeed())
			lock.Release()
			Expect(lock.Acquire()).To(Succeed())
			lock.Release()
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

	Describe("Lock and Unlock", func() {
		It("acquires and releases flock", func() {
			lockPath := filepath.Join(tmpDir, ".test.flock")
			fl := lockfile.NewFileLock(lockPath)
			Expect(fl.Lock()).To(Succeed())
			fl.Unlock()
			Expect(lockPath).To(BeAnExistingFile())
		})

		It("creates parent directories for lock file", func() {
			lockPath := filepath.Join(tmpDir, "nested", "dir", ".test.flock")
			fl := lockfile.NewFileLock(lockPath)
			Expect(fl.Lock()).To(Succeed())
			fl.Unlock()
		})

		It("is safe to unlock without locking", func() {
			fl := lockfile.NewFileLock(filepath.Join(tmpDir, ".unused.flock"))
			Expect(func() { fl.Unlock() }).NotTo(Panic())
		})
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

		It("overwrites existing file content", func() {
			lockPath := filepath.Join(tmpDir, ".write.lock")
			targetPath := filepath.Join(tmpDir, "overwrite.txt")
			Expect(lockfile.LockedWrite(lockPath, targetPath, "first")).To(Succeed())
			Expect(lockfile.LockedWrite(lockPath, targetPath, "second")).To(Succeed())

			data, err := os.ReadFile(targetPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("second"))
		})

		It("handles concurrent writes safely", func() {
			lockPath := filepath.Join(tmpDir, ".concurrent.lock")
			targetPath := filepath.Join(tmpDir, "concurrent.txt")

			var wg sync.WaitGroup
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func(n int) {
					defer wg.Done()
					_ = lockfile.LockedWrite(lockPath, targetPath, "writer-"+strconv.Itoa(n))
				}(i)
			}
			wg.Wait()

			data, err := os.ReadFile(targetPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(HavePrefix("writer-"))
		})

		It("returns error when lock parent dir is unwritable", func() {
			readOnlyDir := filepath.Join(tmpDir, "readonly")
			Expect(os.Mkdir(readOnlyDir, 0o444)).To(Succeed())
			defer os.Chmod(readOnlyDir, 0o755)

			lockPath := filepath.Join(readOnlyDir, "sub", ".lock")
			targetPath := filepath.Join(tmpDir, "target.txt")
			err := lockfile.LockedWrite(lockPath, targetPath, "content")
			Expect(err).To(HaveOccurred())
		})

		It("returns error when target parent dir is unwritable", func() {
			readOnlyDir := filepath.Join(tmpDir, "readonly-target")
			Expect(os.MkdirAll(readOnlyDir, 0o444)).To(Succeed())
			defer os.Chmod(readOnlyDir, 0o755)

			lockPath := filepath.Join(tmpDir, ".write.lock")
			targetPath := filepath.Join(readOnlyDir, "sub", "target.txt")
			err := lockfile.LockedWrite(lockPath, targetPath, "content")
			Expect(err).To(HaveOccurred())
		})
	})
})
