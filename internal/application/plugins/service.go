package plugins

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
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
	Categories     string
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
		detail.Capabilities = s.getEnricherCapabilities(instance)
	}

	return detail, nil
}

// GetSettings returns the settings schema and current values for a plugin.
// Sensitive fields are masked for display (not decrypted).
func (s *Service) GetSettings(ctx context.Context, id string) (*PluginSettings, error) {
	// Verify plugin exists and get plugin data
	plugin, err := s.queries.GetPlugin(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPluginNotFound{PluginID: id}
		}
		return nil, err
	}

	// Get settings values from database
	settingsJSON, err := s.queries.GetPluginSettings(ctx, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	values := json.RawMessage(settingsJSON)
	if len(values) == 0 {
		values = json.RawMessage("{}")
	}

	// Get settings schema - prefer from running plugin, fall back to DB
	var schema json.RawMessage
	if instance, ok := s.manager.GetPlugin(id); ok && instance.CoreClient != nil {
		schemaResp, err := instance.CoreClient.GetSettingsSchema(ctx, &pluginv1.Empty{})
		if err != nil {
			s.logger.Warn("failed to get settings schema from plugin", "plugin", id, "error", err)
		} else if schemaResp != nil && len(schemaResp.JsonSchema) > 0 {
			schema = schemaResp.JsonSchema
			// Cache schema in DB for when plugin isn't running
			if err := s.queries.UpdatePluginSettingsSchema(ctx, id, string(schema)); err != nil {
				s.logger.Warn("failed to cache settings schema", "plugin", id, "error", err)
			}
		}
	}

	// Fall back to cached schema from DB
	if len(schema) == 0 && plugin.SettingsSchema != "" {
		schema = json.RawMessage(plugin.SettingsSchema)
	}
	if len(schema) == 0 {
		schema = json.RawMessage("{}")
	}

	// Mask sensitive fields for display
	maskedValues := s.maskSensitiveFields(values, schema)

	return &PluginSettings{
		PluginID: id,
		Schema:   schema,
		Values:   maskedValues,
	}, nil
}

// UpdateSettings updates a plugin's settings.
func (s *Service) UpdateSettings(ctx context.Context, id string, values json.RawMessage) error {
	// Verify plugin exists
	plugin, err := s.queries.GetPlugin(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrPluginNotFound{PluginID: id}
		}
		return err
	}

	// Get schema for sensitive field handling
	var schema json.RawMessage
	if instance, ok := s.manager.GetPlugin(id); ok && instance.CoreClient != nil {
		schemaResp, err := instance.CoreClient.GetSettingsSchema(ctx, &pluginv1.Empty{})
		if err == nil && schemaResp != nil && len(schemaResp.JsonSchema) > 0 {
			schema = schemaResp.JsonSchema
		}
	}
	if len(schema) == 0 && plugin.SettingsSchema != "" {
		schema = json.RawMessage(plugin.SettingsSchema)
	}

	// Merge with existing values (preserve encrypted values for masked fields)
	mergedValues, err := s.mergeSettings(ctx, id, values, schema)
	if err != nil {
		return err
	}

	// Encrypt sensitive fields before storage
	encryptedValues, err := s.encryptSensitiveFields(mergedValues, schema)
	if err != nil {
		return err
	}

	// If plugin is running, configure it with decrypted values
	if instance, ok := s.manager.GetPlugin(id); ok && instance.CoreClient != nil {
		// Decrypt for the plugin (it needs plaintext values)
		decryptedValues, err := s.decryptSensitiveFields(encryptedValues, schema)
		if err != nil {
			return err
		}
		resp, err := instance.CoreClient.Configure(ctx, &pluginv1.Settings{
			Json: decryptedValues,
		})
		if err != nil {
			return err
		}
		if !resp.Success {
			return &ConfigureError{Message: resp.Error}
		}
	}

	// Persist encrypted settings to database
	if err := s.queries.UpdatePluginSettings(ctx, id, string(encryptedValues)); err != nil {
		return err
	}

	return nil
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

// configurePluginFromSettings sends stored settings to a running plugin.
func (s *Service) configurePluginFromSettings(ctx context.Context, id, settingsJSON, schemaJSON string) error {
	instance, ok := s.manager.GetPlugin(id)
	if !ok || instance.CoreClient == nil {
		return errors.New("plugin not running")
	}

	// Decrypt sensitive fields before sending to plugin
	var schema json.RawMessage
	if schemaJSON != "" {
		schema = json.RawMessage(schemaJSON)
	}

	decryptedValues, err := s.decryptSensitiveFields(json.RawMessage(settingsJSON), schema)
	if err != nil {
		return err
	}

	resp, err := instance.CoreClient.Configure(ctx, &pluginv1.Settings{
		Json: decryptedValues,
	})
	if err != nil {
		return err
	}
	if !resp.Success {
		return &ConfigureError{Message: resp.Error}
	}

	s.logger.Info("reapplied settings after restart", "plugin", id)
	return nil
}

