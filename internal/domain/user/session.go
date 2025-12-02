package user

import (
	"time"
)

// Session represents an authenticated session with a refresh token.
type Session struct {
	ID               int64  // Internal database ID
	PublicID         string // External opaque ID (e.g., "sess_abc123")
	UserID           int64  // Foreign key to users.id
	RefreshTokenHash string
	UserAgent        string
	IPAddress        string
	CreatedAt        time.Time
	LastUsedAt       time.Time
	ExpiresAt        time.Time
}

// NewSession creates a new session.
// ID is set to 0 and will be assigned by the database on insert.
func NewSession(publicID string, userID int64, refreshTokenHash, userAgent, ipAddress string, expiresAt time.Time) *Session {
	now := time.Now()
	return &Session{
		ID:               0, // Assigned by database
		PublicID:         publicID,
		UserID:           userID,
		RefreshTokenHash: refreshTokenHash,
		UserAgent:        userAgent,
		IPAddress:        ipAddress,
		CreatedAt:        now,
		LastUsedAt:       now,
		ExpiresAt:        expiresAt,
	}
}

// IsExpired returns true if the session has expired.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// IsValid returns true if the session is not expired.
func (s *Session) IsValid() bool {
	return !s.IsExpired()
}

// Touch updates the last used timestamp.
func (s *Session) Touch() {
	s.LastUsedAt = time.Now()
}

// Validate validates the session entity.
func (s *Session) Validate() error {
	if s.PublicID == "" {
		return ErrSessionIDEmpty
	}
	if s.UserID == 0 {
		return ErrSessionUserIDEmpty
	}
	if s.RefreshTokenHash == "" {
		return ErrRefreshTokenHashEmpty
	}
	if s.ExpiresAt.IsZero() {
		return ErrSessionExpiresAtEmpty
	}
	return nil
}
