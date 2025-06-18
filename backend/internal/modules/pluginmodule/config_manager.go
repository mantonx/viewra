package pluginmodule

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// PluginConfigManager handles plugin configuration management
type PluginConfigManager struct {
	db     *gorm.DB
	logger hclog.Logger
	cache  map[string]*PluginConfiguration
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
		db:     db,
		logger: logger.Named("config-manager"),
		cache:  make(map[string]*PluginConfiguration),
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
			// Return empty configuration for new plugins
			return &PluginConfiguration{
				PluginID:     pluginID,
				Settings:     make(map[string]ConfigurationValue),
				Version:      "1.0.0",
				LastModified: time.Now(),
			}, nil
		}
		return nil, err
	}

	return pcm.parseDBConfiguration(dbConfig)
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

	dbConfig := database.PluginConfiguration{
		PluginID:     config.PluginID,
		Version:      config.Version,
		SchemaData:   string(schemaJSON),
		SettingsData: string(settingsJSON),
		Dependencies: string(rulesJSON), // Store validation rules in dependencies field
		ModifiedBy:   config.ModifiedBy,
		IsActive:     true,
	}

	// Use upsert (create or update)
	return pcm.db.Save(&dbConfig).Error
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
