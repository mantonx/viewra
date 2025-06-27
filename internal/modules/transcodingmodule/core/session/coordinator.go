// Package session provides session coordination that manages the lifecycle of transcoding sessions
// with proper state transitions, concurrent handling, and resource management.
package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/transcodingmodule/core/process"
	plugins "github.com/mantonx/viewra/sdk"
	"gorm.io/gorm"
)

// SessionCoordinator coordinates all aspects of session management including
// state transitions, process tracking, and concurrent access control
type SessionCoordinator struct {
	// Core components
	sessionManager  *SessionManager
	processRegistry *process.ProcessRegistry
	stateValidator  *StateValidator

	// Database for persistence
	db *gorm.DB

	// Coordination
	coordinationMutex sync.RWMutex

	// Lifecycle management
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	shutdownWg     sync.WaitGroup

	logger hclog.Logger
}

// CoordinatorConfig contains configuration for the session coordinator
type CoordinatorConfig struct {
	// Session management configuration
	SessionConfig Config

	// Process registry configuration
	ProcessConfig process.RegistryConfig

	// Coordination settings
	StateTransitionTimeout  time.Duration
	ResourceCleanupInterval time.Duration

	// Concurrency limits
	MaxConcurrentSessions    int
	MaxConcurrentTransitions int
}

// DefaultCoordinatorConfig returns default configuration
func DefaultCoordinatorConfig() CoordinatorConfig {
	return CoordinatorConfig{
		SessionConfig:            DefaultConfig(),
		ProcessConfig:            process.DefaultRegistryConfig(),
		StateTransitionTimeout:   30 * time.Second,
		ResourceCleanupInterval:  10 * time.Minute,
		MaxConcurrentSessions:    20,
		MaxConcurrentTransitions: 5,
	}
}

// NewSessionCoordinator creates a new session coordinator
func NewSessionCoordinator(db *gorm.DB, logger hclog.Logger, config CoordinatorConfig) *SessionCoordinator {
	ctx, cancel := context.WithCancel(context.Background())

	coordinator := &SessionCoordinator{
		sessionManager:  NewSessionManager(db, logger, config.SessionConfig),
		processRegistry: process.GetRegistry(logger, config.ProcessConfig),
		stateValidator:  NewStateValidator(),
		db:              db,
		shutdownCtx:     ctx,
		shutdownCancel:  cancel,
		logger:          logger.Named("session-coordinator"),
	}

	// Start background resource cleanup if enabled
	if config.ResourceCleanupInterval > 0 {
		coordinator.shutdownWg.Add(1)
		go coordinator.runResourceCleanup(config.ResourceCleanupInterval)
	}

	return coordinator
}

