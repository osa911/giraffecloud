package tunnel

import (
	"fmt"
	"giraffecloud/internal/logging"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type ServiceManager struct {
	executablePath string
	logger         *logging.Logger
	useUserUnit    bool
}

func NewServiceManager() (*ServiceManager, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	return &ServiceManager{
		executablePath: executablePath,
		logger:         logging.GetGlobalLogger(),
	}, nil
}

// NewServiceManagerWithUser returns a service manager that targets user-level units on Linux when userUnit is true
func NewServiceManagerWithUser(userUnit bool) (*ServiceManager, error) {
	sm, err := NewServiceManager()
	if err != nil {
		return nil, err
	}
	sm.useUserUnit = userUnit
	return sm, nil
}

func (sm *ServiceManager) Install() error {
	// Install service based on OS
	var err error
	switch runtime.GOOS {
	case "darwin":
		err = sm.installDarwin()
	case "linux":
		err = sm.installLinux()
	case "windows":
		err = sm.installWindows()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if err != nil {
		return err
	}

	// Add to PATH after successful service installation
	if err := sm.AddToPath(); err != nil {
		sm.logger.Warn("Failed to add to PATH: %v", err)
		// Don't fail installation if PATH update fails
	}

	return nil
}

func (sm *ServiceManager) Uninstall() error {
	// Remove from PATH first (don't fail if this fails)
	if err := sm.RemoveFromPath(); err != nil {
		sm.logger.Warn("Failed to remove from PATH: %v", err)
	}

	// Uninstall service based on OS
	switch runtime.GOOS {
	case "darwin":
		return sm.uninstallDarwin()
	case "linux":
		return sm.uninstallLinux()
	case "windows":
		return sm.uninstallWindows()
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
    <key>EnvironmentVariables</key>
    <dict>
        <key>GIRAFFECLOUD_HOME</key>
        <string>%s/.giraffecloud</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardErrorPath</key>
    <string>%s/.giraffecloud/tunnel.log</string>
    <key>StandardOutPath</key>
    <string>%s/.giraffecloud/tunnel.log</string>
</dict>
</plist>`, sm.executablePath, homeDir, homeDir, homeDir)

	plistPath := filepath.Join(homeDir, "Library/LaunchAgents/com.giraffecloud.tunnel.plist")

	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return err
	}

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return err
	}

	// Load the service
	cmd := exec.Command("launchctl", "load", plistPath)
	if err := cmd.Run(); err != nil {
		return err
	}

	sm.logger.Info("Service installed successfully")
	return nil
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
	if err := os.Remove(plistPath); err != nil {
		return err
	}

	sm.logger.Info("Service uninstalled successfully")
	return nil
}

func (sm *ServiceManager) installLinux() error {
	if sm.useUserUnit {
		// User-level systemd unit (~/.config/systemd/user)
		// Expand explicit home for reliability
		userHome := os.Getenv("HOME")
		serviceContent := fmt.Sprintf(`[Unit]
Description=GiraffeCloud Tunnel Service (User)
After=default.target network-online.target

[Service]
Type=simple
ExecStart=%s connect
Environment=GIRAFFECLOUD_HOME=%s/.giraffecloud
Restart=always
RestartSec=10

[Install]
WantedBy=default.target`, sm.executablePath, userHome)

		// Ensure user unit directory exists
		userDir := filepath.Join(os.Getenv("HOME"), ".config/systemd/user")
		if err := os.MkdirAll(userDir, 0755); err != nil {
			return fmt.Errorf("failed to create user systemd directory: %w", err)
		}

		servicePath := filepath.Join(userDir, "giraffecloud.service")
		if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
			return fmt.Errorf("failed to write user service file: %w", err)
		}

		// Reload user daemon and enable/start
		if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
			return fmt.Errorf("failed to reload user systemd daemon: %w", err)
		}
		if err := exec.Command("systemctl", "--user", "enable", "giraffecloud").Run(); err != nil {
			return fmt.Errorf("failed to enable user service: %w", err)
		}
		if err := exec.Command("systemctl", "--user", "start", "giraffecloud").Run(); err != nil {
			return fmt.Errorf("failed to start user service: %w", err)
		}
		sm.logger.Info("User service installed successfully")
		sm.logger.Info("Note: To start at boot without login, enable lingering: sudo loginctl enable-linger %s", os.Getenv("USER"))
		return nil
	}

	// System-level unit: write config to the invoking user's home, not /root
	// Determine the intended user and home directory for config
	svcUser := os.Getenv("SUDO_USER")
	if svcUser == "" {
		svcUser = os.Getenv("USER")
	}
	userHome := "/root"
	if u, err := user.Lookup(svcUser); err == nil && u != nil && u.HomeDir != "" {
		userHome = u.HomeDir
	} else if svcUser != "root" {
		// Fallback best-effort
		userHome = filepath.Join("/home", svcUser)
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=GiraffeCloud Tunnel Service
After=network.target

[Service]
Type=simple
User=%s
ExecStart=%s connect
Environment=GIRAFFECLOUD_HOME=%s/.giraffecloud
Restart=always
RestartSec=10
StandardOutput=append:/var/log/giraffecloud/tunnel.log
StandardError=append:/var/log/giraffecloud/tunnel.log

[Install]
WantedBy=multi-user.target`, svcUser, sm.executablePath, userHome)

	if err := os.MkdirAll("/var/log/giraffecloud", 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	servicePath := "/etc/systemd/system/giraffecloud.service"
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}
	if err := exec.Command("sudo", "systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}
	if err := exec.Command("sudo", "systemctl", "enable", "giraffecloud").Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}
	if err := exec.Command("sudo", "systemctl", "start", "giraffecloud").Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}
	sm.logger.Info("System service installed successfully")
	return nil
}

func (sm *ServiceManager) uninstallLinux() error {
	if sm.useUserUnit {
		_ = exec.Command("systemctl", "--user", "stop", "giraffecloud").Run()
		_ = exec.Command("systemctl", "--user", "disable", "giraffecloud").Run()
		servicePath := filepath.Join(os.Getenv("HOME"), ".config/systemd/user/giraffecloud.service")
		if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove user service file: %w", err)
		}
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		sm.logger.Info("User service uninstalled successfully")
		return nil
	}

	// System-level
	if err := exec.Command("sudo", "systemctl", "stop", "giraffecloud").Run(); err != nil {
		return fmt.Errorf("failed to stop service: %w", err)
	}
	if err := exec.Command("sudo", "systemctl", "disable", "giraffecloud").Run(); err != nil {
		return fmt.Errorf("failed to disable service: %w", err)
	}
	servicePath := "/etc/systemd/system/giraffecloud.service"
	if err := os.Remove(servicePath); err != nil {
		return fmt.Errorf("failed to remove service file: %w", err)
	}
	if err := exec.Command("sudo", "systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}
	sm.logger.Info("System service uninstalled successfully")
	return nil
}

func (sm *ServiceManager) installWindows() error {
	// Create Windows service using sc.exe
	serviceName := "GiraffeCloudTunnel"
	displayName := "GiraffeCloud Tunnel Service"
	description := "GiraffeCloud secure tunnel service for exposing local applications"

	// Create the service
	cmd := exec.Command("sc", "create", serviceName,
		"binPath=", fmt.Sprintf("\"%s\" connect", sm.executablePath),
		"DisplayName=", displayName,
		"start=", "auto",
		"depend=", "Tcpip")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create Windows service: %w", err)
	}

	// Set service description
	cmd = exec.Command("sc", "description", serviceName, description)
	if err := cmd.Run(); err != nil {
		// Don't fail if description setting fails
		sm.logger.Info("Warning: Failed to set service description")
	}

	// Configure service to restart on failure
	cmd = exec.Command("sc", "failure", serviceName,
		"reset=", "86400", // Reset failure count after 24 hours
		"actions=", "restart/5000/restart/10000/restart/30000") // Restart after 5s, 10s, 30s
	if err := cmd.Run(); err != nil {
		// Don't fail if failure action setting fails
		sm.logger.Info("Warning: Failed to set service failure actions")
	}

	// Set service environment so it uses the same config as the interactive CLI
	// Use the current user's home directory
	userHome, herr := os.UserHomeDir()
	if herr == nil && userHome != "" {
		// REG_MULTI_SZ for Environment; single entry is fine
		regPath := `HKLM\\SYSTEM\\CurrentControlSet\\Services\\` + serviceName
		envValue := fmt.Sprintf("GIRAFFECLOUD_HOME=%s\\.giraffecloud", userHome)
		cmd = exec.Command("reg", "add", regPath, "/v", "Environment", "/t", "REG_MULTI_SZ", "/d", envValue, "/f")
		if err := cmd.Run(); err != nil {
			// Don't fail install if registry write fails
			sm.logger.Info("Warning: Failed to set Windows service environment: %v", err)
		}
	}

	// Start the service (after setting environment)
	cmd = exec.Command("sc", "start", serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start Windows service: %w", err)
	}

	sm.logger.Info("Service installed successfully")
	return nil
}

