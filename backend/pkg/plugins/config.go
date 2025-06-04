package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

// ConfigLoader handles loading plugin configuration from various sources
type ConfigLoader struct {
	pluginDir   string
	pluginID    string
	logger      Logger
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

	// Apply settings to config struct
	return cl.applySettingsToStruct(configValue, settings)
}

// extractSettingsFromCue extracts the settings block from CUE content
func (cl *ConfigLoader) extractSettingsFromCue(content string) (map[string]interface{}, error) {
	settings := make(map[string]interface{})
	lines := strings.Split(content, "\n")
	
	inSettingsBlock := false
	blockDepth := 0
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip comments and empty lines
		if strings.HasPrefix(line, "//") || len(line) == 0 {
			continue
		}
		
		// Detect settings block
		if strings.Contains(line, "settings:") && strings.Contains(line, "{") {
			inSettingsBlock = true
			blockDepth = 1
			continue
		}
		
		if !inSettingsBlock {
			continue
		}
		
		// Track block depth
		blockDepth += strings.Count(line, "{")
		blockDepth -= strings.Count(line, "}")
		
		if blockDepth <= 0 {
			inSettingsBlock = false
			continue
		}
		
		// Parse setting line
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				
				// Remove trailing comma
				value = strings.TrimSuffix(value, ",")
				
				// Parse value
				parsedValue, err := cl.parseSettingValue(value)
				if err != nil {
					cl.logger.Warn("Failed to parse setting value", "key", key, "value", value, "error", err)
					continue
				}
				
				settings[key] = parsedValue
			}
		}
	}
	
	return settings, nil
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
	
	// Handle string values (remove quotes)
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		return value[1:len(value)-1], nil
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
	
	// Fallback to string
	return value, nil
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
		
		// Apply setting value
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