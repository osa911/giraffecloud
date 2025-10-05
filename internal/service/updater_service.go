package service

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/tunnel"
	"giraffecloud/internal/version"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// UpdaterService handles client auto-updates
type UpdaterService struct {
	logger          *logging.Logger
	downloadBaseURL string
	currentExePath  string
	backupDir       string
	tempDir         string
	// OnPrivilegeEscalation is called right before attempting sudo escalation (if any)
	OnPrivilegeEscalation func()
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

	// Resolve symlinks so we replace the real target, not the symlink itself
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil && resolved != "" {
		exePath = resolved
	}

	// Create backup and temp directories under the same base as config
	// Prefer explicit GIRAFFECLOUD_HOME/config dir for consistency
	baseDir, err := tunnel.GetConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to determine config directory: %w", err)
	}

	backupDir := filepath.Join(baseDir, "backups")
	tempDir := filepath.Join(baseDir, "temp")

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
	// Respect release channel from local config
	channel := tunnel.ResolveReleaseChannel()

	versionInfo, err := version.CheckServerVersionWithChannel(serverURL, channel)
	if err != nil {
		return nil, fmt.Errorf("failed to check server version: %w", err)
	}

	if !versionInfo.UpdateAvailable {
		u.logger.Info("No updates available. Current version: %s, Latest version: %s",
			version.Version, versionInfo.LatestVersion)
		return nil, nil
	}

	// Get platform info
	platform := runtime.GOOS
	arch := runtime.GOARCH

	// Construct filename with correct format
	ext := ".tar.gz"
	if platform == "windows" {
		ext = ".zip"
	}

	// Construct filename using short version (e.g., v0.0.0-test.dcbb755)
	filename := fmt.Sprintf("giraffecloud_%s_%s_%s%s",
		platform,
		arch,
		versionInfo.ShortVersion,
		ext)

	// Construct full URL using base URL and release tag
	// For stable: /releases/latest/download/giraffecloud_darwin_arm64_v1.0.0.tar.gz
	// For beta/test: /releases/download/test-dcbb755/giraffecloud_darwin_arm64_v0.0.0-test.dcbb755.tar.gz
	var downloadURL string
	if versionInfo.Channel == "stable" {
		// Expect DB to provide /releases/latest/download as base. Append filename.
		downloadURL = fmt.Sprintf("%s/%s", strings.TrimRight(versionInfo.DownloadURL, "/"), filename)
	} else {
		base := strings.TrimRight(versionInfo.DownloadURL, "/")
		if strings.Contains(base, "/releases/download/") {
			// DB already points to specific tag path. Append filename only.
			downloadURL = fmt.Sprintf("%s/%s", base, filename)
		} else {
			// Construct from generic /releases base
			base = strings.TrimSuffix(base, "/releases")
			downloadURL = fmt.Sprintf("%s/download/%s/%s", base, versionInfo.ReleaseTag, filename)
		}
	}

	// Determine checksum URL (prefer release-level checksums.txt)
	var checksumURL string
	if versionInfo.Channel == "stable" {
		// For latest stable, checksums.txt should also be available under latest/download
		checksumURL = fmt.Sprintf("%s/%s", strings.TrimRight(versionInfo.DownloadURL, "/"), "checksums.txt")
	} else {
		base := strings.TrimRight(versionInfo.DownloadURL, "/")
		if strings.Contains(base, "/releases/download/") {
			checksumURL = fmt.Sprintf("%s/%s", base, "checksums.txt")
		} else {
			base = strings.TrimSuffix(versionInfo.DownloadURL, "/releases")
			checksumURL = fmt.Sprintf("%s/download/%s/%s", base, versionInfo.ReleaseTag, "checksums.txt")
		}
	}

	updateInfo := &UpdateInfo{
		Version:        versionInfo.LatestVersion,
		DownloadURL:    downloadURL,
		ChecksumURL:    checksumURL,
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
		var checksumPath string
		if strings.HasSuffix(updateInfo.ChecksumURL, "checksums.txt") {
			checksumPath = filepath.Join(u.tempDir, "checksums.txt")
			if err := u.downloadFile(updateInfo.ChecksumURL, checksumPath); err != nil {
				u.logger.Warn("Failed to download checksums.txt: %v", err)
			} else {
				if err := u.verifyChecksumFromList(downloadPath, checksumPath); err != nil {
					return "", fmt.Errorf("checksum verification failed: %w", err)
				}
				u.logger.Info("Checksum verification passed")
			}
		} else {
			checksumPath = downloadPath + ".sha256"
			if err := u.downloadFile(updateInfo.ChecksumURL, checksumPath); err != nil {
				u.logger.Warn("Failed to download checksum file: %v", err)
			} else {
				if err := u.verifyChecksum(downloadPath, checksumPath); err != nil {
					return "", fmt.Errorf("checksum verification failed: %w", err)
				}
				u.logger.Info("Checksum verification passed")
			}
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
		// Restore backup on extraction failure
		if restoreErr := u.restoreBackup(backupPath); restoreErr != nil {
			u.logger.Error("Failed to restore backup after extraction failure: %v", restoreErr)
		}
		return fmt.Errorf("failed to extract update: %w", err)
	}

	// Find the new executable
	newExePath, err := u.findExecutable(extractPath)
	if err != nil {
		// Restore backup if we can't find the executable
		if restoreErr := u.restoreBackup(backupPath); restoreErr != nil {
			u.logger.Error("Failed to restore backup after missing executable: %v", restoreErr)
		}
		return fmt.Errorf("failed to find new executable: %w", err)
	}

	// CRITICAL: Verify the new binary is valid before replacing
	if err := u.verifyNewBinary(newExePath); err != nil {
		// Restore backup if binary verification fails
		if restoreErr := u.restoreBackup(backupPath); restoreErr != nil {
			u.logger.Error("Failed to restore backup after binary verification failure: %v", restoreErr)
		}
		return fmt.Errorf("new binary verification failed: %w", err)
	}

	// Replace current executable (attempt graceful stop if running as a service or in-place)
	if err := u.prepareForReplacement(); err != nil {
		u.logger.Warn("Pre-replacement preparation failed: %v", err)
	}

	if err := u.replaceExecutable(newExePath); err != nil {
		// Attempt privilege escalation for permission errors on Unix-like systems when interactive
		if (runtime.GOOS == "linux" || runtime.GOOS == "darwin") && u.shouldAttemptSudo(err) {
			// Notify UI (e.g., to stop spinners) before sudo escalation
			if u.OnPrivilegeEscalation != nil {
				u.OnPrivilegeEscalation()
			}
			u.logger.Warn("Permission issue detected while replacing executable. Attempting sudo install...")
			u.logger.Info("Sudo required to update: %s", u.currentExePath)
			u.logger.Info("If prompted, please enter your system password to continue.")
			u.logger.Info("Manual command: sudo install -m 0755 %q %q", newExePath, u.currentExePath)
			if sudoErr := u.installWithSudo(newExePath); sudoErr == nil {
				u.logger.Info("Replaced executable via sudo successfully")
			} else {
				// Try to restore backup
				if restoreErr := u.restoreBackup(backupPath); restoreErr != nil {
					u.logger.Error("Failed to restore backup after failed update: %v", restoreErr)
				}
				return fmt.Errorf("failed to replace executable (sudo fallback failed): %w", sudoErr)
			}
		} else {
			// Try to restore backup
			if restoreErr := u.restoreBackup(backupPath); restoreErr != nil {
				u.logger.Error("Failed to restore backup after failed update: %v", restoreErr)
			}
			return fmt.Errorf("failed to replace executable: %w", err)
		}
	}

	u.logger.Info("Update installed successfully!")

	// Apply original ownership and best-effort labels
	if info, err := os.Stat(u.currentExePath); err == nil {
		_ = u.applyPostInstallAttributes(info)
	}

	// Clean up
	os.RemoveAll(downloadPath)
	os.RemoveAll(extractPath)

	return nil
}

