package core

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	plugins "github.com/mantonx/viewra/sdk"
	"gorm.io/gorm"
)

// TranscodeService is the main service for managing transcoding operations
type TranscodeService struct {
	config          config.TranscodingConfig
	sessionStore    *SessionStore
	fileManager     *FileManager
	cleanupService  *CleanupService
	providerManager *ProviderManager
	logger          hclog.Logger
	db              *gorm.DB
}

// NewTranscodeService creates a new transcode service
func NewTranscodeService(cfg config.TranscodingConfig, db *gorm.DB, logger hclog.Logger) (*TranscodeService, error) {
	// Create file manager
	fileManager := NewFileManager(cfg.DataDir, logger)
	if err := fileManager.EnsureBaseDirectory(); err != nil {
		return nil, fmt.Errorf("failed to ensure base directory: %w", err)
	}

	// Create session store
	sessionStore := NewSessionStore(db, logger)

	// Create cleanup service
	cleanupConfig := CleanupConfig{
		BaseDirectory:      cfg.DataDir,
		RetentionHours:     cfg.RetentionHours,
		ExtendedHours:      cfg.ExtendedHours,
		MaxTotalSizeGB:     cfg.MaxDiskUsageGB,
		CleanupInterval:    cfg.CleanupInterval,
		LargeFileThreshold: cfg.LargeFileThreshold * 1024 * 1024, // Convert MB to bytes
	}
	cleanupService := NewCleanupService(cleanupConfig, sessionStore, fileManager, logger)

	// Create provider manager
	providerManager := NewProviderManager(sessionStore, logger)

	service := &TranscodeService{
		config:          cfg,
		sessionStore:    sessionStore,
		fileManager:     fileManager,
		cleanupService:  cleanupService,
		providerManager: providerManager,
		logger:          logger.Named("transcode-service"),
		db:              db,
	}

	// Start cleanup service in background
	go cleanupService.Run(context.Background())

	return service, nil
}

// RegisterProvider registers a transcoding provider
func (ts *TranscodeService) RegisterProvider(provider plugins.TranscodingProvider) error {
	return ts.providerManager.RegisterProvider(provider)
}

// StartTranscode starts a new transcoding operation
func (ts *TranscodeService) StartTranscode(ctx context.Context, req *plugins.TranscodeRequest) (*database.TranscodeSession, error) {
	// Check session limits
	activeSessions, err := ts.sessionStore.GetActiveSessions()
	if err != nil {
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}

	if len(activeSessions) >= ts.config.MaxSessions {
		return nil, fmt.Errorf("maximum number of sessions reached: %d", ts.config.MaxSessions)
	}

	// Select provider
	provider, err := ts.providerManager.SelectProvider(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to select provider: %w", err)
	}

	providerInfo := provider.GetInfo()

	// Create session in database
	session, err := ts.sessionStore.CreateSession(providerInfo.ID, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Create session directory
	dirPath, err := ts.fileManager.CreateSessionDirectory(session.ID, providerInfo.ID, req.Container)
	if err != nil {
		ts.sessionStore.FailSession(session.ID, err)
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}
	session.DirectoryPath = dirPath

	// Update session with directory path
	ts.db.Model(session).Update("directory_path", dirPath)

	// Start transcoding with timeout
	transcodeCtx, cancel := context.WithTimeout(ctx, ts.config.SessionTimeout)

	// Start transcoding in goroutine
	go func() {
		defer cancel()

		// Start the transcoding operation
		handle, err := provider.StartTranscode(transcodeCtx, *req)
		if err != nil {
			ts.logger.Error("failed to start transcoding", "error", err, "session_id", session.ID)
			ts.sessionStore.FailSession(session.ID, err)
			return
		}

		// Monitor progress
		ts.monitorProgress(transcodeCtx, session.ID, provider, handle)
	}()

	ts.logger.Info("started transcoding session",
		"session_id", session.ID,
		"provider", providerInfo.ID,
		"container", req.Container)

	return session, nil
}

// monitorProgress monitors the progress of a transcoding operation
func (ts *TranscodeService) monitorProgress(ctx context.Context, sessionID string, provider plugins.TranscodingProvider, handle *plugins.TranscodeHandle) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context cancelled, stop transcoding
			provider.StopTranscode(handle)
			ts.sessionStore.FailSession(sessionID, ctx.Err())
			return

		case <-ticker.C:
			// Get progress
			progress, err := provider.GetProgress(handle)
			if err != nil {
				ts.logger.Warn("failed to get progress", "error", err, "session_id", sessionID)
				continue
			}

			// Update progress in database
			ts.sessionStore.UpdateProgress(sessionID, progress)

			// Check if completed
			if progress.PercentComplete >= 100 {
				ts.completeSession(sessionID, handle)
				return
			}
		}
	}
}

// completeSession marks a session as completed
func (ts *TranscodeService) completeSession(sessionID string, handle *plugins.TranscodeHandle) {
	// Get manifest path
	_, err := ts.fileManager.GetManifestPath(sessionID)
	if err != nil {
		ts.logger.Warn("failed to get manifest path", "error", err, "session_id", sessionID)
	}

	// Get directory size for stats
	dirSize, _ := ts.fileManager.GetDirectorySize(handle.Directory)

	result := &plugins.TranscodeResult{
		Success:      true,
		OutputPath:   handle.Directory,
		ManifestURL:  fmt.Sprintf("/api/playback/stream/%s/manifest", sessionID),
		BytesWritten: dirSize,
	}

	ts.sessionStore.CompleteSession(sessionID, result)
	ts.logger.Info("completed transcoding session", "session_id", sessionID)
}

