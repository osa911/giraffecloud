package tunnel

import (
	"fmt"
	"giraffecloud/internal/logging"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type ServiceManager struct {
	executablePath string
	logger        *logging.Logger
}

func NewServiceManager() (*ServiceManager, error) {
	logger := logging.GetGlobalLogger()
	logger.Info("Creating new service manager")

	executablePath, err := os.Executable()
	if err != nil {
		logger.Error("Failed to get executable path: %v", err)
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	logger.Info("Using executable path: %s", executablePath)

	return &ServiceManager{
		executablePath: executablePath,
		logger:        logger,
	}, nil
}

func (sm *ServiceManager) Install() error {
	sm.logger.Info("Installing service for OS: %s", runtime.GOOS)

	switch runtime.GOOS {
	case "darwin":
		return sm.installDarwin()
	case "linux":
		return sm.installLinux()
	default:
		err := fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
		sm.logger.Error(err.Error())
		return err
	}
}

func (sm *ServiceManager) Uninstall() error {
	sm.logger.Info("Uninstalling service for OS: %s", runtime.GOOS)

	switch runtime.GOOS {
	case "darwin":
		return sm.uninstallDarwin()
	case "linux":
		return sm.uninstallLinux()
	default:
		err := fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
		sm.logger.Error(err.Error())
		return err
	}
}

func (sm *ServiceManager) installDarwin() error {
	sm.logger.Info("Installing service on Darwin/macOS")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		sm.logger.Error("Failed to get home directory: %v", err)
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
	sm.logger.Info("Creating LaunchAgent plist at: %s", plistPath)

	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		sm.logger.Error("Failed to create LaunchAgents directory: %v", err)
		return err
	}

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		sm.logger.Error("Failed to write plist file: %v", err)
		return err
	}

	// Load the service
	sm.logger.Info("Loading LaunchAgent service")
	cmd := exec.Command("launchctl", "load", plistPath)
	if err := cmd.Run(); err != nil {
		sm.logger.Error("Failed to load service: %v", err)
		return err
	}

	sm.logger.Info("Service installed successfully")
	return nil
}

func (sm *ServiceManager) uninstallDarwin() error {
	sm.logger.Info("Uninstalling service from Darwin/macOS")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		sm.logger.Error("Failed to get home directory: %v", err)
		return err
	}

	plistPath := filepath.Join(homeDir, "Library/LaunchAgents/com.giraffecloud.tunnel.plist")
	sm.logger.Info("Unloading LaunchAgent from: %s", plistPath)

	// Unload the service first
	cmd := exec.Command("launchctl", "unload", plistPath)
	if err := cmd.Run(); err != nil {
		sm.logger.Error("Failed to unload service: %v", err)
		return err
	}

	// Remove the plist file
	if err := os.Remove(plistPath); err != nil {
		sm.logger.Error("Failed to remove plist file: %v", err)
		return err
	}

	sm.logger.Info("Service uninstalled successfully")
	return nil
}

func (sm *ServiceManager) installLinux() error {
	sm.logger.Info("Installing service on Linux")

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
	sm.logger.Info("Creating log directory")
	if err := os.MkdirAll("/var/log/giraffecloud", 0755); err != nil {
		sm.logger.Error("Failed to create log directory: %v", err)
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Write service file
	servicePath := "/etc/systemd/system/giraffecloud.service"
	sm.logger.Info("Creating systemd service file at: %s", servicePath)
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		sm.logger.Error("Failed to write service file: %v", err)
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Reload systemd daemon
	sm.logger.Info("Reloading systemd daemon")
	cmd := exec.Command("sudo", "systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		sm.logger.Error("Failed to reload systemd daemon: %v", err)
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	// Enable and start the service
	sm.logger.Info("Enabling service")
	cmd = exec.Command("sudo", "systemctl", "enable", "giraffecloud")
	if err := cmd.Run(); err != nil {
		sm.logger.Error("Failed to enable service: %v", err)
		return fmt.Errorf("failed to enable service: %w", err)
	}

	sm.logger.Info("Starting service")
	cmd = exec.Command("sudo", "systemctl", "start", "giraffecloud")
	if err := cmd.Run(); err != nil {
		sm.logger.Error("Failed to start service: %v", err)
		return fmt.Errorf("failed to start service: %w", err)
	}

	sm.logger.Info("Service installed successfully")
	return nil
}

func (sm *ServiceManager) uninstallLinux() error {
	sm.logger.Info("Uninstalling service from Linux")

	// Stop and disable the service
	sm.logger.Info("Stopping service")
	cmd := exec.Command("sudo", "systemctl", "stop", "giraffecloud")
	if err := cmd.Run(); err != nil {
		sm.logger.Error("Failed to stop service: %v", err)
		return fmt.Errorf("failed to stop service: %w", err)
	}

	sm.logger.Info("Disabling service")
	cmd = exec.Command("sudo", "systemctl", "disable", "giraffecloud")
	if err := cmd.Run(); err != nil {
		sm.logger.Error("Failed to disable service: %v", err)
		return fmt.Errorf("failed to disable service: %w", err)
	}

	// Remove service file
	servicePath := "/etc/systemd/system/giraffecloud.service"
	sm.logger.Info("Removing service file: %s", servicePath)
	if err := os.Remove(servicePath); err != nil {
		sm.logger.Error("Failed to remove service file: %v", err)
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	// Reload systemd daemon
	sm.logger.Info("Reloading systemd daemon")
	cmd = exec.Command("sudo", "systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		sm.logger.Error("Failed to reload systemd daemon: %v", err)
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	sm.logger.Info("Service uninstalled successfully")
	return nil
}