// prepareForReplacement tries to reduce chances of ETXTBSY by signaling the current process
// Note: Best-effort; actual service management is handled by AutoUpdateService when available
func (u *UpdaterService) prepareForReplacement() error {
	// If running as same binary, try to close file descriptors via self-symlink open (no-op here)
	// Placeholder for future enhancements (e.g., check PID file, systemd, etc.)
	return nil
}

// applyPostInstallAttributes preserves owner/group and best-effort labels

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

// verifyChecksumFromList verifies checksum using a checksums.txt file (sha256sum format)
func (u *UpdaterService) verifyChecksumFromList(filePath, checksumsListPath string) error {
	listData, err := os.ReadFile(checksumsListPath)
	if err != nil {
		return err
	}
	wantedFile := filepath.Base(filePath)
	lines := strings.Split(string(listData), "\n")
	var expected string
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			name := fields[len(fields)-1]
			// lines may contain either plain filename or prefixed with * for binary mode in some tools
			name = strings.TrimPrefix(name, "*")
			if name == wantedFile {
				expected = fields[0]
				break
			}
		}
	}
	if expected == "" {
		return fmt.Errorf("no checksum entry found for %s", wantedFile)
	}
	// Compute actual
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return err
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
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
	// Look for the binary directly or in a top-level directory
	binaryName := "giraffecloud"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	// Try direct path first
	directPath := filepath.Join(extractPath, binaryName)
	if info, err := os.Stat(directPath); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
		return directPath, nil
	}

	// Try in giraffecloud directory (fallback for some archive formats)
	dirPath := filepath.Join(extractPath, "giraffecloud", binaryName)
	if info, err := os.Stat(dirPath); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
		return dirPath, nil
	}

	return "", fmt.Errorf("executable not found in archive (tried %s and %s)", directPath, dirPath)
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
		// On Unix systems, prefer atomic rename in destination directory to avoid "text file busy"
		destDir := filepath.Dir(u.currentExePath)
		tempDest := filepath.Join(destDir, ".giraffecloud.new")

		// Copy new executable into destination directory first (ensures same filesystem)
		if err := u.copyFile(newExePath, tempDest); err != nil {
			return err
		}

		// Ensure executable bit is set (preserve from src)
		if info, err := os.Stat(newExePath); err == nil {
			_ = os.Chmod(tempDest, info.Mode())
		}

		// Atomically replace the current executable
		if err := os.Rename(tempDest, u.currentExePath); err != nil {
			// Cleanup temp file on failure
			_ = os.Remove(tempDest)
			return err
		}
	}

	return nil
}

