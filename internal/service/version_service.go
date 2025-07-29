package service

import (
	"context"
	"fmt"
	"giraffecloud/internal/db/ent"
	"giraffecloud/internal/db/ent/clientversion"
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
	ServerVersion        string                 `json:"server_version"`
	BuildTime           string                 `json:"build_time"`
	GitCommit           string                 `json:"git_commit"`
	GoVersion           string                 `json:"go_version"`
	Platform            string                 `json:"platform"`
	MinimumClientVersion string                 `json:"minimum_client_version"`
	LatestClientVersion  string                 `json:"latest_client_version"`
	ClientVersion       string                 `json:"client_version,omitempty"`
	UpdateAvailable     bool                   `json:"update_available"`
	UpdateRequired      bool                   `json:"update_required"`
	DownloadURL         string                 `json:"download_url"`
	ReleaseNotes        string                 `json:"release_notes,omitempty"`
	Channel             string                 `json:"channel"`
	ForceUpdate         bool                   `json:"force_update"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
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

	// Build response
	response := &ClientVersionInfo{
		ServerVersion:        buildInfo.Version,
		BuildTime:           buildInfo.BuildTime,
		GitCommit:           buildInfo.GitCommit,
		GoVersion:           buildInfo.GoVersion,
		Platform:            buildInfo.Platform,
		MinimumClientVersion: clientVersionConfig.MinimumVersion,
		LatestClientVersion:  clientVersionConfig.LatestVersion,
		DownloadURL:         clientVersionConfig.DownloadURL,
		ReleaseNotes:        clientVersionConfig.ReleaseNotes,
		Channel:             clientVersionConfig.Channel,
		ForceUpdate:         clientVersionConfig.ForceUpdate,
		Metadata:            clientVersionConfig.Metadata,
	}

	// Add client-specific information if version provided
	if clientVersion != "" {
		response.ClientVersion = clientVersion
		response.UpdateAvailable = version.IsUpdateAvailable(clientVersion, clientVersionConfig.LatestVersion)
		response.UpdateRequired = version.IsUpdateRequired(clientVersion, clientVersionConfig.MinimumVersion)
	}

	return response, nil
}

// getClientVersionConfig retrieves version config from database
func (v *VersionService) getClientVersionConfig(ctx context.Context, channel, platform, arch string) (*ent.ClientVersion, error) {
	// Try exact match first
	config, err := v.db.ClientVersion.Query().
		Where(
			clientversion.Channel(channel),
			clientversion.Platform(platform),
			clientversion.Arch(arch),
		).
		Only(ctx)

	if err == nil {
		return config, nil
	}

	// Try platform-specific, any arch
	config, err = v.db.ClientVersion.Query().
		Where(
			clientversion.Channel(channel),
			clientversion.Platform(platform),
			clientversion.Arch("all"),
		).
		Only(ctx)

	if err == nil {
		return config, nil
	}

	// Try any platform, specific arch
	config, err = v.db.ClientVersion.Query().
		Where(
			clientversion.Channel(channel),
			clientversion.Platform("all"),
			clientversion.Arch(arch),
		).
		Only(ctx)

	if err == nil {
		return config, nil
	}

	// Try generic config for this channel
	config, err = v.db.ClientVersion.Query().
		Where(
			clientversion.Channel(channel),
			clientversion.Platform("all"),
			clientversion.Arch("all"),
		).
		Only(ctx)

	return config, err
}

// getFallbackVersionInfo returns fallback version info when database is unavailable
func (v *VersionService) getFallbackVersionInfo(buildInfo version.BuildInfo, clientVersion, channel string) *ClientVersionInfo {
	response := &ClientVersionInfo{
		ServerVersion:        buildInfo.Version,
		BuildTime:           buildInfo.BuildTime,
		GitCommit:           buildInfo.GitCommit,
		GoVersion:           buildInfo.GoVersion,
		Platform:            buildInfo.Platform,
		MinimumClientVersion: "v1.0.0",
		LatestClientVersion:  buildInfo.Version,
		DownloadURL:         "https://github.com/osa911/giraffecloud/releases/latest",
		Channel:             channel,
		ForceUpdate:         false,
	}

	if clientVersion != "" {
		response.ClientVersion = clientVersion
		response.UpdateAvailable = version.IsUpdateAvailable(clientVersion, buildInfo.Version)
		response.UpdateRequired = version.IsUpdateRequired(clientVersion, "v1.0.0")
	}

	return response
}

// UpdateClientVersionConfig updates or creates client version configuration
func (v *VersionService) UpdateClientVersionConfig(ctx context.Context, config ClientVersionConfigUpdate) error {
	// Check if config exists
	existing, err := v.db.ClientVersion.Query().
		Where(
			clientversion.Channel(config.Channel),
			clientversion.Platform(config.Platform),
			clientversion.Arch(config.Arch),
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
		GroupBy(clientversion.FieldChannel).
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
			MinimumVersion:    "v1.0.0",
			DownloadURL:       "https://github.com/osa911/giraffecloud/releases/latest",
			ReleaseNotes:      "Stable release",
			AutoUpdateEnabled: true,
			ForceUpdate:       false,
			Metadata:          map[string]interface{}{"release_type": "stable"},
		},
		{
			Channel:           "beta",
			Platform:          "all",
			Arch:              "all",
			LatestVersion:     version.Version,
			MinimumVersion:    "v1.0.0",
			DownloadURL:       "https://github.com/osa911/giraffecloud/releases",
			ReleaseNotes:      "Beta release",
			AutoUpdateEnabled: false,
			ForceUpdate:       false,
			Metadata:          map[string]interface{}{"release_type": "beta"},
		},
		{
			Channel:           "test",
			Platform:          "all",
			Arch:              "all",
			LatestVersion:     version.Version,
			MinimumVersion:    "v1.0.0",
			DownloadURL:       "https://github.com/osa911/giraffecloud/releases",
			ReleaseNotes:      "Test release",
			AutoUpdateEnabled: false,
			ForceUpdate:       false,
			Metadata:          map[string]interface{}{"release_type": "test"},
		},
	}

	for _, config := range defaultConfigs {
		// Check if exists
		existing, err := v.db.ClientVersion.Query().
			Where(
				clientversion.Channel(config.Channel),
				clientversion.Platform(config.Platform),
				clientversion.Arch(config.Arch),
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