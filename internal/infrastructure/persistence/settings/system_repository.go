package settings

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mantonx/viewra/internal/domain/settings"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// SystemRepository implements the settings.SystemRepository interface.
type SystemRepository struct {
	db              *sql.DB
	dbType          string
	sqliteQuerier   sqlc_sqlite.Querier
	postgresQuerier sqlc_postgres.Querier
}

// NewSystemRepository creates a new system settings repository.
func NewSystemRepository(db *sql.DB, dbType string) *SystemRepository {
	r := &SystemRepository{
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

// Get retrieves a system setting by key.
func (r *SystemRepository) Get(ctx context.Context, key string) (*settings.SystemSetting, error) {
	if common.IsSQLite(r.dbType) {
		row, err := r.sqliteQuerier.GetSystemSetting(ctx, key)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, settings.ErrSettingNotFound
			}
			return nil, err
		}
		return sqliteSystemSettingToEntity(row), nil
	}

	// PostgreSQL
	row, err := r.postgresQuerier.GetSystemSetting(ctx, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, settings.ErrSettingNotFound
		}
		return nil, err
	}
	return postgresSystemSettingToEntity(row), nil
}

// GetByCategory retrieves all system settings in a category.
func (r *SystemRepository) GetByCategory(ctx context.Context, category settings.Category) ([]*settings.SystemSetting, error) {
	if common.IsSQLite(r.dbType) {
		rows, err := r.sqliteQuerier.GetSystemSettingsByCategory(ctx, string(category))
		if err != nil {
			return nil, err
		}
		result := make([]*settings.SystemSetting, len(rows))
		for i, row := range rows {
			result[i] = sqliteSystemSettingToEntity(row)
		}
		return result, nil
	}

	// PostgreSQL
	rows, err := r.postgresQuerier.GetSystemSettingsByCategory(ctx, string(category))
	if err != nil {
		return nil, err
	}
	result := make([]*settings.SystemSetting, len(rows))
	for i, row := range rows {
		result[i] = postgresSystemSettingToEntity(row)
	}
	return result, nil
}

// GetAll retrieves all system settings.
func (r *SystemRepository) GetAll(ctx context.Context) ([]*settings.SystemSetting, error) {
	if common.IsSQLite(r.dbType) {
		rows, err := r.sqliteQuerier.GetAllSystemSettings(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]*settings.SystemSetting, len(rows))
		for i, row := range rows {
			result[i] = sqliteSystemSettingToEntity(row)
		}
		return result, nil
	}

	// PostgreSQL
	rows, err := r.postgresQuerier.GetAllSystemSettings(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*settings.SystemSetting, len(rows))
	for i, row := range rows {
		result[i] = postgresSystemSettingToEntity(row)
	}
	return result, nil
}

// Set creates or updates a system setting.
func (r *SystemRepository) Set(ctx context.Context, setting *settings.SystemSetting) error {
	if common.IsSQLite(r.dbType) {
		return r.sqliteQuerier.UpsertSystemSetting(ctx, sqlc_sqlite.UpsertSystemSettingParams{
			Key:         setting.Key,
			Value:       setting.Value,
			ValueType:   string(setting.ValueType),
			Category:    string(setting.Category),
			Description: nullString(setting.Description),
			UpdatedAt:   setting.UpdatedAt,
			UpdatedBy:   nullString(setting.UpdatedBy),
		})
	}

	// PostgreSQL
	return r.postgresQuerier.UpsertSystemSetting(ctx, sqlc_postgres.UpsertSystemSettingParams{
		Key:         setting.Key,
		Value:       setting.Value,
		ValueType:   string(setting.ValueType),
		Category:    string(setting.Category),
		Description: nullStringPg(setting.Description),
		UpdatedAt:   setting.UpdatedAt,
		UpdatedBy:   nullStringPg(setting.UpdatedBy),
	})
}

// Delete removes a system setting.
func (r *SystemRepository) Delete(ctx context.Context, key string) error {
	if common.IsSQLite(r.dbType) {
		return r.sqliteQuerier.DeleteSystemSetting(ctx, key)
	}
	return r.postgresQuerier.DeleteSystemSetting(ctx, key)
}

// Conversion helpers

func sqliteSystemSettingToEntity(row sqlc_sqlite.SystemSetting) *settings.SystemSetting {
	return &settings.SystemSetting{
		Key:         row.Key,
		Value:       row.Value,
		ValueType:   settings.ValueType(row.ValueType),
		Category:    settings.Category(row.Category),
		Description: nullStringValue(row.Description),
		UpdatedAt:   row.UpdatedAt,
		UpdatedBy:   nullStringValue(row.UpdatedBy),
	}
}

func postgresSystemSettingToEntity(row sqlc_postgres.SystemSetting) *settings.SystemSetting {
	return &settings.SystemSetting{
		Key:         row.Key,
		Value:       row.Value,
		ValueType:   settings.ValueType(row.ValueType),
		Category:    settings.Category(row.Category),
		Description: nullStringValuePg(row.Description),
		UpdatedAt:   row.UpdatedAt,
		UpdatedBy:   nullStringValuePg(row.UpdatedBy),
	}
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullStringPg(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullStringValue(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func nullStringValuePg(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}
