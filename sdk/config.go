package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ConfigLoader handles loading plugin configuration from various sources
type ConfigLoader struct {
	pluginDir string
	pluginID  string
	logger    Logger
}

// NewConfigLoader creates a new configuration loader for a plugin
func NewConfigLoader(pluginDir, pluginID string, logger Logger) *ConfigLoader {
	return &ConfigLoader{
		pluginDir: pluginDir,
		pluginID:  pluginID,
		logger:    logger,
	}
}

// LoadConfig loads plugin configuration in priority order:
// 1. Runtime override from host (passed via PluginContext.Config)
// 2. Plugin.cue file settings
// 3. Default struct values
func (cl *ConfigLoader) LoadConfig(config interface{}, runtimeConfig map[string]string) error {
	if config == nil {
		return fmt.Errorf("config struct cannot be nil")
	}

	configValue := reflect.ValueOf(config)
	if configValue.Kind() != reflect.Ptr || configValue.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("config must be a pointer to a struct")
	}

	// Step 1: Apply default values from struct tags
	if err := cl.applyDefaults(configValue.Elem()); err != nil {
		cl.logger.Warn("Failed to apply default configuration", "error", err)
	}

	// Step 2: Load from plugin.cue file
	cuePath := filepath.Join(cl.pluginDir, "plugin.cue")
	if err := cl.loadFromCueFile(configValue.Elem(), cuePath); err != nil {
		cl.logger.Warn("Failed to load configuration from plugin.cue", "error", err, "path", cuePath)
	}

	// Step 3: Apply runtime overrides (highest priority)
	if err := cl.applyRuntimeConfig(configValue.Elem(), runtimeConfig); err != nil {
		cl.logger.Warn("Failed to apply runtime configuration", "error", err)
	}

	cl.logger.Info("Plugin configuration loaded successfully")
	return nil
}

