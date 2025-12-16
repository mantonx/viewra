package plugins

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/events/bus"
	infraplugins "github.com/mantonx/viewra/internal/infrastructure/plugins"
)

// PluginQueries defines the database operations needed by the service.
// Implemented by both sqlc_sqlite.Queries and sqlc_postgres.Queries.
type PluginQueries interface {
	ListPlugins(ctx context.Context) ([]Plugin, error)
	GetPlugin(ctx context.Context, id string) (Plugin, error)
	EnablePlugin(ctx context.Context, id string) error
	DisablePlugin(ctx context.Context, id string) error
	UpsertPlugin(ctx context.Context, p Plugin) error
}

// Plugin represents a plugin record from the database.
// Uses native Go types - the repository handles conversion from sql.Null* types.
type Plugin struct {
	ID            string
	Name          string
	Version       string
	Description   string
	Author        string
	License       string
	Homepage      string
	Categories    string
	IsBuiltin     bool
	Enabled       bool
	Path          string
	HealthStatus  string
	LastHeartbeat time.Time
	RestartCount  int
	InstalledAt   time.Time
	UpdatedAt     time.Time
}

// Service provides plugin management operations.
type Service struct {
	queries PluginQueries
	manager *infraplugins.Manager
	bus     *bus.Bus
	logger  *slog.Logger
}

// NewService creates a new plugin service.
func NewService(queries PluginQueries, manager *infraplugins.Manager, bus *bus.Bus, logger *slog.Logger) *Service {
	return &Service{
		queries: queries,
		manager: manager,
		bus:     bus,
		logger:  logger,
	}
}

// List returns all plugins.
func (s *Service) List(ctx context.Context) ([]PluginSummary, error) {
	rows, err := s.queries.ListPlugins(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]PluginSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, s.toSummary(row))
	}

	return result, nil
}

// Get returns detailed information about a plugin.
func (s *Service) Get(ctx context.Context, id string) (*PluginDetail, error) {
	row, err := s.queries.GetPlugin(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrPluginNotFound{PluginID: id}
		}
		return nil, err
	}

	detail := s.toDetail(row)

	// Enrich with runtime info if plugin is loaded
	if instance, ok := s.manager.GetPlugin(id); ok {
		detail.HealthMessage = instance.Health.Message
		detail.LastHeartbeat = instance.Health.LastHeartbeat
		detail.Capabilities = s.getEnricherCapabilities(instance)
	}

	return detail, nil
}

// GetSettings returns the settings schema and current values for a plugin.
func (s *Service) GetSettings(ctx context.Context, id string) (*PluginSettings, error) {
	// Verify plugin exists
	_, err := s.queries.GetPlugin(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrPluginNotFound{PluginID: id}
		}
		return nil, err
	}

	instance, ok := s.manager.GetPlugin(id)
	if !ok {
		// Plugin exists in DB but not loaded - return empty settings
		return &PluginSettings{
			PluginID: id,
			Schema:   json.RawMessage("{}"),
			Values:   json.RawMessage("{}"),
		}, nil
	}

	// Get settings schema via gRPC
	schema := json.RawMessage("{}")
	if instance.CoreClient != nil {
		schemaResp, err := instance.CoreClient.GetSettingsSchema(ctx, &pluginv1.Empty{})
		if err != nil {
			s.logger.Warn("failed to get settings schema", "plugin", id, "error", err)
		} else if schemaResp != nil {
			schema = schemaResp.JsonSchema
		}
	}

	// Settings values would come from the plugin's config
	// For now, return empty - this can be enhanced when we add settings storage
	values := json.RawMessage("{}")

	return &PluginSettings{
		PluginID: id,
		Schema:   schema,
		Values:   values,
	}, nil
}