// ApplyStoredSettings applies stored settings to all running plugins.
// This should be called after plugins are loaded to restore their configuration.
func (s *Service) ApplyStoredSettings(ctx context.Context) error {
	plugins := s.manager.GetAllPlugins()
	if len(plugins) == 0 {
		return nil
	}

	s.logger.Info("applying stored settings to plugins", "count", len(plugins))

	var lastErr error
	applied := 0
	for _, instance := range plugins {
		// Get stored settings from database
		row, err := s.queries.GetPlugin(ctx, instance.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue // No stored settings
			}
			s.logger.Warn("failed to get stored settings", "plugin", instance.ID, "error", err)
			lastErr = err
			continue
		}

		// Skip if no settings stored
		if row.Settings == "" || row.Settings == "{}" {
			continue
		}

		// Apply settings to the plugin
		if err := s.configurePluginFromSettings(ctx, instance.ID, row.Settings, row.SettingsSchema); err != nil {
			s.logger.Warn("failed to apply stored settings", "plugin", instance.ID, "error", err)
			lastErr = err
			continue
		}

		applied++
	}

	s.logger.Info("finished applying stored settings", "applied", applied, "total", len(plugins))
	return lastErr
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
		HasSettings: row.SettingsSchema != "" && row.SettingsSchema != "{}",
	}

	if row.HealthStatus != "" {
		summary.Health = row.HealthStatus
	}

	// Get runtime info if plugin is loaded
	if instance, ok := s.manager.GetPlugin(row.ID); ok {
		summary.Health = instance.Health.Status.String()

		// Extract x-viewra-meta from settings schema
		if meta := s.getPluginMeta(instance); meta != nil {
			summary.Meta = meta
		}

		// Check if running plugin has settings schema (even if not cached in DB)
		if !summary.HasSettings && instance.CoreClient != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			schemaResp, err := instance.CoreClient.GetSettingsSchema(ctx, &pluginv1.Empty{})
			cancel()
			if err == nil && schemaResp != nil && len(schemaResp.JsonSchema) > 0 {
				summary.HasSettings = true
				// Cache it for next time
				if cacheErr := s.queries.UpdatePluginSettingsSchema(context.Background(), row.ID, string(schemaResp.JsonSchema)); cacheErr != nil {
					s.logger.Warn("failed to cache settings schema", "plugin", row.ID, "error", cacheErr)
				}
			}
		}

		// Get capabilities from manifest
		if instance.Manifest != nil {
			summary.Capabilities = instance.Manifest.Provides
		}

		// Check for missing dependencies from manifest
		if instance.Manifest != nil && len(instance.Manifest.Requires) > 0 {
			summary.MissingDependencies = s.checkMissingDependencies(instance.Manifest.Requires)
		}
	}

	// Get capabilities from capability registry (for route-based capabilities)
	if capRegistry := s.manager.GetCapabilityRegistry(); capRegistry != nil {
		routeCaps := capRegistry.GetCapabilitiesForPlugin(row.ID)
		if len(routeCaps) > 0 {
			// Merge with manifest capabilities, avoiding duplicates
			for _, cap := range routeCaps {
				if !contains(summary.Capabilities, cap) {
					summary.Capabilities = append(summary.Capabilities, cap)
				}
			}
		}
	}

	// Get provider ID if this is a provider plugin
	if providerRegistry := s.manager.GetProviderRegistry(); providerRegistry != nil {
		summary.ProviderID = providerRegistry.GetByPluginID(row.ID)
	}

	return summary
}

// checkMissingDependencies returns which required capabilities are not currently available.
func (s *Service) checkMissingDependencies(requires []string) []string {
	hostPluginsServer := s.manager.GetHostPluginsServer()
	if hostPluginsServer == nil {
		// If no capability broker, all dependencies are missing
		return requires
	}

	var missing []string
	for _, req := range requires {
		if !hostPluginsServer.HasCapability(req) {
			missing = append(missing, req)
		}
	}
	return missing
}

