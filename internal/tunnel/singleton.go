package tunnel

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"giraffecloud/internal/logging"
)

// SingletonManager prevents multiple tunnel instances from running simultaneously
type SingletonManager struct {
	PidFile string
	logger  *logging.Logger
}

// NewSingletonManager creates a new singleton manager
func NewSingletonManager() (*SingletonManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	pidFile := filepath.Join(homeDir, ".giraffecloud", "tunnel.pid")

	return &SingletonManager{
		PidFile: pidFile,
		logger:  logging.GetGlobalLogger(),
	}, nil
}

// AcquireLock attempts to acquire the singleton lock
func (sm *SingletonManager) AcquireLock() error {
	// Create directory if it doesn't exist
	pidDir := filepath.Dir(sm.PidFile)
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return fmt.Errorf("failed to create pid directory: %w", err)
	}

	// Check if PID file exists and if the process is still running
	if sm.isProcessRunning() {
		return fmt.Errorf("tunnel is already running (PID: %d). Use 'giraffecloud service status' to check service status", sm.getPIDFromFile())
	}

	// Write current PID to file
	pid := os.Getpid()
	pidContent := fmt.Sprintf("%d\n", pid)

	if err := os.WriteFile(sm.PidFile, []byte(pidContent), 0644); err != nil {
		return fmt.Errorf("failed to write pid file: %w", err)
	}

	if sm.logger != nil {
		sm.logger.Debug("Acquired singleton lock (PID: %d)", pid)
	}
	return nil
}

// ReleaseLock releases the singleton lock
func (sm *SingletonManager) ReleaseLock() error {
	if err := os.Remove(sm.PidFile); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove pid file: %w", err)
		}
	}
	if sm.logger != nil {
		sm.logger.Debug("Released singleton lock")
	}
	return nil
}

// IsRunning checks if a tunnel instance is currently running
func (sm *SingletonManager) IsRunning() bool {
	return sm.isProcessRunning()
}

// GetRunningPID returns the PID of the running tunnel instance, or 0 if not running
func (sm *SingletonManager) GetRunningPID() int {
	if !sm.isProcessRunning() {
		return 0
	}
	return sm.getPIDFromFile()
}

// isProcessRunning checks if the process in the PID file is still running
func (sm *SingletonManager) isProcessRunning() bool {
	pid := sm.getPIDFromFile()
	if pid == 0 {
		return false
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix systems, sending signal 0 checks if process exists without actually sending a signal
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// getPIDFromFile reads the PID from the PID file
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
	serviceManager, err := NewServiceManager()
	if err != nil {
		if sm.logger != nil {
			sm.logger.Debug("Failed to create service manager: %v", err)
		}
		return nil // Don't fail if we can't check service status
	}

	isRunning, err := serviceManager.IsRunning()
	if err != nil {
		if sm.logger != nil {
			sm.logger.Debug("Failed to check service status: %v", err)
		}
		return nil // Don't fail if we can't check service status
	}

	if isRunning {
		return fmt.Errorf("GiraffeCloud service is already running. Use 'giraffecloud service stop' to stop the service before running the tunnel directly")
	}

	return nil
}

// CleanupStaleLock removes stale PID files (e.g., from crashed processes)
func (sm *SingletonManager) CleanupStaleLock() error {
	if !sm.isProcessRunning() {
		// Process is not running, clean up stale PID file
		if err := os.Remove(sm.PidFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to cleanup stale pid file: %w", err)
		}
		if sm.logger != nil {
			sm.logger.Debug("Cleaned up stale PID file")
		}
	}
	return nil
}

// WaitForLock waits for the lock to become available (useful for service restarts)
func (sm *SingletonManager) WaitForLock(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if !sm.isProcessRunning() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for tunnel lock to become available")
}
