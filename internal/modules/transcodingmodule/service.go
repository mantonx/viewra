// Package transcodingmodule provides video and audio transcoding functionality.
// It implements a file-based transcoding approach with content-addressable storage.
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
//
// The service provides a clean interface for:
//   - Starting transcoding sessions
//   - Monitoring progress
//   - Managing transcoded content
//   - Accessing transcoding statistics
//
// It delegates the actual work to the Manager, which coordinates
// providers, sessions, and storage.
type TranscodingServiceImpl struct {
	manager *Manager
}

// NewTranscodingServiceImpl creates a new transcoding service implementation.
//
// Parameters:
//   - manager: The transcoding manager that handles actual operations
//
// The service acts as a thin wrapper around the manager, providing
// the interface expected by other modules.
func NewTranscodingServiceImpl(manager *Manager) services.TranscodingService {
	return &TranscodingServiceImpl{
		manager: manager,
	}
}

// StartTranscode initiates a new transcoding session.
//
// The method handles:
//   - Request validation
//   - Session creation and persistence
//   - Content deduplication (reuses existing transcodes)
//   - Background transcoding initiation
//
// Parameters:
//   - ctx: Context for cancellation
//   - req: Transcoding parameters (input, codecs, resolution, etc.)
//
// Returns:
//   - TranscodeSession with session ID and status
//   - Error if request invalid or transcoding cannot start
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

// StopSession stops an active transcoding session.
//
// This will:
//   - Terminate the FFmpeg process
//   - Update session status to "cancelled"
//   - Clean up temporary files (via cleanup service)
//
// Parameters:
//   - sessionID: The session to stop
//
// Returns:
//   - Error if session not found or stop failed
func (s *TranscodingServiceImpl) StopSession(sessionID string) error {
	return s.manager.StopTranscode(sessionID)
}

// GetSession retrieves detailed session information from the database.
//
// Parameters:
//   - sessionID: The session to query
//
// Returns:
//   - TranscodeSession with full details including:
//     - Status (queued, running, completed, failed)
//     - Content hash for deduplication
//     - Timestamps and progress
//   - Error if session not found
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

// GetProgress returns real-time progress of a transcoding session.
//
// For file-based transcoding, progress includes:
//   - Percentage complete (estimated)
//   - Current status
//   - Start/end times
//
// Parameters:
//   - sessionID: The session to query
//
// Returns:
//   - TranscodingProgress with current state
//   - Error if session not found
func (s *TranscodingServiceImpl) GetProgress(sessionID string) (*plugins.TranscodingProgress, error) {
	return s.manager.GetProgress(sessionID)
}

// GetStats returns comprehensive transcoding statistics.
//
// Statistics include:
//   - Active session count
//   - Total/completed/failed sessions
//   - Provider information and capabilities
//   - Recent session list
//
// This is useful for monitoring and dashboard displays.
//
// Returns:
//   - TranscodingStats with current metrics
//   - Error if stats cannot be retrieved
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

// GetProviders returns information about all available transcoding providers.
//
// This includes:
//   - Built-in file pipeline provider
//   - External plugin providers (FFmpeg variants)
//
// Each provider info contains:
//   - ID, name, and description
//   - Priority for selection
//   - Version information
//
// Returns:
//   - List of provider information
func (s *TranscodingServiceImpl) GetProviders() []plugins.ProviderInfo {
	return s.manager.GetProviders()
}

// GetContentStore returns the content store for content-addressable storage.
//
// The content store allows other modules (like playback) to:
//   - Retrieve transcoded files by content hash
//   - List available versions for a media file
//   - Check storage statistics
//
// This enables efficient content serving without tight coupling.
//
// Returns:
//   - ContentStore interface for accessing transcoded content
func (s *TranscodingServiceImpl) GetContentStore() services.ContentStore {
	return s.manager.GetContentStore()
}
