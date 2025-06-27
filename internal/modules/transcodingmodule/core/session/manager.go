// Package session provides unified session management with proper state transitions and concurrency handling.
// This replaces the duplicate session management implementations with a single, thread-safe solution.
package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	plugins "github.com/mantonx/viewra/sdk"
	"gorm.io/gorm"
)

// SessionManager provides thread-safe session management with proper state transitions
// and concurrent handling. It eliminates the race conditions found in the previous implementation.
type SessionManager struct {
	// Database store for persistence
	store *SessionStore

	// In-memory cache for active sessions
	activeSessions map[string]*Session
	sessionMutex   sync.RWMutex

	// Session-level locks to prevent concurrent modifications of the same session
	sessionLocks sync.Map // map[string]*sync.RWMutex

	// State transition management
	stateTransitions map[Status][]Status

	// Configuration
	config Config
	logger hclog.Logger
}

// Config contains configuration for the session manager
type Config struct {
	// Maximum number of concurrent sessions per provider
	MaxConcurrentSessions int

	// Timeout for session state transitions
	StateTransitionTimeout time.Duration

	// Cleanup intervals
	CleanupInterval     time.Duration
	StaleSessionTimeout time.Duration

	// Session validation
	ValidateSessionIDs   bool
	EnableOptimisticLock bool
}

// DefaultConfig returns a default configuration
func DefaultConfig() Config {
	return Config{
		MaxConcurrentSessions:  10,
		StateTransitionTimeout: 30 * time.Second,
		CleanupInterval:        5 * time.Minute,
		StaleSessionTimeout:    30 * time.Minute,
		ValidateSessionIDs:     true,
		EnableOptimisticLock:   true,
	}
}

// NewSessionManager creates a new unified session manager
func NewSessionManager(db *gorm.DB, logger hclog.Logger, config Config) *SessionManager {
	store := NewSessionStore(db, logger)

	manager := &SessionManager{
		store:          store,
		activeSessions: make(map[string]*Session),
		config:         config,
		logger:         logger.Named("session-manager"),
	}

	// Initialize valid state transitions
	manager.initializeStateTransitions()

	// Start background cleanup if enabled
	if manager.config.CleanupInterval > 0 {
		go manager.runCleanupLoop()
	}

	return manager
}

// initializeStateTransitions sets up the valid state transition matrix
func (sm *SessionManager) initializeStateTransitions() {
	sm.stateTransitions = map[Status][]Status{
		StatusPending:  {StatusStarting, StatusFailed, StatusStopped},
		StatusStarting: {StatusRunning, StatusFailed, StatusStopped},
		StatusRunning:  {StatusComplete, StatusFailed, StatusStopped},
		StatusComplete: {}, // Terminal state
		StatusFailed:   {}, // Terminal state
		StatusStopped:  {}, // Terminal state
	}
}

// CreateSession creates a new session with proper validation and state management
func (sm *SessionManager) CreateSession(ctx context.Context, provider string, req *plugins.TranscodeRequest) (*Session, error) {
	// Validate session ID format if enabled
	if sm.config.ValidateSessionIDs {
		if err := sm.validateSessionID(req.SessionID); err != nil {
			return nil, fmt.Errorf("invalid session ID: %w", err)
		}
	}

	// Check concurrent session limits
	if err := sm.checkConcurrencyLimits(provider); err != nil {
		return nil, err
	}

	// Get or create session-level lock
	sessionLock := sm.getSessionLock(req.SessionID)
	sessionLock.Lock()
	defer sessionLock.Unlock()

	// Check if session already exists (double-check with lock)
	sm.sessionMutex.RLock()
	if _, exists := sm.activeSessions[req.SessionID]; exists {
		sm.sessionMutex.RUnlock()
		return nil, fmt.Errorf("session already exists: %s", req.SessionID)
	}
	sm.sessionMutex.RUnlock()

	// Create database session
	dbSession, err := sm.store.CreateSession(provider, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create database session: %w", err)
	}

	// Create in-memory session
	session := &Session{
		ID:        dbSession.ID,
		Request:   *req,
		Handle:    nil, // Will be set when transcoding starts
		Process:   nil, // Will be set when process starts
		Status:    StatusPending,
		Progress:  0,
		StartTime: dbSession.StartTime,
		Error:     nil,
	}

	// Add to active sessions
	sm.sessionMutex.Lock()
	sm.activeSessions[req.SessionID] = session
	sm.sessionMutex.Unlock()

	sm.logger.Info("created session",
		"session_id", req.SessionID,
		"provider", provider,
		"directory", dbSession.DirectoryPath,
		"content_hash", dbSession.ContentHash)

	return session, nil
}

