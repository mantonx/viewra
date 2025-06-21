package playbackmodule

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
)

// PlaybackServiceImpl implements the PlaybackService interface
// This provides a clean API for other modules to use playback functionality
type PlaybackServiceImpl struct {
	manager *Manager
}

// NewPlaybackServiceImpl creates a new playback service implementation
func NewPlaybackServiceImpl(manager *Manager) services.PlaybackService {
	return &PlaybackServiceImpl{
		manager: manager,
	}
}

// DecidePlayback determines whether to direct play or transcode
func (p *PlaybackServiceImpl) DecidePlayback(mediaPath string, deviceProfile *types.DeviceProfile) (*types.PlaybackDecision, error) {
	if p.manager == nil {
		return nil, fmt.Errorf("playback manager not available")
	}
	
	// Convert types for internal use (temporary during transition)
	internalProfile := &DeviceProfile{
		UserAgent:       deviceProfile.UserAgent,
		SupportedCodecs: deviceProfile.SupportedCodecs,
		MaxResolution:   deviceProfile.MaxResolution,
		MaxBitrate:      deviceProfile.MaxBitrate,
		SupportsHEVC:    deviceProfile.SupportsHEVC,
		SupportsAV1:     deviceProfile.SupportsAV1,
		SupportsHDR:     deviceProfile.SupportsHDR,
		ClientIP:        deviceProfile.ClientIP,
	}
	
	decision, err := p.manager.DecidePlayback(mediaPath, internalProfile)
	if err != nil {
		return nil, err
	}
	
	// Convert result back to external types
	return &types.PlaybackDecision{
		ShouldTranscode: decision.ShouldTranscode,
		DirectPlayURL:   decision.DirectPlayURL,
		TranscodeParams: decision.TranscodeParams,
		Reason:          decision.Reason,
	}, nil
}

// StartTranscode initiates a new transcoding session
func (p *PlaybackServiceImpl) StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*database.TranscodeSession, error) {
	if p.manager == nil {
		return nil, fmt.Errorf("playback manager not available")
	}
	return p.manager.StartTranscode(req)
}

// GetSession retrieves session information
func (p *PlaybackServiceImpl) GetSession(sessionID string) (*database.TranscodeSession, error) {
	if p.manager == nil {
		return nil, fmt.Errorf("playback manager not available")
	}
	return p.manager.GetSession(sessionID)
}

// StopSession stops a transcoding session
func (p *PlaybackServiceImpl) StopSession(sessionID string) error {
	if p.manager == nil {
		return fmt.Errorf("playback manager not available")
	}
	return p.manager.StopSession(sessionID)
}

// GetStats returns transcoding statistics
func (p *PlaybackServiceImpl) GetStats() (*types.TranscodingStats, error) {
	if p.manager == nil {
		return nil, fmt.Errorf("playback manager not available")
	}
	
	stats, err := p.manager.GetStats()
	if err != nil {
		return nil, err
	}
	
	// Convert internal stats to external types
	backends := make(map[string]*types.BackendStats)
	for id, backend := range stats.Backends {
		backends[id] = &types.BackendStats{
			Name:         backend.Name,
			Priority:     backend.Priority,
			Capabilities: backend.Capabilities,
		}
	}
	
	return &types.TranscodingStats{
		ActiveSessions:    stats.ActiveSessions,
		TotalSessions:     stats.TotalSessions,
		CompletedSessions: stats.CompletedSessions,
		FailedSessions:    stats.FailedSessions,
		TotalBytesOut:     stats.TotalBytesOut,
		AverageSpeed:      stats.AverageSpeed,
		Backends:          backends,
		RecentSessions:    stats.RecentSessions,
	}, nil
}