package service

import (
	"bytes"
	"fmt"
	"os"
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
	cmd := exec.Command("sudo", "systemctl", "restart", m.unitName)
	var stderr bytes.Buffer
	cmd.Stdin = os.Stdin   // Allow sudo password prompt
	cmd.Stdout = os.Stdout // Show sudo output
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 5 { // systemd: unit not found
				return fmt.Errorf("service not found. Run 'sudo giraffecloud service install' first")
			}
		}
		if msg := stderr.String(); msg != "" {
			return fmt.Errorf("failed to restart service: %s", msg)
		}
		return fmt.Errorf("failed to restart service: %w", err)
	}
	return nil
}

func (m *SystemdServiceManager) Stop() error {
	cmd := exec.Command("sudo", "systemctl", "stop", m.unitName)
	var stderr bytes.Buffer
	cmd.Stdin = os.Stdin   // Allow sudo password prompt
	cmd.Stdout = os.Stdout // Show sudo output
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 5 {
				return fmt.Errorf("service not found. Run 'sudo giraffecloud service install' first")
			}
		}
		if msg := stderr.String(); msg != "" {
			return fmt.Errorf("failed to stop service: %s", msg)
		}
		return fmt.Errorf("failed to stop service: %w", err)
	}
	return nil
}

func (m *SystemdServiceManager) Start() error {
	cmd := exec.Command("sudo", "systemctl", "start", m.unitName)
	var stderr bytes.Buffer
	cmd.Stdin = os.Stdin   // Allow sudo password prompt
	cmd.Stdout = os.Stdout // Show sudo output
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 5 {
				return fmt.Errorf("service not found. Run 'sudo giraffecloud service install' first")
			}
		}
		if msg := stderr.String(); msg != "" {
			return fmt.Errorf("failed to start service: %s", msg)
		}
		return fmt.Errorf("failed to start service: %w", err)
	}
	return nil
}

func (m *SystemdServiceManager) String() string {
	return fmt.Sprintf("systemd unit %s", m.unitName)
}
