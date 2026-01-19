package plugins

import (
	"errors"
	"time"
)

// Marketplace errors.
var (
	ErrPluginNotInMarketplace = errors.New("plugin not found in marketplace")
	ErrPluginAlreadyInstalled = errors.New("plugin is already installed")
	ErrNoCompatibleAsset      = errors.New("no compatible binary for this platform")
	ErrChecksumMismatch       = errors.New("checksum verification failed")
	ErrCannotUninstallBuiltin = errors.New("cannot uninstall built-in plugins")
	ErrPluginHasDependents    = errors.New("other plugins depend on this plugin")
)

// PluginSource indicates where a plugin came from.
type PluginSource string

const (
	PluginSourceLocal       PluginSource = "local"       // Manually installed
	PluginSourceMarketplace PluginSource = "marketplace" // Installed from marketplace
)

// MarketplacePlugin represents a plugin available in the marketplace.
type MarketplacePlugin struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Author           string   `json:"author,omitempty"`
	LatestVersion    string   `json:"latest_version"`
	InstalledVersion string   `json:"installed_version,omitempty"`
	IsInstalled      bool     `json:"is_installed"`
	HasUpdate        bool     `json:"has_update"`
	Capabilities     []string `json:"capabilities,omitempty"`
	DownloadCount    int64    `json:"download_count,omitempty"`
	PublishedAt      string   `json:"published_at,omitempty"` // RFC3339
}

// MarketplaceCatalog is the list of available plugins.
type MarketplaceCatalog struct {
	Plugins   []MarketplacePlugin `json:"plugins"`
	UpdatedAt time.Time           `json:"updated_at"`
	CacheAge  string              `json:"cache_age,omitempty"` // Human-readable, e.g. "5m ago"
}

// InstallRequest is the request to install a plugin.
type InstallRequest struct {
	PluginID string `json:"plugin_id" binding:"required"`
	Version  string `json:"version,omitempty"` // Empty means latest
}

// InstallStatus represents the current stage of installation.
type InstallStatus string

const (
	InstallStatusPending    InstallStatus = "pending"
	InstallStatusDownloading InstallStatus = "downloading"
	InstallStatusExtracting InstallStatus = "extracting"
	InstallStatusInstalling InstallStatus = "installing"
	InstallStatusComplete   InstallStatus = "complete"
	InstallStatusFailed     InstallStatus = "failed"
)

// InstallProgress tracks installation progress for SSE streaming.
type InstallProgress struct {
	PluginID string        `json:"plugin_id"`
	Status   InstallStatus `json:"status"`
	Progress int           `json:"progress"` // 0-100
	Message  string        `json:"message,omitempty"`
	Error    string        `json:"error,omitempty"`
	Version  string        `json:"version,omitempty"`
}

// UpdateInfo describes an available update.
type UpdateInfo struct {
	PluginID         string `json:"plugin_id"`
	PluginName       string `json:"plugin_name"`
	InstalledVersion string `json:"installed_version"`
	LatestVersion    string `json:"latest_version"`
}

// UpdatesResponse is the response for checking updates.
type UpdatesResponse struct {
	Updates []UpdateInfo `json:"updates"`
	Count   int          `json:"count"`
}
