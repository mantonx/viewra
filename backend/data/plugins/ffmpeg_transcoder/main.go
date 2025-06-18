package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mantonx/viewra/pkg/plugins"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/mantonx/viewra/data/plugins/ffmpeg_transcoder/internal/config"
	"github.com/mantonx/viewra/data/plugins/ffmpeg_transcoder/internal/models"
	"github.com/mantonx/viewra/data/plugins/ffmpeg_transcoder/internal/services"
)

// Version information
type VersionInfo struct {
	Version   string
	BuildTime string
	GitCommit string
}

// GetVersion returns version information for the plugin
func GetVersion() VersionInfo {
	return VersionInfo{
		Version:   "1.0.0",
		BuildTime: time.Now().Format("2006-01-02 15:04:05"),
		GitCommit: "dev", // This would be set during build in production
	}
}

// FFmpegTranscoderPlugin represents the main plugin implementation
type FFmpegTranscoderPlugin struct {
	// Core services
	db       *gorm.DB
	logger   plugins.Logger
	basePath string
	context  *plugins.PluginContext

	// Plugin services
	transcodingService *services.TranscodingService
	sessionManager     *services.SessionManager
	ffmpegService      *services.FFmpegService

	// SDK services
	healthService      *plugins.BaseHealthService
	configService      *config.FFmpegConfigurationService
	performanceMonitor *plugins.BasePerformanceMonitor

	// Host service connections
	unifiedClient *plugins.UnifiedServiceClient

	// Lazy wrapper for GRPC registration
	lazyTranscodingService *LazyTranscodingService
}

// Plugin lifecycle methods
func (f *FFmpegTranscoderPlugin) Initialize(ctx *plugins.PluginContext) error {
	// Basic nil checks to prevent segmentation fault
	if ctx == nil {
		return fmt.Errorf("plugin context is nil")
	}

	if ctx.Logger == nil {
		return fmt.Errorf("logger in plugin context is nil")
	}

	f.logger = ctx.Logger
	f.basePath = ctx.BasePath
	f.context = ctx

	f.logger.Info("FFmpeg Transcoder initializing", "base_path", f.basePath, "plugin_base_path", ctx.PluginBasePath)

	// Initialize database connection using main application database
	if ctx.DatabaseURL == "" {
		f.logger.Error("DatabaseURL is empty - plugin must use main application database")
		return fmt.Errorf("DatabaseURL is empty - plugin cannot create separate database")
	}

	f.logger.Info("Connecting to main application database", "database_url", ctx.DatabaseURL)

	// Parse database URL and connect to main database
	var db *gorm.DB
	var err error

	if strings.HasPrefix(ctx.DatabaseURL, "sqlite://") {
		dbPath := strings.TrimPrefix(ctx.DatabaseURL, "sqlite://")
		db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	} else if strings.HasPrefix(ctx.DatabaseURL, "postgres://") {
		// For future PostgreSQL support
		return fmt.Errorf("PostgreSQL support not yet implemented in plugin")
	} else {
		return fmt.Errorf("unsupported database URL format: %s", ctx.DatabaseURL)
	}

	if err != nil {
		f.logger.Error("Failed to connect to main database", "error", err, "database_url", ctx.DatabaseURL)
		return fmt.Errorf("failed to connect to main database: %w", err)
	}

	f.db = db
	f.logger.Info("Connected to main application database successfully")

	// Auto-migrate our tables to the main database
	f.logger.Info("Starting database migration for transcode session tables")
	if err := f.db.AutoMigrate(&models.TranscodeSession{}, &models.TranscodeStats{}); err != nil {
		f.logger.Error("Database migration failed", "error", err)
		return fmt.Errorf("failed to migrate transcode session tables: %w", err)
	}
	f.logger.Info("Database migration completed - transcode sessions now stored in main database")

	// Initialize unified client for host services
	if ctx.HostServiceAddr != "" {
		f.logger.Info("Connecting to host services", "addr", ctx.HostServiceAddr)
		client, err := plugins.NewUnifiedServiceClient(ctx.HostServiceAddr)
		if err != nil {
			f.logger.Warn("failed to connect to host services", "error", err)
		} else {
			f.unifiedClient = client
			f.logger.Info("Connected to host services successfully")
		}
	} else {
		f.logger.Info("No host service address provided")
	}

	// Initialize services with dependency injection
	f.logger.Info("Initializing plugin services")
	if err := f.initializeServices(); err != nil {
		f.logger.Error("Failed to initialize services", "error", err)
		return fmt.Errorf("failed to initialize services: %w", err)
	}
	f.logger.Info("Plugin services initialized successfully")

	f.logger.Info("FFmpeg Transcoder initialized successfully with main database integration")
	return nil
}