// CreateSession creates a new session with full coordination
func (sc *SessionCoordinator) CreateSession(ctx context.Context, provider string, req *plugins.TranscodeRequest) (*Session, error) {
	sc.logger.Debug("creating coordinated session",
		"session_id", req.SessionID,
		"provider", provider,
		"media_id", req.MediaID)

	// Create the session through the manager
	session, err := sc.sessionManager.CreateSession(ctx, provider, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	sc.logger.Info("created coordinated session",
		"session_id", session.ID,
		"provider", provider,
		"initial_status", session.Status)

	return session, nil
}

// TransitionSessionState safely transitions a session with validation and coordination
func (sc *SessionCoordinator) TransitionSessionState(ctx context.Context, sessionID string, newStatus Status, data interface{}) error {
	sc.coordinationMutex.RLock()
	defer sc.coordinationMutex.RUnlock()

	// Get current session
	session, err := sc.sessionManager.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	oldStatus := session.Status

	// Validate the state transition
	if err := sc.stateValidator.ValidateTransition(session, newStatus, data); err != nil {
		return fmt.Errorf("invalid state transition: %w", err)
	}

	// Perform the transition
	if err := sc.sessionManager.TransitionSessionState(sessionID, newStatus, data); err != nil {
		return fmt.Errorf("failed to transition state: %w", err)
	}

	// Handle post-transition coordination
	if err := sc.handlePostTransitionActions(ctx, sessionID, oldStatus, newStatus, data); err != nil {
		sc.logger.Error("post-transition action failed",
			"session_id", sessionID,
			"old_status", oldStatus,
			"new_status", newStatus,
			"error", err)
		// Don't return error here as the transition itself succeeded
	}

	sc.logger.Info("coordinated state transition",
		"session_id", sessionID,
		"from", oldStatus,
		"to", newStatus)

	return nil
}

// RegisterProcess registers a process with both session and process management
func (sc *SessionCoordinator) RegisterProcess(sessionID string, pid int, provider string, cmd interface{}) error {
	// Register with process registry
	if err := sc.processRegistry.Register(pid, sessionID, provider, cmd); err != nil {
		return fmt.Errorf("failed to register process: %w", err)
	}

	// Update session with process information
	err := sc.sessionManager.UpdateSession(sessionID, func(session *Session) {
		session.Process = cmd
	})
	if err != nil {
		// Cleanup process registration on session update failure
		sc.processRegistry.Unregister(pid)
		return fmt.Errorf("failed to update session with process: %w", err)
	}

	sc.logger.Debug("registered process with coordination",
		"session_id", sessionID,
		"pid", pid,
		"provider", provider)

	return nil
}

// UnregisterProcess unregisters a process from coordination
func (sc *SessionCoordinator) UnregisterProcess(pid int) error {
	// Get process info before unregistering
	processInfo, exists := sc.processRegistry.GetProcess(pid)
	if !exists {
		return fmt.Errorf("process %d not found", pid)
	}

	sessionID := processInfo.SessionID

	// Unregister from process registry
	if err := sc.processRegistry.Unregister(pid); err != nil {
		return fmt.Errorf("failed to unregister process: %w", err)
	}

	// Update session to remove process reference
	err := sc.sessionManager.UpdateSession(sessionID, func(session *Session) {
		session.Process = nil
	})
	if err != nil {
		sc.logger.Warn("failed to update session after process unregistration",
			"session_id", sessionID,
			"pid", pid,
			"error", err)
	}

	sc.logger.Debug("unregistered process from coordination",
		"session_id", sessionID,
		"pid", pid)

	return nil
}

// StopSession stops a session and all associated processes
func (sc *SessionCoordinator) StopSession(ctx context.Context, sessionID string) error {
	sc.logger.Info("stopping coordinated session", "session_id", sessionID)

	// Get session processes
	processes := sc.processRegistry.GetProcessesBySession(sessionID)

	// Stop all processes for this session
	for _, processInfo := range processes {
		if err := sc.processRegistry.StopProcess(processInfo.PID); err != nil {
			sc.logger.Error("failed to stop process during session stop",
				"session_id", sessionID,
				"pid", processInfo.PID,
				"error", err)
		}
	}

	// Transition session to stopped state
	if err := sc.TransitionSessionState(ctx, sessionID, StatusStopped, nil); err != nil {
		return fmt.Errorf("failed to transition session to stopped: %w", err)
	}

	sc.logger.Info("stopped coordinated session", "session_id", sessionID)
	return nil
}

// UpdateSessionProgress updates session progress with coordination
func (sc *SessionCoordinator) UpdateSessionProgress(sessionID string, progress *plugins.TranscodingProgress) error {
	// Update through session manager
	if err := sc.sessionManager.UpdateSessionProgress(sessionID, progress); err != nil {
		return fmt.Errorf("failed to update session progress: %w", err)
	}

	// Check if we should transition to running state
	session, err := sc.sessionManager.GetSession(sessionID)
	if err == nil && session.Status == StatusStarting && progress.PercentComplete > 0 {
		// Automatically transition to running when progress begins
		if err := sc.sessionManager.TransitionSessionState(sessionID, StatusRunning, nil); err != nil {
			sc.logger.Warn("failed to auto-transition to running state",
				"session_id", sessionID,
				"error", err)
		}
	}

	return nil
}

// GetSession retrieves a session through coordination
func (sc *SessionCoordinator) GetSession(sessionID string) (*Session, error) {
	return sc.sessionManager.GetSession(sessionID)
}

// GetSessionStats returns comprehensive session statistics
func (sc *SessionCoordinator) GetSessionStats() map[string]interface{} {
	sessionStats := sc.sessionManager.GetSessionStats()
	processStats := sc.processRegistry.GetRegistryStats()

	return map[string]interface{}{
		"sessions":  sessionStats,
		"processes": processStats,
		"coordination": map[string]interface{}{
			"total_coordinated_sessions": sessionStats["active_sessions"],
			"state_validator_enabled":    true,
		},
	}
}

// handlePostTransitionActions handles actions that need to occur after state transitions
func (sc *SessionCoordinator) handlePostTransitionActions(ctx context.Context, sessionID string, oldStatus, newStatus Status, data interface{}) error {
	switch newStatus {
	case StatusComplete:
		// Clean up processes
		processes := sc.processRegistry.GetProcessesBySession(sessionID)
		for _, processInfo := range processes {
			sc.processRegistry.Unregister(processInfo.PID)
		}

	case StatusFailed:
		// Clean up processes and mark session as failed
		processes := sc.processRegistry.GetProcessesBySession(sessionID)
		for _, processInfo := range processes {
			sc.processRegistry.StopProcess(processInfo.PID)
		}

	case StatusStopped:
		// Ensure all processes are stopped
		processes := sc.processRegistry.GetProcessesBySession(sessionID)
		for _, processInfo := range processes {
			sc.processRegistry.StopProcess(processInfo.PID)
		}
	}

	return nil
}

// runResourceCleanup runs periodic resource cleanup
func (sc *SessionCoordinator) runResourceCleanup(interval time.Duration) {
	defer sc.shutdownWg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sc.performResourceCleanup()
		case <-sc.shutdownCtx.Done():
			return
		}
	}
}

