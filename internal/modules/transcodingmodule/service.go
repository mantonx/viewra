package transcodingmodule

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/services"
	"github.com/mantonx/viewra/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
)

// TranscodingServiceImpl implements the TranscodingService interface.
// This is the public API that other modules use to interact with transcoding.
type TranscodingServiceImpl struct {
	manager *Manager
}

// NewTranscodingServiceImpl creates a new transcoding service implementation
func NewTranscodingServiceImpl(manager *Manager) services.TranscodingService {
	return &TranscodingServiceImpl{
		manager: manager,
	}
}

// StartTranscode initiates a new transcoding session
func (s *TranscodingServiceImpl) StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*database.TranscodeSession, error) {
	if req == nil {
		return nil, fmt.Errorf("transcode request is nil")
	}

	handle, err := s.manager.StartTranscode(ctx, *req)
	if err != nil {
		return nil, err
	}

	// Convert handle to database session
	if handle == nil {
		return nil, fmt.Errorf("no handle returned from manager")
	}

	// Get session from store using the handle's session ID
	if s.manager.sessionStore != nil {
		return s.manager.sessionStore.GetSession(handle.SessionID)
	}

	// Fallback: create a basic session object
	return &database.TranscodeSession{
		ID:       handle.SessionID,
		Status:   database.TranscodeStatus(handle.Status),
		Provider: handle.Provider,
	}, nil
}

// StopSession stops a transcoding session
func (s *TranscodingServiceImpl) StopSession(sessionID string) error {
	return s.manager.StopTranscode(sessionID)
}

// GetSession retrieves session information
func (s *TranscodingServiceImpl) GetSession(sessionID string) (*database.TranscodeSession, error) {
	if s.manager.sessionStore != nil {
		return s.manager.sessionStore.GetSession(sessionID)
	}
	return nil, fmt.Errorf("session store not available")
}

// StopTranscode stops a transcoding session
func (s *TranscodingServiceImpl) StopTranscode(sessionID string) error {
	return s.manager.StopTranscode(sessionID)
}

// GetProgress returns the progress of a transcoding session
func (s *TranscodingServiceImpl) GetProgress(sessionID string) (*plugins.TranscodingProgress, error) {
	return s.manager.GetProgress(sessionID)
}

// GetStats returns transcoding statistics
func (s *TranscodingServiceImpl) GetStats() (*types.TranscodingStats, error) {
	// Get sessions from session store
	sessions, err := s.manager.sessionStore.GetActiveSessions()
	if err != nil {
		return nil, err
	}

	// Build basic stats
	stats := &types.TranscodingStats{
		ActiveSessions:    len(sessions),
		TotalSessions:     0,
		CompletedSessions: 0,
		FailedSessions:    0,
		Backends:          make(map[string]*types.BackendStats),
		RecentSessions:    sessions,
	}

	// Get provider info
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
func (s *TranscodingServiceImpl) GetProviders() []plugins.ProviderInfo {
	return s.manager.GetProviders()
}

// GetContentStore returns the content store for content-addressable storage
func (s *TranscodingServiceImpl) GetContentStore() services.ContentStore {
	return s.manager.GetContentStore()
}
