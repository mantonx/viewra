package playbackmodule

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/modules/modulemanager"
	"github.com/mantonx/viewra/pkg/plugins"
	"gorm.io/gorm"
)

// Module represents the playback module compatible with module manager
type Module struct {
	id           string
	name         string
	version      string
	core         bool
	initialized  bool
	db           *gorm.DB
	eventBus     events.EventBus
	playbackCore *PlaybackModule
}

// Auto-register the module when imported
func init() {
	Register()
}

// Register registers this module with the module system
func Register() {
	// Create module without database connection - it will be initialized later
	playbackModule := &Module{
		id:      "system.playback",
		name:    "Playback Manager",
		version: "1.0.0",
		core:    true,
	}
	modulemanager.Register(playbackModule)
}

// Module interface implementation

// ID returns the module ID
func (m *Module) ID() string {
	return m.id
}

// Name returns the module name
func (m *Module) Name() string {
	return m.name
}

// GetVersion returns the module version
func (m *Module) GetVersion() string {
	return m.version
}

// Core returns whether this is a core module
func (m *Module) Core() bool {
	return m.core
}

// IsInitialized returns whether the module is initialized
func (m *Module) IsInitialized() bool {
	return m.initialized
}

// Initialize sets up the playback module
func (m *Module) Initialize() error {
	log.Println("INFO: Initializing playback module")
	return nil
}

// Migrate performs any pending migrations
func (m *Module) Migrate(db *gorm.DB) error {
	log.Println("INFO: Migrating playback module schema")
	// Add any database migrations here if needed
	return nil
}

// Init initializes the playback module components
func (m *Module) Init() error {
	log.Println("INFO: Initializing playback module components")

	// Get the database connection and event bus from the global system
	m.db = database.GetDB()
	m.eventBus = events.GetGlobalEventBus()

	// Create a simple logger for the playback core
	logger := hclog.NewNullLogger()

	// Initialize the core playback module (without plugin manager for now)
	m.playbackCore = NewSimplePlaybackModule(logger, m.db)
	if err := m.playbackCore.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize playback core: %w", err)
	}

	m.initialized = true

	// Publish initialization event
	if m.eventBus != nil {
		initEvent := events.NewSystemEvent(
			events.EventInfo,
			"Playback Module Initialized",
			"Playback module has been successfully initialized",
		)
		m.eventBus.PublishAsync(initEvent)
	}

	log.Println("INFO: Playback module initialized successfully")
	return nil
}

// RegisterRoutes registers the playback module API routes
func (m *Module) RegisterRoutes(router *gin.Engine) {
	log.Printf("INFO: Registering playback module routes (initialized: %v, core: %v)", m.initialized, m.playbackCore != nil)

	if m.playbackCore != nil {
		m.playbackCore.RegisterRoutes(router)
	}
}

// Shutdown gracefully shuts down the playback module
func (m *Module) Shutdown(ctx context.Context) error {
	log.Println("INFO: Shutting down playback module")

	// Shutdown playback core if it exists
	// Note: PlaybackModule doesn't have a Shutdown method, but cleanup is handled in its cleanup routine

	m.initialized = false
	log.Println("INFO: Playback module shutdown complete")
	return nil
}

// GetPlaybackCore returns the core playback module
func (m *Module) GetPlaybackCore() *PlaybackModule {
	return m.playbackCore
}

// SetPlaybackCore sets the core playback module (used for plugin integration)
func (m *Module) SetPlaybackCore(core *PlaybackModule) {
	m.playbackCore = core
}

// PlaybackModule manages video playback and transcoding
type PlaybackModule struct {
	logger           hclog.Logger
	planner          PlaybackPlanner
	transcodeManager TranscodeManager
	profileManager   TranscodeProfileManager
	pluginManager    PluginManagerInterface
	ctx              context.Context
	cancel           context.CancelFunc

	// Configuration
	enabled bool
}