// shouldAttemptSudo returns true if the error appears to be a permission issue and we are in an interactive shell
func (u *UpdaterService) shouldAttemptSudo(err error) bool {
	if err == nil {
		return false
	}
	// Permission heuristics
	errStr := strings.ToLower(err.Error())
	if !(strings.Contains(errStr, "permission") || strings.Contains(errStr, "operation not permitted") || strings.Contains(errStr, "read-only file system")) {
		return false
	}
	// Interactive check: stdin is a TTY so sudo can prompt
	if fi, statErr := os.Stdin.Stat(); statErr == nil {
		if (fi.Mode() & os.ModeCharDevice) == 0 {
			return false
		}
	}
	// sudo must be available
	if _, lookErr := exec.LookPath("sudo"); lookErr != nil {
		return false
	}
	return true
}

// installWithSudo replaces the executable using sudo install
func (u *UpdaterService) installWithSudo(newExePath string) error {
	cmd := exec.Command("sudo", "install", "-m", "0755", newExePath, u.currentExePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
	u.logger.Info("Restoring backup from: %s", backupPath)
	err := u.copyFile(backupPath, u.currentExePath)
	
	// If regular copy fails due to permissions, try with sudo
	if err != nil && u.shouldAttemptSudo(err) {
		u.logger.Warn("Permission issue during backup restoration, attempting with sudo...")
		// Use sudo install to restore the backup
		cmd := exec.Command("sudo", "install", "-m", "0755", backupPath, u.currentExePath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if sudoErr := cmd.Run(); sudoErr != nil {
			return fmt.Errorf("failed to restore backup (even with sudo): original error: %w, sudo error: %v", err, sudoErr)
		}
		u.logger.Info("Backup restored successfully via sudo")
		return nil
	}
	
	return err
}

// verifyNewBinary verifies the new binary is valid before installation
func (u *UpdaterService) verifyNewBinary(binaryPath string) error {
	// Check if file exists
	info, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("binary file not found: %w", err)
	}

	// Check if it's a regular file
	if !info.Mode().IsRegular() {
		return fmt.Errorf("binary is not a regular file")
	}

	// Check if it has execute permissions
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("binary is not executable")
	}

	// Try to execute with --version flag (quick sanity check)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it's a context timeout
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("binary verification timed out (might be hung or corrupted)")
		}
		return fmt.Errorf("binary verification failed (exit code or crash): %w\nOutput: %s", err, string(output))
	}

	u.logger.Debug("Binary verification passed: %s", binaryPath)
	return nil
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