// performResourceCleanup performs comprehensive resource cleanup
func (sc *SessionCoordinator) performResourceCleanup() {
	sc.logger.Debug("performing coordinated resource cleanup")

	// Clean up orphaned processes
	killedProcesses := sc.processRegistry.CleanupOrphaned()
	if killedProcesses > 0 {
		sc.logger.Info("cleaned up orphaned processes", "count", killedProcesses)
	}

	// The session manager handles its own cleanup via stale session cleanup
	// This ensures database and in-memory state stay synchronized
}

// ValidateSessionState validates the consistency of a session's state
func (sc *SessionCoordinator) ValidateSessionState(sessionID string) error {
	session, err := sc.sessionManager.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Validate using state validator
	errors := sc.stateValidator.ValidateSessionConsistency(session)
	if len(errors) > 0 {
		return fmt.Errorf("session validation failed: %v", errors)
	}

	return nil
}

// RecoverStrandedSessions attempts to recover sessions that may have been left in inconsistent states
func (sc *SessionCoordinator) RecoverStrandedSessions(ctx context.Context) (int, error) {
	sc.logger.Info("recovering stranded sessions")

	recoveredCount := 0

	// Get all active sessions from database
	var dbSessions []*database.TranscodeSession
	if err := sc.db.Where("status IN ?", []string{"running", "queued", "starting"}).Find(&dbSessions).Error; err != nil {
		return 0, fmt.Errorf("failed to query active sessions: %w", err)
	}

	for _, dbSession := range dbSessions {
		// Check if session exists in memory
		_, err := sc.sessionManager.GetSession(dbSession.ID)
		if err != nil {
			// Session not in memory, attempt recovery
			sc.logger.Info("recovering stranded session",
				"session_id", dbSession.ID,
				"status", dbSession.Status,
				"provider", dbSession.Provider)

			// Mark as failed since we can't recover the exact state
			if err := sc.sessionManager.store.FailSession(dbSession.ID,
				fmt.Errorf("session recovered after server restart")); err != nil {
				sc.logger.Error("failed to mark recovered session as failed",
					"session_id", dbSession.ID,
					"error", err)
			} else {
				recoveredCount++
			}
		}
	}

	sc.logger.Info("session recovery completed", "recovered_count", recoveredCount)
	return recoveredCount, nil
}

// Shutdown gracefully shuts down the session coordinator
func (sc *SessionCoordinator) Shutdown(ctx context.Context) error {
	sc.logger.Info("shutting down session coordinator")

	// Signal shutdown to background goroutines
	sc.shutdownCancel()

	// Shutdown components
	var shutdownErrors []error

	// Shutdown session manager
	if err := sc.sessionManager.Shutdown(ctx); err != nil {
		shutdownErrors = append(shutdownErrors, fmt.Errorf("session manager shutdown failed: %w", err))
	}

	// Shutdown process registry
	if err := sc.processRegistry.Shutdown(ctx); err != nil {
		shutdownErrors = append(shutdownErrors, fmt.Errorf("process registry shutdown failed: %w", err))
	}

	// Wait for background goroutines
	done := make(chan struct{})
	go func() {
		sc.shutdownWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		sc.logger.Info("session coordinator shutdown completed")
	case <-ctx.Done():
		shutdownErrors = append(shutdownErrors, fmt.Errorf("shutdown timeout exceeded"))
	}

	if len(shutdownErrors) > 0 {
		return fmt.Errorf("shutdown errors: %v", shutdownErrors)
	}

	return nil
}