func (f *FFmpegTranscoderPlugin) Start() error {
	f.logger.Info("FFmpeg Transcoder started")

	// Start background tasks
	if f.sessionManager != nil {
		go f.sessionManager.StartCleanupRoutine(context.Background())
	}

	return nil
}

func (f *FFmpegTranscoderPlugin) Stop() error {
	f.logger.Info("FFmpeg Transcoder stopping")

	// Stop all active transcoding sessions
	if f.sessionManager != nil {
		f.sessionManager.StopAllSessions()
		f.sessionManager.StopCleanupRoutine()
	}

	// Cleanup FFmpeg service
	if f.ffmpegService != nil {
		f.ffmpegService.Cleanup()
	}

	// Cleanup resources
	if f.db != nil {
		if sqlDB, err := f.db.DB(); err == nil {
			sqlDB.Close()
		}
	}

	if f.unifiedClient != nil {
		f.unifiedClient.Close()
	}

	f.logger.Info("FFmpeg Transcoder stopped")
	return nil
}

func (f *FFmpegTranscoderPlugin) Info() (*plugins.PluginInfo, error) {
	version := GetVersion()
	return &plugins.PluginInfo{
		ID:          "ffmpeg_transcoder",
		Name:        "FFmpeg Transcoder",
		Version:     version.Version,
		Type:        "transcoder",
		Description: "FFmpeg-based video transcoding service with comprehensive codec support and streaming capabilities",
		Author:      "Viewra Team",
	}, nil
}

// CheckHealth returns nil if the plugin is healthy
func (f *FFmpegTranscoderPlugin) Health() error {
	// Check database connection
	if sqlDB, err := f.db.DB(); err != nil {
		return fmt.Errorf("database error: %w", err)
	} else if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Check FFmpeg availability
	if f.ffmpegService != nil {
		if err := f.ffmpegService.CheckAvailability(); err != nil {
			return fmt.Errorf("FFmpeg not available: %w", err)
		}
	}

	return nil
}

// Database service implementation
func (f *FFmpegTranscoderPlugin) GetModels() []string {
	return []string{
		"TranscodeSession",
		"TranscodeStats",
	}
}

func (f *FFmpegTranscoderPlugin) Migrate(connectionString string) error {
	f.logger.Info("Migrating generic transcode session tables to main database", "connection_string", connectionString)

	// Parse connection string and connect
	var db *gorm.DB
	var err error

	if strings.HasPrefix(connectionString, "sqlite://") {
		dbPath := strings.TrimPrefix(connectionString, "sqlite://")
		db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	} else {
		// Handle the case where connectionString is a direct path (legacy compatibility)
		db, err = gorm.Open(sqlite.Open(connectionString), &gorm.Config{})
	}

	if err != nil {
		return fmt.Errorf("failed to connect to main database for migration: %w", err)
	}

	// Check if we need to migrate from old FFmpeg-specific tables
	var oldTableCount int
	err = db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('ffmpeg_transcode_sessions', 'ffmpeg_transcode_stats')").Scan(&oldTableCount).Error

	// Auto-migrate new generic tables first
	err = db.AutoMigrate(
		&models.TranscodeSession{},
		&models.TranscodeStats{},
	)
	if err != nil {
		return fmt.Errorf("failed to create new generic transcode tables: %w", err)
	}

	// If old tables exist, migrate the data
	if oldTableCount > 0 {
		f.logger.Info("Found old FFmpeg-specific tables, migrating data to generic tables")
		err = f.migrateOldTableData(db)
		if err != nil {
			f.logger.Error("Failed to migrate old table data", "error", err)
			// Continue anyway - new sessions will use new tables
		}
	}

	f.logger.Info("Successfully migrated generic transcode session tables to main database")
	return nil
}

