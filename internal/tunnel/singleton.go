package tunnel

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/osa911/giraffecloud/internal/logging"
)

// SingletonManager prevents multiple tunnel instances from running simultaneously
type SingletonManager struct {
	PidFile string
	lockFile *os.File
	logger   *logging.Logger
}

// NewSingletonManager creates a new singleton manager
func NewSingletonManager() (*SingletonManager, error) {
	// Normalize config home and prefer same directory as other client configs
	EnsureConsistentConfigHome()

	configDir, err := GetConfigDir()
	if err != nil {
		// Graceful fallback when HOME is missing: use temp dir
		fallbackDir := filepath.Join(os.TempDir(), "giraffecloud")
		if mkErr := os.MkdirAll(fallbackDir, 0755); mkErr != nil {
			return nil, fmt.Errorf("failed to resolve config directory for singleton: %w", err)
		}
		pidFile := filepath.Join(fallbackDir, "tunnel.pid")
		return &SingletonManager{
			PidFile: pidFile,
			logger:  logging.GetGlobalLogger(),
		}, nil
	}

	pidFile := filepath.Join(configDir, "tunnel.pid")

	return &SingletonManager{
		PidFile: pidFile,
		logger:  logging.GetGlobalLogger(),
	}, nil
}

// AcquireLock attempts to acquire the singleton lock using advisory file locking (flock)
func (sm *SingletonManager) AcquireLock() error {
	// Create directory if it doesn't exist
	pidDir := filepath.Dir(sm.PidFile)
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return fmt.Errorf("failed to create pid directory: %w", err)
	}

	// Open or create the PID file
	file, err := os.OpenFile(sm.PidFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open pid file: %w", err)
	}

	// Attempt to acquire an exclusive lock without blocking (flock)
	// On Unix-like systems, LOCK_EX | LOCK_NB returns syscall.EWOULDBLOCK if locked
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		file.Close()
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			// Lock is held by another process
			pid := sm.getPIDFromFile()
			return fmt.Errorf("tunnel is already running (PID: %d). Use 'giraffecloud service status' to check service status", pid)
		}
		return fmt.Errorf("failed to acquire advisory lock: %w", err)
	}

	// Lock acquired! Keep the file handle open to maintain the lock
	sm.lockFile = file

	// Write current PID to file for informational purposes
	pid := os.Getpid()
	if err := file.Truncate(0); err != nil {
		sm.logger.Warn("Failed to truncate pid file: %v", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		sm.logger.Warn("Failed to seek pid file: %v", err)
	}
	if _, err := fmt.Fprintf(file, "%d\n", pid); err != nil {
		sm.logger.Warn("Failed to write to pid file: %v", err)
	}

	if sm.logger != nil {
		sm.logger.Debug("Acquired singleton lock (PID: %d)", pid)
	}
	return nil
}

// ReleaseLock releases the singleton lock
func (sm *SingletonManager) ReleaseLock() error {
	if sm.lockFile == nil {
		return nil
	}

	// Unlock and close the file
	_ = syscall.Flock(int(sm.lockFile.Fd()), syscall.LOCK_UN)
	_ = sm.lockFile.Close()
	sm.lockFile = nil

	// Remove the file - ignore errors if it doesn't exist
	if err := os.Remove(sm.PidFile); err != nil && !os.IsNotExist(err) {
		sm.logger.Warn("Failed to remove pid file on release: %v", err)
	}

	if sm.logger != nil {
		sm.logger.Debug("Released singleton lock")
	}
	return nil
}

// IsRunning checks if a tunnel instance is currently running
func (sm *SingletonManager) IsRunning() bool {
	sm.CleanupStaleLock() // Proactively clean up if we can acquire the lock

	file, err := os.OpenFile(sm.PidFile, os.O_RDWR, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		return true // Possibly locked
	}
	defer file.Close()

	// Try to get a shared lock without blocking to check status
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		// We could acquire the lock, so it wasn't running
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		return false
	}

	return true
}

// GetRunningPID returns the PID of the running tunnel instance, or 0 if not running
func (sm *SingletonManager) GetRunningPID() int {
	if !sm.IsRunning() {
		return 0
	}
	return sm.getPIDFromFile()
}

// getPIDFromFile reads the PID from the PID file (helper)
func (sm *SingletonManager) getPIDFromFile() int {
	data, err := os.ReadFile(sm.PidFile)
	if err != nil {
		return 0
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0
	}

	return pid
}

// CheckServiceConflict checks if there's a conflict with the system service
func (sm *SingletonManager) CheckServiceConflict() error {
	// If running under managed service, skip self-conflict detection
	if os.Getenv("GIRAFFECLOUD_IS_SERVICE") == "1" {
		if sm.logger != nil {
			sm.logger.Debug("Running under system service; skipping service conflict check")
		}
		return nil
	}

	serviceManager, err := NewServiceManager()
	if err != nil {
		if sm.logger != nil {
			sm.logger.Debug("Failed to create service manager: %v", err)
		}
		return nil
	}

	isRunning, err := serviceManager.IsRunning()
	if err != nil {
		if sm.logger != nil {
			sm.logger.Debug("Failed to check service status: %v", err)
		}
		return nil
	}

	if isRunning {
		return fmt.Errorf("GiraffeCloud service is already running. Use 'giraffecloud service stop' to stop the service before running the tunnel directly")
	}

	return nil
}

// CleanupStaleLock removes stale PID files if no process holds the kernel lock
func (sm *SingletonManager) CleanupStaleLock() error {
	file, err := os.OpenFile(sm.PidFile, os.O_RDWR, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	// Try to acquire lock - if successful, it was stale
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err == nil {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
		_ = os.Remove(sm.PidFile)
		if sm.logger != nil {
			sm.logger.Debug("Cleaned up stale lock file")
		}
	}
	return nil
}

// WaitForLock waits for the lock to become available
func (sm *SingletonManager) WaitForLock(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if !sm.IsRunning() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for tunnel lock to become available")
}
