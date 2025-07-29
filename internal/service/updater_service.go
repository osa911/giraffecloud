package service

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/version"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// UpdaterService handles client auto-updates
type UpdaterService struct {
	logger           *logging.Logger
	downloadBaseURL  string
	currentExePath   string
	backupDir        string
	tempDir          string
}

// UpdateInfo contains information about an available update
type UpdateInfo struct {
	Version        string
	DownloadURL    string
	ChecksumURL    string
	Checksum       string
	ReleaseNotes   string
	IsRequired     bool
	CurrentVersion string
}

// NewUpdaterService creates a new updater service
func NewUpdaterService(downloadBaseURL string) (*UpdaterService, error) {
	logger := logging.GetGlobalLogger()

	// Get current executable path
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Create backup and temp directories
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	backupDir := filepath.Join(homeDir, ".giraffecloud", "backups")
	tempDir := filepath.Join(homeDir, ".giraffecloud", "temp")

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	return &UpdaterService{
		logger:          logger,
		downloadBaseURL: downloadBaseURL,
		currentExePath:  exePath,
		backupDir:       backupDir,
		tempDir:         tempDir,
	}, nil
}

// CheckForUpdates checks if an update is available
func (u *UpdaterService) CheckForUpdates(serverURL string) (*UpdateInfo, error) {
	versionInfo, err := version.CheckServerVersion(serverURL)
	if err != nil {
		return nil, fmt.Errorf("failed to check server version: %w", err)
	}

	if !versionInfo.UpdateAvailable {
		u.logger.Info("No updates available. Current version: %s, Server version: %s",
			version.Version, versionInfo.ServerVersion)
		return nil, nil
	}

	// Construct download URL for current platform
	platform := runtime.GOOS
	arch := runtime.GOARCH
	filename := fmt.Sprintf("giraffecloud-%s-%s", platform, arch)
	if platform == "windows" {
		filename += ".exe"
	}

	downloadURL := fmt.Sprintf("%s/v%s/giraffecloud_%s_%s_%s.tar.gz",
		strings.TrimRight(u.downloadBaseURL, "/"),
		strings.TrimPrefix(versionInfo.ServerVersion, "v"),
		strings.TrimPrefix(versionInfo.ServerVersion, "v"),
		platform,
		arch)

	updateInfo := &UpdateInfo{
		Version:        versionInfo.ServerVersion,
		DownloadURL:    downloadURL,
		ChecksumURL:    downloadURL + ".sha256",
		IsRequired:     versionInfo.UpdateRequired,
		CurrentVersion: version.Version,
	}

	u.logger.Info("Update available: %s -> %s (Required: %v)",
		updateInfo.CurrentVersion, updateInfo.Version, updateInfo.IsRequired)

	return updateInfo, nil
}

// DownloadUpdate downloads the update package
func (u *UpdaterService) DownloadUpdate(updateInfo *UpdateInfo) (string, error) {
	u.logger.Info("Downloading update from: %s", updateInfo.DownloadURL)

	// Download the update package
	filename := filepath.Base(updateInfo.DownloadURL)
	downloadPath := filepath.Join(u.tempDir, filename)

	if err := u.downloadFile(updateInfo.DownloadURL, downloadPath); err != nil {
		return "", fmt.Errorf("failed to download update: %w", err)
	}

	// Download and verify checksum if available
	if updateInfo.ChecksumURL != "" {
		checksumPath := downloadPath + ".sha256"
		if err := u.downloadFile(updateInfo.ChecksumURL, checksumPath); err != nil {
			u.logger.Warn("Failed to download checksum file: %v", err)
		} else {
			if err := u.verifyChecksum(downloadPath, checksumPath); err != nil {
				return "", fmt.Errorf("checksum verification failed: %w", err)
			}
			u.logger.Info("Checksum verification passed")
		}
	}

	u.logger.Info("Update downloaded successfully: %s", downloadPath)
	return downloadPath, nil
}

// InstallUpdate installs the downloaded update
func (u *UpdaterService) InstallUpdate(downloadPath string) error {
	u.logger.Info("Installing update from: %s", downloadPath)

	// Create backup of current executable
	backupPath, err := u.createBackup()
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	u.logger.Info("Created backup: %s", backupPath)

	// Extract the update
	extractPath := filepath.Join(u.tempDir, "extract")
	if err := os.RemoveAll(extractPath); err != nil {
		u.logger.Warn("Failed to clean extract directory: %v", err)
	}

	if err := os.MkdirAll(extractPath, 0755); err != nil {
		return fmt.Errorf("failed to create extract directory: %w", err)
	}

	if err := u.extractArchive(downloadPath, extractPath); err != nil {
		return fmt.Errorf("failed to extract update: %w", err)
	}

	// Find the new executable
	newExePath, err := u.findExecutable(extractPath)
	if err != nil {
		return fmt.Errorf("failed to find new executable: %w", err)
	}

	// Replace current executable
	if err := u.replaceExecutable(newExePath); err != nil {
		// Try to restore backup
		if restoreErr := u.restoreBackup(backupPath); restoreErr != nil {
			u.logger.Error("Failed to restore backup after failed update: %v", restoreErr)
		}
		return fmt.Errorf("failed to replace executable: %w", err)
	}

	u.logger.Info("Update installed successfully!")

	// Clean up
	os.RemoveAll(downloadPath)
	os.RemoveAll(extractPath)

	return nil
}