// migrateOldTableData migrates data from FFmpeg-specific tables to generic tables
func (f *FFmpegTranscoderPlugin) migrateOldTableData(db *gorm.DB) error {
	// Migrate sessions with error handling for missing columns
	err := db.Exec(`
		INSERT INTO transcode_sessions 
		(id, plugin_id, backend, input_path, output_path, status, progress, start_time, end_time,
		 target_codec, target_container, resolution, bitrate, audio_codec, audio_bitrate, 
		 quality, preset, error, client_ip, user_agent, metadata, created_at, updated_at)
		SELECT 
		 id, 
		 COALESCE(plugin_id, 'ffmpeg_transcoder') as plugin_id,
		 'ffmpeg' as backend, 
		 input_path, 
		 output_path, 
		 status, 
		 progress, 
		 start_time, 
		 end_time,
		 target_codec, 
		 target_container, 
		 resolution, 
		 bitrate, 
		 audio_codec, 
		 audio_bitrate,
		 quality, 
		 preset, 
		 error, 
		 client_ip, 
		 user_agent, 
		 metadata, 
		 created_at, 
		 updated_at
		FROM ffmpeg_transcode_sessions
		WHERE id NOT IN (SELECT id FROM transcode_sessions)
	`).Error

	if err != nil {
		return fmt.Errorf("failed to migrate session data: %w", err)
	}

	// Migrate stats
	err = db.Exec(`
		INSERT INTO transcode_stats 
		(session_id, plugin_id, backend, duration, bytes_processed, bytes_generated, 
		 frames_processed, current_fps, average_fps, cpu_usage, memory_usage, speed, recorded_at)
		SELECT 
		 session_id, 
		 COALESCE(plugin_id, 'ffmpeg_transcoder') as plugin_id,
		 'ffmpeg' as backend, 
		 duration, 
		 bytes_processed, 
		 bytes_generated,
		 frames_processed, 
		 current_fps, 
		 average_fps, 
		 cpu_usage, 
		 memory_usage, 
		 speed, 
		 recorded_at
		FROM ffmpeg_transcode_stats
		WHERE NOT EXISTS (
			SELECT 1 FROM transcode_stats 
			WHERE transcode_stats.session_id = ffmpeg_transcode_stats.session_id 
			AND transcode_stats.recorded_at = ffmpeg_transcode_stats.recorded_at
		)
	`).Error

	if err != nil {
		return fmt.Errorf("failed to migrate stats data: %w", err)
	}

	f.logger.Info("Successfully migrated data from old FFmpeg-specific tables to generic tables")
	return nil
}

func (f *FFmpegTranscoderPlugin) Rollback(connectionString string) error {
	// Implementation for rolling back database changes
	return fmt.Errorf("rollback not implemented")
}

// Service interface implementations
func (f *FFmpegTranscoderPlugin) MetadataScraperService() plugins.MetadataScraperService {
	return nil // Not applicable for transcoding plugin
}

func (f *FFmpegTranscoderPlugin) DatabaseService() plugins.DatabaseService {
	return f
}

func (f *FFmpegTranscoderPlugin) ScannerHookService() plugins.ScannerHookService {
	return nil // Not applicable for transcoding plugin
}

func (f *FFmpegTranscoderPlugin) SearchService() plugins.SearchService {
	return nil // Not applicable for transcoding plugin
}

func (f *FFmpegTranscoderPlugin) AssetService() plugins.AssetService {
	return nil // Not applicable for transcoding plugin
}

func (f *FFmpegTranscoderPlugin) AdminPageService() plugins.AdminPageService {
	return &FFmpegAdminPageService{
		plugin: f,
		logger: f.logger,
	}
}

func (f *FFmpegTranscoderPlugin) APIRegistrationService() plugins.APIRegistrationService {
	return nil // Could be implemented for transcoding API endpoints
}

func (f *FFmpegTranscoderPlugin) HealthMonitorService() plugins.HealthMonitorService {
	if f.healthService == nil {
		return nil
	}
	return f.healthService
}

func (f *FFmpegTranscoderPlugin) ConfigurationService() plugins.ConfigurationService {
	if f.configService == nil {
		return nil
	}
	return f.configService
}

func (f *FFmpegTranscoderPlugin) PerformanceMonitorService() plugins.PerformanceMonitorService {
	if f.performanceMonitor == nil {
		return nil
	}
	return &performanceServiceAdapter{monitor: f.performanceMonitor}
}

func (f *FFmpegTranscoderPlugin) TranscodingService() plugins.TranscodingService {
	// Create lazy service on first call
	if f.lazyTranscodingService == nil {
		f.lazyTranscodingService = NewLazyTranscodingService(f)
	}

	if f.logger != nil {
		f.logger.Info("DEBUG: TranscodingService() called, returning lazy service")
	}

	return f.lazyTranscodingService
}

// Performance service adapter
type performanceServiceAdapter struct {
	monitor *plugins.BasePerformanceMonitor
}

func (p *performanceServiceAdapter) GetPerformanceSnapshot(ctx context.Context) (*plugins.PerformanceSnapshot, error) {
	return p.monitor.GetSnapshot(), nil
}

func (p *performanceServiceAdapter) RecordOperation(operationName string, duration time.Duration, success bool, context string) {
	p.monitor.RecordOperation(operationName, duration, success, context)
}

func (p *performanceServiceAdapter) RecordError(errorType, message, context, operation string) {
	p.monitor.RecordError(errorType, message, context, operation)
}

func (p *performanceServiceAdapter) GetUptimeString() string {
	return p.monitor.GetUptimeString()
}

func (p *performanceServiceAdapter) Reset() {
	p.monitor.Reset()
}

