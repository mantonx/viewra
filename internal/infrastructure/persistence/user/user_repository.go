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

// UserRepository implements the user.UserRepository interface.
type UserRepository struct {
	*common.BaseRepository
}

// NewUserRepository creates a new user repository.
func NewUserRepository(base *common.BaseRepository) *UserRepository {
	return &UserRepository{
		BaseRepository: base,
	}
}

// Create creates a new user. Sets the ID from the database after insert.
func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	if err := u.Validate(); err != nil {
		return err
	}

	row, err := r.Q().CreateUser(ctx, unified.CreateUserParams{
		PublicID:     u.PublicID,
		Username:     u.Username,
		DisplayName:  u.DisplayName,
		PasswordHash: u.PasswordHash,
		IsAdmin:      common.BoolToInt64(u.IsAdmin),
		IsDisabled:   common.BoolToInt64(u.IsDisabled),
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	})
	if err != nil {
		if common.IsUniqueConstraintError(err) {
			return user.ErrUsernameExists
		}
		return err
	}
	u.ID = row.ID
	return nil
}

// GetByID retrieves a user by internal ID.
func (r *UserRepository) GetByID(ctx context.Context, id int64) (*user.User, error) {
	row, err := r.Q().GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, user.ErrUserNotFound
		}
		return nil, err
	}
	return rowToUser(row), nil
}

// GetPublicIDByInternalID retrieves a user's public ID by their internal database ID.
// This is used for dev mode where we have the internal ID but need the public_id.
func (r *UserRepository) GetPublicIDByInternalID(ctx context.Context, id int64) (string, error) {
	u, err := r.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	return u.PublicID, nil
}

// GetByPublicID retrieves a user by public ID (e.g., "usr_abc123").
func (r *UserRepository) GetByPublicID(ctx context.Context, publicID string) (*user.User, error) {
	row, err := r.Q().GetUserByPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, user.ErrUserNotFound
		}
		return nil, err
	}
	return rowToUser(row), nil
}

// GetByUsername retrieves a user by username.
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*user.User, error) {
	row, err := r.Q().GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, user.ErrUserNotFound
		}
		return nil, err
	}
	return rowToUser(row), nil
}

// Update updates an existing user.
func (r *UserRepository) Update(ctx context.Context, u *user.User) error {
	_, err := r.Q().UpdateUser(ctx, unified.UpdateUserParams{
		DisplayName: u.DisplayName,
		IsAdmin:     common.BoolToInt64(u.IsAdmin),
		IsDisabled:  common.BoolToInt64(u.IsDisabled),
		UpdatedAt:   u.UpdatedAt,
		ID:          u.ID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user.ErrUserNotFound
		}
		return err
	}
	return nil
}

// Delete deletes a user by internal ID.
func (r *UserRepository) Delete(ctx context.Context, id int64) error {
	return r.Q().DeleteUser(ctx, id)
}

// List retrieves users with pagination.
func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]*user.User, error) {
	rows, err := r.Q().ListUsers(ctx, unified.ListUsersParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, err
	}
	users := make([]*user.User, len(rows))
	for i, row := range rows {
		users[i] = rowToUser(row)
	}
	return users, nil
}

// Count returns the total number of users.
func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	return r.Q().CountUsers(ctx)
}

// ExistsAny returns true if any users exist.
func (r *UserRepository) ExistsAny(ctx context.Context) (bool, error) {
	count, err := r.Q().ExistsAnyUser(ctx)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpdatePassword updates a user's password hash.
func (r *UserRepository) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	return r.Q().UpdateUserPassword(ctx, unified.UpdateUserPasswordParams{
		PasswordHash: passwordHash,
		UpdatedAt:    time.Now(),
		ID:           id,
	})
}

// rowToUser converts a unified.User row to a domain user.User
func rowToUser(row unified.User) *user.User {
	return &user.User{
		ID:           row.ID,
		PublicID:     row.PublicID,
		Username:     row.Username,
		DisplayName:  row.DisplayName,
		PasswordHash: row.PasswordHash,
		IsAdmin:      common.Int64ToBool(row.IsAdmin),
		IsDisabled:   common.Int64ToBool(row.IsDisabled),
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}
