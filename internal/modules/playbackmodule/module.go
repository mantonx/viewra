// Package playbackmodule provides video playback functionality.
// It handles playback decisions, progressive download, and session management.
package playbackmodule

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/api"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core/cleanup"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core/history"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core/playback"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core/session"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core/streaming"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/models"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/service"
	"github.com/mantonx/viewra/internal/services"
	"gorm.io/gorm"
)

// Module represents the playback module.
// It provides playback decision making, progressive download,
// and session management capabilities.
type Module struct {
	db *gorm.DB

	// Core components
	playbackManager  *playback.Manager
	streamingManager *streaming.Manager
	sessionManager   *session.SessionManager
	cleanupManager   *cleanup.CleanupManager
	historyManager   *history.HistoryManager

	// Service layer
	playbackService services.PlaybackService

	// Dependencies
	mediaService       services.MediaService
	transcodingService services.TranscodingService

	logger hclog.Logger
}

// ID returns the module identifier
func (m *Module) ID() string {
	return "playback"
}

// Name returns the human-readable module name
func (m *Module) Name() string {
	return "Playback Module"
}

// Version returns the module version
func (m *Module) Version() string {
	return "1.0.0"
}

// Core returns whether this is a core module
func (m *Module) Core() bool {
	return true
}

// Migrate performs database migrations for the module
func (m *Module) Migrate(db *gorm.DB) error {
	// Import models package
	models := []interface{}{
		&models.PlaybackSession{},
		&models.UserMediaProgress{},
		&models.PlaybackAnalytics{},
		&models.TranscodeCleanupTask{},
		&models.SessionEvent{},
		&models.PlaybackHistory{},
		&models.UserPlaybackStats{},
		&models.UserPreferences{},
		&models.MediaInteraction{},
		&models.MediaFeatures{},
		&models.UserVector{},
		&models.RecommendationCache{},
		&models.TranscodeCache{},
	}

	// Run migrations
	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate %T: %w", model, err)
		}
	}

	// Create indexes for better performance
	if err := m.createIndexes(db); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	logger.Info("Playback module migrations completed")
	return nil
}

// createIndexes creates additional indexes for performance
func (m *Module) createIndexes(db *gorm.DB) error {
	// Composite index for user history queries
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_playback_sessions_user_time ON playback_sessions(user_id, start_time DESC)").Error; err != nil {
		return err
	}

	// Index for active session queries
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_playback_sessions_active ON playback_sessions(state, last_activity) WHERE state IN ('playing', 'paused')").Error; err != nil {
		return err
	}

	// Index for playback history queries
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_playback_history_user_recent ON playback_histories(user_id, played_at DESC)").Error; err != nil {
		return err
	}

	// Index for recommendation queries
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_media_interactions_user_recent ON media_interactions(user_id, interaction_time DESC)").Error; err != nil {
		return err
	}

	return nil
}

// Init initializes the module and registers its services
func (m *Module) Init() error {
	logger.Info("Initializing playback module")

	// Get required services
	mediaService, err := services.Get("media")
	if err != nil {
		return fmt.Errorf("media service not available: %w", err)
	}
	m.mediaService = mediaService.(services.MediaService)

	transcodingService, err := services.Get("transcoding")
	if err != nil {
		return fmt.Errorf("transcoding service not available: %w", err)
	}
	m.transcodingService = transcodingService.(services.TranscodingService)

	// Set database connection
	m.db = database.GetDB()

	// Create logger
	m.logger = hclog.New(&hclog.LoggerOptions{
		Name:  "playback-module",
		Level: hclog.Info,
	})

	// Initialize core components
	m.playbackManager = playback.NewManager(m.logger.Named("playback"), m.db, m.mediaService, m.transcodingService)
	m.streamingManager = streaming.NewManager(m.logger.Named("streaming"))
	m.sessionManager = session.NewSessionManager(m.logger.Named("session"), m.db)
	m.historyManager = history.NewHistoryManager(m.logger.Named("history"), m.db)

	// Initialize cleanup manager
	cleanupConfig := cleanup.DefaultCleanupConfig()
	m.cleanupManager = cleanup.NewCleanupManager(
		m.logger.Named("cleanup"),
		m.db,
		cleanupConfig,
		m.transcodingService,
	)

	// Start cleanup routine
	if err := m.cleanupManager.Start(); err != nil {
		return fmt.Errorf("failed to start cleanup manager: %w", err)
	}

	// Create and register the playback service
	m.playbackService = service.NewPlaybackService(
		m.logger.Named("service"),
		m.playbackManager.GetDecisionEngine(),
		m.streamingManager.GetProgressiveHandler(),
		m.sessionManager,
		m.historyManager,
		m.mediaService,
		m.transcodingService,
	)

	if err := services.Register("playback", m.playbackService); err != nil {
		return fmt.Errorf("failed to register playback service: %w", err)
	}

	logger.Info("Playback module initialized successfully")
	return nil
}

