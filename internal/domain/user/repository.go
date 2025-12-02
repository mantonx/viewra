package user

import "context"

// UserRepository defines the interface for user persistence.
type UserRepository interface {
	// Create creates a new user.
	// Returns ErrUsernameExists if the username is already taken.
	Create(ctx context.Context, user *User) error

	// GetByID retrieves a user by internal ID.
	// Returns ErrUserNotFound if the user doesn't exist.
	GetByID(ctx context.Context, id int64) (*User, error)

	// GetByPublicID retrieves a user by public ID (e.g., "usr_abc123").
	// Returns ErrUserNotFound if the user doesn't exist.
	GetByPublicID(ctx context.Context, publicID string) (*User, error)

	// GetByUsername retrieves a user by username (case-insensitive).
	// Returns ErrUserNotFound if the user doesn't exist.
	GetByUsername(ctx context.Context, username string) (*User, error)

	// Update updates an existing user.
	// Returns ErrUserNotFound if the user doesn't exist.
	Update(ctx context.Context, user *User) error

	// Delete deletes a user by internal ID.
	// Returns ErrUserNotFound if the user doesn't exist.
	Delete(ctx context.Context, id int64) error

	// List retrieves users with pagination.
	List(ctx context.Context, limit, offset int) ([]*User, error)

	// Count returns the total number of users.
	Count(ctx context.Context) (int64, error)

	// ExistsAny returns true if any users exist in the database.
	ExistsAny(ctx context.Context) (bool, error)

	// UpdatePassword updates a user's password hash.
	UpdatePassword(ctx context.Context, id int64, passwordHash string) error
}

// SessionRepository defines the interface for session persistence.
type SessionRepository interface {
	// Create creates a new session.
	Create(ctx context.Context, session *Session) error

	// GetByID retrieves a session by internal ID.
	// Returns ErrSessionNotFound if the session doesn't exist.
	GetByID(ctx context.Context, id int64) (*Session, error)

	// GetByPublicID retrieves a session by public ID (e.g., "sess_abc123").
	// Returns ErrSessionNotFound if the session doesn't exist.
	GetByPublicID(ctx context.Context, publicID string) (*Session, error)

	// GetByTokenHash retrieves a session by refresh token hash.
	// Returns ErrSessionNotFound if the session doesn't exist.
	GetByTokenHash(ctx context.Context, hash string) (*Session, error)

	// GetByUserID retrieves all sessions for a user by internal user ID.
	GetByUserID(ctx context.Context, userID int64) ([]*Session, error)

	// UpdateLastUsed updates the last_used_at timestamp by internal ID.
	UpdateLastUsed(ctx context.Context, id int64) error

	// Delete deletes a session by internal ID.
	Delete(ctx context.Context, id int64) error

	// DeleteByUserID deletes all sessions for a user by internal user ID.
	DeleteByUserID(ctx context.Context, userID int64) error

	// DeleteExpired deletes all expired sessions.
	// Returns the number of deleted sessions.
	DeleteExpired(ctx context.Context) (int64, error)

	// CountByUserID returns the number of active sessions for a user.
	CountByUserID(ctx context.Context, userID int64) (int64, error)
}