func (sm *ServiceManager) uninstallWindows() error {
	serviceName := "GiraffeCloudTunnel"

	// Stop the service first
	cmd := exec.Command("sc", "stop", serviceName)
	if err := cmd.Run(); err != nil {
		// Don't fail if service is already stopped
		sm.logger.Info("Service may already be stopped")
	}

	// Delete the service
	cmd = exec.Command("sc", "delete", serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete Windows service: %w", err)
	}

	sm.logger.Info("Service uninstalled successfully")
	return nil
}

// IsInstalled checks if the service is installed
func (sm *ServiceManager) IsInstalled() (bool, error) {
	switch runtime.GOOS {
	case "darwin":
		return sm.isInstalledDarwin()
	case "linux":
		return sm.isInstalledLinux()
	case "windows":
		return sm.isInstalledWindows()
	default:
		return false, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// IsRunning checks if the service is currently running
func (sm *ServiceManager) IsRunning() (bool, error) {
	switch runtime.GOOS {
	case "darwin":
		return sm.isRunningDarwin()
	case "linux":
		return sm.isRunningLinux()
	case "windows":
		return sm.isRunningWindows()
	default:
		return false, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// GetLogs retrieves recent service logs
func (sm *ServiceManager) GetLogs() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return sm.getLogsDarwin()
	case "linux":
		return sm.getLogsLinux()
	case "windows":
		return sm.getLogsWindows()
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// Restart restarts the service
func (sm *ServiceManager) Restart() error {
	switch runtime.GOOS {
	case "darwin":
		return sm.restartDarwin()
	case "linux":
		return sm.restartLinux()
	case "windows":
		return sm.restartWindows()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// Stop stops the service
func (sm *ServiceManager) Stop() error {
	switch runtime.GOOS {
	case "darwin":
		return sm.stopDarwin()
	case "linux":
		return sm.stopLinux()
	case "windows":
		return sm.stopWindows()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// Start starts the service
func (sm *ServiceManager) Start() error {
	switch runtime.GOOS {
	case "darwin":
		return sm.startDarwin()
	case "linux":
		return sm.startLinux()
	case "windows":
		return sm.startWindows()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// Darwin-specific implementations
func (sm *ServiceManager) isInstalledDarwin() (bool, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false, err
	}
	plistPath := filepath.Join(homeDir, "Library/LaunchAgents/com.giraffecloud.tunnel.plist")
	_, err = os.Stat(plistPath)
	return !os.IsNotExist(err), nil
}

func (sm *ServiceManager) isRunningDarwin() (bool, error) {
	cmd := exec.Command("launchctl", "list", "com.giraffecloud.tunnel")
	err := cmd.Run()
	return err == nil, nil
}

func (sm *ServiceManager) getLogsDarwin() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	logPath := filepath.Join(homeDir, ".giraffecloud/tunnel.log")

	// Get last 20 lines of log file
	cmd := exec.Command("tail", "-n", "20", logPath)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}
	return string(output), nil
}

// Linux-specific implementations
func (sm *ServiceManager) isInstalledLinux() (bool, error) {
	if sm.useUserUnit {
		servicePath := filepath.Join(os.Getenv("HOME"), ".config/systemd/user/giraffecloud.service")
		_, err := os.Stat(servicePath)
		return !os.IsNotExist(err), nil
	}
	servicePath := "/etc/systemd/system/giraffecloud.service"
	_, err := os.Stat(servicePath)
	return !os.IsNotExist(err), nil
}

func (sm *ServiceManager) isRunningLinux() (bool, error) {
	args := []string{"is-active", "--quiet", "giraffecloud"}
	if sm.useUserUnit {
		args = append([]string{"--user"}, args...)
	}
	cmd := exec.Command("systemctl", args...)
	err := cmd.Run()
	return err == nil, nil
}

func (sm *ServiceManager) getLogsLinux() (string, error) {
	args := []string{"-u", "giraffecloud", "-n", "20", "--no-pager"}
	if sm.useUserUnit {
		args = append([]string{"--user"}, args...)
	}
	cmd := exec.Command("journalctl", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}
	return string(output), nil
}

// Windows-specific implementations
func (sm *ServiceManager) isInstalledWindows() (bool, error) {
	cmd := exec.Command("sc", "query", "GiraffeCloudTunnel")
	err := cmd.Run()
	return err == nil, nil
}

func (sm *ServiceManager) isRunningWindows() (bool, error) {
	cmd := exec.Command("sc", "query", "GiraffeCloudTunnel")
	output, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	// Check if service is running by looking for "RUNNING" in output
	return strings.Contains(string(output), "RUNNING"), nil
}

func (sm *ServiceManager) getLogsWindows() (string, error) {
	// Windows Event Log query for GiraffeCloud service
	cmd := exec.Command("wevtutil", "qe", "Application", "/q:*[System[Provider[@Name='GiraffeCloudTunnel']]]", "/f:text", "/c:20")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to read Windows event logs: %w", err)
	}
	return string(output), nil
}

// Darwin service control methods
func (sm *ServiceManager) restartDarwin() error {
	// Stop then start
	_ = sm.stopDarwin() // Ignore error if not running
	return sm.startDarwin()
}

func (sm *ServiceManager) stopDarwin() error {
	cmd := exec.Command("launchctl", "unload", "-w", filepath.Join(os.Getenv("HOME"), "Library/LaunchAgents/com.giraffecloud.tunnel.plist"))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop Darwin service: %w", err)
	}
	sm.logger.Info("Service stopped successfully")
	return nil
}

func (sm *ServiceManager) startDarwin() error {
	cmd := exec.Command("launchctl", "load", "-w", filepath.Join(os.Getenv("HOME"), "Library/LaunchAgents/com.giraffecloud.tunnel.plist"))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start Darwin service: %w", err)
	}
	sm.logger.Info("Service started successfully")
	return nil
}

// Linux service control methods
func (sm *ServiceManager) restartLinux() error {
	if sm.useUserUnit {
		cmd := exec.Command("systemctl", "--user", "restart", "giraffecloud")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to restart user service: %w", err)
		}
		sm.logger.Info("User service restarted successfully")
		return nil
	}
	cmd := exec.Command("sudo", "systemctl", "restart", "giraffecloud")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restart Linux service: %w", err)
	}
	sm.logger.Info("Service restarted successfully")
	return nil
}

func (sm *ServiceManager) stopLinux() error {
	if sm.useUserUnit {
		cmd := exec.Command("systemctl", "--user", "stop", "giraffecloud")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to stop user service: %w", err)
		}
		sm.logger.Info("User service stopped successfully")
		return nil
	}
	cmd := exec.Command("sudo", "systemctl", "stop", "giraffecloud")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop Linux service: %w", err)
	}
	sm.logger.Info("Service stopped successfully")
	return nil
}