// RegisterRoutes registers HTTP routes for the playback module
func (m *Module) RegisterRoutes(router *gin.Engine) {
	logger.Info("Registering playback module routes")
	
	// Create API handler
	handler := m.CreateAPIHandler()
	
	// Register routes under /api/playback
	playbackGroup := router.Group("/api/playback")
	api.RegisterRoutes(playbackGroup, handler)
	
	// Register analytics endpoint directly on main router
	analyticsGroup := router.Group("/api/analytics")
	{
		analyticsGroup.POST("/session", handler.AnalyticsSession)
	}
	
	logger.Info("Playback module routes registered successfully")
}

// SetDB sets the database connection
func (m *Module) SetDB(db *gorm.DB) {
	m.db = db
}

// GetDecisionEngine returns the decision engine for API handlers
func (m *Module) GetDecisionEngine() *playback.DecisionEngine {
	return m.playbackManager.GetDecisionEngine()
}

// GetProgressiveHandler returns the progressive handler for API handlers
func (m *Module) GetProgressiveHandler() *streaming.ProgressiveHandler {
	return m.streamingManager.GetProgressiveHandler()
}

// GetSessionManager returns the session manager for API handlers
func (m *Module) GetSessionManager() *session.SessionManager {
	return m.sessionManager
}

// CreateAPIHandler creates the API handler for this module
func (m *Module) CreateAPIHandler() *api.Handler {
	return api.NewHandler(m.playbackService, m.mediaService, m.streamingManager.GetProgressiveHandler(), m.sessionManager, m.logger.Named("api"))
}

// Shutdown gracefully shuts down the module
func (m *Module) Shutdown() error {
	logger.Info("Shutting down playback module")

	// Stop cleanup manager
	if m.cleanupManager != nil {
		if err := m.cleanupManager.Stop(); err != nil {
			logger.Error("Failed to stop cleanup manager", "error", err)
		}
	}

	// Clean up active sessions
	if m.sessionManager != nil {
		m.sessionManager.CleanupSessions()
	}

	return nil
}

// GetHistoryManager returns the history manager for API handlers
func (m *Module) GetHistoryManager() *history.HistoryManager {
	return m.historyManager
}

// GetCleanupManager returns the cleanup manager for API handlers
func (m *Module) GetCleanupManager() *cleanup.CleanupManager {
	return m.cleanupManager
}

// GetTranscodeDeduplicator returns the transcode deduplicator for API handlers
func (m *Module) GetTranscodeDeduplicator() *playback.TranscodeDeduplicator {
	return m.playbackManager.GetTranscodeDeduplicator()
}

// Dependencies returns module dependencies
func (m *Module) Dependencies() []string {
	return []string{
		"system.database",
		"system.media",
		"system.transcoding",
	}
}

// RequiredServices returns services this module requires
func (m *Module) RequiredServices() []string {
	return []string{"media", "transcoding"}
}
