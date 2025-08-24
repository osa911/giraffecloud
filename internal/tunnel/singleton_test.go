package tunnel

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSingletonManager_BasicFunctionality(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "giraffecloud-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Override home directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create singleton manager
	sm, err := NewSingletonManager()
	if err != nil {
		t.Fatalf("Failed to create singleton manager: %v", err)
	}

	// Initially should not be running
	if sm.IsRunning() {
		t.Error("Expected not running initially")
	}

	// Acquire lock
	if err := sm.AcquireLock(); err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Should now be running
	if !sm.IsRunning() {
		t.Error("Expected running after acquiring lock")
	}

	// Should not be able to acquire lock again
	if err := sm.AcquireLock(); err == nil {
		t.Error("Expected error when trying to acquire lock twice")
	}

	// Release lock
	if err := sm.ReleaseLock(); err != nil {
		t.Fatalf("Failed to release lock: %v", err)
	}

	// Should not be running after release
	if sm.IsRunning() {
		t.Error("Expected not running after releasing lock")
	}
}

func TestSingletonManager_StaleLockCleanup(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "giraffecloud-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Override home directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create singleton manager
	sm, err := NewSingletonManager()
	if err != nil {
		t.Fatalf("Failed to create singleton manager: %v", err)
	}

	// Create a stale PID file with a non-existent PID
	pidDir := filepath.Dir(sm.PidFile)
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		t.Fatalf("Failed to create pid directory: %v", err)
	}

	// Write a PID file with a very high PID number (likely non-existent)
	if err := os.WriteFile(sm.PidFile, []byte("999999\n"), 0644); err != nil {
		t.Fatalf("Failed to write stale pid file: %v", err)
	}

	// Should detect stale lock and clean it up
	if err := sm.CleanupStaleLock(); err != nil {
		t.Fatalf("Failed to cleanup stale lock: %v", err)
	}

	// Should now be able to acquire lock
	if err := sm.AcquireLock(); err != nil {
		t.Fatalf("Failed to acquire lock after cleanup: %v", err)
	}

	// Clean up
	sm.ReleaseLock()
}

func TestSingletonManager_WaitForLock(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "giraffecloud-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Override home directory for testing
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create singleton manager
	sm, err := NewSingletonManager()
	if err != nil {
		t.Fatalf("Failed to create singleton manager: %v", err)
	}

	// Acquire lock
	if err := sm.AcquireLock(); err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Start a goroutine to release the lock after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		sm.ReleaseLock()
	}()

	// Wait for lock with timeout
	if err := sm.WaitForLock(1 * time.Second); err != nil {
		t.Fatalf("Failed to wait for lock: %v", err)
	}

	// Should now be able to acquire lock
	if err := sm.AcquireLock(); err != nil {
		t.Fatalf("Failed to acquire lock after waiting: %v", err)
	}

	// Clean up
	sm.ReleaseLock()
}
