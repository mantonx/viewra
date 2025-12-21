// Package plugins provides persistence for plugin management.
package plugins

import (
	"context"
	"database/sql"
	"time"

	appplugins "github.com/mantonx/viewra/internal/application/plugins"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements appplugins.PluginQueries for dual-database support.
type Repository struct {
	db              *sql.DB
	dbType          string
	sqliteQuerier   sqlc_sqlite.Querier
	postgresQuerier sqlc_postgres.Querier
}

// NewRepository creates a new plugin repository.
func NewRepository(db *sql.DB, dbType string) *Repository {
	r := &Repository{
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

// ListPlugins returns all plugins.
func (r *Repository) ListPlugins(ctx context.Context) ([]appplugins.Plugin, error) {
	if common.IsPostgres(r.dbType) {
		rows, err := r.postgresQuerier.ListPlugins(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]appplugins.Plugin, len(rows))
		for i, row := range rows {
			result[i] = postgresListPluginsRowToApp(row)
		}
		return result, nil
	}

	rows, err := r.sqliteQuerier.ListPlugins(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]appplugins.Plugin, len(rows))
	for i, row := range rows {
		result[i] = sqliteListPluginsRowToApp(row)
	}
	return result, nil
}

// GetPlugin returns a plugin by ID.
func (r *Repository) GetPlugin(ctx context.Context, id string) (appplugins.Plugin, error) {
	if common.IsPostgres(r.dbType) {
		row, err := r.postgresQuerier.GetPlugin(ctx, id)
		if err != nil {
			return appplugins.Plugin{}, err
		}
		return postgresGetPluginRowToApp(row), nil
	}

	row, err := r.sqliteQuerier.GetPlugin(ctx, id)
	if err != nil {
		return appplugins.Plugin{}, err
	}
	return sqliteGetPluginRowToApp(row), nil
}

// EnablePlugin enables a plugin.
func (r *Repository) EnablePlugin(ctx context.Context, id string) error {
	if common.IsPostgres(r.dbType) {
		return r.postgresQuerier.EnablePlugin(ctx, id)
	}
	return r.sqliteQuerier.EnablePlugin(ctx, id)
}

// DisablePlugin disables a plugin.
func (r *Repository) DisablePlugin(ctx context.Context, id string) error {
	if common.IsPostgres(r.dbType) {
		return r.postgresQuerier.DisablePlugin(ctx, id)
	}
	return r.sqliteQuerier.DisablePlugin(ctx, id)
}

// UpsertPlugin creates or updates a plugin record.
func (r *Repository) UpsertPlugin(ctx context.Context, p appplugins.Plugin) error {
	if common.IsPostgres(r.dbType) {
		return r.postgresQuerier.UpsertPlugin(ctx, sqlc_postgres.UpsertPluginParams{
			ID:          p.ID,
			Name:        p.Name,
			Version:     p.Version,
			Description: sql.NullString{String: p.Description, Valid: p.Description != ""},
			Author:      sql.NullString{String: p.Author, Valid: p.Author != ""},
			License:     sql.NullString{String: p.License, Valid: p.License != ""},
			Homepage:    sql.NullString{String: p.Homepage, Valid: p.Homepage != ""},
			Categories:  []byte(p.Categories),
			Path:        sql.NullString{String: p.Path, Valid: p.Path != ""},
		})
	}
	return r.sqliteQuerier.UpsertPlugin(ctx, sqlc_sqlite.UpsertPluginParams{
		ID:          p.ID,
		Name:        p.Name,
		Version:     p.Version,
		Description: sql.NullString{String: p.Description, Valid: p.Description != ""},
		Author:      sql.NullString{String: p.Author, Valid: p.Author != ""},
		License:     sql.NullString{String: p.License, Valid: p.License != ""},
		Homepage:    sql.NullString{String: p.Homepage, Valid: p.Homepage != ""},
		Categories:  p.Categories,
		Path:        sql.NullString{String: p.Path, Valid: p.Path != ""},
	})
}

// GetPluginSettings returns the settings JSON for a plugin.
func (r *Repository) GetPluginSettings(ctx context.Context, id string) (string, error) {
	if common.IsPostgres(r.dbType) {
		settings, err := r.postgresQuerier.GetPluginSettings(ctx, id)
		if err != nil {
			return "", err
		}
		if settings.Valid {
			return settings.String, nil
		}
		return "{}", nil
	}

	settings, err := r.sqliteQuerier.GetPluginSettings(ctx, id)
	if err != nil {
		return "", err
	}
	if settings.Valid {
		return settings.String, nil
	}
	return "{}", nil
}

// UpdatePluginSettings updates the settings JSON for a plugin.
func (r *Repository) UpdatePluginSettings(ctx context.Context, id string, settings string) error {
	if common.IsPostgres(r.dbType) {
		return r.postgresQuerier.UpdatePluginSettings(ctx, sqlc_postgres.UpdatePluginSettingsParams{
			Settings: sql.NullString{String: settings, Valid: true},
			ID:       id,
		})
	}
	return r.sqliteQuerier.UpdatePluginSettings(ctx, sqlc_sqlite.UpdatePluginSettingsParams{
		Settings: sql.NullString{String: settings, Valid: true},
		ID:       id,
	})
}

// UpdatePluginSettingsSchema updates the settings schema for a plugin.
func (r *Repository) UpdatePluginSettingsSchema(ctx context.Context, id string, schema string) error {
	if common.IsPostgres(r.dbType) {
		return r.postgresQuerier.UpdatePluginSettingsSchema(ctx, sqlc_postgres.UpdatePluginSettingsSchemaParams{
			SettingsSchema: sql.NullString{String: schema, Valid: true},
			ID:             id,
		})
	}
	return r.sqliteQuerier.UpdatePluginSettingsSchema(ctx, sqlc_sqlite.UpdatePluginSettingsSchemaParams{
		SettingsSchema: sql.NullString{String: schema, Valid: true},
		ID:             id,
	})
}

// sqliteGetPluginRowToApp converts a SQLite GetPluginRow to the application type.
func sqliteGetPluginRowToApp(row sqlc_sqlite.GetPluginRow) appplugins.Plugin {
	p := appplugins.Plugin{
		ID:         row.ID,
		Name:       row.Name,
		Version:    row.Version,
		Categories: row.Categories,
		IsBuiltin:  row.IsBuiltin.Valid && row.IsBuiltin.Int64 == 1,
		Enabled:    row.Enabled.Valid && row.Enabled.Int64 == 1,
	}

	// Handle sql.NullString fields
	if row.Description.Valid {
		p.Description = row.Description.String
	}
	if row.Author.Valid {
		p.Author = row.Author.String
	}
	if row.License.Valid {
		p.License = row.License.String
	}
	if row.Homepage.Valid {
		p.Homepage = row.Homepage.String
	}
	if row.Path.Valid {
		p.Path = row.Path.String
	}
	if row.HealthStatus.Valid {
		p.HealthStatus = row.HealthStatus.String
	}
	if row.Settings.Valid {
		p.Settings = row.Settings.String
	}
	if row.SettingsSchema.Valid {
		p.SettingsSchema = row.SettingsSchema.String
	}

	// Handle restart count
	if row.RestartCount.Valid {
		p.RestartCount = int(row.RestartCount.Int64)
	}

	// Handle timestamps (SQLite stores as strings)
	if row.LastHeartbeat.Valid {
		if t, err := time.Parse(time.RFC3339, row.LastHeartbeat.String); err == nil {
			p.LastHeartbeat = t
		}
	}
	if t, err := time.Parse(time.RFC3339, row.InstalledAt); err == nil {
		p.InstalledAt = t
	}
	if t, err := time.Parse(time.RFC3339, row.UpdatedAt); err == nil {
		p.UpdatedAt = t
	}

	return p
}

// sqliteListPluginsRowToApp converts a SQLite ListPluginsRow to the application type.
func sqliteListPluginsRowToApp(row sqlc_sqlite.ListPluginsRow) appplugins.Plugin {
	p := appplugins.Plugin{
		ID:         row.ID,
		Name:       row.Name,
		Version:    row.Version,
		Categories: row.Categories,
		IsBuiltin:  row.IsBuiltin.Valid && row.IsBuiltin.Int64 == 1,
		Enabled:    row.Enabled.Valid && row.Enabled.Int64 == 1,
	}

	// Handle sql.NullString fields
	if row.Description.Valid {
		p.Description = row.Description.String
	}
	if row.Author.Valid {
		p.Author = row.Author.String
	}
	if row.License.Valid {
		p.License = row.License.String
	}
	if row.Homepage.Valid {
		p.Homepage = row.Homepage.String
	}
	if row.Path.Valid {
		p.Path = row.Path.String
	}
	if row.HealthStatus.Valid {
		p.HealthStatus = row.HealthStatus.String
	}
	if row.Settings.Valid {
		p.Settings = row.Settings.String
	}
	if row.SettingsSchema.Valid {
		p.SettingsSchema = row.SettingsSchema.String
	}

	// Handle restart count
	if row.RestartCount.Valid {
		p.RestartCount = int(row.RestartCount.Int64)
	}

	// Handle timestamps (SQLite stores as strings)
	if row.LastHeartbeat.Valid {
		if t, err := time.Parse(time.RFC3339, row.LastHeartbeat.String); err == nil {
			p.LastHeartbeat = t
		}
	}
	if t, err := time.Parse(time.RFC3339, row.InstalledAt); err == nil {
		p.InstalledAt = t
	}
	if t, err := time.Parse(time.RFC3339, row.UpdatedAt); err == nil {
		p.UpdatedAt = t
	}

	return p
}

// postgresGetPluginRowToApp converts a PostgreSQL GetPluginRow to the application type.
func postgresGetPluginRowToApp(row sqlc_postgres.GetPluginRow) appplugins.Plugin {
	p := appplugins.Plugin{
		ID:          row.ID,
		Name:        row.Name,
		Version:     row.Version,
		IsBuiltin:   row.IsBuiltin.Valid && row.IsBuiltin.Bool,
		Enabled:     row.Enabled.Valid && row.Enabled.Bool,
		InstalledAt: row.InstalledAt,
		UpdatedAt:   row.UpdatedAt,
	}

	// Handle sql.NullString fields
	if row.Description.Valid {
		p.Description = row.Description.String
	}
	if row.Author.Valid {
		p.Author = row.Author.String
	}
	if row.License.Valid {
		p.License = row.License.String
	}
	if row.Homepage.Valid {
		p.Homepage = row.Homepage.String
	}
	if row.Path.Valid {
		p.Path = row.Path.String
	}
	if row.HealthStatus.Valid {
		p.HealthStatus = row.HealthStatus.String
	}
	if row.Settings.Valid {
		p.Settings = row.Settings.String
	}
	if row.SettingsSchema.Valid {
		p.SettingsSchema = row.SettingsSchema.String
	}

	// Handle categories (PostgreSQL stores as JSON array)
	if len(row.Categories) > 0 {
		p.Categories = string(row.Categories)
	}

	// Handle restart count
	if row.RestartCount.Valid {
		p.RestartCount = int(row.RestartCount.Int32)
	}

	// Handle last heartbeat
	if row.LastHeartbeat.Valid {
		p.LastHeartbeat = row.LastHeartbeat.Time
	}

	return p
}

// postgresListPluginsRowToApp converts a PostgreSQL ListPluginsRow to the application type.
func postgresListPluginsRowToApp(row sqlc_postgres.ListPluginsRow) appplugins.Plugin {
	p := appplugins.Plugin{
		ID:          row.ID,
		Name:        row.Name,
		Version:     row.Version,
		IsBuiltin:   row.IsBuiltin.Valid && row.IsBuiltin.Bool,
		Enabled:     row.Enabled.Valid && row.Enabled.Bool,
		InstalledAt: row.InstalledAt,
		UpdatedAt:   row.UpdatedAt,
	}

	// Handle sql.NullString fields
	if row.Description.Valid {
		p.Description = row.Description.String
	}
	if row.Author.Valid {
		p.Author = row.Author.String
	}
	if row.License.Valid {
		p.License = row.License.String
	}
	if row.Homepage.Valid {
		p.Homepage = row.Homepage.String
	}
	if row.Path.Valid {
		p.Path = row.Path.String
	}
	if row.HealthStatus.Valid {
		p.HealthStatus = row.HealthStatus.String
	}
	if row.Settings.Valid {
		p.Settings = row.Settings.String
	}
	if row.SettingsSchema.Valid {
		p.SettingsSchema = row.SettingsSchema.String
	}

	// Handle categories (PostgreSQL stores as JSON array)
	if len(row.Categories) > 0 {
		p.Categories = string(row.Categories)
	}

	// Handle restart count
	if row.RestartCount.Valid {
		p.RestartCount = int(row.RestartCount.Int32)
	}

	// Handle last heartbeat
	if row.LastHeartbeat.Valid {
		p.LastHeartbeat = row.LastHeartbeat.Time
	}

	return p
}