// applyDefaults applies default values from struct tags
func (cl *ConfigLoader) applyDefaults(configValue reflect.Value) error {
	configType := configValue.Type()

	for i := 0; i < configValue.NumField(); i++ {
		field := configValue.Field(i)
		fieldType := configType.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Check for default tag
		defaultValue := fieldType.Tag.Get("default")
		if defaultValue == "" {
			continue
		}

		if err := cl.setFieldValue(field, defaultValue); err != nil {
			return fmt.Errorf("failed to set default value for field %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

// loadFromCueFile loads configuration from plugin.cue file (simplified parser)
func (cl *ConfigLoader) loadFromCueFile(configValue reflect.Value, cuePath string) error {
	if _, err := os.Stat(cuePath); os.IsNotExist(err) {
		return fmt.Errorf("plugin.cue file not found: %s", cuePath)
	}

	content, err := os.ReadFile(cuePath)
	if err != nil {
		return fmt.Errorf("failed to read plugin.cue file: %w", err)
	}

	// Parse CUE content for settings block
	settings, err := cl.extractSettingsFromCue(string(content))
	if err != nil {
		return fmt.Errorf("failed to extract settings from CUE: %w", err)
	}

	// Debug log the extracted settings
	cl.logger.Info("Extracted settings from CUE", "settings_count", len(settings))
	for key, value := range settings {
		cl.logger.Info("CUE setting", "key", key, "value", value, "type", fmt.Sprintf("%T", value))
	}

	// Apply settings to config struct
	return cl.applySettingsToStruct(configValue, settings)
}

// extractSettingsFromCue extracts the settings block from CUE content
func (cl *ConfigLoader) extractSettingsFromCue(content string) (map[string]interface{}, error) {
	settings := make(map[string]interface{})
	lines := strings.Split(content, "\n")

	inSettingsBlock := false
	blockDepth := 0
	currentPath := []string{}

	// Track multiline string state
	inMultilineString := false
	multilineKey := ""
	multilineValue := strings.Builder{}

	for i, line := range lines {
		originalLine := line
		line = strings.TrimSpace(line)

		// Skip comments and empty lines (but not during multiline strings)
		if !inMultilineString && (strings.HasPrefix(line, "//") || len(line) == 0) {
			continue
		}

		// Detect settings block
		if !inMultilineString && strings.Contains(line, "settings:") && strings.Contains(line, "{") {
			inSettingsBlock = true
			blockDepth = 1
			continue
		}

		if !inSettingsBlock {
			continue
		}

		// Handle multiline strings
		if inMultilineString {
			// Check if this line ends the multiline string
			if strings.Contains(line, "\"") && !strings.HasSuffix(strings.TrimSpace(strings.Split(line, "\"")[0]), "\\") {
				// End of multiline string
				parts := strings.SplitN(line, "\"", 2)
				if len(parts) > 0 {
					multilineValue.WriteString(parts[0])
				}

				// Parse the complete multiline value
				completeValue := multilineValue.String()
				// Clean up the multiline value - remove extra whitespace but preserve structure
				completeValue = strings.TrimSpace(completeValue)

				// Set the value
				cl.setNestedValue(settings, append(currentPath, multilineKey), completeValue)

				// Reset multiline state
				inMultilineString = false
				multilineKey = ""
				multilineValue.Reset()
				continue
			} else {
				// Continue collecting multiline content
				// Preserve the original line structure but trim leading/trailing whitespace
				multilineValue.WriteString(strings.TrimSpace(originalLine))
				continue
			}
		}

		// Track block depth and parse nested structures
		openBraces := strings.Count(line, "{")
		closeBraces := strings.Count(line, "}")

		// Handle closing braces first to update path
		for j := 0; j < closeBraces; j++ {
			blockDepth--
			if len(currentPath) > 0 {
				currentPath = currentPath[:len(currentPath)-1]
			}
		}

		if blockDepth <= 0 {
			inSettingsBlock = false
			continue
		}

		// Parse setting line or block start
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])

				// Remove trailing comma
				value = strings.TrimSuffix(value, ",")

				// Check if this starts a multiline string
				if strings.HasPrefix(value, "\"") && !strings.HasSuffix(value, "\"") {
					// Start of multiline string
					inMultilineString = true
					multilineKey = key
					multilineValue.Reset()
					// Add the initial part (remove the opening quote)
					if len(value) > 1 {
						multilineValue.WriteString(value[1:])
					}
					continue
				}

				// Check if this is a nested block start
				if strings.Contains(value, "{") {
					// This is a nested block - add to current path
					currentPath = append(currentPath, key)
				} else {
					// This is a value - parse it
					parsedValue, err := cl.parseSettingValue(value)
					if err != nil {
						cl.logger.Warn("Failed to parse setting value", "key", key, "value", value, "error", err, "line", i+1)
						continue
					}

					// Set value in nested structure
					cl.setNestedValue(settings, append(currentPath, key), parsedValue)
				}
			}
		}

		// Update block depth for opening braces
		blockDepth += openBraces
	}

	// Log extracted settings for debugging
	cl.logger.Info("CUE parsing completed", "settings_extracted", len(settings))
	for key, value := range settings {
		if strVal, ok := value.(string); ok && len(strVal) > 50 {
			cl.logger.Debug("Long string setting extracted", "key", key, "length", len(strVal), "preview", strVal[:50]+"...")
		} else {
			cl.logger.Debug("Setting extracted", "key", key, "value", value, "type", fmt.Sprintf("%T", value))
		}
	}

	return settings, nil
}

// setNestedValue sets a value in a nested map structure
func (cl *ConfigLoader) setNestedValue(settings map[string]interface{}, path []string, value interface{}) {
	if len(path) == 0 {
		return
	}

	if len(path) == 1 {
		settings[path[0]] = value
		return
	}

	// Navigate/create nested structure
	current := settings
	for i := 0; i < len(path)-1; i++ {
		key := path[i]
		if _, exists := current[key]; !exists {
			current[key] = make(map[string]interface{})
		}
		if nested, ok := current[key].(map[string]interface{}); ok {
			current = nested
		} else {
			// Type conflict - create new map
			current[key] = make(map[string]interface{})
			current = current[key].(map[string]interface{})
		}
	}

	// Set the final value
	current[path[len(path)-1]] = value
}

