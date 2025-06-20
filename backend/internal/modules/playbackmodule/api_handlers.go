package playbackmodule

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/pkg/plugins"
)

// API Request/Response Types

// PlaybackDecisionRequest represents a request for playback decision
type PlaybackDecisionRequest struct {
	MediaPath     string         `json:"media_path" binding:"required"`
	DeviceProfile DeviceProfile  `json:"device_profile" binding:"required"`
	ForceAnalysis bool           `json:"force_analysis,omitempty"`
	Options       RequestOptions `json:"options,omitempty"`
}

// RequestOptions provides additional options for API requests
type RequestOptions struct {
	Priority       int               `json:"priority,omitempty"`
	Timeout        int               `json:"timeout_seconds,omitempty"`
	PreferredCodec string            `json:"preferred_codec,omitempty"`
	Quality        int               `json:"quality,omitempty"`
	Preset         string            `json:"preset,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// TranscodeStartRequest represents a request to start transcoding
type TranscodeStartRequest struct {
	plugins.TranscodeRequest
	SessionName  string            `json:"session_name,omitempty"`
	CallbackURL  string            `json:"callback_url,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	ForceBackend string            `json:"force_backend,omitempty"`
	Priority     int               `json:"priority,omitempty"`
}

// SessionListRequest represents parameters for listing sessions
type SessionListRequest struct {
	Status    string `form:"status"`
	Backend   string `form:"backend"`
	Limit     int    `form:"limit"`
	Offset    int    `form:"offset"`
	SortBy    string `form:"sort_by"`
	SortOrder string `form:"sort_order"`
}

// SessionUpdateRequest represents a request to update session
type SessionUpdateRequest struct {
	Priority int               `json:"priority,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// BatchOperationRequest represents a batch operation on sessions
type BatchOperationRequest struct {
	SessionIDs []string `json:"session_ids" binding:"required"`
	Operation  string   `json:"operation" binding:"required"`
	Force      bool     `json:"force,omitempty"`
}

// BackendConfigRequest represents backend configuration
type BackendConfigRequest struct {
	MaxSessions int               `json:"max_sessions,omitempty"`
	Priority    int               `json:"priority,omitempty"`
	Enabled     bool              `json:"enabled,omitempty"`
	Settings    map[string]string `json:"settings,omitempty"`
}

// API Response Types

// PlaybackDecisionResponse represents a playback decision response
type PlaybackDecisionResponse struct {
	*PlaybackDecision
	RequestID   string            `json:"request_id"`
	ProcessTime int64             `json:"process_time_ms"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// TranscodeSessionResponse represents an enhanced session response
type TranscodeSessionResponse struct {
	*plugins.TranscodeSession
	QueuePosition  int               `json:"queue_position,omitempty"`
	EstimatedTime  int64             `json:"estimated_completion_ms,omitempty"`
	Speed          float64           `json:"speed,omitempty"`
	BytesProcessed int64             `json:"bytes_processed,omitempty"`
	BytesRemaining int64             `json:"bytes_remaining,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// SessionListResponse represents the response for listing sessions
type SessionListResponse struct {
	Sessions   []TranscodeSessionResponse `json:"sessions"`
	Total      int                        `json:"total"`
	Limit      int                        `json:"limit"`
	Offset     int                        `json:"offset"`
	HasMore    bool                       `json:"has_more"`
	FilteredBy map[string]string          `json:"filtered_by,omitempty"`
}

// BackendInfoResponse represents backend information
type BackendInfoResponse struct {
	ID            string                           `json:"id"`
	Name          string                           `json:"name"`
	Status        string                           `json:"status"`
	Version       string                           `json:"version,omitempty"`
	Capabilities  *plugins.TranscodingCapabilities `json:"capabilities"`
	Statistics    *BackendStatistics               `json:"statistics"`
	Configuration map[string]interface{}           `json:"configuration,omitempty"`
}

// BackendStatistics represents detailed backend statistics
type BackendStatistics struct {
	*BackendStats
	QueueDepth      int                `json:"queue_depth"`
	ProcessingTime  float64            `json:"avg_processing_time_ms"`
	ThroughputMbps  float64            `json:"throughput_mbps"`
	ErrorDetails    []ErrorInfo        `json:"recent_errors,omitempty"`
	PerformanceData []PerformancePoint `json:"performance_history,omitempty"`
}

// ErrorInfo represents error information
type ErrorInfo struct {
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id"`
	Error     string    `json:"error"`
	Context   string    `json:"context,omitempty"`
}

// PerformancePoint represents a performance data point
type PerformancePoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Metric    string    `json:"metric"`
}

