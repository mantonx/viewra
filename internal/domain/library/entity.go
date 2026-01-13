package library

import (
	"path/filepath"
	"strings"
	"time"
)

// Library represents a media library containing movies, TV shows, or music
type Library struct {
	ID        int64
	Name      string
	Path      string
	Type      LibraryType
	CreatedAt time.Time
	UpdatedAt time.Time

	// Filesystem monitoring configuration
	MonitoringEnabled bool              // Whether real-time file monitoring is enabled
	MonitoringConfig  *MonitoringConfig // Optional configuration overrides

	// LastScannedAt is when a scan last completed successfully for this library.
	// Nil means the library has never been scanned.
	LastScannedAt *time.Time
}

// MonitoringConfig contains per-library monitoring settings.
// All fields are optional - zero values use defaults.
type MonitoringConfig struct {
	// Priority for enrichment queue (default: 1000 = PriorityInteractive)
	Priority int `json:"priority,omitempty"`

	// PollingIntervalMinutes is the interval for polling-based monitoring
	// on network drives where fsnotify doesn't work (default: 60)
	PollingIntervalMinutes int `json:"polling_interval_minutes,omitempty"`

	// DebounceSeconds is the window to collect events before processing
	// to handle bulk file operations (default: 5)
	DebounceSeconds int `json:"debounce_seconds,omitempty"`
}

// DefaultMonitoringConfig returns the default monitoring configuration.
func DefaultMonitoringConfig() *MonitoringConfig {
	return &MonitoringConfig{
		Priority:               1000, // PriorityInteractive
		PollingIntervalMinutes: 5,    // Poll every 5 minutes for network drives
		DebounceSeconds:        5,
	}
}

// GetEffectiveConfig returns the monitoring config with defaults applied.
func (l *Library) GetEffectiveConfig() *MonitoringConfig {
	defaults := DefaultMonitoringConfig()
	if l.MonitoringConfig == nil {
		return defaults
	}

	// Apply non-zero values from library config
	if l.MonitoringConfig.Priority > 0 {
		defaults.Priority = l.MonitoringConfig.Priority
	}
	if l.MonitoringConfig.PollingIntervalMinutes > 0 {
		defaults.PollingIntervalMinutes = l.MonitoringConfig.PollingIntervalMinutes
	}
	if l.MonitoringConfig.DebounceSeconds > 0 {
		defaults.DebounceSeconds = l.MonitoringConfig.DebounceSeconds
	}

	return defaults
}

// IsValid validates the library entity
func (l *Library) IsValid() error {
	if err := l.validateName(); err != nil {
		return err
	}

	if err := l.validatePath(); err != nil {
		return err
	}

	return l.validateType()
}

// validateName checks if the library name is valid
func (l *Library) validateName() error {
	name := strings.TrimSpace(l.Name)
	if name == "" {
		return ErrInvalidName
	}

	if len(name) > 100 {
		return ErrNameTooLong
	}

	l.Name = name
	return nil
}

// validatePath validates the library path field
func (l *Library) validatePath() error {
	path := strings.TrimSpace(l.Path)

	if path == "" {
		return ErrEmptyPath
	}

	// Path must be absolute
	if !filepath.IsAbs(path) {
		return ErrPathNotAbsolute
	}

	// Clean path to normalize
	cleanPath := filepath.Clean(path)

	// Check for path traversal attempts (.. after cleaning)
	if strings.Contains(cleanPath, "..") {
		return ErrPathTraversal
	}

	l.Path = cleanPath
	return nil
}

// validateType checks if the library type is valid
func (l *Library) validateType() error {
	switch l.Type {
	case LibraryTypeMovies, LibraryTypeTV, LibraryTypeMusic:
		return nil
	default:
		return ErrInvalidType
	}
}
