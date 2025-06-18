package pluginmodule

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// PluginConfigManager handles plugin configuration management
type PluginConfigManager struct {
	db        *gorm.DB
	logger    hclog.Logger
	cache     map[string]*PluginConfiguration
	cueParser *CUEParser
}

// PluginConfiguration represents a complete plugin configuration
type PluginConfiguration struct {
	PluginID        string                        `json:"plugin_id"`
	Schema          *ConfigurationSchema          `json:"schema,omitempty"`
	Settings        map[string]ConfigurationValue `json:"settings"`
	Version         string                        `json:"version"`
	LastModified    time.Time                     `json:"last_modified"`
	ModifiedBy      string                        `json:"modified_by"`
	ValidationRules []ValidationRule              `json:"validation_rules,omitempty"`
	Dependencies    []string                      `json:"dependencies,omitempty"`
	Permissions     []string                      `json:"permissions,omitempty"`
}

// ConfigurationSchema defines the structure and metadata for plugin configuration
type ConfigurationSchema struct {
	Version     string                           `json:"version"`
	Title       string                           `json:"title"`
	Description string                           `json:"description"`
	Properties  map[string]ConfigurationProperty `json:"properties"`
	Required    []string                         `json:"required,omitempty"`
	Categories  []ConfigurationCategory          `json:"categories,omitempty"`
}

// ConfigurationProperty defines a single configuration property
type ConfigurationProperty struct {
	Type         string                           `json:"type"`        // string, number, boolean, array, object
	Title        string                           `json:"title"`       // Human-readable title
	Description  string                           `json:"description"` // Help text
	Default      interface{}                      `json:"default,omitempty"`
	Enum         []interface{}                    `json:"enum,omitempty"`         // Allowed values
	Minimum      *float64                         `json:"minimum,omitempty"`      // For numeric types
	Maximum      *float64                         `json:"maximum,omitempty"`      // For numeric types
	MinLength    *int                             `json:"minLength,omitempty"`    // For string types
	MaxLength    *int                             `json:"maxLength,omitempty"`    // For string types
	Pattern      string                           `json:"pattern,omitempty"`      // Regex pattern for strings
	Format       string                           `json:"format,omitempty"`       // Format hint (email, url, etc.)
	Items        *ConfigurationProperty           `json:"items,omitempty"`        // For array types
	Properties   map[string]ConfigurationProperty `json:"properties,omitempty"`   // For object types
	Dependencies []string                         `json:"dependencies,omitempty"` // Other properties this depends on
	Sensitive    bool                             `json:"sensitive,omitempty"`    // Hide value in UI
	Advanced     bool                             `json:"advanced,omitempty"`     // Show in advanced section
	IsBasic      bool                             `json:"is_basic"`               // Show in basic section (frontend needs this)
	ReadOnly     bool                             `json:"readOnly,omitempty"`     // Cannot be modified
	Category     string                           `json:"category,omitempty"`     // Group in UI
	Order        int                              `json:"order,omitempty"`        // Display order
	Conditional  *ConditionalProperty             `json:"conditional,omitempty"`  // Show only if condition met
}

// ConditionalProperty defines when a property should be shown
type ConditionalProperty struct {
	Property string      `json:"property"` // Property name to check
	Value    interface{} `json:"value"`    // Value that must match
	Operator string      `json:"operator"` // eq, ne, gt, lt, contains, etc.
}

// ConfigurationCategory groups related properties
type ConfigurationCategory struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Order       int    `json:"order,omitempty"`
	Collapsible bool   `json:"collapsible,omitempty"`
	Collapsed   bool   `json:"collapsed,omitempty"`
}

