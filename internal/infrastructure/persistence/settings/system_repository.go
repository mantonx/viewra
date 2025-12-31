package settings

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mantonx/viewra/internal/domain/settings"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// SystemRepository implements the settings.SystemRepository interface.
type SystemRepository struct {
	*common.BaseRepository
}

// NewSystemRepository creates a new system settings repository.
func NewSystemRepository(db *common.BaseRepository) *SystemRepository {
	return &SystemRepository{
		BaseRepository: db,
	}
}

// Get retrieves a system setting by key.
func (r *SystemRepository) Get(ctx context.Context, key string) (*settings.SystemSetting, error) {
	row, err := r.Q().GetSystemSetting(ctx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, settings.ErrSettingNotFound
		}
		return nil, err
	}
	return systemSettingToEntity(row), nil
}

// GetByCategory retrieves all system settings in a category.
func (r *SystemRepository) GetByCategory(ctx context.Context, category settings.Category) ([]*settings.SystemSetting, error) {
	rows, err := r.Q().GetSystemSettingsByCategory(ctx, string(category))
	if err != nil {
		return nil, err
	}
	result := make([]*settings.SystemSetting, len(rows))
	for i, row := range rows {
		result[i] = systemSettingToEntity(row)
	}
	return result, nil
}

// GetAll retrieves all system settings.
func (r *SystemRepository) GetAll(ctx context.Context) ([]*settings.SystemSetting, error) {
	rows, err := r.Q().GetAllSystemSettings(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*settings.SystemSetting, len(rows))
	for i, row := range rows {
		result[i] = systemSettingToEntity(row)
	}
	return result, nil
}

// Set creates or updates a system setting.
func (r *SystemRepository) Set(ctx context.Context, setting *settings.SystemSetting) error {
	return r.Q().UpsertSystemSetting(ctx, unified.UpsertSystemSettingParams{
		Key:         setting.Key,
		Value:       setting.Value,
		ValueType:   string(setting.ValueType),
		Category:    string(setting.Category),
		Description: common.NullString(setting.Description),
		UpdatedAt:   setting.UpdatedAt,
		UpdatedBy:   common.NullString(setting.UpdatedBy),
	})
}

// Delete removes a system setting.
func (r *SystemRepository) Delete(ctx context.Context, key string) error {
	return r.Q().DeleteSystemSetting(ctx, key)
}

// Conversion helpers

func systemSettingToEntity(row unified.SystemSetting) *settings.SystemSetting {
	return &settings.SystemSetting{
		Key:         row.Key,
		Value:       row.Value,
		ValueType:   settings.ValueType(row.ValueType),
		Category:    settings.Category(row.Category),
		Description: common.ParseNullString(row.Description),
		UpdatedAt:   row.UpdatedAt,
		UpdatedBy:   common.ParseNullString(row.UpdatedBy),
	}
}
