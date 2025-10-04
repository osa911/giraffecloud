package service

import (
	"context"
	"fmt"
	"giraffecloud/internal/logging"
	"giraffecloud/internal/tunnel"
	"sync"
	"time"
)

// AutoUpdateService handles automatic client updates in the background
type AutoUpdateService struct {
	logger          *logging.Logger
	updater         *UpdaterService
	config          *tunnel.AutoUpdateConfig
	ticker          *time.Ticker
	stopChan        chan struct{}
	running         bool
	mutex           sync.RWMutex
	connectionState ConnectionStateProvider
	serviceManager  ServiceManagerProvider
}

// ConnectionStateProvider interface for getting tunnel connection state
type ConnectionStateProvider interface {
	IsConnected() bool
	GetConnectionCount() int
	PreserveState() (interface{}, error)
	RestoreState(state interface{}) error
}

// ServiceManagerProvider interface for managing the system service
type ServiceManagerProvider interface {
	IsRunning() (bool, error)
	Restart() error
	Stop() error
	Start() error
}

// NewAutoUpdateService creates a new auto-update service
func NewAutoUpdateService(config *tunnel.AutoUpdateConfig, connectionState ConnectionStateProvider, serviceManager ServiceManagerProvider) (*AutoUpdateService, error) {
	logger := logging.GetGlobalLogger()

	updater, err := NewUpdaterService(config.DownloadURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create updater service: %w", err)
	}

	return &AutoUpdateService{
		logger:          logger,
		updater:         updater,
		config:          config,
		stopChan:        make(chan struct{}),
		connectionState: connectionState,
		serviceManager:  serviceManager,
	}, nil
}

// Start begins the auto-update background service
func (s *AutoUpdateService) Start(ctx context.Context, serverURL string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return fmt.Errorf("auto-update service is already running")
	}

	if !s.config.Enabled {
		s.logger.Info("Auto-update is disabled in configuration")
		return nil
	}

	s.logger.Info("Starting auto-update service (check interval: %v)", s.config.CheckInterval)
	s.ticker = time.NewTicker(s.config.CheckInterval)
	s.running = true

	go s.run(ctx, serverURL)

	return nil
}

// Stop stops the auto-update background service
func (s *AutoUpdateService) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Info("Stopping auto-update service")
	close(s.stopChan)

	if s.ticker != nil {
		s.ticker.Stop()
	}

	s.running = false
	return nil
}

// IsRunning returns whether the service is currently running
func (s *AutoUpdateService) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.running
}

// run is the main background loop
func (s *AutoUpdateService) run(ctx context.Context, serverURL string) {
	defer func() {
		s.mutex.Lock()
		s.running = false
		s.mutex.Unlock()
	}()

	// Do an initial check after a short delay (one-shot)
	initial := time.After(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Auto-update service stopped due to context cancellation")
			return
		case <-s.stopChan:
			s.logger.Info("Auto-update service stopped")
			return
		case <-initial:
			s.checkAndUpdate(serverURL, false)
			// Disable further initial events
			initial = nil
		case <-s.ticker.C:
			s.checkAndUpdate(serverURL, false)
		}
	}
}

// checkAndUpdate performs the actual update check and installation
func (s *AutoUpdateService) checkAndUpdate(serverURL string, force bool) {
	s.logger.Debug("Checking for updates...")

	// Check if we're in the update window
	if !force && !s.isInUpdateWindow() {
		s.logger.Info("Not in update window, skipping automatic update check")
		return
	}

	// Check for updates
	updateInfo, err := s.updater.CheckForUpdates(serverURL)
	if err != nil {
		s.logger.Error("Failed to check for updates: %v", err)
		return
	}

	if updateInfo == nil {
		s.logger.Info("No updates available")
		return
	}

	s.logger.Info("Update available: %s -> %s (Required: %v)",
		updateInfo.CurrentVersion, updateInfo.Version, updateInfo.IsRequired)

	// Only proceed with automatic installation if configured
	if !updateInfo.IsRequired && s.config.RequiredOnly {
		s.logger.Info("Update is not required and auto-update is set to required-only mode")
		return
	}

	// Perform the update
	if err := s.performUpdate(updateInfo); err != nil {
		s.logger.Error("Failed to perform update: %v", err)
		return
	}

	s.logger.Info("Auto-update completed successfully!")
}

