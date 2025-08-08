package service

import (
	"fmt"
	"os/exec"
)

// SystemdServiceManager manages a systemd unit
type SystemdServiceManager struct {
	unitName string
}

// NewSystemdServiceManager creates a manager for the given systemd unit
func NewSystemdServiceManager(unit string) *SystemdServiceManager {
	if unit == "" {
		unit = "giraffecloud"
	}
	// Accept either service name or full unit
	if len(unit) < 8 || unit[len(unit)-8:] != ".service" {
		unit = unit + ".service"
	}
	return &SystemdServiceManager{unitName: unit}
}

func (m *SystemdServiceManager) IsRunning() (bool, error) {
	cmd := exec.Command("systemctl", "is-active", "--quiet", m.unitName)
	if err := cmd.Run(); err != nil {
		// Non-zero exit means not active
		return false, nil
	}
	return true, nil
}

func (m *SystemdServiceManager) Restart() error {
	return exec.Command("systemctl", "restart", m.unitName).Run()
}

func (m *SystemdServiceManager) Stop() error {
	return exec.Command("systemctl", "stop", m.unitName).Run()
}

func (m *SystemdServiceManager) Start() error {
	return exec.Command("systemctl", "start", m.unitName).Run()
}

func (m *SystemdServiceManager) String() string {
	return fmt.Sprintf("systemd unit %s", m.unitName)
}