// ValidationRule defines custom validation logic
type ValidationRule struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`       // required, custom, dependent, etc.
	Properties []string               `json:"properties"` // Properties this rule applies to
	Message    string                 `json:"message"`    // Error message
	Condition  map[string]interface{} `json:"condition"`  // Rule-specific conditions
	Severity   string                 `json:"severity"`   // error, warning, info
}

// ConfigurationValue represents a typed configuration value with metadata
type ConfigurationValue struct {
	Value       interface{}       `json:"value"`
	Type        string            `json:"type"`
	Source      string            `json:"source"`     // file, env, api, default
	Overridden  bool              `json:"overridden"` // True if value differs from default
	LastChanged time.Time         `json:"last_changed"`
	ChangedBy   string            `json:"changed_by"`
	Validation  *ValidationResult `json:"validation,omitempty"`
}

// ValidationResult contains validation information for a value
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// NewPluginConfigManager creates a new plugin configuration manager
func NewPluginConfigManager(db *gorm.DB, logger hclog.Logger) *PluginConfigManager {
	return &PluginConfigManager{
		db:        db,
		logger:    logger.Named("config-manager"),
		cache:     make(map[string]*PluginConfiguration),
		cueParser: NewCUEParser(),
	}
}

// GetPluginConfiguration retrieves the complete configuration for a plugin
func (pcm *PluginConfigManager) GetPluginConfiguration(pluginID string) (*PluginConfiguration, error) {
	// Check cache first
	if config, exists := pcm.cache[pluginID]; exists {
		return config, nil
	}

	// Load from database
	config, err := pcm.loadConfigurationFromDB(pluginID)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration for plugin %s: %w", pluginID, err)
	}

	// Cache the configuration
	pcm.cache[pluginID] = config
	return config, nil
}

// UpdatePluginConfiguration updates a plugin's configuration
func (pcm *PluginConfigManager) UpdatePluginConfiguration(pluginID string, updates map[string]interface{}, modifiedBy string) (*PluginConfiguration, error) {
	config, err := pcm.GetPluginConfiguration(pluginID)
	if err != nil {
		return nil, err
	}

	// Validate the updates against the schema
	if err := pcm.validateConfigurationUpdates(config, updates); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// Apply updates
	for key, value := range updates {
		if config.Settings == nil {
			config.Settings = make(map[string]ConfigurationValue)
		}

		// Determine the type from schema or existing value
		valueType := pcm.inferValueType(value, config.Schema, key)

		config.Settings[key] = ConfigurationValue{
			Value:       value,
			Type:        valueType,
			Source:      "api",
			Overridden:  pcm.isValueOverridden(value, config.Schema, key),
			LastChanged: time.Now(),
			ChangedBy:   modifiedBy,
		}
	}

	config.LastModified = time.Now()
	config.ModifiedBy = modifiedBy

	// Save to database
	if err := pcm.saveConfigurationToDB(config); err != nil {
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}

	// Update cache
	pcm.cache[pluginID] = config
	return config, nil
}

// GetConfigurationSchema retrieves the schema for a plugin's configuration
func (pcm *PluginConfigManager) GetConfigurationSchema(pluginID string) (*ConfigurationSchema, error) {
	config, err := pcm.GetPluginConfiguration(pluginID)
	if err != nil {
		return nil, err
	}

	return config.Schema, nil
}

// ValidateConfiguration validates an entire configuration against its schema
func (pcm *PluginConfigManager) ValidateConfiguration(pluginID string, settings map[string]interface{}) (*ValidationResult, error) {
	config, err := pcm.GetPluginConfiguration(pluginID)
	if err != nil {
		return nil, err
	}

	return pcm.validateSettings(config.Schema, settings, config.ValidationRules)
}

// ResetConfiguration resets a plugin's configuration to defaults
func (pcm *PluginConfigManager) ResetConfiguration(pluginID string, modifiedBy string) (*PluginConfiguration, error) {
	config, err := pcm.GetPluginConfiguration(pluginID)
	if err != nil {
		return nil, err
	}

	// Reset to default values from schema
	defaultSettings := make(map[string]ConfigurationValue)
	if config.Schema != nil {
		for key, property := range config.Schema.Properties {
			if property.Default != nil {
				defaultSettings[key] = ConfigurationValue{
					Value:       property.Default,
					Type:        property.Type,
					Source:      "default",
					Overridden:  false,
					LastChanged: time.Now(),
					ChangedBy:   modifiedBy,
				}
			}
		}
	}

	config.Settings = defaultSettings
	config.LastModified = time.Now()
	config.ModifiedBy = modifiedBy

	// Save to database
	if err := pcm.saveConfigurationToDB(config); err != nil {
		return nil, fmt.Errorf("failed to reset configuration: %w", err)
	}

	// Update cache
	pcm.cache[pluginID] = config
	return config, nil
}

// RegisterConfigurationSchema registers a schema for a plugin
func (pcm *PluginConfigManager) RegisterConfigurationSchema(pluginID string, schema *ConfigurationSchema) error {
	config, err := pcm.GetPluginConfiguration(pluginID)
	if err != nil {
		// Create new configuration if it doesn't exist
		config = &PluginConfiguration{
			PluginID:     pluginID,
			Settings:     make(map[string]ConfigurationValue),
			Version:      "1.0.0",
			LastModified: time.Now(),
		}
	}

	config.Schema = schema

	// Initialize default values
	if config.Settings == nil {
		config.Settings = make(map[string]ConfigurationValue)
	}

	for key, property := range schema.Properties {
		if _, exists := config.Settings[key]; !exists && property.Default != nil {
			config.Settings[key] = ConfigurationValue{
				Value:       property.Default,
				Type:        property.Type,
				Source:      "default",
				Overridden:  false,
				LastChanged: time.Now(),
				ChangedBy:   "system",
			}
		}
	}

	// Save to database
	if err := pcm.saveConfigurationToDB(config); err != nil {
		return fmt.Errorf("failed to register schema: %w", err)
	}

	// Update cache
	pcm.cache[pluginID] = config
	return nil
}

// GetAllConfigurations returns configurations for all plugins
func (pcm *PluginConfigManager) GetAllConfigurations() (map[string]*PluginConfiguration, error) {
	// Load all configurations from database
	var dbConfigs []database.PluginConfiguration
	if err := pcm.db.Find(&dbConfigs).Error; err != nil {
		return nil, fmt.Errorf("failed to load configurations: %w", err)
	}

	configurations := make(map[string]*PluginConfiguration)
	for _, dbConfig := range dbConfigs {
		config, err := pcm.parseDBConfiguration(dbConfig)
		if err != nil {
			pcm.logger.Error("failed to parse configuration", "plugin_id", dbConfig.PluginID, "error", err)
			continue
		}
		configurations[dbConfig.PluginID] = config
	}

	return configurations, nil
}

// ClearCache clears the configuration cache
func (pcm *PluginConfigManager) ClearCache() {
	pcm.cache = make(map[string]*PluginConfiguration)
}

// Private helper methods

func (pcm *PluginConfigManager) loadConfigurationFromDB(pluginID string) (*PluginConfiguration, error) {
	var dbConfig database.PluginConfiguration
	err := pcm.db.Where("plugin_id = ?", pluginID).First(&dbConfig).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// No database record found, load from CUE file and save defaults
			pcm.logger.Info("no database configuration found, loading from CUE and saving defaults", "plugin_id", pluginID)

			config, err := pcm.loadConfigurationFromCUE(pluginID)
			if err != nil {
				return nil, fmt.Errorf("failed to load configuration from CUE: %w", err)
			}

			// Set metadata for auto-saved configuration
			config.ModifiedBy = "system_auto_save"
			config.LastModified = time.Now()

			// Save the defaults to database
			if err := pcm.saveConfigurationToDB(config); err != nil {
				pcm.logger.Warn("failed to save CUE defaults to database", "plugin_id", pluginID, "error", err)
				// Return the configuration anyway, even if save failed
				return config, nil
			}

			// Cache the configuration
			pcm.cache[pluginID] = config

			pcm.logger.Info("successfully loaded CUE configuration and saved defaults to database",
				"plugin_id", pluginID,
				"settings_count", len(config.Settings))

			return config, nil
		}
		return nil, err
	}

	config, err := pcm.parseDBConfiguration(dbConfig)
	if err != nil {
		return nil, err
	}

	// If no schema in database, try to load from CUE file
	if config.Schema == nil {
		cueConfig, err := pcm.loadConfigurationFromCUE(pluginID)
		if err == nil && cueConfig.Schema != nil {
			config.Schema = cueConfig.Schema
			// Merge in CUE defaults for missing settings
			pcm.mergeDefaultSettings(config, cueConfig)
		}
	}

	return config, nil
}

func (pcm *PluginConfigManager) parseDBConfiguration(dbConfig database.PluginConfiguration) (*PluginConfiguration, error) {
	config := &PluginConfiguration{
		PluginID:     dbConfig.PluginID,
		Version:      dbConfig.Version,
		LastModified: dbConfig.UpdatedAt,
		ModifiedBy:   dbConfig.ModifiedBy,
		Settings:     make(map[string]ConfigurationValue),
	}

	// Parse schema if present
	if dbConfig.SchemaData != "" {
		var schema ConfigurationSchema
		if err := json.Unmarshal([]byte(dbConfig.SchemaData), &schema); err != nil {
			return nil, fmt.Errorf("failed to parse schema: %w", err)
		}
		config.Schema = &schema
	}

	// Parse settings
	if dbConfig.SettingsData != "" {
		var settings map[string]ConfigurationValue
		if err := json.Unmarshal([]byte(dbConfig.SettingsData), &settings); err != nil {
			return nil, fmt.Errorf("failed to parse settings: %w", err)
		}
		config.Settings = settings
	}

	// Parse validation rules from dependencies field (we'll store validation rules there)
	if dbConfig.Dependencies != "" {
		var rules []ValidationRule
		if err := json.Unmarshal([]byte(dbConfig.Dependencies), &rules); err != nil {
			// If it fails, it might be actual dependencies, so ignore the error
			pcm.logger.Debug("failed to parse validation rules from dependencies field", "error", err)
		} else {
			config.ValidationRules = rules
		}
	}

	return config, nil
}

func (pcm *PluginConfigManager) saveConfigurationToDB(config *PluginConfiguration) error {
	// Serialize complex fields
	schemaJSON, _ := json.Marshal(config.Schema)
	settingsJSON, _ := json.Marshal(config.Settings)
	rulesJSON, _ := json.Marshal(config.ValidationRules)

	// Check if configuration already exists
	var existingConfig database.PluginConfiguration
	err := pcm.db.Where("plugin_id = ?", config.PluginID).First(&existingConfig).Error

	dbConfig := database.PluginConfiguration{
		PluginID:     config.PluginID,
		Version:      config.Version,
		SchemaData:   string(schemaJSON),
		SettingsData: string(settingsJSON),
		Dependencies: string(rulesJSON), // Store validation rules in dependencies field
		ModifiedBy:   config.ModifiedBy,
		IsActive:     true,
	}

	if err == nil {
		// Update existing record
		dbConfig.ID = existingConfig.ID
		return pcm.db.Save(&dbConfig).Error
	} else if err == gorm.ErrRecordNotFound {
		// Create new record
		return pcm.db.Create(&dbConfig).Error
	} else {
		// Other error
		return err
	}
}

func (pcm *PluginConfigManager) validateConfigurationUpdates(config *PluginConfiguration, updates map[string]interface{}) error {
	if config.Schema == nil {
		// No schema to validate against
		return nil
	}

	for key, value := range updates {
		property, exists := config.Schema.Properties[key]
		if !exists {
			return fmt.Errorf("property '%s' is not defined in schema", key)
		}

		if err := pcm.validateValue(value, &property); err != nil {
			return fmt.Errorf("validation failed for property '%s': %w", key, err)
		}
	}

	return nil
}

func (pcm *PluginConfigManager) validateValue(value interface{}, property *ConfigurationProperty) error {
	// Type validation
	if !pcm.isValidType(value, property.Type) {
		return fmt.Errorf("expected type %s, got %T", property.Type, value)
	}

	// Enum validation
	if len(property.Enum) > 0 {
		valid := false
		for _, allowedValue := range property.Enum {
			if reflect.DeepEqual(value, allowedValue) {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("value must be one of: %v", property.Enum)
		}
	}

	// String validations
	if property.Type == "string" {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string value")
		}

		if property.MinLength != nil && len(str) < *property.MinLength {
			return fmt.Errorf("string too short, minimum length: %d", *property.MinLength)
		}

		if property.MaxLength != nil && len(str) > *property.MaxLength {
			return fmt.Errorf("string too long, maximum length: %d", *property.MaxLength)
		}

		if property.Pattern != "" {
			// TODO: Add regex validation
		}
	}

	// Numeric validations
	if property.Type == "number" {
		num, ok := value.(float64)
		if !ok {
			return fmt.Errorf("expected numeric value")
		}

		if property.Minimum != nil && num < *property.Minimum {
			return fmt.Errorf("value too small, minimum: %f", *property.Minimum)
		}

		if property.Maximum != nil && num > *property.Maximum {
			return fmt.Errorf("value too large, maximum: %f", *property.Maximum)
		}
	}

	return nil
}

func (pcm *PluginConfigManager) validateSettings(schema *ConfigurationSchema, settings map[string]interface{}, rules []ValidationRule) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	if schema == nil {
		return result, nil
	}

	// Validate required properties
	for _, required := range schema.Required {
		if _, exists := settings[required]; !exists {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("required property '%s' is missing", required))
		}
	}

	// Validate individual properties
	for key, value := range settings {
		if property, exists := schema.Properties[key]; exists {
			if err := pcm.validateValue(value, &property); err != nil {
				result.Valid = false
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %s", key, err.Error()))
			}
		}
	}

	// Apply custom validation rules
	for _, rule := range rules {
		if err := pcm.applyValidationRule(rule, settings, result); err != nil {
			pcm.logger.Error("failed to apply validation rule", "rule_id", rule.ID, "error", err)
		}
	}

	return result, nil
}

// loadConfigurationFromCUE loads configuration from a plugin's CUE file
func (pcm *PluginConfigManager) loadConfigurationFromCUE(pluginID string) (*PluginConfiguration, error) {
	// Try different plugin directory paths based on environment
	possiblePaths := []string{
		fmt.Sprintf("./backend/data/plugins/%s", pluginID), // Development environment
		fmt.Sprintf("/app/data/plugins/%s", pluginID),      // Docker container
		fmt.Sprintf("./data/plugins/%s", pluginID),         // Alternative Docker path
	}

	var pluginDir string
	var rawSchema map[string]interface{}
	var err error

	// Find the first valid plugin directory
	for _, path := range possiblePaths {
		if _, statErr := os.Stat(path); statErr == nil {
			pluginDir = path
			break
		}
	}

	if pluginDir == "" {
		pcm.logger.Debug("Plugin directory not found in any expected location", "plugin", pluginID, "paths", possiblePaths)
		// Return basic configuration if directory not found
		return &PluginConfiguration{
			PluginID:     pluginID,
			Settings:     make(map[string]ConfigurationValue),
			Version:      "1.0.0",
			LastModified: time.Now(),
		}, nil
	}

	// Try to parse CUE configuration
	rawSchema, err = pcm.cueParser.ParsePluginConfiguration(pluginDir)
	if err != nil {
		pcm.logger.Debug("Failed to parse CUE configuration", "plugin", pluginID, "path", pluginDir, "error", err)
		// Return basic configuration if CUE parsing fails
		return &PluginConfiguration{
			PluginID:     pluginID,
			Settings:     make(map[string]ConfigurationValue),
			Version:      "1.0.0",
			LastModified: time.Now(),
		}, nil
	}

	pcm.logger.Debug("Successfully parsed CUE configuration", "plugin", pluginID, "path", pluginDir, "properties", len(rawSchema))

	// Convert raw schema to ConfigurationSchema
	schema := pcm.convertRawSchemaToConfigurationSchema(rawSchema, pluginID)

	// Extract default values and create settings
	settings := make(map[string]ConfigurationValue)
	pcm.extractDefaultsFromRawSchema(rawSchema, settings, "")

	return &PluginConfiguration{
		PluginID:     pluginID,
		Schema:       schema,
		Settings:     settings,
		Version:      "1.0.0",
		LastModified: time.Now(),
		ModifiedBy:   "system",
	}, nil
}

// convertRawSchemaToConfigurationSchema converts the raw map from CUE parser to ConfigurationSchema
func (pcm *PluginConfigManager) convertRawSchemaToConfigurationSchema(rawSchema map[string]interface{}, pluginID string) *ConfigurationSchema {
	schema := &ConfigurationSchema{
		Version:     "1.0.0",
		Title:       pluginID + " Configuration",
		Description: "Configuration schema for " + pluginID + " plugin",
		Properties:  make(map[string]ConfigurationProperty),
		Categories:  []ConfigurationCategory{},
	}

	// Convert each property
	for key, value := range rawSchema {
		if propMap, ok := value.(map[string]interface{}); ok {
			property := pcm.convertMapToConfigurationProperty(propMap)
			schema.Properties[key] = property
		}
	}

	// Create categories based on properties
	pcm.organizePropertiesIntoCategories(schema)
	return schema
}

// convertMapToConfigurationProperty converts a map to ConfigurationProperty
func (pcm *PluginConfigManager) convertMapToConfigurationProperty(propMap map[string]interface{}) ConfigurationProperty {
	prop := ConfigurationProperty{}

	if title, ok := propMap["title"].(string); ok {
		prop.Title = title
	}
	if description, ok := propMap["description"].(string); ok {
		prop.Description = description
	}
	if propType, ok := propMap["type"].(string); ok {
		prop.Type = propType
	}
	if category, ok := propMap["category"].(string); ok {
		prop.Category = category
	}
	if defaultVal, ok := propMap["default"]; ok {
		prop.Default = defaultVal
	}

	// Copy UI metadata fields for basic/advanced classification
	if isBasic, ok := propMap["is_basic"].(bool); ok {
		prop.Advanced = !isBasic // Advanced is the inverse of is_basic
		prop.IsBasic = isBasic   // Set the IsBasic field for frontend
	} else {
		// Default to advanced for all fields for testing
		prop.Advanced = true
		prop.IsBasic = false
	}
	if importance, ok := propMap["importance"].(int); ok {
		prop.Order = importance
	}
	if userFriendly, ok := propMap["user_friendly"].(bool); ok && !userFriendly {
		prop.Advanced = true
	}

	// Handle nested properties for object types
	if properties, ok := propMap["properties"].(map[string]interface{}); ok {
		prop.Properties = make(map[string]ConfigurationProperty)
		for nestedKey, nestedValue := range properties {
			if nestedPropMap, ok := nestedValue.(map[string]interface{}); ok {
				prop.Properties[nestedKey] = pcm.convertMapToConfigurationProperty(nestedPropMap)
			}
		}
	}

	return prop
}

// extractDefaultsFromRawSchema recursively extracts default values from the raw schema
func (pcm *PluginConfigManager) extractDefaultsFromRawSchema(rawSchema map[string]interface{}, settings map[string]ConfigurationValue, prefix string) {
	for key, value := range rawSchema {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		if propMap, ok := value.(map[string]interface{}); ok {
			// Extract default value if present
			if defaultVal, hasDefault := propMap["default"]; hasDefault {
				propType := "string"
				if t, ok := propMap["type"].(string); ok {
					propType = t
				}

				settings[fullKey] = ConfigurationValue{
					Value:       defaultVal,
					Type:        propType,
					Source:      "cue_default",
					Overridden:  false,
					LastChanged: time.Now(),
					ChangedBy:   "system",
				}
			}

			// Recursively process nested properties
			if nestedProps, ok := propMap["properties"].(map[string]interface{}); ok {
				pcm.extractDefaultsFromRawSchema(nestedProps, settings, fullKey)
			}
		}
	}
}

// organizePropertiesIntoCategories creates logical categories for the configuration properties
func (pcm *PluginConfigManager) organizePropertiesIntoCategories(schema *ConfigurationSchema) {
	categoryMap := make(map[string]*ConfigurationCategory)
	categoryOrder := map[string]int{
		"General":     1,
		"API":         2,
		"Performance": 3,
		"Quality":     4,
		"Features":    5,
		"Hardware":    6,
		"Filters":     7,
		"Storage":     8,
		"Monitoring":  9,
		"Logging":     10,
		"Advanced":    11,
	}

	// Create categories based on properties
	for _, property := range schema.Properties {
		categoryName := property.Category
		if categoryName == "" {
			categoryName = "Advanced"
		}

		if _, exists := categoryMap[categoryName]; !exists {
			order := categoryOrder[categoryName]
			if order == 0 {
				order = 99
			}

			categoryMap[categoryName] = &ConfigurationCategory{
				ID:          strings.ToLower(categoryName),
				Title:       categoryName,
				Description: categoryName + " configuration options",
				Order:       order,
				Collapsible: categoryName == "Advanced",
				Collapsed:   categoryName == "Advanced",
			}
		}
	}

	// Convert map to slice
	for _, category := range categoryMap {
		schema.Categories = append(schema.Categories, *category)
	}

	// Sort categories by order
	sort.Slice(schema.Categories, func(i, j int) bool {
		return schema.Categories[i].Order < schema.Categories[j].Order
	})
}

// mergeDefaultSettings merges default settings from CUE into existing configuration
func (pcm *PluginConfigManager) mergeDefaultSettings(config *PluginConfiguration, cueConfig *PluginConfiguration) {
	if config.Settings == nil {
		config.Settings = make(map[string]ConfigurationValue)
	}

	// Add missing settings from CUE defaults
	for key, defaultValue := range cueConfig.Settings {
		if _, exists := config.Settings[key]; !exists {
			config.Settings[key] = defaultValue
		}
	}
}

func (pcm *PluginConfigManager) applyValidationRule(rule ValidationRule, settings map[string]interface{}, result *ValidationResult) error {
	// TODO: Implement custom validation rule logic
	return nil
}

func (pcm *PluginConfigManager) isValidType(value interface{}, expectedType string) bool {
	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := value.(float64)
		if !ok {
			_, ok = value.(int)
		}
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		return reflect.TypeOf(value).Kind() == reflect.Slice
	case "object":
		return reflect.TypeOf(value).Kind() == reflect.Map
	default:
		return true // Unknown type, allow it
	}
}

func (pcm *PluginConfigManager) inferValueType(value interface{}, schema *ConfigurationSchema, key string) string {
	// Try to get type from schema first
	if schema != nil {
		if property, exists := schema.Properties[key]; exists {
			return property.Type
		}
	}

	// Infer from value type
	switch value.(type) {
	case string:
		return "string"
	case float64, int, int64:
		return "number"
	case bool:
		return "boolean"
	default:
		if reflect.TypeOf(value).Kind() == reflect.Slice {
			return "array"
		}
		if reflect.TypeOf(value).Kind() == reflect.Map {
			return "object"
		}
		return "string" // Default fallback
	}
}

func (pcm *PluginConfigManager) isValueOverridden(value interface{}, schema *ConfigurationSchema, key string) bool {
	if schema == nil {
		return true
	}

	property, exists := schema.Properties[key]
	if !exists {
		return true
	}

	return !reflect.DeepEqual(value, property.Default)
}

// AdminPanelIntegration provides methods for admin panel integration
type AdminPanelIntegration struct {
	configManager *PluginConfigManager
}

// NewAdminPanelIntegration creates a new admin panel integration
func NewAdminPanelIntegration(configManager *PluginConfigManager) *AdminPanelIntegration {
	return &AdminPanelIntegration{
		configManager: configManager,
	}
}

// GetAdminUISchema generates a UI schema for rendering configuration forms
func (api *AdminPanelIntegration) GetAdminUISchema(pluginID string) (*AdminUISchema, error) {
	config, err := api.configManager.GetPluginConfiguration(pluginID)
	if err != nil {
		return nil, err
	}

	if config.Schema == nil {
		return nil, fmt.Errorf("no schema available for plugin %s", pluginID)
	}

	return api.convertToAdminUISchema(config.Schema), nil
}

// AdminUISchema provides a UI-friendly representation of configuration schema
type AdminUISchema struct {
	Title       string                     `json:"title"`
	Description string                     `json:"description"`
	Categories  []AdminUICategory          `json:"categories"`
	Properties  map[string]AdminUIProperty `json:"properties"`
}

// AdminUICategory represents a group of related settings in the UI
type AdminUICategory struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Properties  []string `json:"properties"`
	Collapsible bool     `json:"collapsible"`
	Collapsed   bool     `json:"collapsed"`
	Order       int      `json:"order"`
}

// AdminUIProperty provides UI-specific metadata for a configuration property
type AdminUIProperty struct {
	Type        string                 `json:"type"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Default     interface{}            `json:"default,omitempty"`
	Required    bool                   `json:"required"`
	Sensitive   bool                   `json:"sensitive"`
	Advanced    bool                   `json:"advanced"`
	ReadOnly    bool                   `json:"readOnly"`
	Order       int                    `json:"order"`
	UIHints     map[string]interface{} `json:"uiHints,omitempty"`
	Validation  AdminUIValidation      `json:"validation,omitempty"`
}

