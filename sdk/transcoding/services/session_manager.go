package services

import (
	"fmt"
	"sync"
	"time"

	"github.com/mantonx/viewra/sdk"
)

// sessionManager manages transcoding sessions
type sessionManager struct {
	logger   plugins.Logger
	sessions map[string]*Session
	mutex    sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager(logger plugins.Logger) SessionManager {
	return &sessionManager{
		logger:   logger,
		sessions: make(map[string]*Session),
	}
}

// CreateSession creates a new session
func (m *sessionManager) CreateSession(id string, inputPath string, container string) (*Session, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.sessions[id]; exists {
		return nil, fmt.Errorf("session already exists: %s", id)
	}

	session := &Session{
		ID:        id,
		InputPath: inputPath,
		Container: container,
		Status:    SessionStatusPending,
		StartTime: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.sessions[id] = session
	m.logger.Info("session created",
		"session_id", id,
		"input_path", inputPath,
		"container", container,
	)

	return session, nil
}

// GetSession retrieves a session by ID
func (m *sessionManager) GetSession(id string) (*Session, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	session, exists := m.sessions[id]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	return session, nil
}

// UpdateSession updates a session
func (m *sessionManager) UpdateSession(id string, update func(*Session) error) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	session, exists := m.sessions[id]
	if !exists {
		return fmt.Errorf("session not found: %s", id)
	}

	if err := update(session); err != nil {
		return err
	}

	session.UpdatedAt = time.Now()
	return nil
}

// RemoveSession removes a session
func (m *sessionManager) RemoveSession(id string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	session, exists := m.sessions[id]
	if !exists {
		return fmt.Errorf("session not found: %s", id)
	}

	// Cancel the context if it exists
	if session.Cancel != nil {
		session.Cancel()
	}

	delete(m.sessions, id)
	m.logger.Info("session removed", "session_id", id)

	return nil
}

// ListActiveSessions returns all active sessions
func (m *sessionManager) ListActiveSessions() ([]*Session, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var activeSessions []*Session
	for _, session := range m.sessions {
		if session.Status == SessionStatusRunning ||
			session.Status == SessionStatusStarting ||
			session.Status == SessionStatusPending {
			activeSessions = append(activeSessions, session)
		}
	}

	return activeSessions, nil
}

// ListAllSessions returns all sessions
func (m *sessionManager) ListAllSessions() ([]*Session, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}

	return sessions, nil
}

// CleanupStaleSessions removes stale sessions older than maxAge
func (m *sessionManager) CleanupStaleSessions(maxAge time.Duration) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	cutoffTime := time.Now().Add(-maxAge)
	var toRemove []string

	for id, session := range m.sessions {
		// Remove sessions that are completed/failed/cancelled and older than maxAge
		if (session.Status == SessionStatusCompleted ||
			session.Status == SessionStatusFailed ||
			session.Status == SessionStatusCancelled) &&
			session.UpdatedAt.Before(cutoffTime) {
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		session := m.sessions[id]
		if session.Cancel != nil {
			session.Cancel()
		}
		delete(m.sessions, id)
		m.logger.Debug("removed stale session",
			"session_id", id,
			"status", session.Status,
			"age", time.Since(session.UpdatedAt),
		)
	}

	if len(toRemove) > 0 {
		m.logger.Info("cleaned up stale sessions", "count", len(toRemove))
	}

	return nil
}
