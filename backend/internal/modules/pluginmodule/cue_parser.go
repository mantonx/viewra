package pluginmodule

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"github.com/hashicorp/go-hclog"
)

// CUEParser handles parsing CUE configuration files and converting them to JSON schemas
type CUEParser struct {
	logger hclog.Logger
	ctx    *cue.Context
}

// NewCUEParser creates a new CUE parser instance
func NewCUEParser(logger hclog.Logger) *CUEParser {
	return &CUEParser{
		logger: logger.Named("cue-parser"),
		ctx:    cuecontext.New(),
	}
}

// ParsePluginConfiguration parses a plugin's CUE configuration file and extracts the configuration schema
func (cp *CUEParser) ParsePluginConfiguration(pluginDir string) (*ConfigurationSchema, error) {
	pluginCuePath := filepath.Join(pluginDir, "plugin.cue")

	// Check if plugin.cue exists
	if _, err := os.Stat(pluginCuePath); os.IsNotExist(err) {
		cp.logger.Debug("No plugin.cue file found", "path", pluginCuePath)
		return nil, fmt.Errorf("plugin.cue not found in %s", pluginDir)
	}

	cp.logger.Debug("Parsing CUE configuration", "path", pluginCuePath)

	// Load the CUE configuration
	instances := load.Instances([]string{pluginCuePath}, nil)
	if len(instances) == 0 {
		return nil, fmt.Errorf("no CUE instances found")
	}

	if instances[0].Err != nil {
		return nil, fmt.Errorf("failed to load CUE file: %w", instances[0].Err)
	}

	// Build the CUE value
	value := cp.ctx.BuildInstance(instances[0])
	if value.Err() != nil {
		return nil, fmt.Errorf("failed to build CUE value: %w", value.Err())
	}

	// Extract plugin metadata and settings
	schema, err := cp.extractConfigurationSchema(value)
	if err != nil {
		return nil, fmt.Errorf("failed to extract configuration schema: %w", err)
	}

	cp.logger.Info("Successfully parsed CUE configuration",
		"path", pluginCuePath,
		"properties", len(schema.Properties),
		"title", schema.Title)

	return schema, nil
}

// extractConfigurationSchema extracts a JSON schema from the CUE value
func (cp *CUEParser) extractConfigurationSchema(value cue.Value) (*ConfigurationSchema, error) {
	schema := &ConfigurationSchema{
		Version:    "1.0",
		Properties: make(map[string]ConfigurationProperty),
		Categories: []ConfigurationCategory{},
	}

	// Try to extract plugin metadata first
	if pluginValue := value.LookupPath(cue.ParsePath("plugin")); pluginValue.Exists() {
		if name, err := pluginValue.LookupPath(cue.ParsePath("name")).String(); err == nil {
			schema.Title = name
		}
		if desc, err := pluginValue.LookupPath(cue.ParsePath("description")).String(); err == nil {
			schema.Description = desc
		}
	}

	// Also try to extract from #Plugin schema
	if pluginValue := value.LookupPath(cue.ParsePath("#Plugin")); pluginValue.Exists() {
		if name, err := pluginValue.LookupPath(cue.ParsePath("name")).String(); err == nil && schema.Title == "" {
			schema.Title = name
		}
		if desc, err := pluginValue.LookupPath(cue.ParsePath("description")).String(); err == nil && schema.Description == "" {
			schema.Description = desc
		}

		// Extract settings from #Plugin.settings
		if settingsValue := pluginValue.LookupPath(cue.ParsePath("settings")); settingsValue.Exists() {
			err := cp.extractPropertiesFromValue(settingsValue, schema, "")
			if err != nil {
				cp.logger.Warn("Failed to extract settings from #Plugin.settings", "error", err)
			} else {
				// If we successfully extracted from #Plugin.settings, we can skip the root-level extraction
				cp.organizePropertiesIntoCategories(schema)
				return schema, nil
			}
		}
	}

	// Extract root-level configuration sections
	configSections := []string{
		"enabled", "ffmpeg", "quality_profiles", "device_profiles", "quality", "audio",
		"subtitles", "performance", "logging", "hardware", "codecs", "resolutions",
		"content_detection", "adaptive", "cleanup", "health", "features", "filters",
		"api", "features", "artwork", "matching", "cache", "reliability", "debug",
	}

	for _, section := range configSections {
		if sectionValue := value.LookupPath(cue.ParsePath(section)); sectionValue.Exists() {
			err := cp.extractPropertiesFromValue(sectionValue, schema, section)
			if err != nil {
				cp.logger.Debug("Failed to extract section", "section", section, "error", err)
			}
		}
	}

	// Create categories based on extracted properties
	cp.organizePropertiesIntoCategories(schema)

	return schema, nil
}

