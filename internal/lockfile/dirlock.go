package lockfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const defaultStaleTimeout = 300 * time.Second

// DirLock implements a cross-platform lock using mkdir atomicity.
// Works identically on macOS and Linux (POSIX).
type DirLock struct {
	path         string
	staleTimeout time.Duration
}

// NewDirLock creates a new directory-based lock at the given path.
func NewDirLock(path string) *DirLock {
	return &DirLock{path: path, staleTimeout: defaultStaleTimeout}
}

// WithStaleTimeout sets the duration after which a lock is considered stale.
func (l *DirLock) WithStaleTimeout(d time.Duration) *DirLock {
	l.staleTimeout = d
	return l
}

// Acquire attempts to create the lock directory.
// If the lock is held by a dead process or is stale, it reclaims it.
func (l *DirLock) Acquire() error {
	if err := os.Mkdir(l.path, 0o755); err == nil {
		return l.writePID()
	}

	if l.isHeldByDeadProcess() || l.isStale() {
		l.Release()
		if err := os.Mkdir(l.path, 0o755); err == nil {
			return l.writePID()
		}
	}

	return fmt.Errorf("lock held: %s", l.path)
}

// Release removes the lock directory and PID file.
func (l *DirLock) Release() {
	_ = os.Remove(filepath.Join(l.path, "pid"))
	_ = os.Remove(l.path)
}

func (l *DirLock) writePID() error {
	pidFile := filepath.Join(l.path, "pid")
	return os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644)
}

func (l *DirLock) readPID() (int, bool) {
	data, err := os.ReadFile(filepath.Join(l.path, "pid"))
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false
	}
	return pid, true
}

func (l *DirLock) isHeldByDeadProcess() bool {
	pid, ok := l.readPID()
	if !ok {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return true
	}
	// On Unix, FindProcess always succeeds; check with Signal(0)
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return true
	}
	return false
}

func (l *DirLock) isStale() bool {
	info, err := os.Stat(l.path)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) > l.staleTimeout
}
