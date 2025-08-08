package service

import (
	"runtime"
)

// NewDefaultServiceManager returns a platform-appropriate ServiceManagerProvider.
// On Linux, returns a systemd-based manager; on other platforms returns a no-op manager.
func NewDefaultServiceManager() ServiceManagerProvider {
	if runtime.GOOS == "linux" {
		return NewSystemdServiceManager("giraffecloud")
	}
	return &noopServiceManager{}
}

// noopServiceManager is a no-op implementation used on unsupported platforms.
type noopServiceManager struct{}

func (n *noopServiceManager) IsRunning() (bool, error) { return false, nil }
func (n *noopServiceManager) Restart() error           { return nil }
func (n *noopServiceManager) Stop() error              { return nil }
func (n *noopServiceManager) Start() error             { return nil }