// extractPropertiesFromValue recursively extracts properties from a CUE value
func (cp *CUEParser) extractPropertiesFromValue(value cue.Value, schema *ConfigurationSchema, prefix string) error {
	// Handle struct values
	if value.Kind() == cue.StructKind {
		iter, err := value.Fields(cue.All())
		if err != nil {
			return err
		}

		for iter.Next() {
			fieldName := iter.Label()
			fieldValue := iter.Value()

			fullName := fieldName
			if prefix != "" {
				fullName = prefix + "." + fieldName
			}

			property, err := cp.convertCueValueToProperty(fieldValue, fieldName)
			if err != nil {
				cp.logger.Debug("Failed to convert field", "field", fullName, "error", err)
				continue
			}

			property.Category = prefix
			schema.Properties[fullName] = property

			// Recursively handle nested structures
			if fieldValue.Kind() == cue.StructKind {
				cp.extractPropertiesFromValue(fieldValue, schema, fullName)
			}
		}
	}

	return nil
}

// convertCueValueToProperty converts a CUE value to a ConfigurationProperty
func (cp *CUEParser) convertCueValueToProperty(value cue.Value, fieldName string) (ConfigurationProperty, error) {
	prop := ConfigurationProperty{
		Title:       cp.humanizeFieldName(fieldName),
		Description: cp.generateDescription(fieldName),
	}

	// Extract default value and type constraints
	defaultValue, hasDefault := cp.extractDefaultValue(value)
	if hasDefault {
		prop.Default = defaultValue
		prop.Type = cp.inferJSONType(defaultValue)
	}

	// Extract type constraints from CUE
	switch value.Kind() {
	case cue.BoolKind:
		prop.Type = "boolean"
	case cue.StringKind:
		prop.Type = "string"
		if str, err := value.String(); err == nil {
			prop.Default = str
		}
	case cue.IntKind, cue.NumberKind:
		prop.Type = "number"
		if num, err := value.Float64(); err == nil {
			prop.Default = num
		}
	case cue.ListKind:
		prop.Type = "array"
		// Try to infer item type from default value
		if list, err := value.List(); err == nil {
			if list.Next() {
				itemProp, _ := cp.convertCueValueToProperty(list.Value(), "item")
				prop.Items = &itemProp
			}
		}
	case cue.StructKind:
		prop.Type = "object"
		prop.Properties = make(map[string]ConfigurationProperty)
	}

	// Extract constraints (min, max, etc.)
	cp.extractConstraints(value, &prop)

	return prop, nil
}

// extractDefaultValue extracts the default value from a CUE value with disjunction
func (cp *CUEParser) extractDefaultValue(value cue.Value) (interface{}, bool) {
	// Try to get the concrete value first
	if value.IsConcrete() {
		return cp.cueValueToInterface(value)
	}

	// Handle default values in disjunctions by trying to unify with the default marker
	defaultValue := value.Unify(cp.ctx.CompileString("*_"))
	if defaultValue.Exists() && defaultValue.IsConcrete() {
		return cp.cueValueToInterface(defaultValue)
	}

	// Fallback to regular value extraction
	return cp.cueValueToInterface(value)
}

