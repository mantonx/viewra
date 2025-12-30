package user

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/mantonx/viewra/internal/domain/user"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// SessionRepository implements the user.SessionRepository interface.
type SessionRepository struct {
	db              *sql.DB
	dbType          string
	sqliteQuerier   sqlc_sqlite.Querier
	postgresQuerier sqlc_postgres.Querier
}

// NewSessionRepository creates a new session repository.
func NewSessionRepository(db *sql.DB, dbType string) *SessionRepository {
	r := &SessionRepository{
		db:     db,
		dbType: dbType,
	}

	if common.IsPostgres(dbType) {
		r.postgresQuerier = sqlc_postgres.New(db)
	} else {
		r.sqliteQuerier = sqlc_sqlite.New(db)
	}

	return r
}

// Create creates a new session.
func (r *SessionRepository) Create(ctx context.Context, s *user.Session) error {
	if err := s.Validate(); err != nil {
		return err
	}

	if r.dbType == "sqlite" {
		row, err := r.sqliteQuerier.CreateSession(ctx, sqlc_sqlite.CreateSessionParams{
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

	// PostgreSQL
	row, err := r.postgresQuerier.CreateSession(ctx, sqlc_postgres.CreateSessionParams{
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
	if r.dbType == "sqlite" {
		row, err := r.sqliteQuerier.GetSessionByID(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, user.ErrSessionNotFound
			}
			return nil, err
		}
		return sqliteRowToSession(row), nil
	}

	// PostgreSQL
	row, err := r.postgresQuerier.GetSessionByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, user.ErrSessionNotFound
		}
		return nil, err
	}
	return postgresRowToSession(row), nil
}

// GetByPublicID retrieves a session by public ID (e.g., "sess_abc123").
func (r *SessionRepository) GetByPublicID(ctx context.Context, publicID string) (*user.Session, error) {
	if r.dbType == "sqlite" {
		row, err := r.sqliteQuerier.GetSessionByPublicID(ctx, publicID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, user.ErrSessionNotFound
			}
			return nil, err
		}
		return sqliteRowToSession(row), nil
	}

	// PostgreSQL
	row, err := r.postgresQuerier.GetSessionByPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, user.ErrSessionNotFound
		}
		return nil, err
	}
	return postgresRowToSession(row), nil
}

// GetByTokenHash retrieves a session by refresh token hash.
func (r *SessionRepository) GetByTokenHash(ctx context.Context, hash string) (*user.Session, error) {
	if r.dbType == "sqlite" {
		row, err := r.sqliteQuerier.GetSessionByTokenHash(ctx, hash)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, user.ErrSessionNotFound
			}
			return nil, err
		}
		return sqliteRowToSession(row), nil
	}

	// PostgreSQL
	row, err := r.postgresQuerier.GetSessionByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, user.ErrSessionNotFound
		}
		return nil, err
	}
	return postgresRowToSession(row), nil
}

// GetByUserID retrieves all sessions for a user by internal user ID.
func (r *SessionRepository) GetByUserID(ctx context.Context, userID int64) ([]*user.Session, error) {
	if r.dbType == "sqlite" {
		rows, err := r.sqliteQuerier.GetSessionsByUserID(ctx, userID)
		if err != nil {
			return nil, err
		}
		sessions := make([]*user.Session, len(rows))
		for i, row := range rows {
			sessions[i] = sqliteRowToSession(row)
		}
		return sessions, nil
	}

	// PostgreSQL
	rows, err := r.postgresQuerier.GetSessionsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	sessions := make([]*user.Session, len(rows))
	for i, row := range rows {
		sessions[i] = postgresRowToSession(row)
	}
	return sessions, nil
}

// UpdateLastUsed updates the last_used_at timestamp.
func (r *SessionRepository) UpdateLastUsed(ctx context.Context, id int64) error {
	now := time.Now()
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.UpdateSessionLastUsed(ctx, sqlc_sqlite.UpdateSessionLastUsedParams{
			LastUsedAt: now,
			ID:         id,
		})
	}
	return r.postgresQuerier.UpdateSessionLastUsed(ctx, sqlc_postgres.UpdateSessionLastUsedParams{
		LastUsedAt: now,
		ID:         id,
	})
}

// Delete deletes a session by internal ID.
func (r *SessionRepository) Delete(ctx context.Context, id int64) error {
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.DeleteSession(ctx, id)
	}
	return r.postgresQuerier.DeleteSession(ctx, id)
}

// DeleteByUserID deletes all sessions for a user.
func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.DeleteSessionsByUserID(ctx, userID)
	}
	return r.postgresQuerier.DeleteSessionsByUserID(ctx, userID)
}

// DeleteExpired deletes all expired sessions.
func (r *SessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	now := time.Now()
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.DeleteExpiredSessions(ctx, now)
	}
	return r.postgresQuerier.DeleteExpiredSessions(ctx, now)
}

// CountByUserID returns the number of active sessions for a user.
func (r *SessionRepository) CountByUserID(ctx context.Context, userID int64) (int64, error) {
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.CountSessionsByUserID(ctx, userID)
	}
	return r.postgresQuerier.CountSessionsByUserID(ctx, userID)
}

// Helper functions

func sqliteRowToSession(row sqlc_sqlite.Session) *user.Session {
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

func postgresRowToSession(row sqlc_postgres.Session) *user.Session {
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
