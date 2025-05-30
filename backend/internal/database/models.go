package database

import (
	"time"
)

// User represents a user in the system
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"` // Don't include password in JSON responses
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Media represents a media file in the system
type Media struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Filename    string    `gorm:"not null" json:"filename"`
	OriginalName string   `gorm:"not null" json:"original_name"`
	Path        string    `gorm:"not null" json:"path"`
	Size        int64     `json:"size"`
	MimeType    string    `json:"mime_type"`
	Duration    *float64  `json:"duration,omitempty"` // For video/audio files
	Resolution  string    `json:"resolution,omitempty"` // For video/image files
	UserID      uint      `gorm:"not null" json:"user_id"`
	User        User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MediaFilter represents filters for media queries
type MediaFilter struct {
	UserID   *uint   `json:"user_id,omitempty"`
	MimeType *string `json:"mime_type,omitempty"`
	Limit    int     `json:"limit,omitempty"`
	Offset   int     `json:"offset,omitempty"`
}

// MediaLibrary represents a directory to scan for media files
type MediaLibrary struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Path      string    `gorm:"not null" json:"path"`
	Type      string    `gorm:"not null" json:"type"` // "movie" or "tv"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MediaLibraryRequest represents the request to create a new media library
type MediaLibraryRequest struct {
	Path string `json:"path" binding:"required"`
	Type string `json:"type" binding:"required,oneof=movie tv music"`
}

// MediaFile represents a scanned media file on disk
type MediaFile struct {
	ID         uint         `gorm:"primaryKey" json:"id"`
	LibraryID  uint         `gorm:"not null;index:idx_media_files_library_id" json:"library_id"`
	ScanJobID  *uint        `gorm:"index:idx_media_files_scan_job_id" json:"scan_job_id,omitempty"` // Track which job discovered this file
	Path       string       `gorm:"not null;uniqueIndex" json:"path"`
	Size       int64        `gorm:"not null" json:"size"`
	Hash       string       `gorm:"index" json:"hash"`
	MimeType   string       `json:"mime_type"`
	LastSeen   time.Time    `gorm:"not null" json:"last_seen"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
	MusicMetadata *MusicMetadata `gorm:"foreignKey:MediaFileID" json:"music_metadata,omitempty"`
}

// MusicMetadata represents extracted metadata from music files
type MusicMetadata struct {
	ID          uint          `gorm:"primaryKey" json:"id"`
	MediaFileID uint          `gorm:"uniqueIndex;not null" json:"media_file_id"`
	Title       string        `json:"title"`
	Album       string        `json:"album"`
	Artist      string        `json:"artist"`
	AlbumArtist string        `json:"album_artist"`
	Genre       string        `json:"genre"`
	Year        int           `json:"year"`
	Track       int           `json:"track"`
	TrackTotal  int           `json:"track_total"`
	Disc        int           `json:"disc"`
	DiscTotal   int           `json:"disc_total"`
	Duration    time.Duration `json:"duration"`
	Bitrate     int           `json:"bitrate"`
	SampleRate  int           `json:"sample_rate"`  // Sample rate in Hz (e.g., 44100, 48000)
	Channels    int           `json:"channels"`     // Number of audio channels (1=mono, 2=stereo, etc.)
	Format      string        `json:"format"`
	HasArtwork  bool          `json:"has_artwork"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	

	
	// Temporary fields for artwork processing (not stored in database)
	ArtworkData []byte `gorm:"-" json:"-"`
	ArtworkExt  string `gorm:"-" json:"-"`
}