// cueValueToInterface converts a CUE value to a Go interface{}
func (cp *CUEParser) cueValueToInterface(value cue.Value) (interface{}, bool) {
	switch value.Kind() {
	case cue.BoolKind:
		if b, err := value.Bool(); err == nil {
			return b, true
		}
	case cue.StringKind:
		if s, err := value.String(); err == nil {
			return s, true
		}
	case cue.IntKind:
		if i, err := value.Int64(); err == nil {
			return i, true
		}
	case cue.NumberKind:
		if f, err := value.Float64(); err == nil {
			return f, true
		}
	case cue.ListKind:
		var list []interface{}
		iter, err := value.List()
		if err == nil {
			for iter.Next() {
				if item, ok := cp.cueValueToInterface(iter.Value()); ok {
					list = append(list, item)
				}
			}
			return list, true
		}
	}
	return nil, false
}

// extractConstraints extracts validation constraints from CUE value
func (cp *CUEParser) extractConstraints(value cue.Value, prop *ConfigurationProperty) {
	// Try to extract numeric constraints
	if value.Kind() == cue.NumberKind || value.Kind() == cue.IntKind {
		// Look for minimum/maximum constraints in the CUE value
		// This is a simplified approach - full constraint extraction would be more complex
	}

	// Try to extract string constraints
	if value.Kind() == cue.StringKind {
		// Look for pattern, minLength, maxLength constraints
	}
}

// inferJSONType infers JSON schema type from a Go value
func (cp *CUEParser) inferJSONType(value interface{}) string {
	switch value.(type) {
	case bool:
		return "boolean"
	case string:
		return "string"
	case int, int32, int64, float32, float64:
		return "number"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	default:
		return "string"
	}
}

