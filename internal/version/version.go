package version

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"giraffecloud/internal/logging"
)

// These variables are set at build time via -ldflags
var (
	Version   = "dev"     // Set via: -ldflags "-X giraffecloud/internal/version.Version=v1.0.0"
	BuildTime = "unknown" // Set via: -ldflags "-X giraffecloud/internal/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
	GitCommit = "unknown" // Set via: -ldflags "-X giraffecloud/internal/version.GitCommit=$(git rev-parse HEAD)"
)

// BuildInfo contains comprehensive build information
type BuildInfo struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
	GitCommit string `json:"git_commit"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
	Compiler  string `json:"compiler"`
}

// ClientVersionInfo contains version information for the client
type ClientVersionInfo struct {
	// Version information
	LatestVersion   string `json:"latest_version"`   // Latest available version
	MinimumVersion  string `json:"minimum_version"`  // Minimum required version
	CurrentVersion  string `json:"current_version"`  // Current client version (from request)
	UpdateAvailable bool   `json:"update_available"` // Whether an update is available
	UpdateRequired  bool   `json:"update_required"`  // Whether an update is required

	// Release information
	Channel      string `json:"channel"`       // "stable", "beta", or "test"
	ReleaseTag   string `json:"release_tag"`   // e.g., "test-dcbb755"
	ShortVersion string `json:"short_version"` // e.g., "v0.0.0-test.dcbb755"
	DownloadURL  string `json:"download_url"`  // Base URL for downloads
	ReleaseNotes string `json:"release_notes"` // Release notes for this version
}

// GetBuildInfo returns complete build information
func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   Version,
		BuildTime: BuildTime,
		GitCommit: GitCommit,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
		Compiler:  runtime.Compiler,
	}
}

// GetVersionString returns a formatted version string
func GetVersionString() string {
	if BuildTime == "unknown" {
		return Version
	}

	buildTime, err := time.Parse(time.RFC3339, BuildTime)
	if err != nil {
		return Version
	}

	return Version + " (built " + buildTime.Format("2006-01-02 15:04:05 UTC") + ")"
}

// Info returns a formatted version info string for CLI output
func Info() string {
	buildInfo := GetBuildInfo()
	if buildInfo.BuildTime == "unknown" {
		return fmt.Sprintf("%s (development build)", buildInfo.Version)
	}

	buildTime, err := time.Parse(time.RFC3339, buildInfo.BuildTime)
	if err != nil {
		return fmt.Sprintf("%s (built %s)", buildInfo.Version, buildInfo.BuildTime)
	}

	commitInfo := buildInfo.GitCommit
	if commitInfo != "unknown" && len(commitInfo) >= 8 {
		commitInfo = commitInfo[:8]
	}
	return fmt.Sprintf("%s (built %s, commit %s)",
		buildInfo.Version,
		buildTime.Format("2006-01-02 15:04:05 UTC"),
		commitInfo)
}

// CheckServerVersion checks for client updates from the version service (stable channel by default)
func CheckServerVersion(serverURL string) (*ClientVersionInfo, error) {
	return CheckServerVersionWithChannel(serverURL, "")
}

// CheckServerVersionWithChannel checks for client updates and allows specifying a release channel
func CheckServerVersionWithChannel(serverURL string, channel string) (*ClientVersionInfo, error) {
	// Remove any trailing slashes and add the version endpoint
	serverURL = strings.TrimRight(serverURL, "/")
	versionURL := serverURL + "/api/v1/tunnels/version"

	// Add client version and platform info as query parameters
	params := map[string]string{
		"client_version": Version,
		"platform":       runtime.GOOS,
		"arch":           runtime.GOARCH,
	}
	// Build query string including optional channel
	if channel != "" {
		versionURL += fmt.Sprintf("?client_version=%s&platform=%s&arch=%s&channel=%s", Version, runtime.GOOS, runtime.GOARCH, channel)
	} else {
		versionURL += fmt.Sprintf("?client_version=%s&platform=%s&arch=%s", Version, runtime.GOOS, runtime.GOARCH)
	}

	// Log request details
	logger := logging.GetGlobalLogger()
	logger.Debug("🔍 Checking version with parameters:")
	logger.Debug("   URL: %s", versionURL)
	logger.Debug("   Client Version: %s", params["client_version"])
	logger.Debug("   Platform: %s", params["platform"])
	logger.Debug("   Architecture: %s", params["arch"])
	if channel != "" {
		logger.Debug("   Channel: %s", channel)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make request
	resp, err := client.Get(versionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to check version service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("version service returned status %d", resp.StatusCode)
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Log raw response for debugging
	logger.Debug("📥 Received version response:")
	logger.Debug("   Status: %d", resp.StatusCode)
	logger.Debug("   Body: %s", string(body))

	// Parse JSON response
	var versionInfo ClientVersionInfo
	if err := json.Unmarshal(body, &versionInfo); err != nil {
		return nil, fmt.Errorf("failed to parse version response: %w", err)
	}

	// Log parsed version info
	logger.Debug("✅ Parsed version info:")
	logger.Debug("   Latest Version: %s", versionInfo.LatestVersion)
	logger.Debug("   Minimum Version: %s", versionInfo.MinimumVersion)
	logger.Debug("   Channel: %s", versionInfo.Channel)
	logger.Debug("   Update Available: %v", versionInfo.UpdateAvailable)
	logger.Debug("   Update Required: %v", versionInfo.UpdateRequired)

	return &versionInfo, nil
}

// CompareVersions compares two version strings
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func CompareVersions(v1, v2 string) int {
	// Remove 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// Handle special cases
	if v1 == v2 {
		return 0
	}
	if v1 == "dev" || v1 == "unknown" {
		return -1 // Development versions are considered older
	}
	if v2 == "dev" || v2 == "unknown" {
		return 1
	}

	// Extract commit hashes from test/beta versions
	// Formats:
	//  - test builds: "test-<hash>-<n>-g<hash2>" or "v0.0.0-test.<...>.<commit>"
	//  - beta builds: "vX.Y.Z-beta.<...>" (ordering may be ambiguous across different schemes)
	hash1 := extractCommitHash(v1)
	hash2 := extractCommitHash(v2)

	// If both are pre-release (test) builds, compare by commit hash when possible.
	// If hashes are equal -> equal; if different -> server is considered newer.
	isPre1 := strings.Contains(v1, "-test") || strings.HasPrefix(v1, "test-")
	isPre2 := strings.Contains(v2, "-test") || strings.HasPrefix(v2, "test-")
	if isPre1 && isPre2 {
		if hash1 != "" && hash2 != "" {
			if hash1 == hash2 {
				return 0
			}
			return -1
		}
		// Fallback: if formats differ and we cannot extract both hashes, treat different strings as update
		if v1 == v2 {
			return 0
		}
		return -1
	}

	// If only one is a test version, consider test as older than a proper/stable version
	if hash1 != "" {
		return -1 // v1 is test version
	}
	if hash2 != "" {
		return 1 // v2 is test version
	}

	// Split versions into parts
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Pad shorter version with zeros
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for len(parts1) < maxLen {
		parts1 = append(parts1, "0")
	}
	for len(parts2) < maxLen {
		parts2 = append(parts2, "0")
	}

	// Compare each part
	for i := 0; i < maxLen; i++ {
		// Parse as integers, handling non-numeric suffixes
		num1 := parseVersionPart(parts1[i])
		num2 := parseVersionPart(parts2[i])

		if num1 < num2 {
			return -1
		}
		if num1 > num2 {
			return 1
		}
	}

	return 0
}

// extractCommitHash extracts commit hash from test version string
func extractCommitHash(version string) string {
	// Format 1a: test-<branch-sha>-<n>-g<commit>
	if strings.HasPrefix(version, "test-") {
		// Try to find trailing g<hash>
		idx := strings.LastIndex(version, "-g")
		if idx >= 0 && idx+2 < len(version) {
			return version[idx+2:]
		}
		// Fallback: second segment after test-
		parts := strings.Split(version, "-")
		if len(parts) > 1 {
			return parts[1]
		}
	}

	// Format 2: v0.0.0-test.<commit>
	if strings.Contains(version, "-test.") {
		parts := strings.Split(version, ".")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	return ""
}

// parseVersionPart extracts the numeric part from a version component
func parseVersionPart(part string) int {
	// Find the first non-digit character
	i := 0
	for i < len(part) && (part[i] >= '0' && part[i] <= '9') {
		i++
	}

	if i == 0 {
		return 0
	}

	num, err := strconv.Atoi(part[:i])
	if err != nil {
		return 0
	}

	return num
}

// IsUpdateAvailable checks if an update is available
func IsUpdateAvailable(clientVersion, serverVersion string) bool {
	return CompareVersions(clientVersion, serverVersion) < 0
}

// IsUpdateRequired checks if an update is required
func IsUpdateRequired(clientVersion, minimumVersion string) bool {
	// Treat empty or v0.0.0 minimum as no requirement
	if minimumVersion == "" || minimumVersion == "v0.0.0" || minimumVersion == "0.0.0" {
		return false
	}
	return CompareVersions(clientVersion, minimumVersion) < 0
}
