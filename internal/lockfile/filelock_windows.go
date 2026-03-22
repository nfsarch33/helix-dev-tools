//go:build windows

package lockfile

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

const (
	lockfileExclusiveLock = 0x00000002
	lockfileFailImmedi    = 0x00000001
)

// Lock acquires an exclusive lock on the lock file (Windows).
func (fl *FileLock) Lock() error {
	if err := os.MkdirAll(filepath.Dir(fl.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(fl.path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	fl.file = f

	var ol syscall.Overlapped
	r1, _, err := procLockFileEx.Call(
		uintptr(f.Fd()),
		uintptr(lockfileExclusiveLock),
		0,
		1, 0,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r1 == 0 {
		return err
	}
	return nil
}

// Unlock releases the lock and closes the file (Windows).
func (fl *FileLock) Unlock() {
	if fl.file != nil {
		var ol syscall.Overlapped
		_, _, _ = procUnlockFileEx.Call(
			uintptr(fl.file.Fd()),
			0,
			1, 0,
			uintptr(unsafe.Pointer(&ol)),
		)
		_ = fl.file.Close()
		fl.file = nil
	}
}