// SystemInfoResponse represents system-wide information
type SystemInfoResponse struct {
	Status             string                    `json:"status"`
	Version            string                    `json:"version"`
	Uptime             string                    `json:"uptime"`
	TotalSessions      int                       `json:"total_sessions"`
	ActiveSessions     int                       `json:"active_sessions"`
	QueuedSessions     int                       `json:"queued_sessions"`
	AvailableBackends  int                       `json:"available_backends"`
	SystemCapabilities *SystemCapabilities       `json:"capabilities"`
	PerformanceMetrics *SystemPerformanceMetrics `json:"performance"`
	Configuration      *PlaybackModuleConfig     `json:"configuration"`
	BackendSummary     []BackendSummary          `json:"backends"`
}

// SystemCapabilities represents system-wide capabilities
type SystemCapabilities struct {
	SupportedCodecs       []string `json:"supported_codecs"`
	SupportedResolutions  []string `json:"supported_resolutions"`
	SupportedContainers   []string `json:"supported_containers"`
	MaxConcurrentSessions int      `json:"max_concurrent_sessions"`
	HardwareAcceleration  bool     `json:"hardware_acceleration"`
}

// SystemPerformanceMetrics represents system performance
type SystemPerformanceMetrics struct {
	CPUUsage       float64   `json:"cpu_usage_percent"`
	MemoryUsage    int64     `json:"memory_usage_bytes"`
	DiskUsage      int64     `json:"disk_usage_bytes"`
	NetworkTraffic int64     `json:"network_traffic_bytes"`
	TotalProcessed int64     `json:"total_bytes_processed"`
	AverageSpeed   float64   `json:"average_speed_mbps"`
	LastUpdated    time.Time `json:"last_updated"`
}

// BackendSummary represents a summary of a backend
type BackendSummary struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Status         string   `json:"status"`
	Priority       int      `json:"priority"`
	ActiveSessions int      `json:"active_sessions"`
	MaxSessions    int      `json:"max_sessions"`
	Codecs         []string `json:"supported_codecs"`
}

// Enhanced HTTP Handlers

