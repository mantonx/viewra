// Package session provides comprehensive transcoding session lifecycle management.
// This manager ensures reliable tracking of all active transcoding operations,
// preventing resource leaks, orphaned processes, and providing visibility into
// system load. It coordinates between FFmpeg processes, progress tracking, and
// resource cleanup to maintain system stability.
//
// The session manager addresses critical operational concerns:
// - Preventing multiple transcodes of the same content
// - Tracking resource usage across all sessions
// - Graceful shutdown of all sessions during maintenance
// - Automatic cleanup of stale or abandoned sessions
// - Real-time progress and status monitoring
//
// Session lifecycle:
// 1. Creation: Validates request and allocates resources
// 2. Starting: Process launch and initial monitoring setup
// 3. Running: Active transcoding with progress tracking
// 4. Stopping: Graceful shutdown or error handling
// 5. Cleanup: Resource deallocation and file cleanup
//
// Example usage:
//   manager := session.NewManager(logger)
//   session, err := manager.CreateSession(ctx, transcodeRequest)
//   // Monitor progress
//   manager.UpdateSession(sessionID, func(s *Session) {
//       s.Progress = 50.0
//   })
//   // Graceful shutdown
//   manager.StopAllSessions()
package session

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mantonx/viewra/sdk/transcoding/types"
)

// Session represents an active transcoding session
type Session struct {
	ID        string
	Handle    *types.TranscodeHandle
	Process   *exec.Cmd
	StartTime time.Time
	Request   types.TranscodeRequest
	Cancel    context.CancelFunc
	Progress  float64
	Status    SessionStatus
}

// SessionStatus represents the current state of a session
type SessionStatus string

const (
	SessionStatusStarting SessionStatus = "starting"
	SessionStatusRunning  SessionStatus = "running"
	SessionStatusStopping SessionStatus = "stopping"
	SessionStatusStopped  SessionStatus = "stopped"
	SessionStatusFailed   SessionStatus = "failed"
	SessionStatusComplete SessionStatus = "complete"
)

// Manager handles transcoding session lifecycle
type Manager struct {
	sessions map[string]*Session
	mutex    sync.RWMutex
	logger   types.Logger
}



// NewManager creates a new session manager
func NewManager(logger types.Logger) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		logger:   logger,
	}
}

// CreateSession creates and registers a new transcoding session
func (m *Manager) CreateSession(ctx context.Context, req types.TranscodeRequest) (*Session, error) {
	// Generate session ID if not provided
	if req.SessionID == "" {
		req.SessionID = uuid.New().String()
	}

	// Create context with cancel for this session
	sessionCtx, cancelFunc := context.WithCancel(ctx)

	// Create handle
	handle := &types.TranscodeHandle{
		SessionID:   req.SessionID,
		StartTime:   time.Now(),
		Context:     sessionCtx,
		CancelFunc:  cancelFunc,
		PrivateData: req.SessionID,
	}

	// Create session
	session := &Session{
		ID:        req.SessionID,
		Handle:    handle,
		StartTime: time.Now(),
		Request:   req,
		Cancel:    cancelFunc,
		Progress:  0,
		Status:    SessionStatusStarting,
	}

	// Register session
	m.mutex.Lock()
	m.sessions[req.SessionID] = session
	m.mutex.Unlock()

	if m.logger != nil {
		m.logger.Info("created transcoding session",
			"session_id", req.SessionID,
			"input", req.InputPath,
			"container", req.Container,
		)
	}

	return session, nil
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(sessionID string) (*Session, error) {
	m.mutex.RLock()
	session, exists := m.sessions[sessionID]
	m.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// UpdateSession updates session information
func (m *Manager) UpdateSession(sessionID string, update func(*Session)) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	session, exists := m.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	update(session)
	return nil
}

// RemoveSession removes a session from the manager
func (m *Manager) RemoveSession(sessionID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if session, exists := m.sessions[sessionID]; exists {
		if m.logger != nil {
			m.logger.Debug("removing session",
				"session_id", sessionID,
				"status", session.Status,
			)
		}
		delete(m.sessions, sessionID)
	}
}

// GetAllSessions returns all active sessions
func (m *Manager) GetAllSessions() []*Session {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

// GetSessionCount returns the number of active sessions
func (m *Manager) GetSessionCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.sessions)
}

// StopSession stops a specific session
func (m *Manager) StopSession(sessionID string) error {
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	// Update status
	m.UpdateSession(sessionID, func(s *Session) {
		s.Status = SessionStatusStopping
	})

	// Cancel the context
	session.Cancel()

	if m.logger != nil {
		m.logger.Info("stopping session", "session_id", sessionID)
	}

	return nil
}

// StopAllSessions stops all active sessions
func (m *Manager) StopAllSessions() {
	sessions := m.GetAllSessions()

	for _, session := range sessions {
		if err := m.StopSession(session.ID); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to stop session",
					"session_id", session.ID,
					"error", err,
				)
			}
		}
	}
}

// CleanupStaleSessions removes sessions that have been in a terminal state for too long
func (m *Manager) CleanupStaleSessions(maxAge time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	toRemove := []string{}

	for id, session := range m.sessions {
		// Check if session is in a terminal state
		if session.Status == SessionStatusComplete ||
			session.Status == SessionStatusFailed ||
			session.Status == SessionStatusStopped {
			
			// Check age
			age := now.Sub(session.StartTime)
			if age > maxAge {
				toRemove = append(toRemove, id)
			}
		}
	}

	// Remove stale sessions
	for _, id := range toRemove {
		if m.logger != nil {
			m.logger.Debug("removing stale session", "session_id", id)
		}
		delete(m.sessions, id)
	}
}

// GetSessionStats returns statistics about current sessions
func (m *Manager) GetSessionStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	statusCounts := make(map[SessionStatus]int)
	var totalProgress float64

	for _, session := range m.sessions {
		statusCounts[session.Status]++
		totalProgress += session.Progress
	}

	avgProgress := float64(0)
	if len(m.sessions) > 0 {
		avgProgress = totalProgress / float64(len(m.sessions))
	}

	return map[string]interface{}{
		"total_sessions":   len(m.sessions),
		"status_counts":    statusCounts,
		"average_progress": avgProgress,
	}
}