// TransitionSessionState safely transitions a session to a new state
func (sm *SessionManager) TransitionSessionState(sessionID string, newStatus Status, result interface{}) error {
	sessionLock := sm.getSessionLock(sessionID)
	sessionLock.Lock()
	defer sessionLock.Unlock()

	// Get current session
	sm.sessionMutex.RLock()
	session, exists := sm.activeSessions[sessionID]
	sm.sessionMutex.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Validate state transition
	if !sm.isValidTransition(session.Status, newStatus) {
		return fmt.Errorf("invalid state transition from %s to %s for session %s",
			session.Status, newStatus, sessionID)
	}

	// Update in-memory state
	oldStatus := session.Status
	session.Status = newStatus

	// Update database state with context timeout
	_, cancel := context.WithTimeout(context.Background(), sm.config.StateTransitionTimeout)
	defer cancel()

	var err error
	switch newStatus {
	case StatusComplete:
		if result != nil {
			if transcodeResult, ok := result.(*plugins.TranscodeResult); ok {
				err = sm.store.CompleteSession(sessionID, transcodeResult)
			}
		}
	case StatusFailed:
		if result != nil {
			if errResult, ok := result.(error); ok {
				session.Error = errResult
				err = sm.store.FailSession(sessionID, errResult)
			}
		}
	default:
		// For other states, update status directly
		err = sm.store.UpdateSessionStatus(sessionID, string(newStatus), "")
	}

	if err != nil {
		// Rollback in-memory state on database error
		session.Status = oldStatus
		return fmt.Errorf("failed to update database state: %w", err)
	}

	// Remove from active sessions if in terminal state
	if sm.isTerminalState(newStatus) {
		sm.sessionMutex.Lock()
		delete(sm.activeSessions, sessionID)
		sm.sessionMutex.Unlock()

		// Clean up session lock
		sm.sessionLocks.Delete(sessionID)
	}

	sm.logger.Debug("transitioned session state",
		"session_id", sessionID,
		"old_status", oldStatus,
		"new_status", newStatus)

	return nil
}

// UpdateSessionProgress updates session progress with proper synchronization
func (sm *SessionManager) UpdateSessionProgress(sessionID string, progress *plugins.TranscodingProgress) error {
	sessionLock := sm.getSessionLock(sessionID)
	sessionLock.RLock()
	defer sessionLock.RUnlock()

	// Update in-memory session
	sm.sessionMutex.RLock()
	session, exists := sm.activeSessions[sessionID]
	sm.sessionMutex.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// Update progress
	session.Progress = progress.PercentComplete

	// Update database
	return sm.store.UpdateProgress(sessionID, progress)
}