// FFmpegAdminPageService implements the AdminPageService for FFmpeg transcoder
type FFmpegAdminPageService struct {
	plugin *FFmpegTranscoderPlugin
	logger plugins.Logger
}

// GetAdminPages returns the admin pages provided by this plugin
func (a *FFmpegAdminPageService) GetAdminPages() []*plugins.AdminPageConfig {
	return []*plugins.AdminPageConfig{
		{
			ID:       "ffmpeg_config",
			Title:    "FFmpeg Configuration",
			URL:      "/admin/plugins/ffmpeg_transcoder/config",
			Icon:     "settings",
			Category: "transcoding",
			Type:     "configuration",
		},
		{
			ID:       "ffmpeg_monitoring",
			Title:    "Transcoding Monitor",
			URL:      "/admin/plugins/ffmpeg_transcoder/monitor",
			Icon:     "activity",
			Category: "transcoding",
			Type:     "dashboard",
		},
		{
			ID:       "ffmpeg_sessions",
			Title:    "Active Sessions",
			URL:      "/admin/plugins/ffmpeg_transcoder/sessions",
			Icon:     "play",
			Category: "transcoding",
			Type:     "status",
		},
		{
			ID:       "ffmpeg_health",
			Title:    "Health & Performance",
			URL:      "/admin/plugins/ffmpeg_transcoder/health",
			Icon:     "heart",
			Category: "transcoding",
			Type:     "status",
		},
	}
}

// RegisterRoutes registers the admin page routes for this plugin
func (a *FFmpegAdminPageService) RegisterRoutes(basePath string) error {
	a.logger.Info("Registering FFmpeg transcoder admin routes", "base_path", basePath)

	// Here we would register HTTP routes for the admin pages
	// The actual route registration would be handled by the host application
	// We just need to define what routes this plugin provides

	return nil
}

// Service initialization
func (f *FFmpegTranscoderPlugin) initializeServices() error {
	// Load configuration
	configService, err := config.NewFFmpegConfigurationService(f.context, f.logger)
	if err != nil {
		return fmt.Errorf("failed to initialize configuration service: %w", err)
	}
	f.configService = configService

	// Initialize health service
	f.healthService = plugins.NewHealthServiceBuilder("FFmpeg Transcoder").
		WithCustomCounter("transcode_sessions", 0).
		WithCustomCounter("transcode_completed", 0).
		WithCustomCounter("transcode_failed", 0).
		WithCustomGauge("active_sessions", 0.0).
		Build()

	// Initialize performance monitor
	f.performanceMonitor = plugins.NewPerformanceMonitorBuilder("FFmpeg Transcoder").
		WithCustomCounter("ffmpeg_processes", 0).
		WithCustomCounter("bytes_transcoded", 0).
		WithCustomGauge("cpu_usage", 0.0).
		WithCustomGauge("memory_usage", 0.0).
		WithMaxErrorHistory(50).
		Build()

	// Initialize session manager first
	sessionManager, err := services.NewSessionManager(f.db, f.logger, f.performanceMonitor)
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}
	f.sessionManager = sessionManager

	// Clean up any stale running sessions from previous shutdowns
	f.logger.Info("cleaning up stale sessions from previous run")
	f.sessionManager.StopAllSessions()

	// Initialize FFmpeg service with callback to update session status
	statusCallback := func(sessionID, status, errorMsg string) {
		if err := f.sessionManager.UpdateSessionStatus(sessionID, status, errorMsg); err != nil {
			f.logger.Warn("failed to update session status", "session_id", sessionID, "status", status, "error", err)
		}
	}

	ffmpegService, err := services.NewFFmpegService(f.logger, f.configService, statusCallback)
	if err != nil {
		return fmt.Errorf("failed to initialize FFmpeg service: %w", err)
	}
	f.ffmpegService = ffmpegService

	// Initialize transcoding service
	f.logger.Info("DEBUG: Creating transcoding service")
	transcodingService, err := services.NewTranscodingService(
		f.logger,
		f.ffmpegService,
		f.sessionManager,
		f.configService,
		f.performanceMonitor,
	)
	if err != nil {
		f.logger.Error("DEBUG: Failed to create transcoding service", "error", err)
		return fmt.Errorf("failed to initialize transcoding service: %w", err)
	}
	f.transcodingService = transcodingService
	f.logger.Info("DEBUG: Transcoding service created successfully", "service", f.transcodingService)

	// Notify lazy service that the real service is ready
	if f.lazyTranscodingService != nil {
		f.lazyTranscodingService.NotifyReady()
		f.logger.Info("DEBUG: Notified lazy service that transcoding service is ready")
	}

	return nil
}

// Main plugin entry point
func main() {
	plugin := &FFmpegTranscoderPlugin{}
	plugins.StartPlugin(plugin)
}
