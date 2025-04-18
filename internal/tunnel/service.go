package tunnel

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type ServiceManager struct {
	executablePath string
}

func NewServiceManager() (*ServiceManager, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return &ServiceManager{executablePath: executablePath}, nil
}

func (sm *ServiceManager) Install() error {
	switch runtime.GOOS {
	case "darwin":
		return sm.installDarwin()
	case "linux":
		return sm.installLinux()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func (sm *ServiceManager) Uninstall() error {
	switch runtime.GOOS {
	case "darwin":
		return sm.uninstallDarwin()
	case "linux":
		return sm.uninstallLinux()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func (sm *ServiceManager) installDarwin() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Create LaunchAgent plist file
	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.giraffecloud.tunnel</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>connect</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardErrorPath</key>
    <string>%s/.giraffecloud/tunnel.log</string>
    <key>StandardOutPath</key>
    <string>%s/.giraffecloud/tunnel.log</string>
</dict>
</plist>`, sm.executablePath, homeDir, homeDir)

	plistPath := filepath.Join(homeDir, "Library/LaunchAgents/com.giraffecloud.tunnel.plist")
	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return err
	}

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return err
	}

	// Load the service
	cmd := exec.Command("launchctl", "load", plistPath)
	return cmd.Run()
}

func (sm *ServiceManager) uninstallDarwin() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	plistPath := filepath.Join(homeDir, "Library/LaunchAgents/com.giraffecloud.tunnel.plist")

	// Unload the service first
	cmd := exec.Command("launchctl", "unload", plistPath)
	if err := cmd.Run(); err != nil {
		return err
	}

	// Remove the plist file
	return os.Remove(plistPath)
}

func (sm *ServiceManager) installLinux() error {
	// Create systemd service file
	serviceContent := fmt.Sprintf(`[Unit]
Description=GiraffeCloud Tunnel Service
After=network.target

[Service]
Type=simple
User=%s
ExecStart=%s connect
Restart=always
RestartSec=10
StandardOutput=append:/var/log/giraffecloud/tunnel.log
StandardError=append:/var/log/giraffecloud/tunnel.log

[Install]
WantedBy=multi-user.target`, os.Getenv("USER"), sm.executablePath)

	// Create log directory
	if err := os.MkdirAll("/var/log/giraffecloud", 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Write service file
	servicePath := "/etc/systemd/system/giraffecloud.service"
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Reload systemd daemon
	cmd := exec.Command("sudo", "systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	// Enable and start the service
	cmd = exec.Command("sudo", "systemctl", "enable", "giraffecloud")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}

	cmd = exec.Command("sudo", "systemctl", "start", "giraffecloud")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

func (sm *ServiceManager) uninstallLinux() error {
	// Stop and disable the service
	cmd := exec.Command("sudo", "systemctl", "stop", "giraffecloud")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}

	cmd = exec.Command("sudo", "systemctl", "disable", "giraffecloud")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to disable service: %w", err)
	}

	// Remove service file
	if err := os.Remove("/etc/systemd/system/giraffecloud.service"); err != nil {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	// Reload systemd daemon
	cmd = exec.Command("sudo", "systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	return nil
}