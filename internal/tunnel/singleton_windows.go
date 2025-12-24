//go:build windows

package tunnel

import (
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

// lockFile attempts to acquire an exclusive lock on the file without blocking.
func lockFile(file *os.File) error {
	// For Windows, we use LockFileEx with LOCKFILE_EXCLUSIVE_LOCK and LOCKFILE_FAIL_IMMEDIATELY.
	// We lock the first byte of the file.
	var (
		LOCKFILE_EXCLUSIVE_LOCK   uint32 = 0x00000002
		LOCKFILE_FAIL_IMMEDIATELY uint32 = 0x00000001
	)

	h := windows.Handle(file.Fd())
	var ol windows.Overlapped
	return windows.LockFileEx(h, LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ol)
}

// unlockFile releases the advisory lock on the file.
func unlockFile(file *os.File) error {
	h := windows.Handle(file.Fd())
	var ol windows.Overlapped
	return windows.UnlockFileEx(h, 0, 1, 0, &ol)
}

// isLockContended returns true if the error indicates the lock is held by another process.
func isLockContended(err error) bool {
	// ERROR_LOCK_VIOLATION (33) or ERROR_IO_PENDING (997) or ERROR_SHARING_VIOLATION (32)
	if errno, ok := err.(syscall.Errno); ok {
		return errno == 32 || errno == 33 || errno == 997
	}
	if errno, ok := err.(windows.Errno); ok {
		return errno == windows.ERROR_SHARING_VIOLATION || errno == windows.ERROR_LOCK_VIOLATION || errno == windows.ERROR_IO_PENDING
	}
	return false
}
