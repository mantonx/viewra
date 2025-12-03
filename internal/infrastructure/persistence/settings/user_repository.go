package settings

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/mantonx/viewra/internal/domain/settings"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// UserRepository implements the settings.UserRepository interface.
type UserRepository struct {
	db              *sql.DB
	dbType          string
	sqliteQuerier   sqlc_sqlite.Querier
	postgresQuerier sqlc_postgres.Querier
}

// NewUserRepository creates a new user settings repository.
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

// Get retrieves a user setting by user ID and key.
func (r *UserRepository) Get(ctx context.Context, userID, key string) (*settings.UserSetting, error) {
	if r.dbType == "sqlite" {
		row, err := r.sqliteQuerier.GetUserSetting(ctx, sqlc_sqlite.GetUserSettingParams{
			UserID: userID,
			Key:    key,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, settings.ErrSettingNotFound
			}
			return nil, err
		}
		return sqliteUserSettingToEntity(row), nil
	}

	// PostgreSQL
	row, err := r.postgresQuerier.GetUserSetting(ctx, sqlc_postgres.GetUserSettingParams{
		UserID: userID,
		Key:    key,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, settings.ErrSettingNotFound
		}
		return nil, err
	}
	return postgresUserSettingToEntity(row), nil
}

// GetAll retrieves all settings for a user.
func (r *UserRepository) GetAll(ctx context.Context, userID string) ([]*settings.UserSetting, error) {
	if r.dbType == "sqlite" {
		rows, err := r.sqliteQuerier.GetAllUserSettings(ctx, userID)
		if err != nil {
			return nil, err
		}
		result := make([]*settings.UserSetting, len(rows))
		for i, row := range rows {
			result[i] = sqliteUserSettingToEntity(row)
		}
		return result, nil
	}

	// PostgreSQL
	rows, err := r.postgresQuerier.GetAllUserSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]*settings.UserSetting, len(rows))
	for i, row := range rows {
		result[i] = postgresUserSettingToEntity(row)
	}
	return result, nil
}

// Set creates or updates a user setting.
func (r *UserRepository) Set(ctx context.Context, setting *settings.UserSetting) error {
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.UpsertUserSetting(ctx, sqlc_sqlite.UpsertUserSettingParams{
			UserID:    setting.UserID,
			Key:       setting.Key,
			Value:     setting.Value,
			UpdatedAt: setting.UpdatedAt.Format(time.RFC3339),
		})
	}

	// PostgreSQL
	return r.postgresQuerier.UpsertUserSetting(ctx, sqlc_postgres.UpsertUserSettingParams{
		UserID:    setting.UserID,
		Key:       setting.Key,
		Value:     setting.Value,
		UpdatedAt: setting.UpdatedAt,
	})
}

// Delete removes a user setting.
func (r *UserRepository) Delete(ctx context.Context, userID, key string) error {
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.DeleteUserSetting(ctx, sqlc_sqlite.DeleteUserSettingParams{
			UserID: userID,
			Key:    key,
		})
	}

	// PostgreSQL
	return r.postgresQuerier.DeleteUserSetting(ctx, sqlc_postgres.DeleteUserSettingParams{
		UserID: userID,
		Key:    key,
	})
}

// DeleteAll removes all settings for a user.
func (r *UserRepository) DeleteAll(ctx context.Context, userID string) error {
	if r.dbType == "sqlite" {
		return r.sqliteQuerier.DeleteAllUserSettings(ctx, userID)
	}
	return r.postgresQuerier.DeleteAllUserSettings(ctx, userID)
}

// Conversion helpers

func sqliteUserSettingToEntity(row sqlc_sqlite.UserSetting) *settings.UserSetting {
	updatedAt, _ := time.Parse(time.RFC3339, row.UpdatedAt)
	return &settings.UserSetting{
		UserID:    row.UserID,
		Key:       row.Key,
		Value:     row.Value,
		UpdatedAt: updatedAt,
	}
}

func postgresUserSettingToEntity(row sqlc_postgres.UserSetting) *settings.UserSetting {
	return &settings.UserSetting{
		UserID:    row.UserID,
		Key:       row.Key,
		Value:     row.Value,
		UpdatedAt: row.UpdatedAt,
	}
}