func (sm *ServiceManager) startLinux() error {
	if sm.useUserUnit {
		cmd := exec.Command("systemctl", "--user", "start", "giraffecloud")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to start user service: %w", err)
		}
		sm.logger.Info("User service started successfully")
		return nil
	}
	cmd := exec.Command("sudo", "systemctl", "start", "giraffecloud")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start Linux service: %w", err)
	}
	sm.logger.Info("Service started successfully")
	return nil
}

// Windows service control methods
func (sm *ServiceManager) restartWindows() error {
	serviceName := "GiraffeCloudTunnel"

	// Stop the service
	cmd := exec.Command("sc", "stop", serviceName)
	if err := cmd.Run(); err != nil {
		// Don't fail if service is already stopped
		sm.logger.Info("Service may already be stopped")
	}

	// Wait a moment for the service to stop
	time.Sleep(2 * time.Second)

	// Start the service
	cmd = exec.Command("sc", "start", serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start Windows service: %w", err)
	}

	sm.logger.Info("Service restarted successfully")
	return nil
}

func (sm *ServiceManager) stopWindows() error {
	serviceName := "GiraffeCloudTunnel"
	cmd := exec.Command("sc", "stop", serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop Windows service: %w", err)
	}
	sm.logger.Info("Service stopped successfully")
	return nil
}

func (sm *ServiceManager) startWindows() error {
	serviceName := "GiraffeCloudTunnel"
	cmd := exec.Command("sc", "start", serviceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start Windows service: %w", err)
	}
	sm.logger.Info("Service started successfully")
	return nil
}
