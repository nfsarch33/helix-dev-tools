//go:build !windows

package lockfile

import (
	"os"
	"path/filepath"
	"syscall"
)

// Lock acquires an exclusive flock on the lock file (Unix).
func (fl *FileLock) Lock() error {
	if err := os.MkdirAll(filepath.Dir(fl.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(fl.path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	fl.file = f
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX) // #nosec G115 -- fd fits in int on all 64-bit platforms
}

// Unlock releases the flock and closes the file (Unix).
func (fl *FileLock) Unlock() {
	if fl.file != nil {
		_ = syscall.Flock(int(fl.file.Fd()), syscall.LOCK_UN) // #nosec G115 -- fd fits in int on all 64-bit platforms
		_ = fl.file.Close()
		fl.file = nil
	}
}
