package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mantonx/viewra/data/plugins/ffmpeg_transcoder/internal/config"
	"github.com/mantonx/viewra/data/plugins/ffmpeg_transcoder/internal/models"
	"github.com/mantonx/viewra/pkg/plugins"
	"gorm.io/gorm"
)

// TranscodingService implements the plugins.TranscodingService interface
type TranscodingService struct {
	logger         plugins.Logger
	ffmpegService  *FFmpegService
	sessionManager *SessionManager
	configService  *config.FFmpegConfigurationService
	perfMonitor    *plugins.BasePerformanceMonitor
}

// NewTranscodingService creates a new transcoding service
func NewTranscodingService(
	logger plugins.Logger,
	ffmpegService *FFmpegService,
	sessionManager *SessionManager,
	configService *config.FFmpegConfigurationService,
	perfMonitor *plugins.BasePerformanceMonitor,
) (*TranscodingService, error) {
	return &TranscodingService{
		logger:         logger,
		ffmpegService:  ffmpegService,
		sessionManager: sessionManager,
		configService:  configService,
		perfMonitor:    perfMonitor,
	}, nil
}

// GetCapabilities returns what codecs and resolutions this transcoder supports
func (s *TranscodingService) GetCapabilities(ctx context.Context) (*plugins.TranscodingCapabilities, error) {
	cfg := s.configService.GetFFmpegConfig()

	capabilities := &plugins.TranscodingCapabilities{
		Name:                  "ffmpeg",
		SupportedCodecs:       []string{"h264", "hevc", "vp8", "vp9", "av1"},
		SupportedResolutions:  []string{"480p", "720p", "1080p", "1440p", "2160p"},
		SupportedContainers:   []string{"mp4", "webm", "mkv", "dash", "hls"},
		HardwareAcceleration:  false,
		MaxConcurrentSessions: cfg.MaxConcurrentJobs,
		Features: plugins.TranscodingFeatures{
			SubtitleBurnIn:      true,
			SubtitlePassthrough: true,
			MultiAudioTracks:    true,
			HDRSupport:          false,
			ToneMapping:         false,
			StreamingOutput:     true,
			SegmentedOutput:     true, // Enable segmented output for DASH/HLS
		},
		Priority: cfg.Priority,
	}

	// DEBUG: Log what we're returning
	s.logger.Info("DEBUG: GetCapabilities returning",
		"name", capabilities.Name,
		"codecs", capabilities.SupportedCodecs,
		"resolutions", capabilities.SupportedResolutions,
		"containers", capabilities.SupportedContainers,
		"priority", capabilities.Priority,
		"max_sessions", capabilities.MaxConcurrentSessions)

	return capabilities, nil
}

