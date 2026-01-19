package plugins

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"

	"github.com/mantonx/viewra/internal/infrastructure/plugins/github"
	infraplugins "github.com/mantonx/viewra/internal/infrastructure/plugins"
)

const (
	// DefaultCacheTTL is the default time-to-live for the marketplace cache.
	DefaultCacheTTL = 15 * time.Minute
)

// MarketplaceConfig configures the marketplace service.
type MarketplaceConfig struct {
	// Enabled controls whether marketplace features are available.
	Enabled bool

	// CacheTTL is how long to cache the plugin catalog.
	CacheTTL time.Duration

	// PluginDir is where plugins are installed.
	PluginDir string

	// TempDir is for temporary downloads.
	TempDir string
}

// MarketplaceService provides marketplace operations for browsing, installing,
// updating, and uninstalling plugins from GitHub releases.
type MarketplaceService struct {
	config       MarketplaceConfig
	queries      PluginQueries
	manager      *infraplugins.Manager
	githubClient *github.CachedClient
	downloader   *github.Downloader
	logger       *slog.Logger

	// installingMu prevents concurrent installs of the same plugin
	installingMu sync.Map
}

// NewMarketplaceService creates a new marketplace service.
func NewMarketplaceService(
	config MarketplaceConfig,
	queries PluginQueries,
	manager *infraplugins.Manager,
	logger *slog.Logger,
) *MarketplaceService {
	if config.CacheTTL == 0 {
		config.CacheTTL = DefaultCacheTTL
	}

	return &MarketplaceService{
		config:       config,
		queries:      queries,
		manager:      manager,
		githubClient: github.NewCachedClient(logger, config.CacheTTL),
		downloader:   github.NewDownloader(config.PluginDir, config.TempDir, logger),
		logger:       logger,
	}
}

// ListAvailable returns the list of plugins available in the marketplace.
func (s *MarketplaceService) ListAvailable(ctx context.Context) (*MarketplaceCatalog, error) {
	if !s.config.Enabled {
		return &MarketplaceCatalog{
			Plugins:   []MarketplacePlugin{},
			UpdatedAt: time.Now(),
		}, nil
	}

	// Get available plugins from GitHub
	available, err := s.githubClient.ListPluginReleases(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch marketplace: %w", err)
	}

	// Get installed plugins from database
	installed, err := s.queries.ListPlugins(ctx)
	if err != nil {
		return nil, fmt.Errorf("list installed: %w", err)
	}

	installedMap := make(map[string]Plugin)
	for _, p := range installed {
		installedMap[p.ID] = p
	}

	// Build catalog
	plugins := make([]MarketplacePlugin, 0, len(available))
	for _, info := range available {
		mp := MarketplacePlugin{
			ID:            info.ID,
			LatestVersion: info.LatestVersion,
		}

		// Enrich with release info
		if info.LatestRelease != nil {
			mp.Name = info.LatestRelease.Name
			if mp.Name == "" {
				mp.Name = info.ID
			}
			mp.PublishedAt = info.LatestRelease.PublishedAt.Format(time.RFC3339)

			// Sum download counts from all assets
			for _, asset := range info.LatestRelease.Assets {
				mp.DownloadCount += asset.DownloadCount
			}
		}

		// Check installation status
		if inst, ok := installedMap[info.ID]; ok {
			mp.IsInstalled = true
			mp.InstalledVersion = inst.Version
			mp.HasUpdate = s.hasUpdate(inst.Version, info.LatestVersion)
			mp.Description = inst.Description
			mp.Author = inst.Author
			if inst.Capabilities != "" {
				mp.Capabilities = parseCapabilities(inst.Capabilities)
			}
		}

		plugins = append(plugins, mp)
	}

	return &MarketplaceCatalog{
		Plugins:   plugins,
		UpdatedAt: time.Now(),
		CacheAge:  formatDuration(s.githubClient.CacheAge()),
	}, nil
}

