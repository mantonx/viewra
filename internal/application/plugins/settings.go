package plugins

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/infrastructure/crypto"
)

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

		// Update capability broker with configured status
		if hostPluginsServer := s.manager.GetHostPluginsServer(); hostPluginsServer != nil {
			hostPluginsServer.UpdatePluginStatus(id, true, resp.IsConfigured)
		}
	}

	// Persist encrypted settings to database
	if err := s.queries.UpdatePluginSettings(ctx, id, string(encryptedValues)); err != nil {
		return err
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

	// Update capability broker with configured status
	if hostPluginsServer := s.manager.GetHostPluginsServer(); hostPluginsServer != nil {
		hostPluginsServer.UpdatePluginStatus(id, true, resp.IsConfigured)
	}

	s.logger.Info("reapplied settings after restart", "plugin", id, "configured", resp.IsConfigured)
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

// ConfigureError is returned when plugin configuration fails.
type ConfigureError struct {
	Message string
}

func (e *ConfigureError) Error() string {
	return "plugin configuration failed: " + e.Message
}