// StartTranscode initiates a transcoding session
func (s *TranscodingService) StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*plugins.TranscodeSession, error) {
	// Generate session ID
	sessionID := uuid.New().String()

	// Check concurrent session limits
	activeSessions, err := s.sessionManager.CountActiveSessions()
	if err != nil {
		return nil, fmt.Errorf("failed to check active sessions: %w", err)
	}

	cfg := s.configService.GetFFmpegConfig()
	if activeSessions >= cfg.MaxConcurrentJobs {
		return nil, fmt.Errorf("maximum concurrent sessions (%d) reached", cfg.MaxConcurrentJobs)
	}

	// Create session record
	session := &models.TranscodeSession{
		ID:              sessionID,
		InputPath:       req.InputPath,
		Status:          string(plugins.TranscodeStatusPending),
		StartTime:       time.Now(),
		Backend:         "ffmpeg",
		TargetCodec:     req.TargetCodec,
		TargetContainer: req.TargetContainer,
		Resolution:      req.Resolution,
		Bitrate:         req.Bitrate,
		AudioCodec:      req.AudioCodec,
		AudioBitrate:    req.AudioBitrate,
		Quality:         req.Quality,
		Preset:          req.Preset,
		StartTimeOffset: req.StartTime, // Store seek-ahead start time offset
	}

	// Add client information if available
	if req.DeviceProfile != nil {
		session.ClientIP = req.DeviceProfile.ClientIP
		session.UserAgent = req.DeviceProfile.UserAgent
	}

	// Save session to database
	if err := s.sessionManager.CreateSession(session); err != nil {
		// Check if this is a duplicate session error
		if strings.Contains(err.Error(), "active session already exists with ID:") {
			// Extract existing session ID and return it instead
			s.logger.Info("found existing session for input path, reusing",
				"input_path", req.InputPath,
				"existing_session_id", session.ID,
				"original_session_id", sessionID)

			// Get the existing session details
			existingSession, getErr := s.GetTranscodeSession(ctx, session.ID)
			if getErr != nil {
				return nil, fmt.Errorf("existing session found but cannot retrieve details: %w", getErr)
			}

			return existingSession, nil
		}
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Start FFmpeg transcoding
	job, err := s.ffmpegService.StartTranscode(ctx, sessionID, req)
	if err != nil {
		// Update session status to failed
		s.sessionManager.UpdateSessionStatus(sessionID, string(plugins.TranscodeStatusFailed), err.Error())
		return nil, fmt.Errorf("failed to start transcoding: %w", err)
	}

	// Update session status to running
	if err := s.sessionManager.UpdateSessionStatus(sessionID, string(plugins.TranscodeStatusRunning), ""); err != nil {
		s.logger.Warn("failed to update session status", "session_id", sessionID, "error", err)
	}

	// Record performance metrics
	s.perfMonitor.IncrementCounter("transcode_sessions")
	s.perfMonitor.IncrementCounter("ffmpeg_processes")

	// Convert to plugin session format
	pluginSession := &plugins.TranscodeSession{
		ID:        sessionID,
		Request:   req,
		Status:    plugins.TranscodeStatusRunning,
		Progress:  0.0,
		StartTime: job.StartTime,
		Backend:   "ffmpeg",
		Stats:     s.convertFFmpegStats(job.Stats),
	}

	s.logger.Info("transcoding session started", "session_id", sessionID, "input", req.InputPath)
	return pluginSession, nil
}

// GetTranscodeSession retrieves information about an active session
func (s *TranscodingService) GetTranscodeSession(ctx context.Context, sessionID string) (*plugins.TranscodeSession, error) {
	// Get session from database
	dbSession, err := s.sessionManager.GetSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Get FFmpeg job if still active
	job, exists := s.ffmpegService.GetJob(sessionID)

	// Convert to plugin session format
	session := &plugins.TranscodeSession{
		ID:        dbSession.ID,
		Status:    plugins.TranscodeStatus(dbSession.Status),
		StartTime: dbSession.StartTime,
		Backend:   dbSession.Backend,
		Error:     dbSession.Error,
	}

	if dbSession.EndTime != nil {
		session.EndTime = dbSession.EndTime
	}

	// Add real-time stats if job is active
	if exists {
		session.Stats = s.convertFFmpegStats(job.Stats)
		session.Progress = job.Stats.Progress
	}

	// Reconstruct request from database fields
	session.Request = &plugins.TranscodeRequest{
		InputPath:       dbSession.InputPath,
		TargetCodec:     dbSession.TargetCodec,
		TargetContainer: dbSession.TargetContainer,
		Resolution:      dbSession.Resolution,
		Bitrate:         dbSession.Bitrate,
		AudioCodec:      dbSession.AudioCodec,
		AudioBitrate:    dbSession.AudioBitrate,
		Quality:         dbSession.Quality,
		Preset:          dbSession.Preset,
		StartTime:       0, // No seek time support for now
	}

	return session, nil
}

// StopTranscode terminates a transcoding session
func (s *TranscodingService) StopTranscode(ctx context.Context, sessionID string) error {
	// Stop FFmpeg job
	if err := s.ffmpegService.StopTranscode(sessionID); err != nil {
		s.logger.Warn("failed to stop FFmpeg job", "session_id", sessionID, "error", err)
	}

	// Update session status
	if err := s.sessionManager.UpdateSessionStatus(sessionID, string(plugins.TranscodeStatusCancelled), "Stopped by user"); err != nil {
		return fmt.Errorf("failed to update session status: %w", err)
	}

	s.logger.Info("transcoding session stopped", "session_id", sessionID)
	return nil
}

// ListActiveSessions returns all currently active transcoding sessions
func (s *TranscodingService) ListActiveSessions(ctx context.Context) ([]*plugins.TranscodeSession, error) {
	// Get active sessions from database
	dbSessions, err := s.sessionManager.GetActiveSessions()
	if err != nil {
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}

	sessions := make([]*plugins.TranscodeSession, 0, len(dbSessions))
	for _, dbSession := range dbSessions {
		session, err := s.GetTranscodeSession(ctx, dbSession.ID)
		if err != nil {
			s.logger.Warn("failed to get session details", "session_id", dbSession.ID, "error", err)
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// GetTranscodeStream returns the output stream for a transcoding session
func (s *TranscodingService) GetTranscodeStream(ctx context.Context, sessionID string) (io.ReadCloser, error) {
	return s.ffmpegService.GetOutputStream(sessionID)
}

// convertFFmpegStats converts FFmpeg stats to plugin stats format
func (s *TranscodingService) convertFFmpegStats(ffmpegStats *FFmpegStats) *plugins.TranscodeStats {
	if ffmpegStats == nil {
		return nil
	}

	return &plugins.TranscodeStats{
		Duration:        time.Since(ffmpegStats.LastUpdate),
		BytesProcessed:  0, // Would need to track input bytes
		BytesGenerated:  ffmpegStats.TotalSize,
		FramesProcessed: ffmpegStats.Frame,
		CurrentFPS:      ffmpegStats.FPS,
		AverageFPS:      ffmpegStats.FPS, // Could calculate running average
		CPUUsage:        0,               // Would need system monitoring
		MemoryUsage:     0,               // Would need system monitoring
		Speed:           ffmpegStats.Speed,
	}
}

// SessionManager manages transcoding sessions in the main database
type SessionManager struct {
	db            *gorm.DB
	logger        plugins.Logger
	perfMonitor   *plugins.BasePerformanceMonitor
	cleanupStop   chan struct{}
	cleanupDone   chan struct{}
	sessionsMutex *sync.Mutex
	sessions      map[string]*models.TranscodeSession
	pluginID      string // Plugin identifier for scoping
}

// NewSessionManager creates a new session manager
func NewSessionManager(db *gorm.DB, logger plugins.Logger, perfMonitor *plugins.BasePerformanceMonitor) (*SessionManager, error) {
	return &SessionManager{
		db:            db,
		logger:        logger,
		perfMonitor:   perfMonitor,
		cleanupStop:   make(chan struct{}),
		cleanupDone:   make(chan struct{}),
		sessionsMutex: &sync.Mutex{},
		sessions:      make(map[string]*models.TranscodeSession),
		pluginID:      "ffmpeg_transcoder", // Set plugin ID for scoping
	}, nil
}

// CreateSession creates a new transcoding session record with duplicate prevention
func (sm *SessionManager) CreateSession(session *models.TranscodeSession) error {
	// Ensure plugin ID and backend are set for proper scoping
	session.PluginID = sm.pluginID
	if session.Backend == "" {
		session.Backend = "ffmpeg" // Default backend
	}

	// Use a database transaction with row locking to prevent race conditions
	tx := sm.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Error; err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// For seek-ahead functionality, check if this session has a start time offset
	// Only reuse sessions if they have the same start time (or both have no start time)
	var existingSession models.TranscodeSession
	query := tx.Where("plugin_id = ? AND input_path = ? AND status IN ?",
		sm.pluginID, session.InputPath, []string{"pending", "starting", "running"})

	// Add start time condition for precise session matching
	if session.StartTimeOffset > 0 {
		// For seek-ahead sessions, only match if start times are identical
		query = query.Where("start_time_offset = ?", session.StartTimeOffset)
	} else {
		// For regular sessions, only match sessions without start time offset
		query = query.Where("start_time_offset = 0 OR start_time_offset IS NULL")
	}

	err := query.First(&existingSession).Error

	if err != nil && err != gorm.ErrRecordNotFound {
		tx.Rollback()
		return fmt.Errorf("failed to check for existing sessions: %w", err)
	}

	// If an identical session exists (same input path and start time), reuse it
	if err != gorm.ErrRecordNotFound {
		tx.Rollback()
		sm.logger.Info("found existing active session with matching parameters",
			"input_path", session.InputPath,
			"start_time_offset", session.StartTimeOffset,
			"existing_session_id", existingSession.ID,
			"existing_status", existingSession.Status,
			"plugin_id", sm.pluginID)

		// Return the existing session ID in the original session object
		session.ID = existingSession.ID
		session.Status = existingSession.Status
		session.StartTime = existingSession.StartTime
		return fmt.Errorf("active session already exists with ID: %s", existingSession.ID)
	}

	// Create the new session within the transaction
	if err := tx.Create(session).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit session creation: %w", err)
	}

	sm.logger.Debug("session created in main database",
		"session_id", session.ID,
		"plugin_id", sm.pluginID,
		"backend", session.Backend,
		"input_path", session.InputPath)
	return nil
}

// GetSession retrieves a session by ID (scoped to this plugin)
func (sm *SessionManager) GetSession(sessionID string) (*models.TranscodeSession, error) {
	var session models.TranscodeSession
	if err := sm.db.Where("id = ? AND plugin_id = ?", sessionID, sm.pluginID).First(&session).Error; err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	return &session, nil
}

// UpdateSessionStatus updates the status of a session (scoped to this plugin)
func (sm *SessionManager) UpdateSessionStatus(sessionID, status, errorMsg string) error {
	updates := map[string]interface{}{
		"status": status,
	}

	if errorMsg != "" {
		updates["error"] = errorMsg
	}

	if status == string(plugins.TranscodeStatusCompleted) ||
		status == string(plugins.TranscodeStatusFailed) ||
		status == string(plugins.TranscodeStatusCancelled) {
		now := time.Now()
		updates["end_time"] = &now
	}

	result := sm.db.Model(&models.TranscodeSession{}).Where("id = ? AND plugin_id = ?", sessionID, sm.pluginID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update session status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		sm.logger.Warn("session not found for status update", "session_id", sessionID, "plugin_id", sm.pluginID)
		return fmt.Errorf("session not found or not owned by plugin")
	}

	sm.logger.Debug("session status updated in main database", "session_id", sessionID, "status", status, "plugin_id", sm.pluginID)
	return nil
}

// UpdateSessionProgress updates the progress of a session (scoped to this plugin)
func (sm *SessionManager) UpdateSessionProgress(sessionID string, progress float64) error {
	if err := sm.db.Model(&models.TranscodeSession{}).Where("id = ? AND plugin_id = ?", sessionID, sm.pluginID).Update("progress", progress).Error; err != nil {
		return fmt.Errorf("failed to update session progress: %w", err)
	}
	return nil
}

// GetActiveSessions returns all active sessions (scoped to this plugin)
func (sm *SessionManager) GetActiveSessions() ([]*models.TranscodeSession, error) {
	var sessions []*models.TranscodeSession
	if err := sm.db.Where("plugin_id = ? AND status IN ?", sm.pluginID, []string{
		string(plugins.TranscodeStatusPending),
		string(plugins.TranscodeStatusStarting),
		string(plugins.TranscodeStatusRunning),
	}).Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}
	return sessions, nil
}

// CountActiveSessions returns the number of active sessions (scoped to this plugin)
func (sm *SessionManager) CountActiveSessions() (int, error) {
	var count int64
	if err := sm.db.Model(&models.TranscodeSession{}).Where("plugin_id = ? AND status IN ?", sm.pluginID, []string{
		string(plugins.TranscodeStatusPending),
		string(plugins.TranscodeStatusStarting),
		string(plugins.TranscodeStatusRunning),
	}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count active sessions: %w", err)
	}
	return int(count), nil
}

// RecordStats records transcoding statistics (scoped to this plugin)
func (sm *SessionManager) RecordStats(sessionID string, stats *plugins.TranscodeStats) error {
	// Get the session to determine the backend
	session, err := sm.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session for stats: %w", err)
	}

	dbStats := &models.TranscodeStats{
		SessionID:       sessionID,
		PluginID:        sm.pluginID,     // Add plugin scoping
		Backend:         session.Backend, // Add backend from session
		Duration:        stats.Duration.Milliseconds(),
		BytesProcessed:  stats.BytesProcessed,
		BytesGenerated:  stats.BytesGenerated,
		FramesProcessed: stats.FramesProcessed,
		CurrentFPS:      stats.CurrentFPS,
		AverageFPS:      stats.AverageFPS,
		CPUUsage:        stats.CPUUsage,
		MemoryUsage:     stats.MemoryUsage,
		Speed:           stats.Speed,
		RecordedAt:      time.Now(),
	}

	if err := sm.db.Create(dbStats).Error; err != nil {
		return fmt.Errorf("failed to record stats: %w", err)
	}

	return nil
}

// StartCleanupRoutine starts the session cleanup routine
func (sm *SessionManager) StartCleanupRoutine(ctx context.Context) {
	defer close(sm.cleanupDone)

	// Run cleanup more frequently for better session management
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	sm.logger.Info("session cleanup routine started")

	for {
		select {
		case <-ctx.Done():
			sm.logger.Info("session cleanup routine stopped (context cancelled)")
			return
		case <-sm.cleanupStop:
			sm.logger.Info("session cleanup routine stopped")
			return
		case <-ticker.C:
			sm.cleanupExpiredSessions()
			sm.cleanupFailedSessions()
		}
	}
}

// StopCleanupRoutine stops the cleanup routine
func (sm *SessionManager) StopCleanupRoutine() {
	close(sm.cleanupStop)
	<-sm.cleanupDone
}

// cleanupExpiredSessions removes old completed/failed sessions (scoped to this plugin)
func (sm *SessionManager) cleanupExpiredSessions() {
	// Remove sessions older than 24 hours that are completed/failed/cancelled
	cutoff := time.Now().Add(-24 * time.Hour)

	result := sm.db.Where("plugin_id = ? AND end_time < ? AND status IN ?", sm.pluginID, cutoff, []string{
		string(plugins.TranscodeStatusCompleted),
		string(plugins.TranscodeStatusFailed),
		string(plugins.TranscodeStatusCancelled),
	}).Delete(&models.TranscodeSession{})

	if result.Error != nil {
		sm.logger.Error("failed to cleanup expired sessions", "error", result.Error, "plugin_id", sm.pluginID)
		return
	}

	if result.RowsAffected > 0 {
		sm.logger.Info("cleaned up expired sessions from main database", "count", result.RowsAffected, "plugin_id", sm.pluginID)

		// Also cleanup associated stats
		sm.db.Where("plugin_id = ? AND recorded_at < ?", sm.pluginID, cutoff).Delete(&models.TranscodeStats{})
	}

	// File system cleanup with retention policy
	sm.cleanupTranscodingFiles()
}

// cleanupFailedSessions removes sessions that have failed or crashed
func (sm *SessionManager) cleanupFailedSessions() {
	sm.sessionsMutex.Lock()
	defer sm.sessionsMutex.Unlock()

	failedSessions := []string{}

	for sessionID, session := range sm.sessions {
		if session.Status == string(plugins.TranscodeStatusFailed) ||
			session.Status == string(plugins.TranscodeStatusCancelled) {
			failedSessions = append(failedSessions, sessionID)
		} else if session.Status == string(plugins.TranscodeStatusRunning) {
			// Check if session has been running too long without activity
			if time.Since(session.StartTime) > 2*time.Hour {
				sm.logger.Warn("session running too long, marking for cleanup",
					"session_id", sessionID, "duration", time.Since(session.StartTime))
				failedSessions = append(failedSessions, sessionID)
			}
		}
	}

	// Remove failed sessions
	for _, sessionID := range failedSessions {
		if session, exists := sm.sessions[sessionID]; exists {
			sm.logger.Info("cleaning up failed session",
				"session_id", sessionID, "status", session.Status,
				"duration", time.Since(session.StartTime))
			delete(sm.sessions, sessionID)
		}
	}

	if len(failedSessions) > 0 {
		sm.logger.Info("cleaned up failed sessions", "count", len(failedSessions))
	}
}

// StopAllSessions stops all active transcoding sessions (scoped to this plugin)
func (sm *SessionManager) StopAllSessions() {
	sm.logger.Info("stopping all active sessions for plugin cleanup", "plugin_id", sm.pluginID)

	sessions, err := sm.GetActiveSessions()
	if err != nil {
		sm.logger.Error("failed to get active sessions for cleanup", "error", err, "plugin_id", sm.pluginID)
		return
	}

	if len(sessions) == 0 {
		sm.logger.Info("no active sessions to stop", "plugin_id", sm.pluginID)
		return
	}

	stopped := 0
	for _, session := range sessions {
		if err := sm.UpdateSessionStatus(session.ID, string(plugins.TranscodeStatusCancelled), "Plugin shutdown"); err != nil {
			sm.logger.Warn("failed to update session status during cleanup", "session_id", session.ID, "error", err, "plugin_id", sm.pluginID)
		} else {
			stopped++
		}
	}

	sm.logger.Info("stopped active sessions for plugin cleanup", "requested", len(sessions), "stopped", stopped, "plugin_id", sm.pluginID)

	// Clear the in-memory session cache
	sm.sessionsMutex.Lock()
	sm.sessions = make(map[string]*models.TranscodeSession)
	sm.sessionsMutex.Unlock()
}

// cleanupTranscodingFiles removes old transcoding files and directories with intelligent retention
func (sm *SessionManager) cleanupTranscodingFiles() {
	// Get transcoding directory from environment or config
	transcodingDir := "/viewra-data/transcoding"
	if dir := os.Getenv("TRANSCODING_DATA_DIR"); dir != "" {
		transcodingDir = dir
	}

	// Get cleanup configuration (use defaults if config not available)
	fileRetentionHours := 2     // Default: Keep files for 2 hours (active streaming window)
	extendedRetentionHours := 8 // Default: Keep smaller files for 8 hours
	maxSizeLimitGB := 10        // Default: Emergency cleanup above 10GB
	largeFileSizeMB := 500      // Default: Files larger than 500MB are considered large

	// Multi-tier retention policy (configurable):
	// - Keep files for N hours (active streaming window)
	// - Keep files for extended hours if they're reasonably sized (< XMB total per session)
	// - Remove everything older than 24 hours (absolute safety limit)
	// - Emergency cleanup if total size > XGB

	now := time.Now()
	activeWindow := now.Add(-time.Duration(fileRetentionHours) * time.Hour)
	extendedWindow := now.Add(-time.Duration(extendedRetentionHours) * time.Hour)
	maxRetention := now.Add(-24 * time.Hour) // Absolute maximum retention (hardcoded safety)

	sm.logger.Debug("starting file system cleanup", "dir", transcodingDir)

	// First pass: Remove anything older than 24 hours (absolute limit)
	absoluteExpiredCount := sm.cleanupDirectoriesByAge(transcodingDir, maxRetention, "absolute_expired")
	if absoluteExpiredCount > 0 {
		sm.logger.Info("removed absolutely expired directories", "count", absoluteExpiredCount)
	}

	// Second pass: Check total disk usage and apply size-based cleanup if needed
	totalSize, dirCount := sm.calculateDirectorySize(transcodingDir)
	sm.logger.Info("transcoding directory stats", "total_size_gb", float64(totalSize)/(1024*1024*1024), "directory_count", dirCount)

	// Emergency cleanup if over configured limit
	sizeLimitBytes := int64(maxSizeLimitGB) * 1024 * 1024 * 1024
	if totalSize > sizeLimitBytes {
		sm.logger.Warn("transcoding directory too large, performing emergency cleanup", "size_gb", float64(totalSize)/(1024*1024*1024))
		emergencyCleanedCount := sm.cleanupDirectoriesByAge(transcodingDir, extendedWindow, "emergency_size")
		sm.logger.Info("emergency cleanup completed", "directories_removed", emergencyCleanedCount)
	} else {
		// Normal cleanup: remove files older than active window, but keep smaller ones longer
		normalCleanedCount := sm.cleanupDirectoriesIntelligent(transcodingDir, activeWindow, extendedWindow, largeFileSizeMB)
		sm.logger.Debug("intelligent cleanup completed", "directories_removed", normalCleanedCount)
	}
}

// cleanupDirectoriesByAge removes directories older than the specified time
func (sm *SessionManager) cleanupDirectoriesByAge(baseDir string, cutoffTime time.Time, reason string) int {
	cleanedCount := 0

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		sm.logger.Warn("failed to read transcoding directory", "dir", baseDir, "error", err)
		return 0
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "dash_") {
			continue
		}

		dirPath := filepath.Join(baseDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoffTime) {
			sm.logger.Debug("removing old transcoding directory", "path", dirPath, "age", time.Since(info.ModTime()), "reason", reason)
			if err := os.RemoveAll(dirPath); err != nil {
				sm.logger.Warn("failed to remove old directory", "path", dirPath, "error", err)
			} else {
				cleanedCount++
			}
		}
	}

	return cleanedCount
}

