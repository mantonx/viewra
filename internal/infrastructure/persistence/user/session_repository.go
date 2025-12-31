package user

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/mantonx/viewra/internal/domain/user"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// SessionRepository implements the user.SessionRepository interface.
type SessionRepository struct {
	*common.BaseRepository
}

// NewSessionRepository creates a new session repository.
func NewSessionRepository(base *common.BaseRepository) *SessionRepository {
	return &SessionRepository{
		BaseRepository: base,
	}
}

// Create creates a new session.
func (r *SessionRepository) Create(ctx context.Context, s *user.Session) error {
	if err := s.Validate(); err != nil {
		return err
	}

	row, err := r.Q().CreateSession(ctx, unified.CreateSessionParams{
		PublicID:         s.PublicID,
		UserID:           s.UserID,
		RefreshTokenHash: s.RefreshTokenHash,
		UserAgent:        sql.NullString{String: s.UserAgent, Valid: s.UserAgent != ""},
		IpAddress:        sql.NullString{String: s.IPAddress, Valid: s.IPAddress != ""},
		CreatedAt:        s.CreatedAt,
		LastUsedAt:       s.LastUsedAt,
		ExpiresAt:        s.ExpiresAt,
	})
	if err != nil {
		return err
	}
	s.ID = row.ID
	return nil
}

// GetByID retrieves a session by internal ID.
func (r *SessionRepository) GetByID(ctx context.Context, id int64) (*user.Session, error) {
	row, err := r.Q().GetSessionByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, user.ErrSessionNotFound
		}
		return nil, err
	}
	return rowToSession(row), nil
}

// GetByPublicID retrieves a session by public ID (e.g., "sess_abc123").
func (r *SessionRepository) GetByPublicID(ctx context.Context, publicID string) (*user.Session, error) {
	row, err := r.Q().GetSessionByPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, user.ErrSessionNotFound
		}
		return nil, err
	}
	return rowToSession(row), nil
}

// GetByTokenHash retrieves a session by refresh token hash.
func (r *SessionRepository) GetByTokenHash(ctx context.Context, hash string) (*user.Session, error) {
	row, err := r.Q().GetSessionByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, user.ErrSessionNotFound
		}
		return nil, err
	}
	return rowToSession(row), nil
}

// GetByUserID retrieves all sessions for a user by internal user ID.
func (r *SessionRepository) GetByUserID(ctx context.Context, userID int64) ([]*user.Session, error) {
	rows, err := r.Q().GetSessionsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	sessions := make([]*user.Session, len(rows))
	for i, row := range rows {
		sessions[i] = rowToSession(row)
	}
	return sessions, nil
}

// UpdateLastUsed updates the last_used_at timestamp.
func (r *SessionRepository) UpdateLastUsed(ctx context.Context, id int64) error {
	return r.Q().UpdateSessionLastUsed(ctx, unified.UpdateSessionLastUsedParams{
		LastUsedAt: time.Now(),
		ID:         id,
	})
}

// Delete deletes a session by internal ID.
func (r *SessionRepository) Delete(ctx context.Context, id int64) error {
	return r.Q().DeleteSession(ctx, id)
}

// DeleteByUserID deletes all sessions for a user.
func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	return r.Q().DeleteSessionsByUserID(ctx, userID)
}

// DeleteExpired deletes all expired sessions.
func (r *SessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	return r.Q().DeleteExpiredSessions(ctx, time.Now())
}

// CountByUserID returns the number of active sessions for a user.
func (r *SessionRepository) CountByUserID(ctx context.Context, userID int64) (int64, error) {
	return r.Q().CountSessionsByUserID(ctx, userID)
}

// rowToSession converts a unified.Session row to a domain user.Session
func rowToSession(row unified.Session) *user.Session {
	return &user.Session{
		ID:               row.ID,
		PublicID:         row.PublicID,
		UserID:           row.UserID,
		RefreshTokenHash: row.RefreshTokenHash,
		UserAgent:        row.UserAgent.String,
		IPAddress:        row.IpAddress.String,
		CreatedAt:        row.CreatedAt,
		LastUsedAt:       row.LastUsedAt,
		ExpiresAt:        row.ExpiresAt,
	}
}
