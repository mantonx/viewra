package settings

import (
	"encoding/json"
	"time"
)

// ValueType represents the type of a setting value.
type ValueType string

const (
	TypeString ValueType = "string"
	TypeInt    ValueType = "int"
	TypeBool   ValueType = "bool"
	TypeJSON   ValueType = "json"
)

// Category represents a logical grouping of settings.
type Category string

const (
	CategoryServer      Category = "server"
	CategoryTranscoding Category = "transcoding"
	CategoryScanning    Category = "scanning"
	CategorySecurity    Category = "security"
	CategoryPlayback    Category = "playback"
	CategoryUI          Category = "ui"
	CategorySystem      Category = "system" // Read-only system info
	CategoryAI          Category = "ai"     // AI/LLM provider settings
)

// SettingSource indicates where a setting value comes from.
type SettingSource string

const (
	SourceDefault  SettingSource = "default"  // Using default value
	SourceDatabase SettingSource = "database" // Set via UI/API
	SourceEnvVar   SettingSource = "env_var"  // Overridden by environment variable
	SourceDetected SettingSource = "detected" // Auto-detected (read-only)
)

// EffectiveValue represents a resolved setting value with its source.
type EffectiveValue struct {
	Value    any           `json:"value"`
	Source   SettingSource `json:"source"`
	EnvVar   string        `json:"envVar,omitempty"` // Env var name if Source is env_var
	Locked   bool          `json:"locked"`           // True if cannot be changed via UI
	ReadOnly bool          `json:"readOnly"`         // True for display-only values
}

// SystemSetting represents a system-wide configuration setting.
type SystemSetting struct {
	Key         string
	Value       string // JSON-encoded
	ValueType   ValueType
	Category    Category
	Description string
	UpdatedAt   time.Time
	UpdatedBy   string // User public ID who last updated, may be empty
}

// UserSetting represents a per-user preference setting.
type UserSetting struct {
	UserID    string // User public ID
	Key       string
	Value     string // JSON-encoded
	UpdatedAt time.Time
}

// GetString returns the setting value as a string.
// Returns empty string if the value cannot be decoded.
func (s *SystemSetting) GetString() string {
	var v string
	if err := json.Unmarshal([]byte(s.Value), &v); err != nil {
		return ""
	}
	return v
}

// GetInt returns the setting value as an integer.
// Returns 0 if the value cannot be decoded.
func (s *SystemSetting) GetInt() int {
	var v int
	if err := json.Unmarshal([]byte(s.Value), &v); err != nil {
		return 0
	}
	return v
}

// GetBool returns the setting value as a boolean.
// Returns false if the value cannot be decoded.
func (s *SystemSetting) GetBool() bool {
	var v bool
	if err := json.Unmarshal([]byte(s.Value), &v); err != nil {
		return false
	}
	return v
}

// GetJSON unmarshals the setting value into the provided destination.
func (s *SystemSetting) GetJSON(dest any) error {
	return json.Unmarshal([]byte(s.Value), dest)
}

// GetString returns the user setting value as a string.
func (s *UserSetting) GetString() string {
	var v string
	if err := json.Unmarshal([]byte(s.Value), &v); err != nil {
		return ""
	}
	return v
}

// GetInt returns the user setting value as an integer.
func (s *UserSetting) GetInt() int {
	var v int
	if err := json.Unmarshal([]byte(s.Value), &v); err != nil {
		return 0
	}
	return v
}

// GetBool returns the user setting value as a boolean.
func (s *UserSetting) GetBool() bool {
	var v bool
	if err := json.Unmarshal([]byte(s.Value), &v); err != nil {
		return false
	}
	return v
}

// GetJSON unmarshals the user setting value into the provided destination.
func (s *UserSetting) GetJSON(dest any) error {
	return json.Unmarshal([]byte(s.Value), dest)
}

// EncodeValue encodes a value to JSON for storage.
func EncodeValue(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
