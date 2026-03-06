package lockfile

import (
	"os"
	"path/filepath"
	"syscall"
)

// FileLock provides flock-based locking for file writes.
type FileLock struct {
	path string
	file *os.File
}

// NewFileLock creates a file lock at the given path.
func NewFileLock(path string) *FileLock {
	return &FileLock{path: path}
}

// Lock acquires an exclusive flock on the lock file.
func (fl *FileLock) Lock() error {
	if err := os.MkdirAll(filepath.Dir(fl.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(fl.path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	fl.file = f
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// Unlock releases the flock and closes the file.
func (fl *FileLock) Unlock() {
	if fl.file != nil {
		_ = syscall.Flock(int(fl.file.Fd()), syscall.LOCK_UN)
		_ = fl.file.Close()
		fl.file = nil
	}
}

// LockedWrite writes content to filepath while holding an exclusive flock.
func LockedWrite(lockPath, targetPath, content string) error {
	fl := NewFileLock(lockPath)
	if err := fl.Lock(); err != nil {
		return err
	}
	defer fl.Unlock()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(targetPath, []byte(content), 0o644)
}