// humanizeFieldName converts field names to human-readable titles
func (cp *CUEParser) humanizeFieldName(fieldName string) string {
	// Convert snake_case to Title Case
	words := strings.Split(fieldName, "_")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

// generateDescription generates helpful descriptions for common configuration fields
func (cp *CUEParser) generateDescription(fieldName string) string {
	descriptions := map[string]string{
		"enabled":          "Enable or disable this feature",
		"api_key":          "API key for external service authentication",
		"timeout":          "Timeout duration in seconds",
		"max_retries":      "Maximum number of retry attempts",
		"cache_duration":   "How long to cache results (in hours)",
		"rate_limit":       "Maximum requests per time period",
		"quality":          "Quality setting for encoding",
		"bitrate":          "Target bitrate in kbps",
		"preset":           "Encoding preset (speed vs quality tradeoff)",
		"threads":          "Number of threads to use (0 = auto)",
		"max_concurrent":   "Maximum concurrent operations",
		"cleanup_interval": "How often to run cleanup (in minutes)",
		"debug":            "Enable debug logging",
		"log_level":        "Logging verbosity level",
	}

	if desc, exists := descriptions[fieldName]; exists {
		return desc
	}

	// Generate description based on field name patterns
	if strings.Contains(fieldName, "enable") || strings.Contains(fieldName, "enabled") {
		return "Enable or disable this feature"
	}
	if strings.Contains(fieldName, "max") {
		return "Maximum allowed value"
	}
	if strings.Contains(fieldName, "min") {
		return "Minimum required value"
	}
	if strings.Contains(fieldName, "timeout") {
		return "Timeout duration in seconds"
	}

	return fmt.Sprintf("Configuration setting for %s", cp.humanizeFieldName(fieldName))
}

// organizePropertiesIntoCategories creates logical categories for the configuration properties
func (cp *CUEParser) organizePropertiesIntoCategories(schema *ConfigurationSchema) {
	categories := map[string]ConfigurationCategory{
		"general": {
			ID:          "general",
			Title:       "General Settings",
			Description: "Basic configuration options",
			Order:       1,
		},
		"api": {
			ID:          "api",
			Title:       "API Configuration",
			Description: "External API settings and authentication",
			Order:       2,
		},
		"performance": {
			ID:          "performance",
			Title:       "Performance",
			Description: "Performance and resource management settings",
			Order:       3,
		},
		"quality": {
			ID:          "quality",
			Title:       "Quality Settings",
			Description: "Encoding quality and output parameters",
			Order:       4,
		},
		"features": {
			ID:          "features",
			Title:       "Features",
			Description: "Feature toggles and optional functionality",
			Order:       5,
		},
		"advanced": {
			ID:          "advanced",
			Title:       "Advanced",
			Description: "Advanced configuration options",
			Order:       6,
			Collapsible: true,
			Collapsed:   true,
		},
	}

	// Assign categories to properties
	for propName, prop := range schema.Properties {
		categoryID := cp.determineCategoryForProperty(propName, prop)
		prop.Category = categoryID
		schema.Properties[propName] = prop
	}

	// Convert map to slice
	for _, category := range categories {
		schema.Categories = append(schema.Categories, category)
	}
}

// determineCategoryForProperty determines the appropriate category for a property
func (cp *CUEParser) determineCategoryForProperty(propName string, prop ConfigurationProperty) string {
	name := strings.ToLower(propName)

	if strings.Contains(name, "api") || strings.Contains(name, "key") || strings.Contains(name, "auth") {
		return "api"
	}
	if strings.Contains(name, "performance") || strings.Contains(name, "thread") || strings.Contains(name, "concurrent") {
		return "performance"
	}
	if strings.Contains(name, "quality") || strings.Contains(name, "bitrate") || strings.Contains(name, "preset") {
		return "quality"
	}
	if strings.Contains(name, "feature") || strings.Contains(name, "enable") {
		return "features"
	}
	if strings.Contains(name, "debug") || strings.Contains(name, "log") || strings.Contains(name, "advanced") {
		return "advanced"
	}

	return "general"
}

// GetPluginConfigurationDefaults extracts default values from a CUE file
func (cp *CUEParser) GetPluginConfigurationDefaults(pluginDir string) (map[string]interface{}, error) {
	pluginCuePath := filepath.Join(pluginDir, "plugin.cue")

	if _, err := os.Stat(pluginCuePath); os.IsNotExist(err) {
		return make(map[string]interface{}), nil
	}

	instances := load.Instances([]string{pluginCuePath}, nil)
	if len(instances) == 0 || instances[0].Err != nil {
		return nil, fmt.Errorf("failed to load CUE file")
	}

	value := cp.ctx.BuildInstance(instances[0])
	if value.Err() != nil {
		return nil, fmt.Errorf("failed to build CUE value: %w", value.Err())
	}

	defaults := make(map[string]interface{})
	cp.extractDefaults(value, defaults, "")

	return defaults, nil
}

// extractDefaults recursively extracts default values from CUE
func (cp *CUEParser) extractDefaults(value cue.Value, defaults map[string]interface{}, prefix string) {
	if value.Kind() == cue.StructKind {
		iter, err := value.Fields(cue.All())
		if err != nil {
			return
		}

		for iter.Next() {
			fieldName := iter.Label()
			fieldValue := iter.Value()

			fullName := fieldName
			if prefix != "" {
				fullName = prefix + "." + fieldName
			}

			if defaultVal, hasDefault := cp.extractDefaultValue(fieldValue); hasDefault {
				defaults[fullName] = defaultVal
			}

			// Recursively handle nested structures
			if fieldValue.Kind() == cue.StructKind {
				cp.extractDefaults(fieldValue, defaults, fullName)
			}
		}
	}
}

// ValidateConfiguration validates configuration values against CUE constraints
func (cp *CUEParser) ValidateConfiguration(pluginDir string, config map[string]interface{}) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// TODO: Implement CUE-based validation
	// This would involve loading the CUE schema and validating the config against it

	return result, nil
}