// AdminUIValidation provides UI validation rules
type AdminUIValidation struct {
	Required  bool          `json:"required"`
	Pattern   string        `json:"pattern,omitempty"`
	MinLength *int          `json:"minLength,omitempty"`
	MaxLength *int          `json:"maxLength,omitempty"`
	Minimum   *float64      `json:"minimum,omitempty"`
	Maximum   *float64      `json:"maximum,omitempty"`
	Enum      []interface{} `json:"enum,omitempty"`
}

func (api *AdminPanelIntegration) convertToAdminUISchema(schema *ConfigurationSchema) *AdminUISchema {
	uiSchema := &AdminUISchema{
		Title:       schema.Title,
		Description: schema.Description,
		Categories:  []AdminUICategory{},
		Properties:  make(map[string]AdminUIProperty),
	}

	// Convert categories
	for _, category := range schema.Categories {
		uiCategory := AdminUICategory{
			ID:          category.ID,
			Title:       category.Title,
			Description: category.Description,
			Properties:  []string{},
			Collapsible: category.Collapsible,
			Collapsed:   category.Collapsed,
			Order:       category.Order,
		}
		uiSchema.Categories = append(uiSchema.Categories, uiCategory)
	}

	// Convert properties
	for key, property := range schema.Properties {
		uiProperty := AdminUIProperty{
			Type:        property.Type,
			Title:       property.Title,
			Description: property.Description,
			Default:     property.Default,
			Required:    api.isPropertyRequired(key, schema.Required),
			Sensitive:   property.Sensitive,
			Advanced:    property.Advanced,
			ReadOnly:    property.ReadOnly,
			Order:       property.Order,
			UIHints:     make(map[string]interface{}),
			Validation: AdminUIValidation{
				Required:  api.isPropertyRequired(key, schema.Required),
				Pattern:   property.Pattern,
				MinLength: property.MinLength,
				MaxLength: property.MaxLength,
				Minimum:   property.Minimum,
				Maximum:   property.Maximum,
				Enum:      property.Enum,
			},
		}

		// Add UI hints based on property characteristics
		if property.Format != "" {
			uiProperty.UIHints["format"] = property.Format
		}

		uiSchema.Properties[key] = uiProperty
	}

	return uiSchema
}