// ScanJob represents a background scanning operation
type ScanJob struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	LibraryID uint      `gorm:"not null" json:"library_id"`
	Library   MediaLibrary `gorm:"foreignKey:LibraryID" json:"library,omitempty"`
	Status    string    `gorm:"not null;default:'pending'" json:"status"` // pending, running, completed, failed, paused
	Progress  int       `gorm:"default:0" json:"progress"` // 0-100
	FilesFound int      `gorm:"default:0" json:"files_found"`
	FilesProcessed int  `gorm:"default:0" json:"files_processed"`
	BytesProcessed int64 `gorm:"default:0" json:"bytes_processed"`
	ErrorMessage string `json:"error_message,omitempty"`
	StatusMessage string `json:"status_message,omitempty"` // For informational messages like recovery status
	StartedAt *time.Time `json:"started_at,omitempty"`
	ResumedAt *time.Time `json:"resumed_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// =============================================================================
// PLUGIN SYSTEM MODELS
// =============================================================================

// Plugin represents an installed plugin
type Plugin struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	PluginID     string    `gorm:"uniqueIndex;not null" json:"plugin_id"` // Unique plugin identifier
	Name         string    `gorm:"not null" json:"name"`
	Version      string    `gorm:"not null" json:"version"`
	Description  string    `json:"description"`
	Author       string    `json:"author"`
	Website      string    `json:"website,omitempty"`
	Repository   string    `json:"repository,omitempty"`
	License      string    `json:"license,omitempty"`
	Type         string    `gorm:"not null" json:"type"` // metadata_scraper, admin_page, ui_component, etc.
	Status       string    `gorm:"not null;default:'disabled'" json:"status"` // enabled, disabled, installing, error, updating
	InstallPath  string    `gorm:"not null" json:"install_path"`
	ManifestData string    `gorm:"type:text" json:"manifest_data"` // JSON-encoded manifest
	ConfigData   string    `gorm:"type:text" json:"config_data"`   // JSON-encoded configuration
	ErrorMessage string    `json:"error_message,omitempty"`
	InstalledAt  time.Time `json:"installed_at"`
	EnabledAt    *time.Time `json:"enabled_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// PluginPermission represents a permission granted to a plugin
type PluginPermission struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	PluginID   uint   `gorm:"not null;index" json:"plugin_id"`
	Plugin     Plugin `gorm:"foreignKey:PluginID" json:"plugin,omitempty"`
	Permission string `gorm:"not null" json:"permission"` // database_read, database_write, file_system, etc.
	Granted    bool   `gorm:"default:false" json:"granted"`
	GrantedAt  *time.Time `json:"granted_at,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// PluginEvent represents events generated by plugins
type PluginEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PluginID  uint      `gorm:"not null;index" json:"plugin_id"`
	Plugin    Plugin    `gorm:"foreignKey:PluginID" json:"plugin,omitempty"`
	EventType string    `gorm:"not null" json:"event_type"` // install, enable, disable, error, etc.
	Message   string    `json:"message"`
	Data      string    `gorm:"type:text" json:"data"` // JSON-encoded event data
	CreatedAt time.Time `json:"created_at"`
}

// SystemEvent represents a system event in the database (for the new event bus system)
type SystemEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	EventID   string    `gorm:"uniqueIndex;not null" json:"event_id"`
	Type      string    `gorm:"not null;index" json:"type"`
	Source    string    `gorm:"not null;index" json:"source"`
	Target    string    `gorm:"index" json:"target"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Data      string    `gorm:"type:text" json:"data"` // JSON-encoded event data
	Priority  int       `gorm:"not null;index" json:"priority"`
	Tags      string    `gorm:"type:text" json:"tags"` // JSON-encoded tags
	TTL       *int64    `json:"ttl"` // TTL in seconds
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PluginHook represents hooks that plugins can register for
type PluginHook struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PluginID  uint      `gorm:"not null;index" json:"plugin_id"`
	Plugin    Plugin    `gorm:"foreignKey:PluginID" json:"plugin,omitempty"`
	HookName  string    `gorm:"not null" json:"hook_name"` // file_scanned, metadata_extracted, etc.
	Handler   string    `gorm:"not null" json:"handler"`   // Function/endpoint to call
	Priority  int       `gorm:"default:100" json:"priority"` // Execution order (lower = earlier)
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PluginAdminPage represents admin pages provided by plugins
type PluginAdminPage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PluginID  uint      `gorm:"not null;index" json:"plugin_id"`
	Plugin    Plugin    `gorm:"foreignKey:PluginID" json:"plugin,omitempty"`
	PageID    string    `gorm:"not null" json:"page_id"`
	Title     string    `gorm:"not null" json:"title"`
	Path      string    `gorm:"not null" json:"path"`
	Icon      string    `json:"icon,omitempty"`
	Category  string    `json:"category,omitempty"`
	URL       string    `gorm:"not null" json:"url"`
	Type      string    `gorm:"not null" json:"type"` // iframe, module, component
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	SortOrder int       `gorm:"default:100" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PluginUIComponent represents UI components provided by plugins
type PluginUIComponent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PluginID  uint      `gorm:"not null;index" json:"plugin_id"`
	Plugin    Plugin    `gorm:"foreignKey:PluginID" json:"plugin,omitempty"`
	ComponentID string  `gorm:"not null" json:"component_id"`
	Name      string    `gorm:"not null" json:"name"`
	Type      string    `gorm:"not null" json:"type"` // widget, modal, page
	Props     string    `gorm:"type:text" json:"props"` // JSON-encoded props
	URL       string    `gorm:"not null" json:"url"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}