// contains checks if a string slice contains a value.
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// getPluginMeta extracts x-viewra-meta from a plugin's settings schema.
func (s *Service) getPluginMeta(instance *infraplugins.PluginInstance) map[string]any {
	if instance.CoreClient == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	schemaResp, err := instance.CoreClient.GetSettingsSchema(ctx, &pluginv1.Empty{})
	if err != nil {
		return nil
	}

	if len(schemaResp.JsonSchema) == 0 {
		return nil
	}

	var schema map[string]any
	if err := json.Unmarshal(schemaResp.JsonSchema, &schema); err != nil {
		return nil
	}

	meta, ok := schema["x-viewra-meta"].(map[string]any)
	if !ok {
		return nil
	}

	return meta
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

// parseCategories parses categories from JSON array or comma-separated string.
func parseCategories(cats string) []string {
	if cats == "" {
		return nil
	}

	// Try parsing as JSON array first
	cats = strings.TrimSpace(cats)
	if strings.HasPrefix(cats, "[") {
		var result []string
		if err := json.Unmarshal([]byte(cats), &result); err == nil {
			return result
		}
	}

	// Fall back to comma-separated format
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

// maskSensitiveFields masks sensitive field values for display.
// Returns a new JSON with sensitive fields replaced by a mask.
func (s *Service) maskSensitiveFields(values, schema json.RawMessage) json.RawMessage {
	if len(values) == 0 || len(schema) == 0 {
		return values
	}

	sensitiveFields := s.schemaParser.GetSensitiveFields(schema)
	if len(sensitiveFields) == 0 {
		return values
	}

	var valuesMap map[string]any
	if err := json.Unmarshal(values, &valuesMap); err != nil {
		return values
	}

	for fieldName := range sensitiveFields {
		if val, ok := valuesMap[fieldName].(string); ok && val != "" {
			valuesMap[fieldName] = crypto.MaskAPIKey(val)
		}
	}

	masked, err := json.Marshal(valuesMap)
	if err != nil {
		return values
	}
	return masked
}

// encryptSensitiveFields encrypts sensitive field values before storage.
// Returns a new JSON with sensitive fields encrypted.
func (s *Service) encryptSensitiveFields(values, schema json.RawMessage) (json.RawMessage, error) {
	if s.encryptor == nil || len(values) == 0 || len(schema) == 0 {
		return values, nil
	}

	sensitiveFields := s.schemaParser.GetSensitiveFields(schema)
	if len(sensitiveFields) == 0 {
		return values, nil
	}

	var valuesMap map[string]any
	if err := json.Unmarshal(values, &valuesMap); err != nil {
		return values, nil
	}

	for fieldName := range sensitiveFields {
		if val, ok := valuesMap[fieldName].(string); ok && val != "" {
			// Skip if already encrypted or if it's a masked value
			if crypto.IsEncrypted(val) || isMaskedValue(val) {
				continue
			}
			encrypted, err := s.encryptor.Encrypt(val)
			if err != nil {
				return nil, err
			}
			valuesMap[fieldName] = encrypted
		}
	}

	encrypted, err := json.Marshal(valuesMap)
	if err != nil {
		return nil, err
	}
	return encrypted, nil
}

// decryptSensitiveFields decrypts sensitive field values after retrieval.
// Returns a new JSON with sensitive fields decrypted (for passing to plugins).
func (s *Service) decryptSensitiveFields(values, schema json.RawMessage) (json.RawMessage, error) {
	if s.encryptor == nil || len(values) == 0 || len(schema) == 0 {
		return values, nil
	}

	sensitiveFields := s.schemaParser.GetSensitiveFields(schema)
	if len(sensitiveFields) == 0 {
		return values, nil
	}

	var valuesMap map[string]any
	if err := json.Unmarshal(values, &valuesMap); err != nil {
		return values, nil
	}

	for fieldName := range sensitiveFields {
		if val, ok := valuesMap[fieldName].(string); ok && val != "" {
			if crypto.IsEncrypted(val) {
				decrypted, err := s.encryptor.Decrypt(val)
				if err != nil {
					s.logger.Warn("failed to decrypt sensitive field", "field", fieldName, "error", err)
					continue
				}
				valuesMap[fieldName] = decrypted
			}
		}
	}

	decrypted, err := json.Marshal(valuesMap)
	if err != nil {
		return nil, err
	}
	return decrypted, nil
}

// isMaskedValue returns true if the value is a masked placeholder.
func isMaskedValue(val string) bool {
	return val == "" || val == "••••••••" || val == "••••••••••••" || val == "********"
}

// mergeSettings merges new values with existing values, preserving encrypted values for masked fields.
func (s *Service) mergeSettings(ctx context.Context, pluginID string, newValues, schema json.RawMessage) (json.RawMessage, error) {
	if len(schema) == 0 {
		return newValues, nil
	}

	sensitiveFields := s.schemaParser.GetSensitiveFields(schema)
	if len(sensitiveFields) == 0 {
		return newValues, nil
	}

	// Parse new values
	var newMap map[string]any
	if err := json.Unmarshal(newValues, &newMap); err != nil {
		return newValues, nil
	}

	// Check if any sensitive field has a masked value that needs preserving
	needsExisting := false
	for fieldName := range sensitiveFields {
		if val, ok := newMap[fieldName].(string); ok && isMaskedValue(val) {
			needsExisting = true
			break
		}
	}

	if !needsExisting {
		return newValues, nil
	}

	// Get existing values from database
	existingJSON, err := s.queries.GetPluginSettings(ctx, pluginID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if existingJSON == "" {
		return newValues, nil
	}

	var existingMap map[string]any
	if err := json.Unmarshal([]byte(existingJSON), &existingMap); err != nil {
		return newValues, nil
	}

	// Replace masked values with existing encrypted values
	for fieldName := range sensitiveFields {
		if val, ok := newMap[fieldName].(string); ok && isMaskedValue(val) {
			if existingVal, ok := existingMap[fieldName].(string); ok {
				newMap[fieldName] = existingVal
			}
		}
	}

	merged, err := json.Marshal(newMap)
	if err != nil {
		return nil, err
	}
	return merged, nil
}
