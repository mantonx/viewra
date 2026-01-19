// Package plugins provides persistence for plugin management.
package plugins

import (
	"context"
	"database/sql"

	appplugins "github.com/mantonx/viewra/internal/application/plugins"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements appplugins.PluginQueries using the unified Querier pattern.
type Repository struct {
	*common.BaseRepository
}

// NewRepository creates a new plugin repository.
func NewRepository(base *common.BaseRepository) *Repository {
	return &Repository{
		BaseRepository: base,
	}
}

// ListPlugins returns all plugins.
func (r *Repository) ListPlugins(ctx context.Context) ([]appplugins.Plugin, error) {
	rows, err := r.Q().ListPlugins(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]appplugins.Plugin, len(rows))
	for i, row := range rows {
		result[i] = listPluginsRowToApp(row)
	}
	return result, nil
}

// GetPlugin returns a plugin by ID.
func (r *Repository) GetPlugin(ctx context.Context, id string) (appplugins.Plugin, error) {
	row, err := r.Q().GetPlugin(ctx, id)
	if err != nil {
		return appplugins.Plugin{}, err
	}
	return getPluginRowToApp(row), nil
}

// EnablePlugin enables a plugin.
func (r *Repository) EnablePlugin(ctx context.Context, id string) error {
	return r.Q().EnablePlugin(ctx, id)
}

// DisablePlugin disables a plugin.
func (r *Repository) DisablePlugin(ctx context.Context, id string) error {
	return r.Q().DisablePlugin(ctx, id)
}

// UpsertPlugin creates or updates a plugin record.
func (r *Repository) UpsertPlugin(ctx context.Context, p appplugins.Plugin) error {
	return r.Q().UpsertPlugin(ctx, unified.UpsertPluginParams{
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
	settings, err := r.Q().GetPluginSettings(ctx, id)
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
	return r.Q().UpdatePluginSettings(ctx, unified.UpdatePluginSettingsParams{
		Settings: sql.NullString{String: settings, Valid: true},
		ID:       id,
	})
}

// UpdatePluginSettingsSchema updates the settings schema for a plugin.
func (r *Repository) UpdatePluginSettingsSchema(ctx context.Context, id string, schema string) error {
	return r.Q().UpdatePluginSettingsSchema(ctx, unified.UpdatePluginSettingsSchemaParams{
		SettingsSchema: sql.NullString{String: schema, Valid: true},
		ID:             id,
	})
}

// DeletePlugin removes a plugin from the database.
// Built-in plugins cannot be deleted (enforced by the SQL query).
func (r *Repository) DeletePlugin(ctx context.Context, id string) error {
	return r.Q().DeletePlugin(ctx, id)
}

// UpdatePluginMarketplaceInfo updates marketplace-related fields for a plugin.
func (r *Repository) UpdatePluginMarketplaceInfo(ctx context.Context, id, source, sourceURL, checksum string) error {
	return r.Q().UpdatePluginMarketplaceInfo(ctx, unified.UpdatePluginMarketplaceInfoParams{
		Source:    source,
		SourceUrl: sql.NullString{String: sourceURL, Valid: sourceURL != ""},
		Checksum:  sql.NullString{String: checksum, Valid: checksum != ""},
		ID:        id,
	})
}

// getPluginRowToApp converts a unified GetPluginRow to the application type.
func getPluginRowToApp(row unified.GetPluginRow) appplugins.Plugin {
	p := appplugins.Plugin{
		ID:          row.ID,
		Name:        row.Name,
		Version:     row.Version,
		Categories:  row.Categories,
		IsBuiltin:   common.NullInt64ToBool(row.IsBuiltin),
		Enabled:     common.NullInt64ToBool(row.Enabled),
		InstalledAt: row.InstalledAt,
		UpdatedAt:   row.UpdatedAt,
	}

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
	if row.RestartCount.Valid {
		p.RestartCount = int(row.RestartCount.Int64)
	}
	if row.LastHeartbeat.Valid {
		p.LastHeartbeat = row.LastHeartbeat.Time
	}

	return p
}

// listPluginsRowToApp converts a unified ListPluginsRow to the application type.
func listPluginsRowToApp(row unified.ListPluginsRow) appplugins.Plugin {
	p := appplugins.Plugin{
		ID:          row.ID,
		Name:        row.Name,
		Version:     row.Version,
		Categories:  row.Categories,
		IsBuiltin:   common.NullInt64ToBool(row.IsBuiltin),
		Enabled:     common.NullInt64ToBool(row.Enabled),
		InstalledAt: row.InstalledAt,
		UpdatedAt:   row.UpdatedAt,
	}

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
	if row.RestartCount.Valid {
		p.RestartCount = int(row.RestartCount.Int64)
	}
	if row.LastHeartbeat.Valid {
		p.LastHeartbeat = row.LastHeartbeat.Time
	}

	return p
}