// parseSettingValue parses a setting value from CUE format
func (cl *ConfigLoader) parseSettingValue(value string) (interface{}, error) {
	value = strings.TrimSpace(value)

	// Handle boolean values
	if value == "true" {
		return true, nil
	}
	if value == "false" {
		return false, nil
	}

	// Handle string values (remove quotes and clean whitespace)
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		// Extract string content and clean up any extra whitespace
		stringValue := value[1 : len(value)-1]
		// Clean up any extra whitespace that might have been introduced during parsing
		stringValue = strings.TrimSpace(stringValue)
		// Remove any internal line breaks or extra spaces that might have been added
		stringValue = strings.ReplaceAll(stringValue, "\n", "")
		stringValue = strings.ReplaceAll(stringValue, "\r", "")
		// Normalize multiple spaces to single spaces
		for strings.Contains(stringValue, "  ") {
			stringValue = strings.ReplaceAll(stringValue, "  ", " ")
		}
		return stringValue, nil
	}

	// Handle integer values
	if intVal, err := strconv.Atoi(value); err == nil {
		return intVal, nil
	}

	// Handle float values
	if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
		return floatVal, nil
	}

	// Handle default values like "*true", "*false", "*1200"
	if strings.HasPrefix(value, "*") {
		return cl.parseSettingValue(value[1:])
	}

	// Handle union types like "bool | *true" - extract default
	if strings.Contains(value, "|") && strings.Contains(value, "*") {
		parts := strings.Split(value, "|")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "*") {
				return cl.parseSettingValue(part[1:])
			}
		}
	}

	// Fallback to string (clean any whitespace)
	cleanValue := strings.TrimSpace(value)
	cleanValue = strings.ReplaceAll(cleanValue, "\n", "")
	cleanValue = strings.ReplaceAll(cleanValue, "\r", "")
	return cleanValue, nil
}

// applySettingsToStruct applies parsed settings to the config struct
func (cl *ConfigLoader) applySettingsToStruct(configValue reflect.Value, settings map[string]interface{}) error {
	configType := configValue.Type()

	for i := 0; i < configValue.NumField(); i++ {
		field := configValue.Field(i)
		fieldType := configType.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Look for matching setting (case-insensitive)
		var settingValue interface{}
		var found bool

		// Try exact match first
		if val, ok := settings[fieldType.Name]; ok {
			settingValue = val
			found = true
		} else {
			// Try JSON tag match
			jsonTag := fieldType.Tag.Get("json")
			if jsonTag != "" {
				jsonName := strings.Split(jsonTag, ",")[0]
				if val, ok := settings[jsonName]; ok {
					settingValue = val
					found = true
				}
			}
		}

		if !found {
			continue
		}

		// Handle nested structures
		if field.Kind() == reflect.Struct && settingValue != nil {
			if nestedMap, ok := settingValue.(map[string]interface{}); ok {
				// Recursively apply nested settings
				if err := cl.applySettingsToStruct(field, nestedMap); err != nil {
					cl.logger.Warn("Failed to apply nested settings", "field", fieldType.Name, "error", err)
				}
				continue
			}
		}

		// Apply setting value for primitive types
		if err := cl.setFieldValueFromInterface(field, settingValue); err != nil {
			cl.logger.Warn("Failed to apply setting", "field", fieldType.Name, "value", settingValue, "error", err)
		}
	}

	return nil
}

// applyRuntimeConfig applies runtime configuration overrides
func (cl *ConfigLoader) applyRuntimeConfig(configValue reflect.Value, runtimeConfig map[string]string) error {
	if len(runtimeConfig) == 0 {
		return nil
	}

	configType := configValue.Type()

	for i := 0; i < configValue.NumField(); i++ {
		field := configValue.Field(i)
		fieldType := configType.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Look for runtime override
		var overrideValue string
		var found bool

		// Try field name
		if val, ok := runtimeConfig[fieldType.Name]; ok {
			overrideValue = val
			found = true
		} else {
			// Try JSON tag
			jsonTag := fieldType.Tag.Get("json")
			if jsonTag != "" {
				jsonName := strings.Split(jsonTag, ",")[0]
				if val, ok := runtimeConfig[jsonName]; ok {
					overrideValue = val
					found = true
				}
			}
		}

		if !found {
			continue
		}

		// Apply override
		if err := cl.setFieldValue(field, overrideValue); err != nil {
			return fmt.Errorf("failed to apply runtime override for field %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

// setFieldValue sets a field value from a string
func (cl *ConfigLoader) setFieldValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value: %s", value)
		}
		field.SetBool(boolVal)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer value: %s", value)
		}
		field.SetInt(intVal)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid unsigned integer value: %s", value)
		}
		field.SetUint(uintVal)
	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid float value: %s", value)
		}
		field.SetFloat(floatVal)
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}

	return nil
}

