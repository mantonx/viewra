package plugins

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/mantonx/viewra/internal/infrastructure/crypto"
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
	DeletePlugin(ctx context.Context, id string) error
	GetPluginSettings(ctx context.Context, id string) (string, error)
	UpdatePluginSettings(ctx context.Context, id string, settings string) error
	UpdatePluginSettingsSchema(ctx context.Context, id string, schema string) error
}

// Plugin represents a plugin record from the database.
// Uses native Go types - the repository handles conversion from sql.Null* types.
type Plugin struct {
	ID             string
	Name           string
	Version        string
	Description    string
	Author         string
	License        string
	Homepage       string
	Capabilities   string // JSON array of capability strings
	IsBuiltin      bool
	Enabled        bool
	Path           string
	HealthStatus   string
	LastHeartbeat  time.Time
	RestartCount   int
	Settings       string // JSON string of plugin settings
	SettingsSchema string // JSON Schema for plugin settings
	InstalledAt    time.Time
	UpdatedAt      time.Time
}

// Service provides plugin management operations.
type Service struct {
	queries      PluginQueries
	manager      *infraplugins.Manager
	bus          *bus.Bus
	logger       *slog.Logger
	encryptor    *crypto.Encryptor
	schemaParser *SchemaParser
}

// NewService creates a new plugin service.
func NewService(queries PluginQueries, manager *infraplugins.Manager, bus *bus.Bus, logger *slog.Logger) *Service {
	return &Service{
		queries:      queries,
		manager:      manager,
		bus:          bus,
		logger:       logger,
		schemaParser: NewSchemaParser(),
	}
}

// SetEncryptor sets the encryptor for sensitive field encryption.
// If not set, sensitive fields will be stored in plaintext.
func (s *Service) SetEncryptor(encryptor *crypto.Encryptor) {
	s.encryptor = encryptor
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPluginNotFound{PluginID: id}
		}
		return nil, err
	}

	detail := s.toDetail(row)

	// Enrich with runtime info if plugin is loaded
	if instance, ok := s.manager.GetPlugin(id); ok {
		detail.HealthMessage = instance.Health.Message
		detail.LastHeartbeat = instance.Health.LastHeartbeat
		detail.EnricherCapabilities = s.getEnricherCapabilities(instance)
	}

	return detail, nil
}

// Enable enables a plugin.
func (s *Service) Enable(ctx context.Context, id string) error {
	row, err := s.queries.GetPlugin(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
		if errors.Is(err, sql.ErrNoRows) {
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
		if errors.Is(err, sql.ErrNoRows) {
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

	if _, err := s.manager.LoadPlugin(ctx, row.Path); err != nil {
		return err
	}

	// Reapply saved settings after reload
	if row.Settings != "" {
		if err := s.configurePluginFromSettings(ctx, id, row.Settings, row.SettingsSchema); err != nil {
			s.logger.Warn("failed to reapply settings after restart", "plugin", id, "error", err)
			// Don't fail - plugin is running, just not configured
		}
	}

	return nil
}

// GetHealth returns detailed health information for a plugin.
func (s *Service) GetHealth(ctx context.Context, id string) (*PluginHealthDetail, error) {
	row, err := s.queries.GetPlugin(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
		if errors.Is(err, sql.ErrNoRows) {
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

// RegisterPlugin persists an external plugin to the database.
// This is called when external plugins are loaded at startup.
func (s *Service) RegisterPlugin(ctx context.Context, p Plugin) error {
	return s.queries.UpsertPlugin(ctx, p)
}
