package tunnel

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// AddToPath adds the giraffecloud binary directory to system PATH
func (sm *ServiceManager) AddToPath() error {
	switch runtime.GOOS {
	case "darwin", "linux":
		return sm.addToUnixPath()
	case "windows":
		return sm.addToWindowsPath()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// RemoveFromPath removes the giraffecloud binary directory from system PATH
func (sm *ServiceManager) RemoveFromPath() error {
	switch runtime.GOOS {
	case "darwin", "linux":
		return sm.removeFromUnixPath()
	case "windows":
		return sm.removeFromWindowsPath()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func (sm *ServiceManager) addToUnixPath() error {
	// Create /usr/local/bin if it doesn't exist
	binDir := "/usr/local/bin"
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", binDir, err)
	}

	// Create symlink
	symlink := filepath.Join(binDir, "giraffecloud")

	// Remove existing symlink if it exists
	if _, err := os.Lstat(symlink); err == nil {
		if err := os.Remove(symlink); err != nil {
			return fmt.Errorf("failed to remove existing symlink: %w", err)
		}
	}

	// Create new symlink
	if err := os.Symlink(sm.executablePath, symlink); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	sm.logger.Info("Added giraffecloud to PATH at %s", symlink)
	return nil
}

func (sm *ServiceManager) removeFromUnixPath() error {
	symlink := "/usr/local/bin/giraffecloud"
	if err := os.Remove(symlink); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove symlink: %w", err)
	}

	sm.logger.Info("Removed giraffecloud from PATH")
	return nil
}

func (sm *ServiceManager) addToWindowsPath() error {
	// Get the directory containing the executable
	binDir := filepath.Dir(sm.executablePath)

	// Get current PATH
	cmd := exec.Command("powershell", "-Command", "[Environment]::GetEnvironmentVariable('PATH', 'User')")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get PATH: %w", err)
	}

	currentPath := strings.TrimSpace(string(output))
	paths := strings.Split(currentPath, ";")

	// Check if already in PATH
	for _, path := range paths {
		if strings.EqualFold(path, binDir) {
			return nil // Already in PATH
		}
	}

	// Add to PATH
	newPath := currentPath
	if newPath != "" {
		newPath += ";"
	}
	newPath += binDir

	// Update PATH using PowerShell
	cmd = exec.Command("powershell", "-Command",
		fmt.Sprintf("[Environment]::SetEnvironmentVariable('PATH', '%s', 'User')", newPath))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update PATH: %w", err)
	}

	sm.logger.Info("Added giraffecloud to PATH at %s", binDir)
	return nil
}

func (sm *ServiceManager) removeFromWindowsPath() error {
	// Get the directory containing the executable
	binDir := filepath.Dir(sm.executablePath)

	// Get current PATH
	cmd := exec.Command("powershell", "-Command", "[Environment]::GetEnvironmentVariable('PATH', 'User')")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get PATH: %w", err)
	}

	currentPath := strings.TrimSpace(string(output))
	paths := strings.Split(currentPath, ";")

	// Remove directory from PATH
	var newPaths []string
	for _, path := range paths {
		if !strings.EqualFold(path, binDir) {
			newPaths = append(newPaths, path)
		}
	}

	// Update PATH using PowerShell
	newPath := strings.Join(newPaths, ";")
	cmd = exec.Command("powershell", "-Command",
		fmt.Sprintf("[Environment]::SetEnvironmentVariable('PATH', '%s', 'User')", newPath))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update PATH: %w", err)
	}

	sm.logger.Info("Removed giraffecloud from PATH")
	return nil
}
