package service

import (
	"context"
	"fmt"
	"giraffecloud/internal/db/ent"
	clientVersionEntity "giraffecloud/internal/db/ent/clientversion" // Entity for client version management
	"giraffecloud/internal/logging"
	"giraffecloud/internal/version"
	"runtime"
)

// VersionService handles client version management
type VersionService struct {
	db     *ent.Client
	logger *logging.Logger
}

// ClientVersionInfo represents version information for a specific client
type ClientVersionInfo struct {
	// Version information
	LatestVersion     string                 `json:"latest_version"`      // Latest available version
	MinimumVersion    string                 `json:"minimum_version"`     // Minimum required version
	CurrentVersion    string                 `json:"current_version"`     // Current client version (from request)
	UpdateAvailable   bool                   `json:"update_available"`    // Whether an update is available
	UpdateRequired    bool                   `json:"update_required"`     // Whether an update is required

	// Release information
	Channel           string                 `json:"channel"`             // "stable", "beta", or "test"
	ReleaseTag       string                 `json:"release_tag"`         // e.g., "test-dcbb755"
	ShortVersion     string                 `json:"short_version"`       // e.g., "v0.0.0-test.dcbb755"
	DownloadURL      string                 `json:"download_url"`        // Base URL for downloads
	ReleaseNotes     string                 `json:"release_notes"`       // Release notes for this version
}

// NewVersionService creates a new version service
func NewVersionService(db *ent.Client) *VersionService {
	return &VersionService{
		db:     db,
		logger: logging.GetGlobalLogger(),
	}
}

// GetVersionInfo returns version information for a client
func (v *VersionService) GetVersionInfo(ctx context.Context, clientVersion, channel, platform, arch string) (*ClientVersionInfo, error) {
	// If no channel specified, default to stable
	if channel == "" {
		channel = "stable"
	}

	// Normalize platform and arch
	if platform == "" {
		platform = runtime.GOOS
	}
	if arch == "" {
		arch = runtime.GOARCH
	}

	// Get server build info
	buildInfo := version.GetBuildInfo()

	// Query database for client version configuration
	clientVersionConfig, err := v.getClientVersionConfig(ctx, channel, platform, arch)
	if err != nil {
		v.logger.Warn("Failed to get client version config from database, using defaults: %v", err)
		// Fallback to hardcoded values
		return v.getFallbackVersionInfo(buildInfo, clientVersion, channel), nil
	}

	// Extract release tag and short version from metadata
	var releaseTag, shortVersion string
	if clientVersionConfig.Metadata != nil {
		if rt, ok := clientVersionConfig.Metadata["release_tag"].(string); ok {
			releaseTag = rt
		}
		if sv, ok := clientVersionConfig.Metadata["short_version"].(string); ok {
			shortVersion = sv
		}
	}

	// Build response
	response := &ClientVersionInfo{
		// Version information
		MinimumVersion:    clientVersionConfig.MinimumVersion,
		LatestVersion:     clientVersionConfig.LatestVersion,
		CurrentVersion:    clientVersion,
		UpdateAvailable:   version.IsUpdateAvailable(clientVersion, clientVersionConfig.LatestVersion),
		UpdateRequired:    version.IsUpdateRequired(clientVersion, clientVersionConfig.MinimumVersion),

		// Release information
		Channel:           clientVersionConfig.Channel,
		ReleaseTag:       releaseTag,
		ShortVersion:     shortVersion,
		DownloadURL:      clientVersionConfig.DownloadURL,
		ReleaseNotes:     clientVersionConfig.ReleaseNotes,
	}

	return response, nil
}

// getClientVersionConfig retrieves version config from database
func (v *VersionService) getClientVersionConfig(ctx context.Context, channel, platform, arch string) (*ent.ClientVersion, error) {
	// Try exact match first
	config, err := v.db.ClientVersion.Query().
		Where(
			clientVersionEntity.Channel(channel),
			clientVersionEntity.Platform(platform),
			clientVersionEntity.Arch(arch),
		).
		Only(ctx)

	if err == nil {
		return config, nil
	}

	// Try platform-specific, any arch
	config, err = v.db.ClientVersion.Query().
		Where(
			clientVersionEntity.Channel(channel),
			clientVersionEntity.Platform(platform),
			clientVersionEntity.Arch("all"),
		).
		Only(ctx)

	if err == nil {
		return config, nil
	}

	// Try any platform, specific arch
	config, err = v.db.ClientVersion.Query().
		Where(
			clientVersionEntity.Channel(channel),
			clientVersionEntity.Platform("all"),
			clientVersionEntity.Arch(arch),
		).
		Only(ctx)

	if err == nil {
		return config, nil
	}

	// Try generic config for this channel
	config, err = v.db.ClientVersion.Query().
		Where(
			clientVersionEntity.Channel(channel),
			clientVersionEntity.Platform("all"),
			clientVersionEntity.Arch("all"),
		).
		Only(ctx)

	return config, err
}

// getFallbackVersionInfo returns fallback version info when database is unavailable
func (v *VersionService) getFallbackVersionInfo(buildInfo version.BuildInfo, clientVersion, channel string) *ClientVersionInfo {
	// When database is unavailable, use client's current version as both minimum and latest
	// This means no updates will be required or available until DB is back
	response := &ClientVersionInfo{
		// Version information
		MinimumVersion:    clientVersion, // Use client's version as minimum
		LatestVersion:     clientVersion, // Use client's version as latest
		CurrentVersion:    clientVersion,
		UpdateAvailable:   false, // No updates during DB outage
		UpdateRequired:    false, // No forced updates during DB outage

		// Release information
		Channel:           channel,
		ReleaseTag:       "", // No release info during DB outage
		ShortVersion:     clientVersion,
		DownloadURL:      "", // No downloads during DB outage
		ReleaseNotes:     "Version service temporarily unavailable",
	}

	return response
}

