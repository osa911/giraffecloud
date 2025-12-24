//go:build !windows

package tunnel

import (
	"os"
	"syscall"
)

// lockFile attempts to acquire an exclusive lock on the file without blocking.
func lockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// unlockFile releases the advisory lock on the file.
func unlockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}

// isLockContended returns true if the error indicates the lock is held by another process.
func isLockContended(err error) bool {
	return err == syscall.EWOULDBLOCK || err == syscall.EAGAIN
}