// UpdateSettings updates a plugin's settings.
func (s *Service) UpdateSettings(ctx context.Context, id string, values json.RawMessage) error {
	// Verify plugin exists
	_, err := s.queries.GetPlugin(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrPluginNotFound{PluginID: id}
		}
		return err
	}

	instance, ok := s.manager.GetPlugin(id)
	if !ok {
		return ErrPluginNotFound{PluginID: id}
	}

	if instance.CoreClient == nil {
		return ErrPluginNotFound{PluginID: id}
	}

	// Send settings to plugin via gRPC
	resp, err := instance.CoreClient.Configure(ctx, &pluginv1.Settings{
		Json: values,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return &ConfigureError{Message: resp.Error}
	}

	return nil
}

// Enable enables a plugin.
func (s *Service) Enable(ctx context.Context, id string) error {
	row, err := s.queries.GetPlugin(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrPluginNotFound{PluginID: id}
		}
		return err
	}

	// Already enabled
	if row.Enabled {
		return nil
	}

	if err := s.queries.EnablePlugin(ctx, id); err != nil {
		return err
	}

	// Load the plugin if it has a path
	if row.Path != "" {
		if _, err := s.manager.LoadPlugin(ctx, row.Path); err != nil {
			s.logger.Error("failed to load plugin after enabling", "plugin", id, "error", err)
			// Don't fail - plugin is enabled in DB, will load on next restart
		}
	}

	return nil
}

// Disable disables a plugin.
func (s *Service) Disable(ctx context.Context, id string) error {
	row, err := s.queries.GetPlugin(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrPluginNotFound{PluginID: id}
		}
		return err
	}

	// Cannot disable built-in plugins
	if row.IsBuiltin {
		return ErrBuiltinPlugin{PluginID: id}
	}

	// Already disabled
	if !row.Enabled {
		return nil
	}

	if err := s.queries.DisablePlugin(ctx, id); err != nil {
		return err
	}

	// Unload the plugin
	if err := s.manager.UnloadPlugin(ctx, id); err != nil {
		s.logger.Warn("failed to unload plugin after disabling", "plugin", id, "error", err)
		// Don't fail - plugin is disabled in DB
	}

	return nil
}

// Restart restarts a plugin process.
func (s *Service) Restart(ctx context.Context, id string) error {
	row, err := s.queries.GetPlugin(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrPluginNotFound{PluginID: id}
		}
		return err
	}

	if row.Path == "" {
		return ErrPluginNotFound{PluginID: id}
	}

	// Unload then reload
	if err := s.manager.UnloadPlugin(ctx, id); err != nil {
		s.logger.Warn("failed to unload plugin during restart", "plugin", id, "error", err)
	}

	_, err = s.manager.LoadPlugin(ctx, row.Path)
	return err
}

// GetHealth returns detailed health information for a plugin.
func (s *Service) GetHealth(ctx context.Context, id string) (*PluginHealthDetail, error) {
	row, err := s.queries.GetPlugin(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrPluginNotFound{PluginID: id}
		}
		return nil, err
	}

	detail := &PluginHealthDetail{
		Status:   "unknown",
		Restarts: row.RestartCount,
	}

	if row.HealthStatus != "" {
		detail.Status = row.HealthStatus
	}

	if !row.LastHeartbeat.IsZero() {
		detail.LastHeartbeat = row.LastHeartbeat
		detail.UptimeSeconds = int64(time.Since(row.LastHeartbeat).Seconds())
	}

	// Enrich with runtime info if available
	if instance, ok := s.manager.GetPlugin(id); ok {
		detail.Status = instance.Health.Status.String()
		detail.Message = instance.Health.Message
		detail.LastHeartbeat = instance.Health.LastHeartbeat
		detail.ErrorRate = instance.Health.ErrorRate
		detail.AvgLatencyMs = instance.Health.AvgLatency.Milliseconds()
		detail.Restarts = instance.Health.Restarts
	}

	return detail, nil
}