// createBackup creates a backup of the current executable
func (u *UpdaterService) createBackup() (string, error) {
	backupName := fmt.Sprintf("giraffecloud_%s_%s.backup",
		version.Version,
		time.Now().Format("20060102_150405"))
	backupPath := filepath.Join(u.backupDir, backupName)

	src, err := os.Open(u.currentExePath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}

	// Copy permissions
	if info, err := os.Stat(u.currentExePath); err == nil {
		os.Chmod(backupPath, info.Mode())
	}

	return backupPath, nil
}

// downloadFile downloads a file from URL to local path
func (u *UpdaterService) downloadFile(url, path string) error {
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	return err
}

// verifyChecksum verifies the downloaded file's checksum
func (u *UpdaterService) verifyChecksum(filePath, checksumPath string) error {
	// Read expected checksum
	checksumData, err := os.ReadFile(checksumPath)
	if err != nil {
		return err
	}

	expectedChecksum := strings.TrimSpace(strings.Fields(string(checksumData))[0])

	// Calculate actual checksum
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	return nil
}

// extractArchive extracts a tar.gz archive
func (u *UpdaterService) extractArchive(archivePath, extractPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		path := filepath.Join(extractPath, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(path)
			if err != nil {
				return err
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}

			outFile.Close()

			if err := os.Chmod(path, os.FileMode(header.Mode)); err != nil {
				return err
			}
		}
	}

	return nil
}

// findExecutable finds the executable in the extracted directory
func (u *UpdaterService) findExecutable(extractPath string) (string, error) {
	var exePath string

	err := filepath.Walk(extractPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Look for giraffecloud executable
		if strings.Contains(info.Name(), "giraffecloud") && !info.IsDir() {
			// Check if it's executable
			if info.Mode()&0111 != 0 {
				exePath = path
				return io.EOF // Stop walking
			}
		}

		return nil
	})

	if err != nil && err != io.EOF {
		return "", err
	}

	if exePath == "" {
		return "", fmt.Errorf("executable not found in archive")
	}

	return exePath, nil
}

// replaceExecutable replaces the current executable with the new one
func (u *UpdaterService) replaceExecutable(newExePath string) error {
	// On Windows, we might need special handling
	if runtime.GOOS == "windows" {
		// Try to rename current exe to .old
		oldPath := u.currentExePath + ".old"
		if err := os.Rename(u.currentExePath, oldPath); err != nil {
			return err
		}

		// Copy new executable
		if err := u.copyFile(newExePath, u.currentExePath); err != nil {
			// Try to restore
			os.Rename(oldPath, u.currentExePath)
			return err
		}

		// Remove old file
		os.Remove(oldPath)
	} else {
		// On Unix systems, we can replace directly
		if err := u.copyFile(newExePath, u.currentExePath); err != nil {
			return err
		}
	}

	return nil
}

// copyFile copies a file from src to dst
func (u *UpdaterService) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Copy permissions
	if info, err := os.Stat(src); err == nil {
		os.Chmod(dst, info.Mode())
	}

	return nil
}

// restoreBackup restores a backup file
func (u *UpdaterService) restoreBackup(backupPath string) error {
	return u.copyFile(backupPath, u.currentExePath)
}

// CleanupOldBackups removes old backup files (keeps last 5)
func (u *UpdaterService) CleanupOldBackups() error {
	entries, err := os.ReadDir(u.backupDir)
	if err != nil {
		return err
	}

	// Filter backup files and sort by modification time
	var backupFiles []os.FileInfo
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".backup") {
			if info, err := entry.Info(); err == nil {
				backupFiles = append(backupFiles, info)
			}
		}
	}

	// Keep only the 5 most recent backups
	if len(backupFiles) > 5 {
		for i := 0; i < len(backupFiles)-5; i++ {
			backupPath := filepath.Join(u.backupDir, backupFiles[i].Name())
			if err := os.Remove(backupPath); err != nil {
				u.logger.Warn("Failed to remove old backup %s: %v", backupPath, err)
			}
		}
	}

	return nil
}