// UpdateClientVersionConfig updates or creates client version configuration
func (v *VersionService) UpdateClientVersionConfig(ctx context.Context, config ClientVersionConfigUpdate) error {
	// Check if config exists
	existing, err := v.db.ClientVersion.Query().
		Where(
			clientVersionEntity.Channel(config.Channel),
			clientVersionEntity.Platform(config.Platform),
			clientVersionEntity.Arch(config.Arch),
		).
		Only(ctx)

	if err != nil && !ent.IsNotFound(err) {
		return fmt.Errorf("failed to query existing config: %w", err)
	}

	if existing != nil {
		// Update existing
		_, err = existing.Update().
			SetLatestVersion(config.LatestVersion).
			SetMinimumVersion(config.MinimumVersion).
			SetDownloadURL(config.DownloadURL).
			SetReleaseNotes(config.ReleaseNotes).
			SetAutoUpdateEnabled(config.AutoUpdateEnabled).
			SetForceUpdate(config.ForceUpdate).
			SetMetadata(config.Metadata).
			Save(ctx)

		if err != nil {
			return fmt.Errorf("failed to update client version config: %w", err)
		}
	} else {
		// Create new
		_, err = v.db.ClientVersion.Create().
			SetID(fmt.Sprintf("%s-%s-%s", config.Channel, config.Platform, config.Arch)).
			SetChannel(config.Channel).
			SetPlatform(config.Platform).
			SetArch(config.Arch).
			SetLatestVersion(config.LatestVersion).
			SetMinimumVersion(config.MinimumVersion).
			SetDownloadURL(config.DownloadURL).
			SetReleaseNotes(config.ReleaseNotes).
			SetAutoUpdateEnabled(config.AutoUpdateEnabled).
			SetForceUpdate(config.ForceUpdate).
			SetMetadata(config.Metadata).
			Save(ctx)

		if err != nil {
			return fmt.Errorf("failed to create client version config: %w", err)
		}
	}

	v.logger.Info("Updated client version config for channel=%s platform=%s arch=%s version=%s",
		config.Channel, config.Platform, config.Arch, config.LatestVersion)

	return nil
}

// ClientVersionConfigUpdate represents a version config update
type ClientVersionConfigUpdate struct {
	Channel           string                 `json:"channel"`
	Platform          string                 `json:"platform"`
	Arch              string                 `json:"arch"`
	LatestVersion     string                 `json:"latest_version"`
	MinimumVersion    string                 `json:"minimum_version"`
	DownloadURL       string                 `json:"download_url"`
	ReleaseNotes      string                 `json:"release_notes"`
	AutoUpdateEnabled bool                   `json:"auto_update_enabled"`
	ForceUpdate       bool                   `json:"force_update"`
	Metadata          map[string]interface{} `json:"metadata"`
}

// GetActiveChannels returns all active release channels
func (v *VersionService) GetActiveChannels(ctx context.Context) ([]string, error) {
	channels, err := v.db.ClientVersion.Query().
		GroupBy(clientVersionEntity.FieldChannel).
		Strings(ctx)

	if err != nil {
		return []string{"stable"}, err
	}

	return channels, nil
}

// InitializeDefaultConfigs creates default version configurations if they don't exist
func (v *VersionService) InitializeDefaultConfigs(ctx context.Context) error {
	defaultConfigs := []ClientVersionConfigUpdate{
		{
			Channel:           "stable",
			Platform:          "all",
			Arch:              "all",
			LatestVersion:     version.Version,
			MinimumVersion:    "v0.0.0", // Most permissive default
			DownloadURL:       "https://github.com/osa911/giraffecloud/releases/latest",
			ReleaseNotes:      "GiraffeCloud CLI Client - Stable Channel",
			AutoUpdateEnabled: true,
			ForceUpdate:       false,
			Metadata:          map[string]interface{}{"release_type": "stable"},
		},
		{
			Channel:           "beta",
			Platform:          "all",
			Arch:              "all",
			LatestVersion:     version.Version,
			MinimumVersion:    "v0.0.0", // Most permissive default
			DownloadURL:       "https://github.com/osa911/giraffecloud/releases",
			ReleaseNotes:      "GiraffeCloud CLI Client - Beta Channel",
			AutoUpdateEnabled: false,
			ForceUpdate:       false,
			Metadata:          map[string]interface{}{"release_type": "beta"},
		},
		{
			Channel:           "test",
			Platform:          "all",
			Arch:              "all",
			LatestVersion:     version.Version,
			MinimumVersion:    "v0.0.0", // Most permissive default
			DownloadURL:       "https://github.com/osa911/giraffecloud/releases",
			ReleaseNotes:      "GiraffeCloud CLI Client - Test Channel",
			AutoUpdateEnabled: false,
			ForceUpdate:       false,
			Metadata:          map[string]interface{}{"release_type": "test"},
		},
	}

	for _, config := range defaultConfigs {
		// Check if exists
		existing, err := v.db.ClientVersion.Query().
			Where(
				clientVersionEntity.Channel(config.Channel),
				clientVersionEntity.Platform(config.Platform),
				clientVersionEntity.Arch(config.Arch),
			).
			Exist(ctx)

		if err != nil {
			return fmt.Errorf("failed to check existing config: %w", err)
		}

		if !existing {
			err = v.UpdateClientVersionConfig(ctx, config)
			if err != nil {
				return fmt.Errorf("failed to create default config: %w", err)
			}
		}
	}

	return nil
}