// GetLogs returns recent log entries for a plugin.
func (s *Service) GetLogs(ctx context.Context, id string, opts LogOptions) ([]LogEntry, error) {
	// Verify plugin exists
	_, err := s.queries.GetPlugin(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrPluginNotFound{PluginID: id}
		}
		return nil, err
	}

	if opts.Limit <= 0 {
		opts.Limit = 100
	}

	// Get recent events from bus and filter by plugin
	allEvents := s.bus.Recent(opts.Limit * 10) // Get more to account for filtering

	entries := make([]LogEntry, 0)
	for _, e := range allEvents {
		// Filter by plugin source
		source, ok := e.Data["source"].(string)
		if !ok || source != id {
			continue
		}

		// Filter by level
		level, _ := e.Data["level"].(string)
		if opts.Level != "" && !strings.EqualFold(level, opts.Level) {
			continue
		}

		// Filter by time
		if !opts.Since.IsZero() && e.Timestamp.Before(opts.Since) {
			continue
		}

		msg, _ := e.Data["msg"].(string)
		entry := LogEntry{
			Timestamp: e.Timestamp,
			Level:     level,
			Message:   msg,
			Fields:    make(map[string]string),
		}

		// Copy additional fields
		for k, v := range e.Data {
			if k != "source" && k != "level" && k != "msg" {
				if str, ok := v.(string); ok {
					entry.Fields[k] = str
				}
			}
		}

		entries = append(entries, entry)
		if len(entries) >= opts.Limit {
			break
		}
	}

	return entries, nil
}

// toSummary converts a database row to a PluginSummary.
func (s *Service) toSummary(row Plugin) PluginSummary {
	summary := PluginSummary{
		ID:          row.ID,
		Name:        row.Name,
		Version:     row.Version,
		Description: row.Description,
		Author:      row.Author,
		Categories:  parseCategories(row.Categories),
		Enabled:     row.Enabled,
		IsBuiltin:   row.IsBuiltin,
		Health:      "unknown",
	}

	if row.HealthStatus != "" {
		summary.Health = row.HealthStatus
	}

	// Get runtime health if plugin is loaded
	if instance, ok := s.manager.GetPlugin(row.ID); ok {
		summary.Health = instance.Health.Status.String()
	}

	return summary
}

// toDetail converts a database row to a PluginDetail.
func (s *Service) toDetail(row Plugin) *PluginDetail {
	summary := s.toSummary(row)

	detail := &PluginDetail{
		PluginSummary: summary,
		License:       row.License,
		Homepage:      row.Homepage,
		RestartCount:  row.RestartCount,
		InstalledAt:   row.InstalledAt,
		UpdatedAt:     row.UpdatedAt,
		LastHeartbeat: row.LastHeartbeat,
	}

	return detail
}

// getEnricherCapabilities extracts enricher capabilities from a plugin instance.
func (s *Service) getEnricherCapabilities(instance *infraplugins.PluginInstance) *EnricherCapabilities {
	if instance.EnricherClient == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	caps, err := instance.EnricherClient.GetCapabilities(ctx, &pluginv1.Empty{})
	if err != nil {
		s.logger.Warn("failed to get enricher capabilities", "plugin", instance.ID, "error", err)
		return nil
	}

	return &EnricherCapabilities{
		MediaTypes: caps.MediaTypes,
		Provides:   caps.Provides,
		IsLocal:    caps.IsLocal,
		RateLimit:  int(caps.RateLimit),
		Requires:   caps.Requires,
	}
}

// parseCategories splits a comma-separated categories string.
func parseCategories(cats string) []string {
	if cats == "" {
		return nil
	}
	parts := strings.Split(cats, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// RegisterPlugin persists an external plugin to the database.
// This is called when external plugins are loaded at startup.
func (s *Service) RegisterPlugin(ctx context.Context, p Plugin) error {
	return s.queries.UpsertPlugin(ctx, p)
}

// ConfigureError is returned when plugin configuration fails.
type ConfigureError struct {
	Message string
}

func (e *ConfigureError) Error() string {
	return "plugin configuration failed: " + e.Message
}