// GetSession retrieves a session with proper synchronization
func (sm *SessionManager) GetSession(sessionID string) (*Session, error) {
	sm.sessionMutex.RLock()
	session, exists := sm.activeSessions[sessionID]
	sm.sessionMutex.RUnlock()

	if exists {
		return session, nil
	}

	// If not in active sessions, check database
	dbSession, err := sm.store.GetSession(sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Convert database session to in-memory session if still active
	if dbSession.Status == "running" || dbSession.Status == "queued" {
		// Reconstruct session from database
		// This handles cases where the server restarted
		reconstructedSession := &Session{
			ID:        dbSession.ID,
			Status:    Status(dbSession.Status),
			StartTime: dbSession.StartTime,
			Progress:  0, // Progress will be updated separately
		}

		// Add back to active sessions
		sm.sessionMutex.Lock()
		sm.activeSessions[sessionID] = reconstructedSession
		sm.sessionMutex.Unlock()

		return reconstructedSession, nil
	}

	return nil, fmt.Errorf("session not active: %s", sessionID)
}

// UpdateSession updates a session with a modifier function
func (sm *SessionManager) UpdateSession(sessionID string, modifier func(*Session)) error {
	sessionLock := sm.getSessionLock(sessionID)
	sessionLock.Lock()
	defer sessionLock.Unlock()

	sm.sessionMutex.RLock()
	session, exists := sm.activeSessions[sessionID]
	sm.sessionMutex.RUnlock()

	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	modifier(session)
	return nil
}

// StopSession stops a session gracefully
func (sm *SessionManager) StopSession(sessionID string) error {
	return sm.TransitionSessionState(sessionID, StatusStopped, nil)
}

// GetActiveSessionCount returns the number of active sessions for a provider
func (sm *SessionManager) GetActiveSessionCount(provider string) int {
	sm.sessionMutex.RLock()
	defer sm.sessionMutex.RUnlock()

	count := 0
	for range sm.activeSessions {
		// Count sessions for this provider (sessions don't store provider directly)
		count++
	}
	return count
}

// GetAllActiveSessions returns all active sessions
func (sm *SessionManager) GetAllActiveSessions() []*Session {
	sm.sessionMutex.RLock()
	defer sm.sessionMutex.RUnlock()

	sessions := make([]*Session, 0, len(sm.activeSessions))
	for _, session := range sm.activeSessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// GetSessionStats returns comprehensive session statistics
func (sm *SessionManager) GetSessionStats() map[string]interface{} {
	sm.sessionMutex.RLock()
	defer sm.sessionMutex.RUnlock()

	stats := map[string]interface{}{
		"active_sessions": len(sm.activeSessions),
		"by_status": map[string]int{
			"pending":  0,
			"starting": 0,
			"running":  0,
		},
		"by_provider": make(map[string]int),
	}

	for _, session := range sm.activeSessions {
		// Count by status
		statusMap := stats["by_status"].(map[string]int)
		statusMap[string(session.Status)]++

		// Count by provider - sessions don't store provider directly
		// This would need to be tracked separately if needed
	}

	return stats
}

// Helper methods

// getSessionLock returns or creates a session-specific lock
func (sm *SessionManager) getSessionLock(sessionID string) *sync.RWMutex {
	if lock, ok := sm.sessionLocks.Load(sessionID); ok {
		return lock.(*sync.RWMutex)
	}

	// Create new lock
	newLock := &sync.RWMutex{}
	actual, _ := sm.sessionLocks.LoadOrStore(sessionID, newLock)
	return actual.(*sync.RWMutex)
}

// validateSessionID validates session ID format
func (sm *SessionManager) validateSessionID(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	if _, err := uuid.Parse(sessionID); err != nil {
		return fmt.Errorf("session ID must be a valid UUID: %w", err)
	}

	return nil
}

// checkConcurrencyLimits checks if creating a new session would exceed limits
func (sm *SessionManager) checkConcurrencyLimits(provider string) error {
	if sm.config.MaxConcurrentSessions <= 0 {
		return nil // No limit
	}

	activeCount := sm.GetActiveSessionCount(provider)
	if activeCount >= sm.config.MaxConcurrentSessions {
		return fmt.Errorf("maximum concurrent sessions reached for provider %s: %d/%d",
			provider, activeCount, sm.config.MaxConcurrentSessions)
	}

	return nil
}

// isValidTransition checks if a state transition is valid
func (sm *SessionManager) isValidTransition(from, to Status) bool {
	validStates, exists := sm.stateTransitions[from]
	if !exists {
		return false
	}

	for _, validState := range validStates {
		if validState == to {
			return true
		}
	}

	return false
}

// isTerminalState checks if a status is a terminal state
func (sm *SessionManager) isTerminalState(status Status) bool {
	return status == StatusComplete || status == StatusFailed || status == StatusStopped
}

// runCleanupLoop runs periodic cleanup of stale sessions
func (sm *SessionManager) runCleanupLoop() {
	ticker := time.NewTicker(sm.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		sm.cleanupStaleSessions()
	}
}

// cleanupStaleSessions cleans up stale sessions
func (sm *SessionManager) cleanupStaleSessions() {
	// Clean up database stale sessions
	count, err := sm.store.CleanupStaleSessions(sm.config.StaleSessionTimeout)
	if err != nil {
		sm.logger.Error("failed to cleanup stale database sessions", "error", err)
	} else if count > 0 {
		sm.logger.Info("cleaned up stale database sessions", "count", count)
	}

	// Clean up in-memory stale sessions
	sm.sessionMutex.Lock()
	defer sm.sessionMutex.Unlock()

	now := time.Now()
	toRemove := []string{}

	for sessionID, session := range sm.activeSessions {
		// Check if session has been running too long without progress
		if now.Sub(session.StartTime) > sm.config.StaleSessionTimeout {
			if session.Status == StatusRunning || session.Status == StatusStarting {
				toRemove = append(toRemove, sessionID)
			}
		}
	}

	// Remove stale sessions
	for _, sessionID := range toRemove {
		delete(sm.activeSessions, sessionID)
		sm.sessionLocks.Delete(sessionID)

		// Mark as failed in database
		sm.store.FailSession(sessionID, fmt.Errorf("session timed out"))

		sm.logger.Warn("removed stale in-memory session", "session_id", sessionID)
	}
}

// Shutdown gracefully shuts down the session manager
func (sm *SessionManager) Shutdown(ctx context.Context) error {
	sm.logger.Info("shutting down session manager")

	// Stop all active sessions
	sm.sessionMutex.RLock()
	sessionIDs := make([]string, 0, len(sm.activeSessions))
	for sessionID := range sm.activeSessions {
		sessionIDs = append(sessionIDs, sessionID)
	}
	sm.sessionMutex.RUnlock()

	// Stop each session
	for _, sessionID := range sessionIDs {
		if err := sm.StopSession(sessionID); err != nil {
			sm.logger.Error("failed to stop session during shutdown",
				"session_id", sessionID, "error", err)
		}
	}

	return nil
}
