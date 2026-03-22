package lockfile

import (
	"os"
	"path/filepath"
)

// FileLock provides file-based locking for concurrent writes.
type FileLock struct {
	path string
	file *os.File
}

// NewFileLock creates a file lock at the given path.
func NewFileLock(path string) *FileLock {
	return &FileLock{path: path}
}

// LockedWrite writes content to filepath while holding an exclusive lock.
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
