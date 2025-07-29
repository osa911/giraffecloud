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
)

// These variables are set at build time via -ldflags
var (
	Version   = "dev"     // Set via: -ldflags "-X giraffecloud/internal/version.Version=v1.0.0"
	BuildTime = "unknown" // Set via: -ldflags "-X giraffecloud/internal/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
	GitCommit = "unknown" // Set via: -ldflags "-X giraffecloud/internal/version.GitCommit=$(git rev-parse HEAD)"
)

// BuildInfo contains comprehensive build information
type BuildInfo struct {
	Version     string `json:"version"`
	BuildTime   string `json:"build_time"`
	GitCommit   string `json:"git_commit"`
	GoVersion   string `json:"go_version"`
	Platform    string `json:"platform"`
	Compiler    string `json:"compiler"`
}

// ServerVersionInfo contains version information from the server
type ServerVersionInfo struct {
	ServerVersion        string `json:"server_version"`
	BuildTime           string `json:"build_time"`
	GitCommit           string `json:"git_commit"`
	GoVersion           string `json:"go_version"`
	Platform            string `json:"platform"`
	MinimumClientVersion string `json:"minimum_client_version"`
	ClientVersion       string `json:"client_version,omitempty"`
	UpdateAvailable     bool   `json:"update_available"`
	UpdateRequired      bool   `json:"update_required"`
	DownloadURL         string `json:"download_url"`
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

	return fmt.Sprintf("%s (built %s, commit %s)",
		buildInfo.Version,
		buildTime.Format("2006-01-02 15:04:05 UTC"),
		buildInfo.GitCommit[:8])
}

// CheckServerVersion checks the server version and compares it with the client
func CheckServerVersion(serverURL string) (*ServerVersionInfo, error) {
	// Remove any trailing slashes and add the version endpoint
	serverURL = strings.TrimRight(serverURL, "/")
	versionURL := serverURL + "/api/v1/tunnels/version"

	// Add client version as query parameter
	versionURL += "?client_version=" + Version

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make request
	resp, err := client.Get(versionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to check server version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse JSON response
	var versionInfo ServerVersionInfo
	if err := json.Unmarshal(body, &versionInfo); err != nil {
		return nil, fmt.Errorf("failed to parse version response: %w", err)
	}

	return &versionInfo, nil
}

// CompareVersions compares two semantic version strings
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
	return CompareVersions(clientVersion, minimumVersion) < 0
}