package settings

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mantonx/viewra/internal/domain/settings"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// UserRepository implements the settings.UserRepository interface.
type UserRepository struct {
	*common.BaseRepository
}

// NewUserRepository creates a new user settings repository.
func NewUserRepository(db *common.BaseRepository) *UserRepository {
	return &UserRepository{
		BaseRepository: db,
	}
}

// Get retrieves a user setting by user ID and key.
func (r *UserRepository) Get(ctx context.Context, userID, key string) (*settings.UserSetting, error) {
	row, err := r.Q().GetUserSetting(ctx, unified.GetUserSettingParams{
		UserID: userID,
		Key:    key,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, settings.ErrSettingNotFound
		}
		return nil, err
	}
	return userSettingToEntity(row), nil
}

// GetAll retrieves all settings for a user.
func (r *UserRepository) GetAll(ctx context.Context, userID string) ([]*settings.UserSetting, error) {
	rows, err := r.Q().GetAllUserSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make([]*settings.UserSetting, len(rows))
	for i, row := range rows {
		result[i] = userSettingToEntity(row)
	}
	return result, nil
}

// Set creates or updates a user setting.
func (r *UserRepository) Set(ctx context.Context, setting *settings.UserSetting) error {
	return r.Q().UpsertUserSetting(ctx, unified.UpsertUserSettingParams{
		UserID:    setting.UserID,
		Key:       setting.Key,
		Value:     setting.Value,
		UpdatedAt: setting.UpdatedAt,
	})
}

// Delete removes a user setting.
func (r *UserRepository) Delete(ctx context.Context, userID, key string) error {
	return r.Q().DeleteUserSetting(ctx, unified.DeleteUserSettingParams{
		UserID: userID,
		Key:    key,
	})
}

// DeleteAll removes all settings for a user.
func (r *UserRepository) DeleteAll(ctx context.Context, userID string) error {
	return r.Q().DeleteAllUserSettings(ctx, userID)
}

// Conversion helpers

func userSettingToEntity(row unified.UserSetting) *settings.UserSetting {
	return &settings.UserSetting{
		UserID:    row.UserID,
		Key:       row.Key,
		Value:     row.Value,
		UpdatedAt: row.UpdatedAt,
	}
}
