package plugin

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/google/uuid"
	"github.com/mantonx/viewra/data/plugins/ffmpeg_transcoder/internal/types"
	"github.com/mantonx/viewra/pkg/plugins"
)

// transcodingServiceAdapter adapts the internal plugin to the SDK interface
type transcodingServiceAdapter struct {
	plugin *FFmpegTranscoderPlugin
	// Track requests by session ID since internal sessions don't store them
	sessionRequests map[string]*plugins.TranscodeRequest
	mutex           sync.RWMutex
}

// newTranscodingServiceAdapter creates a new adapter
func newTranscodingServiceAdapter(plugin *FFmpegTranscoderPlugin) *transcodingServiceAdapter {
	return &transcodingServiceAdapter{
		plugin:          plugin,
		sessionRequests: make(map[string]*plugins.TranscodeRequest),
	}
}

// GetCapabilities returns the transcoding capabilities
func (a *transcodingServiceAdapter) GetCapabilities(ctx context.Context) (*plugins.TranscodingCapabilities, error) {
	if a.plugin.transcodingService == nil {
		return nil, fmt.Errorf("transcoding service not initialized")
	}

	return a.plugin.transcodingService.GetCapabilities(), nil
}

// StartTranscode starts a new transcoding session
func (a *transcodingServiceAdapter) StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*plugins.TranscodeSession, error) {
	a.plugin.logger.Debug("StartTranscode called",
		"inputPath", req.InputPath,
		"sessionId", req.SessionID,
		"codecOpts", req.CodecOpts)

	// Use the session ID provided by the main app
	sessionID := req.SessionID
	if sessionID == "" {
		// Fall back to generating a UUID if no session ID provided
		sessionID = uuid.New().String()
		a.plugin.logger.Warn("No session ID provided in request, generating new UUID", "sessionId", sessionID)
	}

	// Create a new request with the session ID in the environment
	// This ensures the wrapper can use the correct session ID
	if req.Environment == nil {
		req.Environment = make(map[string]string)
	}
	req.Environment["session_id"] = sessionID

	// The transcodingService (which is the wrapper) will handle everything
	session, err := a.plugin.transcodingService.StartTranscode(ctx, req)
	if err != nil {
		a.plugin.logger.Error("Failed to start transcoding",
			"sessionId", sessionID,
			"error", err)
		return nil, err
	}

	// Convert internal session to plugin session
	pluginSession := &plugins.TranscodeSession{
		ID:        session.ID,
		Request:   req,
		Status:    plugins.TranscodeStatus(session.Status),
		Progress:  session.Progress,
		StartTime: session.StartTime,
		Backend:   "ffmpeg_transcoder",
		Stats:     &plugins.TranscodeStats{},
		Metadata:  make(map[string]interface{}),
	}

	// Store the request for later retrieval
	a.mutex.Lock()
	a.sessionRequests[session.ID] = req
	a.mutex.Unlock()

	// Add output path to metadata
	if session.OutputPath != "" {
		pluginSession.Metadata["output_path"] = session.OutputPath
		// Also store input path for seek-ahead
		pluginSession.Metadata["input_path"] = req.InputPath
	}
	if session.SessionDir != "" {
		pluginSession.Metadata["session_dir"] = session.SessionDir
	}

	a.plugin.logger.Info("Started transcoding session",
		"sessionId", session.ID,
		"inputPath", req.InputPath,
		"outputPath", session.OutputPath)

	return pluginSession, nil
}

// GetTranscodeSession retrieves information about an active session
func (a *transcodingServiceAdapter) GetTranscodeSession(ctx context.Context, sessionID string) (*plugins.TranscodeSession, error) {
	if a.plugin.transcodingService == nil {
		return nil, fmt.Errorf("transcoding service not initialized")
	}

	// Get the session from the service
	session, err := a.plugin.transcodingService.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	// Convert to plugin session
	return a.convertSession(session), nil
}

// StopTranscode terminates a transcoding session
func (a *transcodingServiceAdapter) StopTranscode(ctx context.Context, sessionID string) error {
	if a.plugin.transcodingService == nil {
		return fmt.Errorf("transcoding service not initialized")
	}

	err := a.plugin.transcodingService.StopSession(sessionID)

	// Clean up the stored request regardless of error
	a.mutex.Lock()
	delete(a.sessionRequests, sessionID)
	a.mutex.Unlock()

	return err
}

// ListActiveSessions returns all currently active transcoding sessions
func (a *transcodingServiceAdapter) ListActiveSessions(ctx context.Context) ([]*plugins.TranscodeSession, error) {
	if a.plugin.transcodingService == nil {
		return nil, fmt.Errorf("transcoding service not initialized")
	}

	// Get sessions from the service
	sessions, err := a.plugin.transcodingService.ListSessions()
	if err != nil {
		return nil, err
	}

	// Convert to plugin sessions
	var pluginSessions []*plugins.TranscodeSession
	for _, session := range sessions {
		pluginSessions = append(pluginSessions, a.convertSession(session))
	}

	return pluginSessions, nil
}

// GetTranscodeStream returns the output stream for a transcoding session
func (a *transcodingServiceAdapter) GetTranscodeStream(ctx context.Context, sessionID string) (io.ReadCloser, error) {
	if a.plugin.transcodingService == nil {
		return nil, fmt.Errorf("transcoding service not initialized")
	}

	// Get session to find output path
	session, err := a.plugin.transcodingService.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	// For DASH/HLS, we need to return the manifest file
	// For progressive download, return the output file
	if session.OutputPath == "" {
		return nil, fmt.Errorf("session has no output path")
	}

	// Open the output file
	file, err := os.Open(session.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open output file: %w", err)
	}

	return file, nil
}

// convertSession converts internal session to plugin session
func (a *transcodingServiceAdapter) convertSession(session *types.Session) *plugins.TranscodeSession {
	if session == nil {
		return nil
	}

	// Map internal status to plugin status
	var status plugins.TranscodeStatus
	switch session.Status {
	case types.StatusPending:
		status = plugins.StatusPending
	case types.StatusStarting:
		status = plugins.StatusStarting
	case types.StatusRunning:
		status = plugins.StatusRunning
	case types.StatusCompleted:
		status = plugins.StatusCompleted
	case types.StatusFailed:
		status = plugins.StatusFailed
	case types.StatusCancelled:
		status = plugins.StatusCancelled
	case types.StatusTimeout:
		status = plugins.StatusFailed
	default:
		status = plugins.StatusPending
	}

	// Retrieve the stored request
	a.mutex.RLock()
	request := a.sessionRequests[session.ID]
	a.mutex.RUnlock()

	pluginSession := &plugins.TranscodeSession{
		ID:        session.ID,
		Request:   request, // Include the stored request
		Status:    status,
		Progress:  session.Progress,
		StartTime: session.StartTime,
		Backend:   "ffmpeg_transcoder",
		Stats:     &plugins.TranscodeStats{},
		Metadata: map[string]interface{}{
			"output_path": session.OutputPath,
			"session_dir": session.SessionDir,
			"container":   session.Container,
		},
	}

	// Also include input path in metadata if we have the request
	if request != nil && request.InputPath != "" {
		pluginSession.Metadata["input_path"] = request.InputPath
	}

	return pluginSession
}
