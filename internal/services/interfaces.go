package services

import (
	"context"

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
// This serves as the reference pattern for other module services
type PlaybackService interface {
	// DecidePlayback determines whether to direct play or transcode
	DecidePlayback(mediaPath string, deviceProfile *types.DeviceProfile) (*types.PlaybackDecision, error)
	
	// StartTranscode initiates a new transcoding session
	StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*database.TranscodeSession, error)
	
	// GetSession retrieves session information
	GetSession(sessionID string) (*database.TranscodeSession, error)
	
	// StopSession stops a transcoding session
	StopSession(sessionID string) error
	
	// GetStats returns transcoding statistics
	GetStats() (*types.TranscodingStats, error)
}

// Future service interfaces should follow this pattern:
//
// type MediaService interface {
//     GetFile(ctx context.Context, id string) (*database.MediaFile, error)
//     ScanLibrary(ctx context.Context, libraryID uint32) error
//     UpdateMetadata(ctx context.Context, fileID string, metadata map[string]string) error
// }
//
// type PluginService interface {
//     ListPlugins(ctx context.Context) ([]PluginInfo, error)
//     EnablePlugin(ctx context.Context, pluginID string) error
//     GetPluginStatus(ctx context.Context, pluginID string) (*PluginStatus, error)
// }
//
// type ScannerService interface {
//     StartScan(ctx context.Context, libraryID uint32) (*ScanJob, error)
//     GetScanProgress(ctx context.Context, jobID string) (*ScanProgress, error)
//     StopScan(ctx context.Context, jobID string) error
// }