// cleanupDirectoriesIntelligent applies intelligent cleanup based on size and age
func (sm *SessionManager) cleanupDirectoriesIntelligent(baseDir string, activeWindow, extendedWindow time.Time, largeFileSizeMB int) int {
	cleanedCount := 0

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		sm.logger.Warn("failed to read transcoding directory", "dir", baseDir, "error", err)
		return 0
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "dash_") {
			continue
		}

		dirPath := filepath.Join(baseDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Always remove if older than extended window
		if info.ModTime().Before(extendedWindow) {
			sm.logger.Debug("removing extended-old directory", "path", dirPath, "age", time.Since(info.ModTime()))
			if err := os.RemoveAll(dirPath); err != nil {
				sm.logger.Warn("failed to remove extended-old directory", "path", dirPath, "error", err)
			} else {
				cleanedCount++
			}
			continue
		}

		// For directories between active and extended window, check size
		if info.ModTime().Before(activeWindow) {
			dirSize := sm.calculateSingleDirectorySize(dirPath)

			// Remove if larger than configured limit
			largeSizeBytes := int64(largeFileSizeMB) * 1024 * 1024
			if dirSize > largeSizeBytes {
				sm.logger.Debug("removing large inactive directory", "path", dirPath, "size_mb", dirSize/(1024*1024), "age", time.Since(info.ModTime()))
				if err := os.RemoveAll(dirPath); err != nil {
					sm.logger.Warn("failed to remove large directory", "path", dirPath, "error", err)
				} else {
					cleanedCount++
				}
			}
		}
	}

	return cleanedCount
}

// calculateDirectorySize calculates total size of all transcoding directories
func (sm *SessionManager) calculateDirectorySize(baseDir string) (int64, int) {
	var totalSize int64
	dirCount := 0

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return 0, 0
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "dash_") {
			dirPath := filepath.Join(baseDir, entry.Name())
			size := sm.calculateSingleDirectorySize(dirPath)
			totalSize += size
			dirCount++
		}
	}

	return totalSize, dirCount
}

// calculateSingleDirectorySize calculates size of a single directory
func (sm *SessionManager) calculateSingleDirectorySize(dirPath string) int64 {
	var size int64

	filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Continue walking even if there's an error
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				size += info.Size()
			}
		}
		return nil
	})

	return size
}