// StopTranscode stops a transcoding operation
func (ts *TranscodeService) StopTranscode(sessionID string) error {
	session, err := ts.sessionStore.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	// Get provider
	_, err = ts.providerManager.GetProvider(session.Provider)
	if err != nil {
		return fmt.Errorf("provider not found: %w", err)
	}

	// Stop the transcoding (would need to store handle in session)
	// For now, just mark as cancelled
	ts.db.Model(session).Update("status", database.TranscodeStatusCancelled)

	// Cleanup files
	ts.cleanupService.CleanupSession(sessionID)

	ts.logger.Info("stopped transcoding session", "session_id", sessionID)
	return nil
}

// GetSession returns session information
func (ts *TranscodeService) GetSession(sessionID string) (*database.TranscodeSession, error) {
	return ts.sessionStore.GetSession(sessionID)
}

// ListSessions lists transcoding sessions
func (ts *TranscodeService) ListSessions(provider string, filter SessionFilter) ([]*database.TranscodeSession, error) {
	// If no provider specified, get all
	if provider == "" {
		return ts.sessionStore.GetActiveSessions()
	}

	return ts.sessionStore.ListProviderSessions(provider, filter)
}

// GetProviders returns all registered providers
func (ts *TranscodeService) GetProviders() []plugins.ProviderInfo {
	return ts.providerManager.ListProviders()
}

// GetCleanupStats returns cleanup statistics
func (ts *TranscodeService) GetCleanupStats() (*CleanupStats, error) {
	return ts.cleanupService.GetCleanupStats()
}

// GetDashboardData returns dashboard data for the transcoding system
func (ts *TranscodeService) GetDashboardData() (*TranscodingDashboardData, error) {
	// Get active sessions
	activeSessions, _ := ts.sessionStore.GetActiveSessions()

	// Get provider resources
	resources := ts.providerManager.GetProviderResources()

	// Get cleanup stats
	cleanupStats, _ := ts.cleanupService.GetCleanupStats()

	// Build overview
	overview := TranscodingOverview{
		ActiveSessions: len(activeSessions),
		TotalProcessed: 0, // Would need to query completed sessions
		DiskUsage:      plugins.FormatBytes(cleanupStats.TotalSize),
		ProviderStatus: make(map[string]string),
	}

	// Add provider status
	for _, info := range ts.providerManager.ListProviders() {
		if res, ok := resources[info.ID]; ok && res.ActiveSessions > 0 {
			overview.ProviderStatus[info.ID] = "active"
		} else {
			overview.ProviderStatus[info.ID] = "idle"
		}
	}

	// Convert sessions to views
	var sessionViews []TranscodeSessionView
	for _, session := range activeSessions {
		view := TranscodeSessionView{
			ID:       session.ID,
			Provider: session.Provider,
			Status:   string(session.Status),
			Progress: 0,
		}

		if session.Progress != "" {
			if progress, err := session.GetProgress(); err == nil && progress != nil {
				view.Progress = progress.PercentComplete
			}
		}

		if session.Request != "" {
			if request, err := session.GetRequest(); err == nil && request != nil {
				view.InputPath = request.InputPath
				view.Quality = request.Quality
				view.SpeedPriority = request.SpeedPriority
			}
		}

		sessionViews = append(sessionViews, view)
	}

	return &TranscodingDashboardData{
		Overview: overview,
		Sessions: sessionViews,
		Hardware: HardwareStatus{
			Available: ts.getAvailableHardware(),
		},
	}, nil
}

// getAvailableHardware returns available hardware accelerators
func (ts *TranscodeService) getAvailableHardware() []HardwareInfo {
	var hardware []HardwareInfo
	seen := make(map[string]bool)

	for _, provider := range ts.providerManager.ListProviders() {
		p, _ := ts.providerManager.GetProvider(provider.ID)
		if p == nil {
			continue
		}

		for _, accel := range p.GetHardwareAccelerators() {
			if !seen[accel.ID] && accel.Available {
				hardware = append(hardware, HardwareInfo{
					Type:      accel.Type,
					Name:      accel.Name,
					Available: true,
				})
				seen[accel.ID] = true
			}
		}
	}

	return hardware
}

// Types for dashboard

type TranscodingDashboardData struct {
	Overview    TranscodingOverview    `json:"overview"`
	Sessions    []TranscodeSessionView `json:"sessions"`
	Hardware    HardwareStatus         `json:"hardware"`
	Performance PerformanceMetrics     `json:"performance"`
}

type TranscodingOverview struct {
	ActiveSessions int               `json:"active_sessions"`
	QueuedSessions int               `json:"queued_sessions"`
	TotalProcessed int64             `json:"total_processed"`
	DiskUsage      string            `json:"disk_usage"`
	ProviderStatus map[string]string `json:"provider_status"`
}

type TranscodeSessionView struct {
	ID            string                `json:"id"`
	Provider      string                `json:"provider"`
	Status        string                `json:"status"`
	Progress      float64               `json:"progress"`
	InputPath     string                `json:"input_path"`
	Quality       int                   `json:"quality"`
	SpeedPriority plugins.SpeedPriority `json:"speed_priority"`
	StartTime     time.Time             `json:"start_time"`
}

type HardwareStatus struct {
	Available []HardwareInfo `json:"available"`
}

type HardwareInfo struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	Available bool   `json:"available"`
}

type PerformanceMetrics struct {
	AverageSpeed    float64 `json:"average_speed"`
	TotalBandwidth  int64   `json:"total_bandwidth"`
	SessionsPerHour int     `json:"sessions_per_hour"`
}