// setFieldValueFromInterface sets a field value from an interface{}
func (cl *ConfigLoader) setFieldValueFromInterface(field reflect.Value, value interface{}) error {
	switch field.Kind() {
	case reflect.String:
		if str, ok := value.(string); ok {
			field.SetString(str)
		} else {
			field.SetString(fmt.Sprintf("%v", value))
		}
	case reflect.Bool:
		if boolVal, ok := value.(bool); ok {
			field.SetBool(boolVal)
		} else {
			return fmt.Errorf("cannot convert %v to bool", value)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch v := value.(type) {
		case int:
			field.SetInt(int64(v))
		case int64:
			field.SetInt(v)
		case float64:
			field.SetInt(int64(v))
		default:
			return fmt.Errorf("cannot convert %v to int", value)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch v := value.(type) {
		case int:
			field.SetUint(uint64(v))
		case int64:
			field.SetUint(uint64(v))
		case uint64:
			field.SetUint(v)
		case float64:
			field.SetUint(uint64(v))
		default:
			return fmt.Errorf("cannot convert %v to uint", value)
		}
	case reflect.Float32, reflect.Float64:
		switch v := value.(type) {
		case float64:
			field.SetFloat(v)
		case int:
			field.SetFloat(float64(v))
		case int64:
			field.SetFloat(float64(v))
		default:
			return fmt.Errorf("cannot convert %v to float", value)
		}
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}

	return nil
}

// BaseConfigurationService provides a standard implementation of ConfigurationService
// that can be embedded or used directly by plugins
type BaseConfigurationService struct {
	mutex         sync.RWMutex
	configPath    string
	pluginName    string
	configuration *PluginConfiguration
	schema        *ConfigurationSchema
	callbacks     []ConfigurationCallback
}

// ConfigurationCallback is called when configuration changes
type ConfigurationCallback func(oldConfig, newConfig *PluginConfiguration) error

// NewBaseConfigurationService creates a new base configuration service
func NewBaseConfigurationService(pluginName, configPath string) *BaseConfigurationService {
	return &BaseConfigurationService{
		configPath: configPath,
		pluginName: pluginName,
		configuration: &PluginConfiguration{
			Version:      "1.0.0",
			Enabled:      true,
			Settings:     make(map[string]interface{}),
			Features:     make(map[string]bool),
			LastModified: time.Now(),
			ModifiedBy:   "system",
		},
		callbacks: make([]ConfigurationCallback, 0),
	}
}

// GetConfiguration returns the current plugin configuration
func (c *BaseConfigurationService) GetConfiguration(ctx context.Context) (*PluginConfiguration, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Return a deep copy to prevent external modification
	configCopy := *c.configuration
	configCopy.Settings = make(map[string]interface{})
	for k, v := range c.configuration.Settings {
		configCopy.Settings[k] = v
	}
	configCopy.Features = make(map[string]bool)
	for k, v := range c.configuration.Features {
		configCopy.Features[k] = v
	}

	return &configCopy, nil
}

// UpdateConfiguration updates plugin configuration at runtime
func (c *BaseConfigurationService) UpdateConfiguration(ctx context.Context, config *PluginConfiguration) error {
	// Validate the configuration first
	validationResult, err := c.ValidateConfiguration(config)
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if !validationResult.Valid {
		return fmt.Errorf("invalid configuration: %v", validationResult.Errors)
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	oldConfig := c.configuration

	// Update configuration
	config.LastModified = time.Now()
	config.Version = c.incrementVersion(c.configuration.Version)
	c.configuration = config

	// Call callbacks
	for _, callback := range c.callbacks {
		if err := callback(oldConfig, config); err != nil {
			// Rollback on callback failure
			c.configuration = oldConfig
			return fmt.Errorf("configuration callback failed: %w", err)
		}
	}

	// Persist to file
	if err := c.saveConfigurationToFile(); err != nil {
		// Rollback on save failure
		c.configuration = oldConfig
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	return nil
}

// ReloadConfiguration reloads configuration from source
func (c *BaseConfigurationService) ReloadConfiguration(ctx context.Context) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.loadConfigurationFromFile()
}

// ValidateConfiguration validates a configuration before applying
func (c *BaseConfigurationService) ValidateConfiguration(config *PluginConfiguration) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	// Basic validation
	if config.Version == "" {
		result.Errors = append(result.Errors, "version is required")
	}

	if config.Settings == nil {
		result.Errors = append(result.Errors, "settings cannot be nil")
	}

	if config.Features == nil {
		result.Errors = append(result.Errors, "features cannot be nil")
	}

	// Schema-based validation if schema is available
	if c.schema != nil {
		if err := c.validateAgainstSchema(config); err != nil {
			result.Errors = append(result.Errors, err.Error())
		}
	}

	// Plugin-specific validation can be added by overriding this method

	result.Valid = len(result.Errors) == 0
	return result, nil
}

// GetConfigurationSchema returns the JSON schema for this plugin's configuration
func (c *BaseConfigurationService) GetConfigurationSchema() (*ConfigurationSchema, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.schema == nil {
		return c.createDefaultSchema(), nil
	}

	return c.schema, nil
}

// SetConfigurationSchema sets the JSON schema for validation
func (c *BaseConfigurationService) SetConfigurationSchema(schema *ConfigurationSchema) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.schema = schema
}

// AddConfigurationCallback adds a callback that's called when configuration changes
func (c *BaseConfigurationService) AddConfigurationCallback(callback ConfigurationCallback) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.callbacks = append(c.callbacks, callback)
}