// NewPlaybackModule creates a new playback module instance with plugin system
func NewPlaybackModule(logger hclog.Logger, pluginManager PluginManagerInterface) *PlaybackModule {
	ctx, cancel := context.WithCancel(context.Background())
	return &PlaybackModule{
		logger:           logger.Named("playback-module"),
		planner:          NewPlaybackPlanner(),
		transcodeManager: NewTranscodeManager(logger, nil, pluginManager),
		pluginManager:    pluginManager,
		ctx:              ctx,
		cancel:           cancel,
		enabled:          true,
	}
}

// NewSimplePlaybackModule creates a new playback module instance without plugin support
func NewSimplePlaybackModule(logger hclog.Logger, db *gorm.DB) *PlaybackModule {
	ctx, cancel := context.WithCancel(context.Background())
	return &PlaybackModule{
		logger:           logger.Named("playback-module"),
		planner:          NewPlaybackPlanner(),
		transcodeManager: NewTranscodeManager(logger, db, nil),
		pluginManager:    nil, // No plugin manager for fallback scenarios
		ctx:              ctx,
		cancel:           cancel,
		enabled:          true,
	}
}

// Initialize sets up the playback module
func (pm *PlaybackModule) Initialize() error {
	pm.logger.Info("initializing playback module")

	// Initialize the transcoding manager
	if initializer, ok := pm.transcodeManager.(interface{ Initialize() error }); ok {
		if err := initializer.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize transcode manager: %w", err)
		}
	}

	// Discover transcoding plugins only if using plugin manager
	if pm.pluginManager != nil {
		if err := pm.transcodeManager.DiscoverTranscodingPlugins(); err != nil {
			pm.logger.Warn("failed to discover transcoding plugins", "error", err)
		}
	}

	// Start the session cleanup service
	pm.startCleanupService()

	pm.logger.Info("playback module initialized successfully")
	return nil
}

// RegisterRoutes sets up HTTP endpoints for the playback module
func (pm *PlaybackModule) RegisterRoutes(router *gin.Engine) {
	playbackGroup := router.Group("/api/playback")
	{
		// Playback decision endpoint
		playbackGroup.POST("/decide", pm.handlePlaybackDecision)

		// Session management endpoints
		playbackGroup.POST("/start", pm.handleStartTranscode)
		playbackGroup.POST("/seek-ahead", pm.handleSeekAhead)
		playbackGroup.GET("/session/:sessionId", pm.handleGetSession)
		playbackGroup.DELETE("/session/:sessionId", pm.handleStopTranscode)
		playbackGroup.GET("/sessions", pm.handleListSessions)

		// Statistics endpoint
		playbackGroup.GET("/stats", pm.handleGetStats)

		// Health check endpoint
		playbackGroup.GET("/health", pm.handleHealthCheck)
		playbackGroup.HEAD("/health", pm.handleHealthCheck)

		// Plugin management endpoints
		playbackGroup.POST("/plugins/refresh", pm.handleRefreshPlugins)

		// Cleanup management endpoints
		playbackGroup.POST("/cleanup/run", pm.handleManualCleanup)
		playbackGroup.GET("/cleanup/stats", pm.handleCleanupStats)

		// DASH/HLS segment routes (specific patterns MUST come before catch-all)
		// Support both GET and HEAD requests for segment access
		// These patterns match exactly what FFmpeg generates:
		// - init-$RepresentationID$.m4s (e.g., init-0.m4s, init-1.m4s)
		// - chunk-$RepresentationID$-$Number$.m4s (e.g., chunk-0-1.m4s, chunk-1-1.m4s)
		playbackGroup.GET("/stream/:sessionId/:segmentFile", pm.handleDashSegmentSpecific)
		playbackGroup.HEAD("/stream/:sessionId/:segmentFile", pm.handleDashSegmentSpecific)

		// DASH/HLS streaming endpoints
		playbackGroup.GET("/stream/:sessionId/manifest.mpd", pm.handleDashManifest)
		playbackGroup.HEAD("/stream/:sessionId/manifest.mpd", pm.handleDashManifest)
		playbackGroup.GET("/stream/:sessionId/playlist.m3u8", pm.handleHlsPlaylist)
		playbackGroup.HEAD("/stream/:sessionId/playlist.m3u8", pm.handleHlsPlaylist)
		playbackGroup.GET("/stream/:sessionId/segment/:segmentName", pm.handleSegment)

		// Progressive streaming endpoint (catch-all MUST come last)
		playbackGroup.GET("/stream/:sessionId", pm.handleStreamTranscode)
	}

	pm.logger.Info("playback module routes registered")
}

