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

// UserRepository implements the user.UserRepository interface.
type UserRepository struct {
	db              *sql.DB
	dbType          string
	sqliteQuerier   sqlc_sqlite.Querier
	postgresQuerier sqlc_postgres.Querier
}

// NewUserRepository creates a new user repository.
func NewUserRepository(db *sql.DB, dbType string) *UserRepository {
	r := &UserRepository{
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

// Create creates a new user. Sets the ID from the database after insert.
func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	if err := u.Validate(); err != nil {
		return err
	}

	if r.dbType == "sqlite" {
		row, err := r.sqliteQuerier.CreateUser(ctx, sqlc_sqlite.CreateUserParams{
			PublicID:     u.PublicID,
			Username:     u.Username,
			DisplayName:  u.DisplayName,
			PasswordHash: u.PasswordHash,
			IsAdmin:      boolToInt64(u.IsAdmin),
			IsDisabled:   boolToInt64(u.IsDisabled),
			CreatedAt:    u.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    u.UpdatedAt.Format(time.RFC3339),
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

	// PostgreSQL
	row, err := r.postgresQuerier.CreateUser(ctx, sqlc_postgres.CreateUserParams{
		PublicID:     u.PublicID,
		Username:     u.Username,
		DisplayName:  u.DisplayName,
		PasswordHash: u.PasswordHash,
		IsAdmin:      u.IsAdmin,
		IsDisabled:   u.IsDisabled,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	})
	if err != nil {
		if common.IsUniqueConstraintError(err) {
			return user.ErrUsernameExists
		}
		return err
	}
	u.ID = int64(row.ID)
	return nil
}

// GetByID retrieves a user by internal ID.
func (r *UserRepository) GetByID(ctx context.Context, id int64) (*user.User, error) {
	if r.dbType == "sqlite" {
		row, err := r.sqliteQuerier.GetUserByID(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, user.ErrUserNotFound
			}
			return nil, err
		}
		return sqliteRowToUser(row), nil
	}

	// PostgreSQL
	row, err := r.postgresQuerier.GetUserByID(ctx, int32(id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, user.ErrUserNotFound
		}
		return nil, err
	}
	return postgresRowToUser(row), nil
}

// GetByPublicID retrieves a user by public ID (e.g., "usr_abc123").
func (r *UserRepository) GetByPublicID(ctx context.Context, publicID string) (*user.User, error) {
	if r.dbType == "sqlite" {
		row, err := r.sqliteQuerier.GetUserByPublicID(ctx, publicID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, user.ErrUserNotFound
			}
			return nil, err
		}
		return sqliteRowToUser(row), nil
	}

	// PostgreSQL
	row, err := r.postgresQuerier.GetUserByPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, user.ErrUserNotFound
		}
		return nil, err
	}
	return postgresRowToUser(row), nil
}

// GetByUsername retrieves a user by username.
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*user.User, error) {
	if r.dbType == "sqlite" {
		row, err := r.sqliteQuerier.GetUserByUsername(ctx, username)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, user.ErrUserNotFound
			}
			return nil, err
		}
		return sqliteRowToUser(row), nil
	}

	// PostgreSQL
	row, err := r.postgresQuerier.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, user.ErrUserNotFound
		}
		return nil, err
	}
	return postgresRowToUser(row), nil
}

// Update updates an existing user.
func (r *UserRepository) Update(ctx context.Context, u *user.User) error {
	if r.dbType == "sqlite" {
		_, err := r.sqliteQuerier.UpdateUser(ctx, sqlc_sqlite.UpdateUserParams{
			DisplayName: u.DisplayName,
			IsAdmin:     boolToInt64(u.IsAdmin),
			IsDisabled:  boolToInt64(u.IsDisabled),
			UpdatedAt:   u.UpdatedAt.Format(time.RFC3339),
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

	// PostgreSQL
	_, err := r.postgresQuerier.UpdateUser(ctx, sqlc_postgres.UpdateUserParams{
		DisplayName: u.DisplayName,
		IsAdmin:     u.IsAdmin,
		IsDisabled:  u.IsDisabled,
		UpdatedAt:   u.UpdatedAt,
		ID:          int32(u.ID),
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
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.DeleteUser(ctx, id)
	}
	return r.postgresQuerier.DeleteUser(ctx, int32(id))
}

// List retrieves users with pagination.
func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]*user.User, error) {
	if r.dbType == "sqlite" {
		rows, err := r.sqliteQuerier.ListUsers(ctx, sqlc_sqlite.ListUsersParams{
			Limit:  int64(limit),
			Offset: int64(offset),
		})
		if err != nil {
			return nil, err
		}
		users := make([]*user.User, len(rows))
		for i, row := range rows {
			users[i] = sqliteRowToUser(row)
		}
		return users, nil
	}

	// PostgreSQL
	rows, err := r.postgresQuerier.ListUsers(ctx, sqlc_postgres.ListUsersParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}
	users := make([]*user.User, len(rows))
	for i, row := range rows {
		users[i] = postgresRowToUser(row)
	}
	return users, nil
}

// Count returns the total number of users.
func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.CountUsers(ctx)
	}
	return r.postgresQuerier.CountUsers(ctx)
}

// ExistsAny returns true if any users exist.
func (r *UserRepository) ExistsAny(ctx context.Context) (bool, error) {
	if r.dbType == "sqlite" {
		count, err := r.sqliteQuerier.ExistsAnyUser(ctx)
		if err != nil {
			return false, err
		}
		return count > 0, nil
	}
	return r.postgresQuerier.ExistsAnyUser(ctx)
}

// UpdatePassword updates a user's password hash.
func (r *UserRepository) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	now := time.Now()
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.UpdateUserPassword(ctx, sqlc_sqlite.UpdateUserPasswordParams{
			PasswordHash: passwordHash,
			UpdatedAt:    now.Format(time.RFC3339),
			ID:           id,
		})
	}
	return r.postgresQuerier.UpdateUserPassword(ctx, sqlc_postgres.UpdateUserPasswordParams{
		PasswordHash: passwordHash,
		UpdatedAt:    now,
		ID:           int32(id),
	})
}

// Helper functions

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func int64ToBool(i int64) bool {
	return i != 0
}

func sqliteRowToUser(row sqlc_sqlite.User) *user.User {
	createdAt, _ := time.Parse(time.RFC3339, row.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339, row.UpdatedAt)

	return &user.User{
		ID:           row.ID,
		PublicID:     row.PublicID,
		Username:     row.Username,
		DisplayName:  row.DisplayName,
		PasswordHash: row.PasswordHash,
		IsAdmin:      int64ToBool(row.IsAdmin),
		IsDisabled:   int64ToBool(row.IsDisabled),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

func postgresRowToUser(row sqlc_postgres.User) *user.User {
	return &user.User{
		ID:           int64(row.ID),
		PublicID:     row.PublicID,
		Username:     row.Username,
		DisplayName:  row.DisplayName,
		PasswordHash: row.PasswordHash,
		IsAdmin:      row.IsAdmin,
		IsDisabled:   row.IsDisabled,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}
