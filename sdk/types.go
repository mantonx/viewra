package plugins

import (
	"context"
	"fmt"
	"time"
)

// PluginManagerInterface defines proper types for plugin managers
// to replace interface{} usage throughout the system
type PluginManagerInterface interface {
	GetPlugin(pluginID string) (Plugin, bool)
	GetAllPlugins() []Plugin
	NotifyMediaFileScanned(mediaFileID, filePath string, metadata map[string]string) error
}

// Plugin represents a generic plugin interface that can be implemented
// by both core and external plugins
type Plugin interface {
	GetID() string
	GetName() string
	GetType() string
	GetVersion() string
	IsEnabled() bool
}

// AssetManagerInterface defines the asset management interface
// to replace interface{} usage in enrichment module
type AssetManagerInterface interface {
	SaveAsset(ctx context.Context, req *SaveAssetRequest) (*SaveAssetResponse, error)
	AssetExists(ctx context.Context, req *AssetExistsRequest) (*AssetExistsResponse, error)
	RemoveAsset(ctx context.Context, req *RemoveAssetRequest) (*RemoveAssetResponse, error)
}

// ScannerManagerInterface defines the scanner management interface
// to replace interface{} usage in media module
type ScannerManagerInterface interface {
	GetAllScans() ([]ScanJob, error)
	TerminateScan(jobID uint32) error
	CleanupJobsByLibrary(libraryID uint32) (int64, error)
	CleanupOrphanedAssets() (int, int, error)
	CleanupOrphanedFiles() (int, error)
}

// ScanJob represents a minimal scan job for interface compliance
type ScanJob struct {
	ID        uint32 `json:"id"`
	LibraryID uint32 `json:"library_id"`
	Status    string `json:"status"`
}

// ConfigurationValue represents a strongly typed configuration value
// to replace map[string]interface{}
type ConfigurationValue struct {
	Type        string      `json:"type"`        // "string", "int", "bool", "float", "duration"
	Value       interface{} `json:"value"`       // Actual value
	Required    bool        `json:"required"`    // Whether this setting is required
	Description string      `json:"description"` // Human-readable description
	Default     interface{} `json:"default"`     // Default value if not set
	Validation  *Validation `json:"validation"`  // Validation rules
}

// Validation represents validation rules for configuration values
type Validation struct {
	MinValue      *float64      `json:"min_value,omitempty"`      // Minimum numeric value
	MaxValue      *float64      `json:"max_value,omitempty"`      // Maximum numeric value
	MinLength     *int          `json:"min_length,omitempty"`     // Minimum string length
	MaxLength     *int          `json:"max_length,omitempty"`     // Maximum string length
	Pattern       string        `json:"pattern,omitempty"`        // Regex pattern for strings
	AllowedValues []interface{} `json:"allowed_values,omitempty"` // Enum-like values
}

// TypedConfiguration replaces map[string]interface{} with structured configuration
type TypedConfiguration struct {
	Version  string                        `json:"version"`
	Settings map[string]ConfigurationValue `json:"settings"`
	Features map[string]bool               `json:"features"`
	Metadata ConfigurationMetadata         `json:"metadata"`
}

// ConfigurationMetadata provides metadata about the configuration
type ConfigurationMetadata struct {
	LastModified  time.Time `json:"last_modified"`
	ModifiedBy    string    `json:"modified_by"`
	SchemaVersion string    `json:"schema_version"`
	Source        string    `json:"source"` // "file", "api", "env"
}

// GetTypedValue safely extracts a typed value from configuration
func (cv *ConfigurationValue) GetTypedValue() (interface{}, error) {
	switch cv.Type {
	case "string":
		if str, ok := cv.Value.(string); ok {
			return str, nil
		}
		return "", ErrInvalidConfigType
	case "int":
		switch v := cv.Value.(type) {
		case int:
			return v, nil
		case int64:
			return int(v), nil
		case float64:
			return int(v), nil
		default:
			return 0, ErrInvalidConfigType
		}
	case "bool":
		if b, ok := cv.Value.(bool); ok {
			return b, nil
		}
		return false, ErrInvalidConfigType
	case "float":
		switch v := cv.Value.(type) {
		case float64:
			return v, nil
		case int:
			return float64(v), nil
		case int64:
			return float64(v), nil
		default:
			return 0.0, ErrInvalidConfigType
		}
	case "duration":
		if str, ok := cv.Value.(string); ok {
			return time.ParseDuration(str)
		}
		return time.Duration(0), ErrInvalidConfigType
	default:
		return cv.Value, nil
	}
}

// Common errors
var (
	ErrInvalidConfigType = fmt.Errorf("invalid configuration type")
	ErrValidationFailed  = fmt.Errorf("configuration validation failed")
)

// MetadataMap provides a strongly typed wrapper for metadata
type MetadataMap struct {
	data map[string]string
}

// NewMetadataMap creates a new metadata map from various sources
func NewMetadataMap(source interface{}) *MetadataMap {
	mm := &MetadataMap{
		data: make(map[string]string),
	}

	switch v := source.(type) {
	case map[string]string:
		mm.data = v
	case map[string]interface{}:
		for key, value := range v {
			if str, ok := value.(string); ok {
				mm.data[key] = str
			} else {
				mm.data[key] = fmt.Sprintf("%v", value)
			}
		}
	case nil:
		// Empty map is fine
	default:
		// Log warning about unsupported type
	}

	return mm
}

// Get retrieves a value from the metadata map
func (mm *MetadataMap) Get(key string) (string, bool) {
	value, exists := mm.data[key]
	return value, exists
}

// Set sets a value in the metadata map
func (mm *MetadataMap) Set(key, value string) {
	mm.data[key] = value
}

// ToMap returns the underlying map[string]string
func (mm *MetadataMap) ToMap() map[string]string {
	result := make(map[string]string)
	for k, v := range mm.data {
		result[k] = v
	}
	return result
}

// AsInterface returns map[string]interface{} for backwards compatibility
func (mm *MetadataMap) AsInterface() map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range mm.data {
		result[k] = v
	}
	return result
}