func (api *AdminPanelIntegration) isPropertyRequired(propertyName string, required []string) bool {
	for _, req := range required {
		if req == propertyName {
			return true
		}
	}
	return false
}

// CreateDefaultConfiguration creates a new configuration record with defaults from CUE file
func (pcm *PluginConfigManager) CreateDefaultConfiguration(pluginID string) (*PluginConfiguration, error) {
	pcm.logger.Info("creating default configuration for plugin", "plugin_id", pluginID)

	// Load configuration from CUE file to get defaults and schema
	config, err := pcm.loadConfigurationFromCUE(pluginID)
	if err != nil {
		return nil, fmt.Errorf("failed to load defaults from CUE file: %w", err)
	}

	// Set metadata for the new configuration
	config.ModifiedBy = "system_auto_populate"
	config.LastModified = time.Now()

	// Save to database immediately
	if err := pcm.saveConfigurationToDB(config); err != nil {
		return nil, fmt.Errorf("failed to save default configuration: %w", err)
	}

	// Cache the configuration
	pcm.cache[pluginID] = config

	pcm.logger.Info("successfully created default configuration",
		"plugin_id", pluginID,
		"settings_count", len(config.Settings),
		"schema_properties", len(config.Schema.Properties))

	return config, nil
}

// EnsureConfigurationExists ensures a plugin has a configuration record, creating defaults if needed
func (pcm *PluginConfigManager) EnsureConfigurationExists(pluginID string) (*PluginConfiguration, error) {
	// Try to load existing configuration
	config, err := pcm.GetPluginConfiguration(pluginID)
	if err == nil {
		return config, nil
	}

	// If not found, create default configuration
	pcm.logger.Info("no configuration found for plugin, creating defaults", "plugin_id", pluginID)
	return pcm.CreateDefaultConfiguration(pluginID)
}