// RefreshTranscodingPlugins refreshes the list of available transcoding plugins
func (pm *PlaybackModule) RefreshTranscodingPlugins() error {
	return pm.transcodeManager.DiscoverTranscodingPlugins()
}

// HTTP Handlers

// handlePlaybackDecision analyzes media and returns playback decision
func (pm *PlaybackModule) handlePlaybackDecision(c *gin.Context) {
	var request struct {
		MediaPath string        `json:"media_path" binding:"required"`
		Profile   DeviceProfile `json:"device_profile" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert DeviceProfile to plugins.DeviceProfile
	pluginProfile := &plugins.DeviceProfile{
		UserAgent:       request.Profile.UserAgent,
		SupportedCodecs: request.Profile.SupportedCodecs,
		MaxResolution:   request.Profile.MaxResolution,
		MaxBitrate:      request.Profile.MaxBitrate,
		SupportsHEVC:    request.Profile.SupportsHEVC,
		SupportsAV1:     request.Profile.SupportsAV1,
		SupportsHDR:     request.Profile.SupportsHDR,
		ClientIP:        request.Profile.ClientIP,
	}

	decision, err := pm.planner.DecidePlayback(request.MediaPath, pluginProfile)
	if err != nil {
		pm.logger.Error("playback decision failed", "error", err, "media_path", request.MediaPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, decision)
}

// waitForManifest waits for the manifest file to be created for DASH/HLS sessions
func (pm *PlaybackModule) waitForManifest(session *plugins.TranscodeSession, maxWaitSeconds int) error {
	pm.logger.Info("waitForManifest called", "session_id", session.ID, "max_wait", maxWaitSeconds)

	// Get container type from CodecOpts
	var container string
	if session.Request != nil && session.Request.CodecOpts != nil {
		container = session.Request.CodecOpts.Container
		pm.logger.Info("container detected", "container", container, "session_id", session.ID)
	} else {
		pm.logger.Info("no container in session request", "session_id", session.ID, "request_nil", session.Request == nil)
	}

	// Only wait for DASH/HLS sessions
	if container != "dash" && container != "hls" {
		return nil
	}

	sessionDir := pm.getSessionDirectory(session.ID, session)
	var manifestFile string

	switch container {
	case "dash":
		manifestFile = "manifest.mpd"
	case "hls":
		manifestFile = "playlist.m3u8"
	default:
		return nil
	}

	manifestPath := filepath.Join(sessionDir, manifestFile)

	pm.logger.Info("waiting for manifest file", "path", manifestPath, "session_id", session.ID, "session_dir", sessionDir, "backend", session.Backend)

	// Debug: List actual directories in transcoding folder
	cfg := config.Get()
	if entries, err := os.ReadDir(cfg.Transcoding.DataDir); err == nil {
		var dirs []string
		for _, entry := range entries {
			if entry.IsDir() && strings.Contains(entry.Name(), session.ID) {
				dirs = append(dirs, entry.Name())
			}
		}
		pm.logger.Info("actual directories found with session ID", "session_id", session.ID, "directories", dirs)
	}

	// Wait for the manifest file to be created
	for i := 0; i < maxWaitSeconds; i++ {
		if _, err := os.Stat(manifestPath); err == nil {
			pm.logger.Debug("manifest file ready", "path", manifestPath, "session_id", session.ID, "wait_time", i)
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("manifest file not created within %d seconds", maxWaitSeconds)
}

// handleStartTranscode initiates a new transcoding session
func (pm *PlaybackModule) handleStartTranscode(c *gin.Context) {
	pm.logger.Info("handleStartTranscode called")
	var request plugins.TranscodeRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		pm.logger.Error("failed to bind JSON request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Log request info with nil safety
	container := "none"
	if request.CodecOpts != nil {
		container = request.CodecOpts.Container
	}
	pm.logger.Info("JSON request bound successfully", "input_path", request.InputPath, "container", container)

	pm.logger.Info("calling transcodeManager.StartTranscode")
	session, err := pm.transcodeManager.StartTranscode(&request)

	if err != nil {
		pm.logger.Error("failed to start transcode", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start transcoding session: " + err.Error()})
		return
	}

	pm.logger.Info("transcode session created successfully", "session_id", session.ID)

	// No manifest waiting - return immediately for DASH streaming
	pm.logger.Info("returning session info immediately (no manifest waiting)")

	// Return the session information
	c.JSON(http.StatusOK, gin.H{
		"id":           session.ID,
		"status":       session.Status,
		"manifest_url": fmt.Sprintf("/api/playback/stream/%s/manifest.mpd", session.ID),
		"backend":      session.Backend,
	})
}

// handleStopTranscode terminates a transcoding session
func (pm *PlaybackModule) handleStopTranscode(c *gin.Context) {
	sessionID := c.Param("sessionId")

	if err := pm.transcodeManager.StopSession(sessionID); err != nil {
		pm.logger.Error("failed to stop transcode", "error", err, "session_id", sessionID)
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "session stopped"})
}

// handleGetSession retrieves transcoding session information
func (pm *PlaybackModule) handleGetSession(c *gin.Context) {
	sessionID := c.Param("sessionId")

	session, err := pm.transcodeManager.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

// handleStreamTranscode streams transcoded video data
func (pm *PlaybackModule) handleStreamTranscode(c *gin.Context) {
	sessionID := c.Param("sessionId")

	// Get the transcoding service for this session
	transcodingService, err := pm.transcodeManager.GetTranscodeStream(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Get the stream from the transcoding service
	stream, err := transcodingService.GetTranscodeStream(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "stream not available"})
		return
	}
	defer stream.Close()

	// Set appropriate headers for video streaming
	c.Header("Content-Type", "video/mp4")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// Stream the transcoded data directly
	buffer := make([]byte, 32768) // 32KB buffer
	for {
		n, err := stream.Read(buffer)
		if err != nil {
			if err != io.EOF {
				pm.logger.Error("error reading from transcode stream", "error", err)
			}
			break
		}

		if _, writeErr := c.Writer.Write(buffer[:n]); writeErr != nil {
			pm.logger.Error("error writing to response", "error", writeErr)
			break
		}

		c.Writer.Flush()
	}
}

// handleListSessions returns all active transcoding sessions
func (pm *PlaybackModule) handleListSessions(c *gin.Context) {
	sessions, err := pm.transcodeManager.ListSessions()
	if err != nil {
		pm.logger.Error("failed to list sessions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// handleGetStats returns transcoding statistics
func (pm *PlaybackModule) handleGetStats(c *gin.Context) {
	stats, err := pm.transcodeManager.GetStats()
	if err != nil {
		pm.logger.Error("failed to get stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// handleHealthCheck returns module health status
func (pm *PlaybackModule) handleHealthCheck(c *gin.Context) {
	health := gin.H{
		"status":  "healthy",
		"enabled": pm.enabled,
		"uptime":  time.Since(time.Now()).String(), // This would be tracked properly
	}

	c.JSON(http.StatusOK, health)
}

// getSessionDirectory returns the correct session directory path based on container type
func (pm *PlaybackModule) getSessionDirectory(sessionID string, session *plugins.TranscodeSession) string {
	cfg := config.Get()

	// Determine directory name based on container type, using plugin naming convention
	var dirName string
	if session != nil && session.Request != nil && session.Request.CodecOpts != nil {
		switch session.Request.CodecOpts.Container {
		case "dash":
			// Plugin naming convention: [container]_[backend]_[sessionID]
			dirName = fmt.Sprintf("dash_%s_%s", session.Backend, sessionID)
		case "hls":
			dirName = fmt.Sprintf("hls_%s_%s", session.Backend, sessionID)
		default:
			// For progressive streaming, use software_[backend]_[sessionID] format
			dirName = fmt.Sprintf("software_%s_%s", session.Backend, sessionID)
		}
	} else {
		// Fallback: try to detect existing directories with this session ID
		return pm.findSessionDirectory(sessionID)
	}

	return filepath.Join(cfg.Transcoding.DataDir, dirName)
}

// findSessionDirectory searches for existing session directories that end with the session ID
func (pm *PlaybackModule) findSessionDirectory(sessionID string) string {
	cfg := config.Get()

	// Try to find existing directory that ends with this session ID
	entries, err := os.ReadDir(cfg.Transcoding.DataDir)
	if err != nil {
		pm.logger.Warn("failed to read transcoding directory", "error", err)
		return filepath.Join(cfg.Transcoding.DataDir, fmt.Sprintf("session_%s", sessionID))
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), sessionID) {
			return filepath.Join(cfg.Transcoding.DataDir, entry.Name())
		}
	}

	// Fallback to generic session directory
	return filepath.Join(cfg.Transcoding.DataDir, fmt.Sprintf("session_%s", sessionID))
}

// handleDashManifest serves DASH manifest files
func (pm *PlaybackModule) handleDashManifest(c *gin.Context) {
	sessionID := c.Param("sessionId")

	// Verify session exists
	session, err := pm.transcodeManager.GetSession(sessionID)
	if err != nil {
		pm.logger.Error("session not found for DASH manifest", "session_id", sessionID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Check session is ready for manifest serving
	if session.Status != "running" && session.Status != "completed" {
		pm.logger.Warn("session not ready for manifest serving", "session_id", sessionID, "status", session.Status)
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not ready"})
		return
	}

	// Construct manifest file path using the correct directory structure
	sessionDir := pm.getSessionDirectory(sessionID, session)
	manifestPath := filepath.Join(sessionDir, "manifest.mpd")

	container := ""
	if session.Request != nil && session.Request.CodecOpts != nil {
		container = session.Request.CodecOpts.Container
	}
	pm.logger.Debug("looking for DASH manifest", "session_id", sessionID, "path", manifestPath, "container", container)

	// Check if manifest file exists
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		pm.logger.Error("DASH manifest file not found", "path", manifestPath, "session_id", sessionID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Manifest file not found"})
		return
	}

	// Set appropriate headers for DASH manifest
	c.Header("Content-Type", "application/dash+xml")
	c.Header("Cache-Control", "no-cache")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Range")

	pm.logger.Debug("serving DASH manifest", "path", manifestPath, "session_id", sessionID)

	// For HEAD requests, just return headers without body
	if c.Request.Method == "HEAD" {
		c.Status(http.StatusOK)
		return
	}

	// Serve the manifest file directly for GET requests
	c.File(manifestPath)
}

// handleHlsPlaylist serves HLS playlist files
func (pm *PlaybackModule) handleHlsPlaylist(c *gin.Context) {
	sessionID := c.Param("sessionId")

	// Get the transcoding service for this session
	transcodingService, err := pm.transcodeManager.GetTranscodeStream(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Set appropriate headers for HLS playlist
	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.Header("Access-Control-Allow-Origin", "*")

	// For HEAD requests, just return headers without body
	if c.Request.Method == "HEAD" {
		c.Status(http.StatusOK)
		return
	}

	// Get the stream from the transcoding service (should be HLS playlist)
	stream, err := transcodingService.GetTranscodeStream(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "HLS playlist not available"})
		return
	}
	defer stream.Close()

	// Stream the playlist data
	c.Stream(func(w io.Writer) bool {
		buffer := make([]byte, 4096)
		n, err := stream.Read(buffer)
		if err != nil {
			return false
		}
		w.Write(buffer[:n])
		return false // HLS playlist is typically served as a complete file
	})
}

// handleSegment serves DASH/HLS segment files
func (pm *PlaybackModule) handleSegment(c *gin.Context) {
	sessionID := c.Param("sessionId")
	segmentName := c.Param("segmentName")

	pm.logger.Info("serving segment", "session_id", sessionID, "segment", segmentName)

	// Verify session exists and get session info
	session, err := pm.transcodeManager.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Get the correct session directory path based on container type
	sessionDir := pm.getSessionDirectory(sessionID, session)

	sessionContainer := ""
	if session.Request != nil && session.Request.CodecOpts != nil {
		sessionContainer = session.Request.CodecOpts.Container
	}
	pm.logger.Debug("using session directory", "session_id", sessionID, "dir", sessionDir, "container", sessionContainer)

	// Construct full path to segment file
	segmentPath := filepath.Join(sessionDir, segmentName)

	// Security check - ensure the segment file is within the session directory
	if !strings.HasPrefix(segmentPath, sessionDir) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid segment path"})
		return
	}

	// Check if segment file exists
	if _, err := os.Stat(segmentPath); os.IsNotExist(err) {
		// For active transcoding sessions, the segment might not be ready yet
		// Wait a bit and try again
		retries := 3
		for i := 0; i < retries; i++ {
			time.Sleep(200 * time.Millisecond)
			if _, err := os.Stat(segmentPath); err == nil {
				break
			}
			if i == retries-1 {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "Segment not found",
					"path":  segmentName,
				})
				return
			}
		}
	}

	// Determine content type based on segment file extension
	contentType := "video/mp4" // Default for DASH segments
	if strings.HasSuffix(segmentName, ".ts") {
		contentType = "video/mp2t" // HLS segments
	} else if strings.HasSuffix(segmentName, ".m4s") {
		contentType = "video/iso.segment" // DASH segments
	}

	// Set appropriate headers for segment serving
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=3600") // Cache segments for 1 hour
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Range")
	c.Header("Accept-Ranges", "bytes")

	// Serve the segment file
	c.File(segmentPath)
}

// handleDashSegmentSpecific serves DASH/HLS segments with specific route patterns
func (pm *PlaybackModule) handleDashSegmentSpecific(c *gin.Context) {
	sessionID := c.Param("sessionId")
	segmentFile := c.Param("segmentFile")

	// Validate segment file name (security check)
	if !strings.HasSuffix(segmentFile, ".m4s") && !strings.HasSuffix(segmentFile, ".ts") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid segment file type"})
		return
	}

	pm.logger.Info("serving segment via catch-all", "session_id", sessionID, "segment", segmentFile)

	// Verify session exists and get session info
	session, err := pm.transcodeManager.GetSession(sessionID)
	if err != nil {
		pm.logger.Error("session not found for segment", "session_id", sessionID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Construct session directory path using the correct directory structure
	sessionDir := pm.getSessionDirectory(sessionID, session)
	fullSegmentPath := filepath.Join(sessionDir, segmentFile)

	debugContainer := ""
	if session.Request != nil && session.Request.CodecOpts != nil {
		debugContainer = session.Request.CodecOpts.Container
	}
	pm.logger.Debug("serving segment via specific route", "session_id", sessionID, "dir", sessionDir, "segment", segmentFile, "container", debugContainer)

	// Security check - ensure the segment file is within the session directory
	if !strings.HasPrefix(fullSegmentPath, sessionDir) {
		pm.logger.Error("invalid segment path attempted", "session_id", sessionID, "segment", segmentFile, "attempted_path", fullSegmentPath)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid segment path"})
		return
	}

	// Check if segment file exists
	if _, err := os.Stat(fullSegmentPath); os.IsNotExist(err) {
		pm.logger.Warn("segment file not found, retrying", "session_id", sessionID, "segment", segmentFile, "path", fullSegmentPath)

		// For active transcoding sessions, the segment might not be ready yet
		retries := 3
		for i := 0; i < retries; i++ {
			time.Sleep(200 * time.Millisecond)
			if _, err := os.Stat(fullSegmentPath); err == nil {
				break
			}
			if i == retries-1 {
				pm.logger.Error("segment not found after retries", "session_id", sessionID, "segment", segmentFile, "path", fullSegmentPath)
				c.JSON(http.StatusNotFound, gin.H{
					"error":   "Segment not found",
					"segment": segmentFile,
				})
				return
			}
		}
	}

	// Determine content type based on segment file extension
	contentType := "video/mp4" // Default
	if strings.HasSuffix(segmentFile, ".ts") {
		contentType = "video/mp2t" // HLS segments
	} else if strings.HasSuffix(segmentFile, ".m4s") {
		contentType = "video/iso.segment" // DASH segments
	}

	// Set appropriate headers for segment serving
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=3600") // Cache segments for 1 hour
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Range")
	c.Header("Accept-Ranges", "bytes")

	pm.logger.Debug("serving segment file", "session_id", sessionID, "segment", segmentFile, "path", fullSegmentPath, "content_type", contentType)

	// Serve the segment file
	c.File(fullSegmentPath)
}

// Profile Management Handlers

// handleListProfiles returns available transcode profiles
func (pm *PlaybackModule) handleListProfiles(c *gin.Context) {
	if pm.profileManager == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "profile manager not available"})
		return
	}

	profiles := pm.profileManager.ListProfiles()
	c.JSON(http.StatusOK, gin.H{"profiles": profiles})
}

// handleCreateProfile creates a new transcode profile
func (pm *PlaybackModule) handleCreateProfile(c *gin.Context) {
	if pm.profileManager == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "profile manager not available"})
		return
	}

	var profile TranscodeProfile
	if err := c.ShouldBindJSON(&profile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := pm.profileManager.CreateProfile(profile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "profile created"})
}

// handleDeleteProfile removes a transcode profile
func (pm *PlaybackModule) handleDeleteProfile(c *gin.Context) {
	if pm.profileManager == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "profile manager not available"})
		return
	}

	name := c.Param("name")
	if err := pm.profileManager.DeleteProfile(name); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "profile deleted"})
}

// Utility Methods

// startCleanupService starts a goroutine that periodically cleans up expired transcoding sessions
func (pm *PlaybackModule) startCleanupService() {
	// Get cleanup configuration from environment or use defaults
	retentionHours := 2  // Default 2 hours retention
	maxSizeLimitGB := 10 // Default 10GB limit

	if envRetention := os.Getenv("VIEWRA_TRANSCODING_RETENTION_HOURS"); envRetention != "" {
		if hours, err := strconv.Atoi(envRetention); err == nil && hours > 0 {
			retentionHours = hours
		}
	}

	if envMaxSize := os.Getenv("VIEWRA_TRANSCODING_MAX_SIZE_GB"); envMaxSize != "" {
		if maxSize, err := strconv.Atoi(envMaxSize); err == nil && maxSize > 0 {
			maxSizeLimitGB = maxSize
		}
	}

	// Create transcoding helper for cleanup operations
	helper := plugins.NewTranscodingHelper(pm.logger)

	go func() {
		ticker := time.NewTicker(30 * time.Minute) // Run cleanup every 30 minutes
		defer ticker.Stop()

		pm.logger.Info("started transcoding cleanup service",
			"retention_hours", retentionHours,
			"max_size_gb", maxSizeLimitGB,
			"cleanup_interval", "30m",
		)

		for {
			select {
			case <-ticker.C:
				pm.runPeriodicCleanup(helper, retentionHours, maxSizeLimitGB)
			case <-pm.ctx.Done():
				pm.logger.Info("stopping transcoding cleanup service")
				return
			}
		}
	}()
}

// runPeriodicCleanup performs the actual cleanup of expired sessions
func (pm *PlaybackModule) runPeriodicCleanup(helper *plugins.TranscodingHelper, retentionHours, maxSizeLimitGB int) {
	pm.logger.Debug("running periodic transcoding cleanup")

	stats, err := helper.CleanupExpiredSessions(retentionHours, maxSizeLimitGB)
	if err != nil {
		pm.logger.Error("failed to cleanup expired sessions", "error", err)
		return
	}

	if stats.DirectoriesRemoved > 0 {
		pm.logger.Info("completed transcoding cleanup",
			"directories_removed", stats.DirectoriesRemoved,
			"size_freed_gb", fmt.Sprintf("%.2f", stats.SizeFreedGB),
			"total_directories", stats.TotalDirectories,
			"total_size_gb", fmt.Sprintf("%.2f", stats.TotalSizeGB),
		)
	} else {
		pm.logger.Debug("no transcoding files required cleanup")
	}
}

// SetEnabled enables or disables the playback module
func (pm *PlaybackModule) SetEnabled(enabled bool) {
	pm.enabled = enabled
	pm.logger.Info("playback module enabled status changed", "enabled", enabled)
}

// IsEnabled returns whether the playback module is enabled
func (pm *PlaybackModule) IsEnabled() bool {
	return pm.enabled
}

// GetPlanner returns the playback planner instance
func (pm *PlaybackModule) GetPlanner() PlaybackPlanner {
	return pm.planner
}

// GetTranscodeManager returns the transcode manager instance
func (pm *PlaybackModule) GetTranscodeManager() TranscodeManager {
	return pm.transcodeManager
}

// SetProfileManager sets the profile manager (optional component)
func (pm *PlaybackModule) SetProfileManager(manager TranscodeProfileManager) {
	pm.profileManager = manager
	pm.logger.Info("profile manager registered")
}

// handlePlaybackStatus provides status information about transcoding sessions and plugin health
func (pm *PlaybackModule) handlePlaybackStatus(c *gin.Context) {
	status := gin.H{
		"timestamp": time.Now(),
		"transcoding": gin.H{
			"active_sessions": 0,
			"max_sessions":    30,
		},
		"plugins": gin.H{
			"available": []string{},
			"health":    "unknown",
		},
	}

	// Get transcoding status from unified manager
	if sessions, err := pm.transcodeManager.ListSessions(); err == nil {
		status["transcoding"].(gin.H)["active_sessions"] = len(sessions)
	}

	// Get stats from unified transcode manager
	if tm, ok := pm.transcodeManager.(*TranscodeManagerImpl); ok {
		transcoders := tm.GetAvailablePlugins()
		status["plugins"].(gin.H)["available"] = transcoders

		if len(transcoders) > 0 {
			status["plugins"].(gin.H)["health"] = "healthy"
		} else {
			status["plugins"].(gin.H)["health"] = "no_plugins"
		}
	}

	c.JSON(http.StatusOK, status)
}

// Cleanup management endpoints

// handleManualCleanup manually triggers a cleanup of transcoding files
func (pm *PlaybackModule) handleManualCleanup(c *gin.Context) {
	pm.transcodeManager.Cleanup()
	c.JSON(http.StatusOK, gin.H{"message": "cleanup triggered successfully"})
}

// handleCleanupStats returns statistics about the cleanup process
func (pm *PlaybackModule) handleCleanupStats(c *gin.Context) {
	stats, err := pm.transcodeManager.GetCleanupStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}