// handlePlaybackDecisionEnhanced analyzes media and returns playback decision with enhanced metadata
func (pm *PlaybackModule) handlePlaybackDecisionEnhanced(c *gin.Context) {
	startTime := time.Now()
	requestID := fmt.Sprintf("req_%d", time.Now().UnixNano())

	var request PlaybackDecisionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Convert DeviceProfile to plugins.DeviceProfile
	pluginProfile := &plugins.DeviceProfile{
		UserAgent:       request.DeviceProfile.UserAgent,
		SupportedCodecs: request.DeviceProfile.SupportedCodecs,
		MaxResolution:   request.DeviceProfile.MaxResolution,
		MaxBitrate:      request.DeviceProfile.MaxBitrate,
		SupportsHEVC:    request.DeviceProfile.SupportsHEVC,
		SupportsAV1:     request.DeviceProfile.SupportsAV1,
		SupportsHDR:     request.DeviceProfile.SupportsHDR,
		ClientIP:        request.DeviceProfile.ClientIP,
	}

	decision, err := pm.planner.DecidePlayback(request.MediaPath, pluginProfile)
	if err != nil {
		pm.logger.Error("playback decision failed",
			"error", err,
			"media_path", request.MediaPath,
			"request_id", requestID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	// Apply request options to decision
	if decision.TranscodeParams != nil && request.Options.PreferredCodec != "" {
		if decision.TranscodeParams.CodecOpts == nil {
			decision.TranscodeParams.CodecOpts = &plugins.CodecOptions{}
		}
		decision.TranscodeParams.CodecOpts.Video = request.Options.PreferredCodec
	}
	if decision.TranscodeParams != nil && request.Options.Quality > 0 {
		if decision.TranscodeParams.CodecOpts == nil {
			decision.TranscodeParams.CodecOpts = &plugins.CodecOptions{}
		}
		decision.TranscodeParams.CodecOpts.Quality = request.Options.Quality
	}
	if decision.TranscodeParams != nil && request.Options.Preset != "" {
		if decision.TranscodeParams.CodecOpts == nil {
			decision.TranscodeParams.CodecOpts = &plugins.CodecOptions{}
		}
		decision.TranscodeParams.CodecOpts.Preset = request.Options.Preset
	}

	processTime := time.Since(startTime).Milliseconds()

	response := &PlaybackDecisionResponse{
		PlaybackDecision: decision,
		RequestID:        requestID,
		ProcessTime:      processTime,
		Metadata:         request.Options.Metadata,
	}

	pm.logger.Info("playback decision completed",
		"request_id", requestID,
		"should_transcode", decision.ShouldTranscode,
		"process_time_ms", processTime)

	c.JSON(http.StatusOK, response)
}

// handleStartTranscodeEnhanced initiates a new transcoding session with enhanced options
func (pm *PlaybackModule) handleStartTranscodeEnhanced(c *gin.Context) {
	fmt.Printf("=== HANDLESTARTTRANSCODE FUNCTION ENTRY ===\n")

	var request TranscodeStartRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Convert to internal request format
	transcodeReq := &plugins.TranscodeRequest{
		InputPath: request.InputPath,
		CodecOpts: &plugins.CodecOptions{
			Video:     request.CodecOpts.Video,
			Audio:     request.CodecOpts.Audio,
			Container: request.CodecOpts.Container,
		},
		Environment: make(map[string]string),
	}

	// Add environment variables
	if request.Environment != nil {
		for k, v := range request.Environment {
			transcodeReq.Environment[k] = v
		}
	}

	fmt.Printf("DEBUG: About to call transcodeManager.StartTranscode\n")
	// Start transcoding session
	session, err := pm.transcodeManager.StartTranscode(transcodeReq)
	if err != nil {
		fmt.Printf("DEBUG: transcodeManager.StartTranscode failed with error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	fmt.Printf("DEBUG: transcodeManager.StartTranscode succeeded\n")

	// Convert to enhanced response
	response := &TranscodeSessionResponse{
		TranscodeSession: session,
		Metadata:         request.Metadata,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	pm.logger.Info("transcoding session started",
		"session_id", session.ID,
		"session_name", request.SessionName,
		"backend", session.Backend)

	c.JSON(http.StatusCreated, response)
}

// handleListSessionsEnhanced returns all transcoding sessions with filtering and pagination
func (pm *PlaybackModule) handleListSessionsEnhanced(c *gin.Context) {
	var request SessionListRequest
	if err := c.ShouldBindQuery(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set defaults
	if request.Limit <= 0 || request.Limit > 100 {
		request.Limit = 20
	}
	if request.SortBy == "" {
		request.SortBy = "start_time"
	}
	if request.SortOrder == "" {
		request.SortOrder = "desc"
	}

	sessions, err := pm.transcodeManager.ListSessions()
	if err != nil {
		pm.logger.Error("failed to list sessions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Apply filters
	filteredSessions := pm.filterSessions(sessions, request)

	// Apply pagination
	total := len(filteredSessions)
	start := request.Offset
	end := start + request.Limit
	if end > total {
		end = total
	}
	if start >= total {
		start = total
		end = total
	}

	paginatedSessions := filteredSessions[start:end]

	// Convert to enhanced response
	enhancedSessions := make([]TranscodeSessionResponse, len(paginatedSessions))
	for i, session := range paginatedSessions {
		enhancedSessions[i] = TranscodeSessionResponse{
			TranscodeSession: session,
			CreatedAt:        session.StartTime,
			UpdatedAt:        session.StartTime, // Would be tracked separately in real implementation
		}
	}

	response := &SessionListResponse{
		Sessions: enhancedSessions,
		Total:    total,
		Limit:    request.Limit,
		Offset:   request.Offset,
		HasMore:  end < total,
		FilteredBy: map[string]string{
			"status":  request.Status,
			"backend": request.Backend,
		},
	}

	c.JSON(http.StatusOK, response)
}

// handleUpdateSession updates session metadata or priority
func (pm *PlaybackModule) handleUpdateSession(c *gin.Context) {
	sessionID := c.Param("sessionId")

	var request SessionUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := pm.transcodeManager.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Update session (this would require extending the transcoding interface)
	// For now, just return the session with updated metadata indicator
	response := &TranscodeSessionResponse{
		TranscodeSession: session,
		Metadata:         request.Metadata,
		UpdatedAt:        time.Now(),
	}

	pm.logger.Info("session updated", "session_id", sessionID)
	c.JSON(http.StatusOK, response)
}

// handleBatchOperation performs batch operations on multiple sessions
func (pm *PlaybackModule) handleBatchOperation(c *gin.Context) {
	var request BatchOperationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	results := make(map[string]interface{})

	switch strings.ToLower(request.Operation) {
	case "stop":
		for _, sessionID := range request.SessionIDs {
			err := pm.transcodeManager.StopSession(sessionID)
			if err != nil {
				results[sessionID] = gin.H{"success": false, "error": err.Error()}
			} else {
				results[sessionID] = gin.H{"success": true}
			}
		}
	case "priority":
		// Would require extending interface to support priority updates
		for _, sessionID := range request.SessionIDs {
			results[sessionID] = gin.H{"success": false, "error": "priority update not implemented"}
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported operation"})
		return
	}

	response := gin.H{
		"operation": request.Operation,
		"results":   results,
		"total":     len(request.SessionIDs),
	}

	pm.logger.Info("batch operation completed",
		"operation", request.Operation,
		"session_count", len(request.SessionIDs))

	c.JSON(http.StatusOK, response)
}

// handleGetBackendInfo returns detailed information about transcoding backends
func (pm *PlaybackModule) handleGetBackendInfo(c *gin.Context) {
	backendID := c.Param("backendId")

	stats, err := pm.transcodeManager.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if backendID != "" {
		// Get specific backend info
		if backend, exists := stats.Backends[backendID]; exists {
			response := &BackendInfoResponse{
				ID:           backendID,
				Name:         backend.Name,
				Status:       "running", // Would be determined from plugin status
				Capabilities: backend.Capabilities,
				Statistics: &BackendStatistics{
					BackendStats: backend,
					QueueDepth:   0, // Would be tracked separately
				},
			}
			c.JSON(http.StatusOK, response)
		} else {
			c.JSON(http.StatusNotFound, gin.H{"error": "backend not found"})
		}
		return
	}

	// Get all backends info
	backends := make([]BackendInfoResponse, 0, len(stats.Backends))
	for id, backend := range stats.Backends {
		backends = append(backends, BackendInfoResponse{
			ID:           id,
			Name:         backend.Name,
			Status:       "running",
			Capabilities: backend.Capabilities,
			Statistics: &BackendStatistics{
				BackendStats: backend,
				QueueDepth:   0,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{"backends": backends})
}

// handleSystemInfo returns comprehensive system information
func (pm *PlaybackModule) handleSystemInfo(c *gin.Context) {
	stats, err := pm.transcodeManager.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Collect system capabilities
	allCodecs := make(map[string]bool)
	allResolutions := make(map[string]bool)
	allContainers := make(map[string]bool)
	maxSessions := 0
	hasHardwareAccel := false

	for _, backend := range stats.Backends {
		for _, codec := range backend.Capabilities.SupportedCodecs {
			allCodecs[codec] = true
		}
		for _, res := range backend.Capabilities.SupportedResolutions {
			allResolutions[res] = true
		}
		for _, container := range backend.Capabilities.SupportedContainers {
			allContainers[container] = true
		}
		maxSessions += backend.Capabilities.MaxConcurrentSessions
		if backend.Capabilities.HardwareAcceleration {
			hasHardwareAccel = true
		}
	}

	capabilities := &SystemCapabilities{
		SupportedCodecs:       keys(allCodecs),
		SupportedResolutions:  keys(allResolutions),
		SupportedContainers:   keys(allContainers),
		MaxConcurrentSessions: maxSessions,
		HardwareAcceleration:  hasHardwareAccel,
	}

	// Create backend summaries
	backendSummaries := make([]BackendSummary, 0, len(stats.Backends))
	for id, backend := range stats.Backends {
		backendSummaries = append(backendSummaries, BackendSummary{
			ID:             id,
			Name:           backend.Name,
			Status:         "running",
			Priority:       backend.Priority,
			ActiveSessions: backend.ActiveSessions,
			MaxSessions:    backend.Capabilities.MaxConcurrentSessions,
			Codecs:         backend.Capabilities.SupportedCodecs,
		})
	}

	response := &SystemInfoResponse{
		Status:             "healthy",
		Version:            "1.0.0",  // Would come from build info
		Uptime:             "1h 30m", // Would be tracked
		TotalSessions:      int(stats.TotalSessions),
		ActiveSessions:     stats.ActiveSessions,
		QueuedSessions:     0, // Would be tracked
		AvailableBackends:  len(stats.Backends),
		SystemCapabilities: capabilities,
		PerformanceMetrics: &SystemPerformanceMetrics{
			AverageSpeed: stats.AverageSpeed,
			LastUpdated:  time.Now(),
		},
		BackendSummary: backendSummaries,
	}

	c.JSON(http.StatusOK, response)
}

// handleRefreshPlugins manually refreshes the plugin discovery
func (pm *PlaybackModule) handleRefreshPlugins(c *gin.Context) {
	err := pm.RefreshTranscodingPlugins()
	if err != nil {
		pm.logger.Error("failed to refresh plugins", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	stats, err := pm.transcodeManager.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "refresh succeeded but failed to get updated stats"})
		return
	}

	response := gin.H{
		"message":            "plugins refreshed successfully",
		"available_backends": len(stats.Backends),
		"backends":           make([]string, 0, len(stats.Backends)),
	}

	for id := range stats.Backends {
		response["backends"] = append(response["backends"].([]string), id)
	}

	pm.logger.Info("plugins refreshed", "backend_count", len(stats.Backends))
	c.JSON(http.StatusOK, response)
}

// handleSeekAhead starts transcoding from a specific timestamp for seek-ahead functionality
func (pm *PlaybackModule) handleSeekAhead(c *gin.Context) {
	var request struct {
		SessionID string `json:"session_id" binding:"required"`
		SeekTime  int    `json:"seek_time" binding:"required"` // Time in seconds
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get the current session to extract the original request
	currentSession, err := pm.transcodeManager.GetSession(request.SessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "original session not found"})
		return
	}

	// Try to get input path from multiple sources
	var inputPath string

	// First try: Get from session Request field if available
	if currentSession.Request != nil && currentSession.Request.InputPath != "" {
		inputPath = currentSession.Request.InputPath
	}

	// Second try: Get from session metadata
	if inputPath == "" && currentSession.Metadata != nil {
		if path, ok := currentSession.Metadata["input_path"].(string); ok && path != "" {
			inputPath = path
		}
	}

	// Third try: Extract from output path pattern (if it contains the original media file ID)
	if inputPath == "" && currentSession.Metadata != nil {
		// Try to extract from session_dir or output_path
		if sessionDir, ok := currentSession.Metadata["session_dir"].(string); ok {
			// Look for media file ID in the path
			// This is a fallback - ideally we should have the input path stored
			pm.logger.Warn("seek-ahead: input path not found in session, unable to extract from session dir", "session_id", request.SessionID, "session_dir", sessionDir)
		}
	}

	if inputPath == "" {
		pm.logger.Error("seek-ahead: cannot determine input path for session", "session_id", request.SessionID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot determine input path for session"})
		return
	}

	// Create a new transcode request starting from the seek time
	seekRequest := &plugins.TranscodeRequest{
		InputPath: inputPath,
		SessionID: fmt.Sprintf("%s_seek_%d", request.SessionID, request.SeekTime),
		Seek:      time.Duration(request.SeekTime) * time.Second,
		Environment: map[string]string{
			"SEEK_START": fmt.Sprintf("%d", request.SeekTime),
		},
	}

	// Copy codec options and device profile from the original session if available
	if currentSession.Request != nil {
		seekRequest.CodecOpts = currentSession.Request.CodecOpts
		seekRequest.DeviceProfile = currentSession.Request.DeviceProfile
	} else {
		// Use defaults if original request is not available
		seekRequest.CodecOpts = &plugins.CodecOptions{
			Container: "dash",
			Video:     "h264",
			Audio:     "aac",
		}
	}

	// Start new transcoding session from seek time
	newSession, err := pm.transcodeManager.StartTranscode(seekRequest)
	if err != nil {
		pm.logger.Error("failed to start seek-ahead transcode", "error", err, "seek_time", request.SeekTime)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start seek-ahead transcoding"})
		return
	}

	// Construct response with new session info
	manifestURL := ""
	switch seekRequest.CodecOpts.Container {
	case "dash":
		manifestURL = fmt.Sprintf("/api/playback/stream/%s/manifest.mpd", newSession.ID)
	case "hls":
		manifestURL = fmt.Sprintf("/api/playback/stream/%s/playlist.m3u8", newSession.ID)
	default:
		manifestURL = fmt.Sprintf("/api/playback/stream/%s", newSession.ID)
	}

	pm.logger.Info("seek-ahead transcoding started",
		"original_session_id", request.SessionID,
		"new_session_id", newSession.ID,
		"seek_time", request.SeekTime,
		"manifest_url", manifestURL)

	c.JSON(http.StatusOK, gin.H{
		"message":      "seek-ahead transcoding started",
		"session_id":   newSession.ID,
		"manifest_url": manifestURL,
		"seek_time":    request.SeekTime,
		"status":       string(newSession.Status),
		"backend":      newSession.Backend,
	})
}

// Utility functions

// filterSessions applies filters to the session list
func (pm *PlaybackModule) filterSessions(sessions []*plugins.TranscodeSession, request SessionListRequest) []*plugins.TranscodeSession {
	var filtered []*plugins.TranscodeSession

	for _, session := range sessions {
		// Apply status filter
		if request.Status != "" && string(session.Status) != request.Status {
			continue
		}

		// Apply backend filter
		if request.Backend != "" && session.Backend != request.Backend {
			continue
		}

		filtered = append(filtered, session)
	}

	return filtered
}

// keys extracts keys from a map[string]bool
func keys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
