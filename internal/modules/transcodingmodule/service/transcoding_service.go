package service

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/transcoding"
	transcTypes "github.com/mantonx/viewra/internal/modules/transcodingmodule/types"
	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
)

// transcodingServiceImpl implements the TranscodingService interface as a thin wrapper
// All business logic is delegated to the core transcoding Manager
type transcodingServiceImpl struct {
	manager *transcoding.Manager
}

// NewTranscodingService creates a new transcoding service implementation
func NewTranscodingService(manager *transcoding.Manager) services.TranscodingService {
	return &transcodingServiceImpl{
		manager: manager,
	}
}

// StartTranscode initiates a new transcoding session
func (s *transcodingServiceImpl) StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*database.TranscodeSession, error) {
	if req == nil {
		return nil, fmt.Errorf("transcode request is nil")
	}

	// NOTE: This is a temporary workaround. The manager should be refactored to return
	// database.TranscodeSession directly to avoid this conversion layer.
	handle, err := s.manager.StartTranscode(ctx, *req)
	if err != nil {
		return nil, err
	}

	if handle == nil {
		return nil, fmt.Errorf("no handle returned from manager")
	}

	// Get session info and convert to database type
	sessionInfo, err := s.manager.GetSession(handle.SessionID)
	if err != nil {
		// Fallback with minimal info
		return &database.TranscodeSession{
			ID:       handle.SessionID,
			Status:   database.TranscodeStatus(handle.Status),
			Provider: handle.Provider,
		}, nil
	}

	// Convert types - this conversion should ideally be eliminated
	return s.convertToTranscodeSession(sessionInfo), nil
}

// GetSession retrieves session information
func (s *transcodingServiceImpl) GetSession(sessionID string) (*database.TranscodeSession, error) {
	sessionInfo, err := s.manager.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	return s.convertToTranscodeSession(sessionInfo), nil
}

// StopSession stops a transcoding session
func (s *transcodingServiceImpl) StopSession(sessionID string) error {
	return s.manager.StopTranscode(sessionID)
}

// GetProgress returns the progress of a transcoding session
func (s *transcodingServiceImpl) GetProgress(sessionID string) (*plugins.TranscodingProgress, error) {
	return s.manager.GetProgress(sessionID)
}

// GetStats returns transcoding statistics
func (s *transcodingServiceImpl) GetStats() (*types.TranscodingStats, error) {
	// Get all sessions
	sessionInfos := s.manager.GetAllSessions()
	
	// Convert to database sessions for stats
	sessions := make([]*database.TranscodeSession, 0, len(sessionInfos))
	for _, info := range sessionInfos {
		sessions = append(sessions, s.convertToTranscodeSession(info))
	}

	// Build stats
	stats := &types.TranscodingStats{
		ActiveSessions:    len(sessions),
		TotalSessions:     0,
		CompletedSessions: 0,
		FailedSessions:    0,
		Backends:          make(map[string]*types.BackendStats),
		RecentSessions:    sessions,
	}

	// Add provider info
	providers := s.manager.GetProviders()
	for _, info := range providers {
		stats.Backends[info.ID] = &types.BackendStats{
			Name:         info.Name,
			Priority:     info.Priority,
			Capabilities: make(map[string]interface{}),
		}
	}

	return stats, nil
}

// GetProviders returns available transcoding providers
func (s *transcodingServiceImpl) GetProviders() []plugins.ProviderInfo {
	return s.manager.GetProviders()
}

// GetContentStore returns the content store for content-addressable storage
func (s *transcodingServiceImpl) GetContentStore() services.ContentStore {
	return s.manager.GetContentStore()
}

// convertToTranscodeSession converts internal SessionInfo to database TranscodeSession
// TODO: This conversion should be eliminated by refactoring the manager to work
// with database types directly, following the clean architecture pattern.
func (s *transcodingServiceImpl) convertToTranscodeSession(info *transcTypes.SessionInfo) *database.TranscodeSession {
	return &database.TranscodeSession{
		ID:          info.SessionID,
		Status:      database.TranscodeStatus(info.Status),
		Provider:    info.Provider,
		ContentHash: info.ContentHash,
		StartTime:   info.StartTime,
		// Note: Some fields from SessionInfo may not have direct mappings
		// This highlights the architectural mismatch that needs addressing
	}
}