// GetDetails returns details for a specific marketplace plugin.
func (s *MarketplaceService) GetDetails(ctx context.Context, pluginID string) (*MarketplacePlugin, error) {
	if !s.config.Enabled {
		return nil, ErrPluginNotInMarketplace
	}

	release, err := s.githubClient.GetLatestRelease(ctx, pluginID)
	if err != nil {
		return nil, ErrPluginNotInMarketplace
	}

	mp := &MarketplacePlugin{
		ID:            release.PluginID,
		Name:          release.Name,
		LatestVersion: release.Version,
		PublishedAt:   release.PublishedAt.Format(time.RFC3339),
	}

	// Check if installed
	if inst, err := s.queries.GetPlugin(ctx, pluginID); err == nil {
		mp.IsInstalled = true
		mp.InstalledVersion = inst.Version
		mp.HasUpdate = s.hasUpdate(inst.Version, release.Version)
		mp.Description = inst.Description
		mp.Author = inst.Author
	}

	return mp, nil
}

// Install downloads and installs a plugin from the marketplace.
func (s *MarketplaceService) Install(ctx context.Context, req InstallRequest, progress chan<- InstallProgress) error {
	if !s.config.Enabled {
		return fmt.Errorf("marketplace is disabled")
	}

	// Prevent concurrent installs of the same plugin
	if _, loaded := s.installingMu.LoadOrStore(req.PluginID, true); loaded {
		return fmt.Errorf("installation already in progress for %s", req.PluginID)
	}
	defer s.installingMu.Delete(req.PluginID)

	// Check if already installed
	if _, err := s.queries.GetPlugin(ctx, req.PluginID); err == nil {
		return ErrPluginAlreadyInstalled
	}

	s.sendProgress(progress, req.PluginID, InstallStatusPending, 0, "Fetching release info...", "", req.Version)

	// Get release from GitHub
	var release *github.Release
	var err error
	if req.Version != "" {
		release, err = s.githubClient.GetRelease(ctx, req.PluginID, req.Version)
	} else {
		release, err = s.githubClient.GetLatestRelease(ctx, req.PluginID)
	}
	if err != nil {
		s.sendProgress(progress, req.PluginID, InstallStatusFailed, 0, "", err.Error(), req.Version)
		return fmt.Errorf("fetch release: %w", err)
	}

	// Find compatible asset
	asset, err := s.githubClient.GetCompatibleAsset(release)
	if err != nil {
		s.sendProgress(progress, req.PluginID, InstallStatusFailed, 0, "", err.Error(), release.Version)
		return ErrNoCompatibleAsset
	}

	s.sendProgress(progress, req.PluginID, InstallStatusDownloading, 5, "Downloading...", "", release.Version)

	// Download with progress callback
	downloadProgress := func(percent int) {
		// Map 0-100 to 5-70 range
		mapped := 5 + (percent * 65 / 100)
		s.sendProgress(progress, req.PluginID, InstallStatusDownloading, mapped, fmt.Sprintf("Downloading: %d%%", percent), "", release.Version)
	}

	result, err := s.downloader.Download(ctx, asset, release.Checksum, downloadProgress)
	if err != nil {
		if strings.Contains(err.Error(), "checksum") {
			s.sendProgress(progress, req.PluginID, InstallStatusFailed, 0, "", "Checksum verification failed", release.Version)
			return ErrChecksumMismatch
		}
		s.sendProgress(progress, req.PluginID, InstallStatusFailed, 0, "", err.Error(), release.Version)
		return fmt.Errorf("download: %w", err)
	}

	s.sendProgress(progress, req.PluginID, InstallStatusInstalling, 75, "Registering plugin...", "", release.Version)

	// Register in database
	plugin := Plugin{
		ID:           result.Manifest.ID,
		Name:         result.Manifest.Name,
		Version:      result.Manifest.Version,
		Description:  result.Manifest.Description,
		Author:       result.Manifest.Author,
		License:      result.Manifest.License,
		Homepage:     result.Manifest.Homepage,
		Capabilities: strings.Join(result.Manifest.Capabilities, ","),
		IsBuiltin:    false,
		Enabled:      true,
		Path:         result.PluginDir,
		InstalledAt:  time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.queries.UpsertPlugin(ctx, plugin); err != nil {
		// Clean up on failure
		s.downloader.Remove(req.PluginID)
		s.sendProgress(progress, req.PluginID, InstallStatusFailed, 0, "", err.Error(), release.Version)
		return fmt.Errorf("register plugin: %w", err)
	}

	s.sendProgress(progress, req.PluginID, InstallStatusInstalling, 85, "Loading plugin...", "", release.Version)

	// Load the plugin
	binaryPath := s.downloader.GetPluginPath(req.PluginID)
	if _, err := s.manager.LoadPlugin(ctx, binaryPath); err != nil {
		// Don't fail the install, plugin can be loaded on restart
		s.logger.Warn("failed to load newly installed plugin", "plugin", req.PluginID, "error", err)
	}

	s.sendProgress(progress, req.PluginID, InstallStatusComplete, 100, "Installation complete", "", release.Version)

	s.logger.Info("plugin installed from marketplace",
		"plugin", req.PluginID,
		"version", release.Version,
	)

	return nil
}

// Update updates an installed plugin to the latest version.
func (s *MarketplaceService) Update(ctx context.Context, pluginID string, progress chan<- InstallProgress) error {
	if !s.config.Enabled {
		return fmt.Errorf("marketplace is disabled")
	}

	// Get installed plugin
	installed, err := s.queries.GetPlugin(ctx, pluginID)
	if err != nil {
		return ErrPluginNotFound{PluginID: pluginID}
	}

	if installed.IsBuiltin {
		return ErrCannotUninstallBuiltin
	}

	// Get latest release
	release, err := s.githubClient.GetLatestRelease(ctx, pluginID)
	if err != nil {
		return ErrPluginNotInMarketplace
	}

	if !s.hasUpdate(installed.Version, release.Version) {
		return fmt.Errorf("already at latest version: %s", installed.Version)
	}

	s.sendProgress(progress, pluginID, InstallStatusPending, 0, "Stopping current version...", "", release.Version)

	// Unload current plugin
	if err := s.manager.UnloadPlugin(ctx, pluginID); err != nil {
		s.logger.Warn("failed to unload plugin for update", "plugin", pluginID, "error", err)
	}

	// Install new version (reusing install logic)
	req := InstallRequest{
		PluginID: pluginID,
		Version:  release.Version,
	}

	// The downloader will handle replacing the existing directory
	return s.installInternal(ctx, req, progress, release)
}

// installInternal handles the actual installation after validation.
func (s *MarketplaceService) installInternal(ctx context.Context, req InstallRequest, progress chan<- InstallProgress, release *github.Release) error {
	// Find compatible asset
	asset, err := s.githubClient.GetCompatibleAsset(release)
	if err != nil {
		s.sendProgress(progress, req.PluginID, InstallStatusFailed, 0, "", err.Error(), release.Version)
		return ErrNoCompatibleAsset
	}

	s.sendProgress(progress, req.PluginID, InstallStatusDownloading, 5, "Downloading update...", "", release.Version)

	downloadProgress := func(percent int) {
		mapped := 5 + (percent * 65 / 100)
		s.sendProgress(progress, req.PluginID, InstallStatusDownloading, mapped, fmt.Sprintf("Downloading: %d%%", percent), "", release.Version)
	}

	result, err := s.downloader.Download(ctx, asset, release.Checksum, downloadProgress)
	if err != nil {
		s.sendProgress(progress, req.PluginID, InstallStatusFailed, 0, "", err.Error(), release.Version)
		return fmt.Errorf("download: %w", err)
	}

	s.sendProgress(progress, req.PluginID, InstallStatusInstalling, 75, "Updating database...", "", release.Version)

	// Update database
	plugin := Plugin{
		ID:           result.Manifest.ID,
		Name:         result.Manifest.Name,
		Version:      result.Manifest.Version,
		Description:  result.Manifest.Description,
		Author:       result.Manifest.Author,
		License:      result.Manifest.License,
		Homepage:     result.Manifest.Homepage,
		Capabilities: strings.Join(result.Manifest.Capabilities, ","),
		IsBuiltin:    false,
		Enabled:      true,
		Path:         result.PluginDir,
		UpdatedAt:    time.Now(),
	}

	if err := s.queries.UpsertPlugin(ctx, plugin); err != nil {
		s.sendProgress(progress, req.PluginID, InstallStatusFailed, 0, "", err.Error(), release.Version)
		return fmt.Errorf("update database: %w", err)
	}

	s.sendProgress(progress, req.PluginID, InstallStatusInstalling, 85, "Loading updated plugin...", "", release.Version)

	// Load the plugin
	binaryPath := s.downloader.GetPluginPath(req.PluginID)
	if _, err := s.manager.LoadPlugin(ctx, binaryPath); err != nil {
		s.logger.Warn("failed to load updated plugin", "plugin", req.PluginID, "error", err)
	}

	s.sendProgress(progress, req.PluginID, InstallStatusComplete, 100, "Update complete", "", release.Version)

	s.logger.Info("plugin updated from marketplace",
		"plugin", req.PluginID,
		"version", release.Version,
	)

	return nil
}

// Uninstall removes a plugin.
func (s *MarketplaceService) Uninstall(ctx context.Context, pluginID string) error {
	// Get plugin info
	plugin, err := s.queries.GetPlugin(ctx, pluginID)
	if err != nil {
		return ErrPluginNotFound{PluginID: pluginID}
	}

	if plugin.IsBuiltin {
		return ErrCannotUninstallBuiltin
	}

	// Unload if running
	if err := s.manager.UnloadPlugin(ctx, pluginID); err != nil {
		s.logger.Debug("plugin not loaded, continuing uninstall", "plugin", pluginID, "error", err)
	}

	// Remove from filesystem
	if err := s.downloader.Remove(pluginID); err != nil {
		s.logger.Warn("failed to remove plugin files", "plugin", pluginID, "error", err)
	}

	// Remove from database
	if err := s.queries.DeletePlugin(ctx, pluginID); err != nil {
		return fmt.Errorf("delete from database: %w", err)
	}

	s.logger.Info("plugin uninstalled", "plugin", pluginID)
	return nil
}

// CheckUpdates returns a list of installed plugins that have updates available.
func (s *MarketplaceService) CheckUpdates(ctx context.Context) (*UpdatesResponse, error) {
	if !s.config.Enabled {
		return &UpdatesResponse{Updates: []UpdateInfo{}}, nil
	}

	// Get available plugins
	available, err := s.githubClient.ListPluginReleases(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch marketplace: %w", err)
	}

	// Get installed plugins
	installed, err := s.queries.ListPlugins(ctx)
	if err != nil {
		return nil, fmt.Errorf("list installed: %w", err)
	}

	var updates []UpdateInfo
	for _, inst := range installed {
		if inst.IsBuiltin {
			continue
		}

		info, ok := available[inst.ID]
		if !ok {
			continue
		}

		if s.hasUpdate(inst.Version, info.LatestVersion) {
			updates = append(updates, UpdateInfo{
				PluginID:         inst.ID,
				PluginName:       inst.Name,
				InstalledVersion: inst.Version,
				LatestVersion:    info.LatestVersion,
			})
		}
	}

	return &UpdatesResponse{
		Updates: updates,
		Count:   len(updates),
	}, nil
}

// RefreshCatalog forces a refresh of the cached catalog.
func (s *MarketplaceService) RefreshCatalog(ctx context.Context) error {
	return s.githubClient.RefreshCache(ctx)
}

// hasUpdate checks if available version is newer than installed.
func (s *MarketplaceService) hasUpdate(installed, available string) bool {
	// Ensure "v" prefix for semver package
	if !strings.HasPrefix(installed, "v") {
		installed = "v" + installed
	}
	if !strings.HasPrefix(available, "v") {
		available = "v" + available
	}

	return semver.Compare(available, installed) > 0
}

// sendProgress sends a progress update to the channel if provided.
func (s *MarketplaceService) sendProgress(ch chan<- InstallProgress, pluginID string, status InstallStatus, progress int, message, errMsg, version string) {
	if ch == nil {
		return
	}

	select {
	case ch <- InstallProgress{
		PluginID: pluginID,
		Status:   status,
		Progress: progress,
		Message:  message,
		Error:    errMsg,
		Version:  version,
	}:
	default:
		// Channel full or closed, skip
	}
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm ago", int(d.Minutes()))
}