// GetSetting returns a specific setting value
func (c *BaseConfigurationService) GetSetting(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	value, exists := c.configuration.Settings[key]
	return value, exists
}

// SetSetting updates a specific setting value
func (c *BaseConfigurationService) SetSetting(key string, value interface{}) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.configuration.Settings[key] = value
	c.configuration.LastModified = time.Now()

	return c.saveConfigurationToFile()
}

// GetFeature returns whether a feature is enabled
func (c *BaseConfigurationService) GetFeature(key string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.configuration.Features[key]
}

// SetFeature enables or disables a feature
func (c *BaseConfigurationService) SetFeature(key string, enabled bool) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.configuration.Features[key] = enabled
	c.configuration.LastModified = time.Now()

	return c.saveConfigurationToFile()
}

// Initialize loads the configuration from file or creates default
func (c *BaseConfigurationService) Initialize() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Try to load from file first
	if err := c.loadConfigurationFromFile(); err != nil {
		// If file doesn't exist, create default configuration
		if os.IsNotExist(err) {
			c.configuration.Settings = c.getDefaultSettings()
			c.configuration.Features = c.getDefaultFeatures()
			return c.saveConfigurationToFile()
		}
		return err
	}

	return nil
}

// Private helper methods

func (c *BaseConfigurationService) loadConfigurationFromFile() error {
	if c.configPath == "" {
		return fmt.Errorf("configuration path not set")
	}

	data, err := os.ReadFile(c.configPath)
	if err != nil {
		return err
	}

	var config PluginConfiguration
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse configuration file: %w", err)
	}

	c.configuration = &config
	return nil
}

func (c *BaseConfigurationService) saveConfigurationToFile() error {
	if c.configPath == "" {
		return fmt.Errorf("configuration path not set")
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(c.configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c.configuration, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	if err := os.WriteFile(c.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	return nil
}

func (c *BaseConfigurationService) incrementVersion(currentVersion string) string {
	// Simple version increment logic
	// In a real implementation, you might use semver parsing
	return fmt.Sprintf("%s.%d", currentVersion, time.Now().Unix())
}

func (c *BaseConfigurationService) validateAgainstSchema(config *PluginConfiguration) error {
	// Basic schema validation
	// In a real implementation, you would use a JSON schema validator library
	if c.schema.Schema != nil {
		// Validate against JSON schema (simplified)
		for key, required := range c.schema.Schema {
			if required == true {
				if _, exists := config.Settings[key]; !exists {
					return fmt.Errorf("required setting '%s' is missing", key)
				}
			}
		}
	}
	return nil
}

func (c *BaseConfigurationService) createDefaultSchema() *ConfigurationSchema {
	return &ConfigurationSchema{
		Schema: map[string]interface{}{
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether the plugin is enabled",
				"default":     true,
			},
			"log_level": map[string]interface{}{
				"type":        "string",
				"description": "Logging level for the plugin",
				"enum":        []string{"debug", "info", "warn", "error"},
				"default":     "info",
			},
		},
		Examples: map[string]interface{}{
			"basic": map[string]interface{}{
				"version": "1.0.0",
				"enabled": true,
				"settings": map[string]interface{}{
					"log_level": "info",
				},
				"features": map[string]interface{}{
					"auto_update": true,
				},
			},
		},
		Defaults: map[string]interface{}{
			"enabled":   true,
			"log_level": "info",
		},
	}
}

func (c *BaseConfigurationService) getDefaultSettings() map[string]interface{} {
	return map[string]interface{}{
		"log_level":   "info",
		"auto_update": true,
		"max_retries": 3,
		"timeout":     "30s",
	}
}

func (c *BaseConfigurationService) getDefaultFeatures() map[string]bool {
	return map[string]bool{
		"health_monitoring":  true,
		"metrics_collection": true,
		"auto_configuration": true,
	}
}