// performUpdate handles the actual update process with connection preservation
func (s *AutoUpdateService) performUpdate(updateInfo *UpdateInfo) error {
	s.logger.Info("Starting automatic update process...")

	var preservedState interface{}
	var err error

	// Preserve connection state if configured
	if s.config.PreserveConnection && s.connectionState != nil {
		if s.connectionState.IsConnected() {
			s.logger.Info("Preserving connection state before update...")
			preservedState, err = s.connectionState.PreserveState()
			if err != nil {
				s.logger.Warn("Failed to preserve connection state: %v", err)
			} else {
				s.logger.Info("Connection state preserved successfully")
			}
		}
	}

	// Download the update
	s.logger.Info("Downloading update...")
	downloadPath, err := s.updater.DownloadUpdate(updateInfo)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}

	// Install the update
	s.logger.Info("Installing update...")
	if err := s.updater.InstallUpdate(downloadPath); err != nil {
		return fmt.Errorf("failed to install update: %w", err)
	}

	// Restart service if configured and running as a service
	if s.config.RestartService && s.serviceManager != nil {
		if isRunning, err := s.serviceManager.IsRunning(); err == nil && isRunning {
			s.logger.Info("Restarting service after update...")
			if err := s.restartServiceGracefully(); err != nil {
				s.logger.Error("Failed to restart service: %v", err)
				// Continue anyway, the update is already installed
			}
		}
	}

	// Restore connection state if we preserved it
	if preservedState != nil && s.connectionState != nil {
		s.logger.Info("Attempting to restore connection state...")

		// Give the new version a moment to start
		time.Sleep(3 * time.Second)

		if err := s.connectionState.RestoreState(preservedState); err != nil {
			s.logger.Warn("Failed to restore connection state: %v", err)
		} else {
			s.logger.Info("Connection state restored successfully")
		}
	}

	// Cleanup old backups
	if err := s.updater.CleanupOldBackups(); err != nil {
		s.logger.Warn("Failed to cleanup old backups: %v", err)
	}

	return nil
}

// restartServiceGracefully restarts the service with a grace period
func (s *AutoUpdateService) restartServiceGracefully() error {
	s.logger.Info("Gracefully restarting service...")

	// CRITICAL: When running inside the service, we need to detach the restart
	// to avoid the race condition where the service terminates itself before
	// the restart command completes.
	go func() {
		// Small delay to allow this function to return and logs to flush
		time.Sleep(1 * time.Second)

		s.logger.Info("Executing detached service restart...")
		if err := s.serviceManager.Restart(); err != nil {
			s.logger.Error("Detached service restart failed: %v", err)
		} else {
			s.logger.Info("Detached service restart command issued successfully")
		}
	}()

	s.logger.Info("Service restart scheduled - process will terminate shortly")
	return nil
}

// isInUpdateWindow checks if current time is within the configured update window
func (s *AutoUpdateService) isInUpdateWindow() bool {
	if s.config.UpdateWindow == nil {
		return true // No window configured, always allow updates
	}

	window := s.config.UpdateWindow
	now := time.Now()

	// Convert to the configured timezone
	if window.Timezone != "" {
		if loc, err := time.LoadLocation(window.Timezone); err == nil {
			now = now.In(loc)
		}
	}

	currentHour := now.Hour()

	// Handle same-day window
	if window.StartHour <= window.EndHour {
		return currentHour >= window.StartHour && currentHour < window.EndHour
	}

	// Handle overnight window (e.g., 22:00 to 06:00)
	return currentHour >= window.StartHour || currentHour < window.EndHour
}

// CheckNow forces an immediate update check
func (s *AutoUpdateService) CheckNow(serverURL string) {
	s.logger.Info("Performing immediate update check...")
	s.checkAndUpdate(serverURL, true)
}

// GetStatus returns the current status of the auto-update service
func (s *AutoUpdateService) GetStatus() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	status := map[string]interface{}{
		"running":        s.running,
		"enabled":        s.config.Enabled,
		"check_interval": s.config.CheckInterval.String(),
		"required_only":  s.config.RequiredOnly,
	}

	if s.config.UpdateWindow != nil {
		status["update_window"] = map[string]interface{}{
			"start_hour": s.config.UpdateWindow.StartHour,
			"end_hour":   s.config.UpdateWindow.EndHour,
			"timezone":   s.config.UpdateWindow.Timezone,
			"in_window":  s.isInUpdateWindow(),
		}
	}

	return status
}
