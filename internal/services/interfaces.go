package services

import (
	"context"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
)

// Standard service interface pattern for all modules
//
// Each module should define a clean interface following this pattern:
// - Clear, focused functionality
// - Context-aware operations
// - Proper error handling
// - No internal types exposed

// PlaybackService defines the clean interface for playback operations
// This service focuses on playback decisions and delegates transcoding to TranscodingService
type PlaybackService interface {
	// DecidePlayback determines whether to direct play or transcode based on
	// media file characteristics and device capabilities
	DecidePlayback(mediaPath string, deviceProfile *types.DeviceProfile) (*types.PlaybackDecision, error)

	// GetMediaInfo analyzes a media file and returns its characteristics
	GetMediaInfo(mediaPath string) (*types.MediaInfo, error)

	// ValidatePlayback checks if a media file can be played on the given device
	ValidatePlayback(mediaPath string, deviceProfile *types.DeviceProfile) error

	// GetSupportedFormats returns formats supported for direct playback
	GetSupportedFormats(deviceProfile *types.DeviceProfile) []string

	// GetRecommendedTranscodeParams returns optimal transcoding parameters if needed
	GetRecommendedTranscodeParams(mediaPath string, deviceProfile *types.DeviceProfile) (*plugins.TranscodeRequest, error)
}

// TranscodingService defines the interface for transcoding operations
// This service handles all transcoding-related functionality
type TranscodingService interface {
	// StartTranscode initiates a new transcoding session
	StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*database.TranscodeSession, error)

	// GetSession retrieves session information
	GetSession(sessionID string) (*database.TranscodeSession, error)

	// StopSession stops a transcoding session
	StopSession(sessionID string) error

	// GetProgress returns the progress of a transcoding session
	GetProgress(sessionID string) (*plugins.TranscodingProgress, error)

	// GetStats returns transcoding statistics
	GetStats() (*types.TranscodingStats, error)

	// GetProviders returns available transcoding providers
	GetProviders() []plugins.ProviderInfo

	// GetContentStore returns the content store for content-addressable storage
	// This allows other modules to serve content files
	GetContentStore() ContentStore
}

// ContentStore defines the interface for content-addressable storage
type ContentStore interface {
	// Get retrieves content metadata and path by hash
	Get(contentHash string) (metadata interface{}, contentPath string, err error)

	// ListByMediaID returns all content versions for a media ID
	ListByMediaID(mediaID string) ([]interface{}, error)

	// GetStats returns storage statistics
	GetStats() (interface{}, error)

	// ListExpired returns content that has expired
	ListExpired() ([]interface{}, error)

	// Delete removes content by hash
	Delete(contentHash string) error
}

// SessionStore defines the interface for session management
type SessionStore interface {
	// ListActiveSessionsByContentHash returns active sessions with the specified content hash
	ListActiveSessionsByContentHash(contentHash string) ([]*database.TranscodeSession, error)
}

// MediaService defines the interface for media file management
type MediaService interface {
	// File operations
	GetFile(ctx context.Context, id string) (*database.MediaFile, error)
	GetFileByPath(ctx context.Context, path string) (*database.MediaFile, error)
	ListFiles(ctx context.Context, filter types.MediaFilter) ([]*database.MediaFile, error)
	UpdateFile(ctx context.Context, id string, updates map[string]interface{}) error
	DeleteFile(ctx context.Context, id string) error

	// Library operations
	GetLibrary(ctx context.Context, id uint32) (*database.MediaLibrary, error)
	ScanLibrary(ctx context.Context, libraryID uint32) error
	UpdateMetadata(ctx context.Context, fileID string, metadata map[string]string) error

	// Media info
	GetMediaInfo(ctx context.Context, filePath string) (*types.MediaInfo, error)
}

// PluginService defines the interface for plugin management operations
type PluginService interface {
	// Discovery and lifecycle
	ListPlugins(ctx context.Context) ([]plugins.PluginInfo, error)
	GetPlugin(ctx context.Context, pluginID string) (plugins.Plugin, error)
	EnablePlugin(ctx context.Context, pluginID string) error
	DisablePlugin(ctx context.Context, pluginID string) error
	GetPluginStatus(ctx context.Context, pluginID string) (*types.PluginStatus, error)

	// Plugin type specific getters
	GetMetadataScrapers() []plugins.MetadataScraperService
	GetEnrichmentServices() []plugins.EnrichmentService
	GetTranscodingProviders() []plugins.TranscodingProvider

	// Configuration
	UpdatePluginConfig(ctx context.Context, pluginID string, config map[string]interface{}) error
	GetPluginConfig(ctx context.Context, pluginID string) (map[string]interface{}, error)
}

// ScannerService defines the interface for media scanning operations
type ScannerService interface {
	// Scan operations
	StartScan(ctx context.Context, libraryID uint32) (*types.ScanJob, error)
	GetScanProgress(ctx context.Context, jobID string) (*types.ScanProgress, error)
	StopScan(ctx context.Context, jobID string) error
	GetActiveScanJobs(ctx context.Context) ([]*types.ScanJob, error)

	// Configuration
	SetScanInterval(ctx context.Context, libraryID uint32, interval time.Duration) error
	GetScanHistory(ctx context.Context, libraryID uint32) ([]*types.ScanResult, error)
}

// AssetService defines the interface for asset management
type AssetService interface {
	// Asset operations
	GetAsset(ctx context.Context, assetType, assetName string) (string, error)
	SaveAsset(ctx context.Context, assetType, assetName string, data []byte) error
	DeleteAsset(ctx context.Context, assetType, assetName string) error
	ListAssets(ctx context.Context, assetType string) ([]string, error)

	// URL generation
	GetAssetURL(assetType, assetName string) string
}

// EnrichmentService defines the interface for metadata enrichment
type EnrichmentService interface {
	// Enrichment operations
	EnrichMovie(ctx context.Context, movieID string) error
	EnrichEpisode(ctx context.Context, episodeID string) error
	EnrichSeries(ctx context.Context, seriesID string) error

	// Batch operations
	BatchEnrich(ctx context.Context, mediaType string, ids []string) error
	GetEnrichmentStatus(ctx context.Context, jobID string) (*types.EnrichmentStatus, error)
}
