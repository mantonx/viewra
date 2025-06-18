package database

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// User represents a user in the system
type User struct {
	ID        uint32    `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"` // Don't include password in JSON responses
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MediaLibrary represents a directory to scan for media files
type MediaLibrary struct {
	ID        uint32    `gorm:"primaryKey" json:"id"`
	Path      string    `gorm:"not null" json:"path"`
	Type      string    `gorm:"not null" json:"type"` // "movie", "tv", "music"
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MediaLibraryRequest represents the request to create a new media library
type MediaLibraryRequest struct {
	Path string `json:"path" binding:"required"`
	Type string `json:"type" binding:"required,oneof=movie tv music"`
}

// MediaType enum for media_files.media_type and related fields
type MediaType string

const (
	MediaTypeMovie   MediaType = "movie"
	MediaTypeEpisode MediaType = "episode"
	MediaTypeTrack   MediaType = "track"
	MediaTypeImage   MediaType = "image"
)

func (mt MediaType) Value() (driver.Value, error) {
	return string(mt), nil
}

func (mt *MediaType) Scan(value interface{}) error {
	if value == nil {
		*mt = ""
		return nil
	}
	switch s := value.(type) {
	case string:
		*mt = MediaType(s)
	case []byte:
		*mt = MediaType(s)
	default:
		return fmt.Errorf("cannot scan %T into MediaType", value)
	}
	return nil
}

// =============================================================================
// CORE MEDIA FILES TABLE
// =============================================================================

// MediaFile represents each file version of a media item
type MediaFile struct {
	ID          string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	MediaID     string    `gorm:"type:varchar(36);not null;index" json:"media_id"` // FK to movie, episode, or track
	MediaType   MediaType `gorm:"type:text;not null;index" json:"media_type"`      // ENUM: movie, episode, track
	LibraryID   uint32    `gorm:"not null;index" json:"library_id"`                // FK to MediaLibrary
	ScanJobID   *uint32   `gorm:"index" json:"scan_job_id,omitempty"`              // Track which job discovered this file
	Path        string    `gorm:"not null;uniqueIndex" json:"path"`                // Absolute or relative file path
	Container   string    `json:"container"`                                       // e.g., mkv, mp4, flac
	VideoCodec  string    `json:"video_codec"`                                     // Optional: h264, vp9, etc.
	AudioCodec  string    `json:"audio_codec"`                                     // Optional: aac, flac, dts
	Channels    string    `json:"channels"`                                        // e.g., 2.0, 5.1, 7.1
	SampleRate  int       `json:"sample_rate"`                                     // Audio sample rate in Hz (e.g., 44100, 48000, 96000)
	Resolution  string    `json:"resolution"`                                      // e.g., 1080p, 4K
	Duration    int       `json:"duration"`                                        // In seconds
	SizeBytes   int64     `gorm:"not null" json:"size_bytes"`                      // File size
	BitrateKbps int       `json:"bitrate_kbps"`                                    // Total bitrate estimate
	Language    string    `json:"language"`                                        // Default language (e.g. en)
	Hash        string    `gorm:"index" json:"hash"`                               // SHA256 or similar, for deduplication
	VersionName string    `json:"version_name"`                                    // Optional (e.g., "Director's Cut", "Remastered")

	// Comprehensive technical metadata as JSON (from FFmpeg probe)
	TechnicalInfo   string `gorm:"type:text" json:"technical_info"`   // Complete VideoTechnicalInfo as JSON
	VideoStreams    string `gorm:"type:text" json:"video_streams"`    // VideoStreamInfo array as JSON
	AudioStreams    string `gorm:"type:text" json:"audio_streams"`    // AudioStreamInfo array as JSON
	SubtitleStreams string `gorm:"type:text" json:"subtitle_streams"` // SubtitleStreamInfo array as JSON

	// Enhanced technical fields for easier querying
	VideoWidth      int    `json:"video_width"`      // Video width in pixels
	VideoHeight     int    `json:"video_height"`     // Video height in pixels
	VideoFramerate  string `json:"video_framerate"`  // Video framerate (e.g., "23.976")
	VideoProfile    string `json:"video_profile"`    // Video codec profile (e.g., "Main 10")
	VideoLevel      int    `json:"video_level"`      // Video codec level
	VideoBitDepth   string `json:"video_bit_depth"`  // Video bit depth
	AspectRatio     string `json:"aspect_ratio"`     // Display aspect ratio
	PixelFormat     string `json:"pixel_format"`     // Pixel format (e.g., "yuv420p10le")
	ColorSpace      string `json:"color_space"`      // Color space (e.g., "bt709")
	ColorPrimaries  string `json:"color_primaries"`  // Color primaries
	ColorTransfer   string `json:"color_transfer"`   // Color transfer
	HDRFormat       string `json:"hdr_format"`       // HDR format (HDR10, DV, etc.)
	Interlaced      string `json:"interlaced"`       // Interlaced status
	ReferenceFrames int    `json:"reference_frames"` // Number of reference frames

	// Enhanced audio fields
	AudioChannels   int    `json:"audio_channels"`    // Number of audio channels
	AudioLayout     string `json:"audio_layout"`      // Audio channel layout
	AudioSampleRate int    `json:"audio_sample_rate"` // Audio sample rate
	AudioBitDepth   int    `json:"audio_bit_depth"`   // Audio bit depth
	AudioLanguage   string `json:"audio_language"`    // Primary audio language
	AudioProfile    string `json:"audio_profile"`     // Audio codec profile

	LastSeen  time.Time `gorm:"not null" json:"last_seen"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// =============================================================================
// SHARED ASSET TABLE
// =============================================================================

// MediaAsset handles associated assets (artwork, subtitles, thumbnails)
// Using the new clean entity-based schema
type MediaAsset struct {
	ID string `gorm:"type:varchar(36);primaryKey" json:"id"`

	// Entity-based fields (clean schema)
	EntityType string `gorm:"type:text;not null;index" json:"entity_type"`      // movie, tv_show, album, etc.
	EntityID   string `gorm:"type:varchar(36);not null;index" json:"entity_id"` // FK to movie, episode, track, etc.
	Type       string `gorm:"not null;index" json:"type"`                       // poster, backdrop, logo, cover, etc.
	Source     string `gorm:"not null;index" json:"source"`                     // plugin, local, user, etc.
	PluginID   string `gorm:"index" json:"plugin_id,omitempty"`                 // Specific plugin identifier

	// Asset details
	Path      string `gorm:"not null" json:"path"`           // File path or URL
	Language  string `json:"language,omitempty"`             // For subtitles, audio tracks
	Format    string `json:"format"`                         // File format/codec
	Width     int    `gorm:"default:0" json:"width"`         // Image width
	Height    int    `gorm:"default:0" json:"height"`        // Image height
	Preferred bool   `gorm:"default:false" json:"preferred"` // Preferred asset flag

	// Optional compatibility fields
	Resolution string `json:"resolution,omitempty"`        // Calculated from width x height
	SizeBytes  int64  `gorm:"default:0" json:"size_bytes"` // File size

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// =============================================================================
// PEOPLE AND ROLES TABLES
// =============================================================================

// People - Unified table for cast, crew, artists
type People struct {
	ID        string     `gorm:"type:varchar(36);primaryKey" json:"id"`
	Name      string     `gorm:"not null;index" json:"name"`
	Birthdate *time.Time `json:"birthdate"` // Optional
	Image     string     `json:"image"`     // URL or path to portrait
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Roles - Many-to-many relationship between people and media entities
type Roles struct {
	PersonID  string    `gorm:"type:varchar(36);not null;index" json:"person_id"` // FK to people
	MediaID   string    `gorm:"type:varchar(36);not null;index" json:"media_id"`  // FK to movie, episode, or track
	MediaType MediaType `gorm:"type:text;not null;index" json:"media_type"`       // ENUM: movie, episode, track
	Role      string    `gorm:"not null;index" json:"role"`                       // e.g. director, actor, composer, guest
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// =============================================================================
// MUSIC TABLES
// =============================================================================

// Artist table
type Artist struct {
	ID          string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	Name        string    `gorm:"not null;index" json:"name"`
	Description string    `json:"description"`
	Image       string    `json:"image"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Album table
type Album struct {
	ID          string     `gorm:"type:varchar(36);primaryKey" json:"id"`
	Title       string     `gorm:"not null;index" json:"title"`
	ArtistID    string     `gorm:"type:varchar(36);not null;index" json:"artist_id"` // FK to Artist
	Artist      Artist     `gorm:"foreignKey:ArtistID" json:"artist,omitempty"`
	ReleaseDate *time.Time `json:"release_date"`
	Artwork     string     `json:"artwork"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Track table
type Track struct {
	ID          string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	Title       string    `gorm:"not null;index" json:"title"`
	AlbumID     string    `gorm:"type:varchar(36);not null;index" json:"album_id"` // FK to Album
	Album       Album     `gorm:"foreignKey:AlbumID" json:"album,omitempty"`
	ArtistID    string    `gorm:"type:varchar(36);not null;index" json:"artist_id"` // FK to Artist
	Artist      Artist    `gorm:"foreignKey:ArtistID" json:"artist,omitempty"`
	TrackNumber int       `json:"track_number"`
	Duration    int       `json:"duration"` // In seconds
	Lyrics      string    `gorm:"type:text" json:"lyrics"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// =============================================================================
// MOVIE TABLES
// =============================================================================

// Movie table
type Movie struct {
	ID            string     `gorm:"type:varchar(36);primaryKey" json:"id"`
	Title         string     `gorm:"not null;index" json:"title"`
	OriginalTitle string     `json:"original_title"`
	Overview      string     `gorm:"type:text" json:"overview"`
	Tagline       string     `json:"tagline"`
	ReleaseDate   *time.Time `json:"release_date"`
	Runtime       int        `json:"runtime"` // In minutes

	// Ratings & Compliance
	Rating     string  `json:"rating"`      // e.g. PG-13, R, etc.
	TmdbRating float64 `json:"tmdb_rating"` // TMDb vote average
	VoteCount  int     `json:"vote_count"`
	Popularity float64 `json:"popularity"`

	// Status & Production Info
	Status string `json:"status"` // Released, In Production, Post Production, etc.
	Adult  bool   `json:"adult"`  // Adult content flag
	Video  bool   `json:"video"`  // Video release flag

	// Financial Data
	Budget  int64 `json:"budget"`  // Production budget in USD
	Revenue int64 `json:"revenue"` // Global box office in USD

	// Artwork & Media
	Poster   string `json:"poster"`
	Backdrop string `json:"backdrop"`

	// Text Data & Metadata (stored as JSON for flexibility)
	Genres              string `gorm:"type:text" json:"genres"`               // JSON array of genres
	ProductionCompanies string `gorm:"type:text" json:"production_companies"` // JSON array of companies
	ProductionCountries string `gorm:"type:text" json:"production_countries"` // JSON array of countries
	SpokenLanguages     string `gorm:"type:text" json:"spoken_languages"`     // JSON array of languages
	Keywords            string `gorm:"type:text" json:"keywords"`             // JSON array of keywords/tags

	// Cast & Crew (summary data)
	MainCast string `gorm:"type:text" json:"main_cast"` // JSON array of main cast
	MainCrew string `gorm:"type:text" json:"main_crew"` // JSON array of main crew

	// External IDs & Links
	TmdbID      string `gorm:"index" json:"tmdb_id"`
	ImdbID      string `gorm:"index" json:"imdb_id"`
	ExternalIDs string `gorm:"type:text" json:"external_ids"` // JSON object with all external IDs

	// Language & Region
	OriginalLanguage string `json:"original_language"`

	// Franchise & Collection
	Collection string `gorm:"type:text" json:"collection"` // JSON object if part of collection/franchise

	// Awards & Recognition (can be populated by other plugins)
	Awards string `gorm:"type:text" json:"awards"` // JSON array of awards

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// =============================================================================
// TV SHOW TABLES
// =============================================================================

// TVShow table
type TVShow struct {
	ID           string     `gorm:"type:varchar(36);primaryKey" json:"id"`
	Title        string     `gorm:"not null;index" json:"title"`
	Description  string     `gorm:"type:text" json:"description"`
	FirstAirDate *time.Time `json:"first_air_date"`
	Status       string     `json:"status"` // e.g., Running, Ended
	Poster       string     `json:"poster"`
	Backdrop     string     `json:"backdrop"`
	TmdbID       string     `gorm:"index" json:"tmdb_id"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Season table
type Season struct {
	ID           string     `gorm:"type:varchar(36);primaryKey" json:"id"`
	TVShowID     string     `gorm:"type:varchar(36);not null;index" json:"tv_show_id"` // FK to TVShow
	TVShow       TVShow     `gorm:"foreignKey:TVShowID" json:"tv_show,omitempty"`
	SeasonNumber int        `gorm:"not null;index" json:"season_number"`
	Description  string     `gorm:"type:text" json:"description"`
	Poster       string     `json:"poster"`
	AirDate      *time.Time `json:"air_date"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Episode table
type Episode struct {
	ID            string     `gorm:"type:varchar(36);primaryKey" json:"id"`
	SeasonID      string     `gorm:"type:varchar(36);not null;index" json:"season_id"` // FK to Season
	Season        Season     `gorm:"foreignKey:SeasonID" json:"season,omitempty"`
	Title         string     `gorm:"not null;index" json:"title"`
	EpisodeNumber int        `gorm:"not null;index" json:"episode_number"`
	AirDate       *time.Time `json:"air_date"`
	Description   string     `gorm:"type:text" json:"description"`
	Duration      int        `json:"duration"` // In seconds
	StillImage    string     `json:"still_image"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// =============================================================================
// METADATA ENRICHMENT TABLES
// =============================================================================

// MediaExternalIDs - Handles multiple metadata sources
type MediaExternalIDs struct {
	MediaID    string    `gorm:"type:varchar(36);not null;index" json:"media_id"`
	MediaType  MediaType `gorm:"type:text;not null;index" json:"media_type"`
	Source     string    `gorm:"not null;index" json:"source"` // e.g. tmdb, tvdb, musicbrainz
	ExternalID string    `gorm:"not null" json:"external_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// MediaEnrichment - Stores raw enriched metadata blobs
type MediaEnrichment struct {
	MediaID   string    `gorm:"type:varchar(36);not null;index" json:"media_id"`
	MediaType MediaType `gorm:"type:text;not null;index" json:"media_type"`
	Plugin    string    `gorm:"not null;index" json:"plugin"`
	Payload   string    `gorm:"type:text" json:"payload"` // Use TEXT for SQLite compatibility
	UpdatedAt time.Time `json:"updated_at"`
}

// =============================================================================
// SCAN JOB (remains mostly the same)
// =============================================================================

// ScanJob represents a background scanning operation
type ScanJob struct {
	ID             uint32       `gorm:"primaryKey" json:"id"`
	LibraryID      uint32       `gorm:"not null;index:idx_scan_jobs_library_id" json:"library_id"`
	Library        MediaLibrary `gorm:"foreignKey:LibraryID" json:"library,omitempty"`
	Status         string       `gorm:"not null;default:'pending'" json:"status"` // pending, running, completed, failed, paused
	Progress       float64      `gorm:"default:0" json:"progress"`                // 0.0-100.0 with decimal precision
	FilesFound     int          `gorm:"default:0" json:"files_found"`
	FilesProcessed int          `gorm:"default:0" json:"files_processed"`
	FilesSkipped   int          `gorm:"default:0" json:"files_skipped"`
	BytesProcessed int64        `gorm:"default:0" json:"bytes_processed"`
	ErrorMessage   string       `json:"error_message,omitempty"`
	StatusMessage  string       `json:"status_message,omitempty"` // For informational messages like recovery status
	StartedAt      *time.Time   `json:"started_at,omitempty"`
	ResumedAt      *time.Time   `json:"resumed_at,omitempty"`
	CompletedAt    *time.Time   `json:"completed_at,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// =============================================================================
// PLUGIN SYSTEM MODELS (unchanged)
// =============================================================================

// Plugin represents an installed plugin
type Plugin struct {
	ID           uint32     `gorm:"primaryKey" json:"id"`
	PluginID     string     `gorm:"uniqueIndex;not null" json:"plugin_id"` // Unique plugin identifier
	Name         string     `gorm:"not null" json:"name"`
	Version      string     `gorm:"not null" json:"version"`
	Description  string     `json:"description"`
	Author       string     `json:"author"`
	Website      string     `json:"website,omitempty"`
	Repository   string     `json:"repository,omitempty"`
	License      string     `json:"license,omitempty"`
	Type         string     `gorm:"not null" json:"type"`                      // metadata_scraper, admin_page, ui_component, etc.
	Status       string     `gorm:"not null;default:'disabled'" json:"status"` // enabled, disabled, installing, error, updating
	InstallPath  string     `gorm:"not null" json:"install_path"`
	ManifestData string     `gorm:"type:text" json:"manifest_data"` // JSON-encoded manifest
	ConfigData   string     `gorm:"type:text" json:"config_data"`   // JSON-encoded configuration
	ErrorMessage string     `json:"error_message,omitempty"`
	InstalledAt  time.Time  `json:"installed_at"`
	EnabledAt    *time.Time `json:"enabled_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// PluginPermission represents a permission granted to a plugin
type PluginPermission struct {
	ID         uint32     `gorm:"primaryKey" json:"id"`
	PluginID   string     `gorm:"not null;index" json:"plugin_id"` // FK to Plugin.PluginID (string)
	Permission string     `gorm:"not null" json:"permission"`      // database_read, database_write, file_system, etc.
	Granted    bool       `gorm:"default:false" json:"granted"`
	GrantedAt  *time.Time `json:"granted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// PluginEvent represents events generated by plugins
type PluginEvent struct {
	ID        uint32    `gorm:"primaryKey" json:"id"`
	PluginID  string    `gorm:"not null;index" json:"plugin_id"` // FK to Plugin.PluginID (string)
	EventType string    `gorm:"not null" json:"event_type"`      // install, enable, disable, error, etc.
	Message   string    `json:"message"`
	Data      string    `gorm:"type:text" json:"data"` // JSON-encoded event data
	CreatedAt time.Time `json:"created_at"`
}

// SystemEvent represents a system event in the database (for the new event bus system)
type SystemEvent struct {
	ID        uint32    `gorm:"primaryKey" json:"id"`
	EventID   string    `gorm:"uniqueIndex;not null" json:"event_id"`
	Type      string    `gorm:"not null;index" json:"type"`
	Source    string    `gorm:"not null;index" json:"source"`
	Target    string    `gorm:"index" json:"target"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Data      string    `gorm:"type:text" json:"data"` // JSON-encoded event data
	Priority  int       `gorm:"not null;index" json:"priority"`
	Tags      string    `gorm:"type:text" json:"tags"` // JSON-encoded tags
	TTL       *int64    `json:"ttl"`                   // TTL in seconds
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PluginHook represents hooks that plugins can register for
type PluginHook struct {
	ID        uint32    `gorm:"primaryKey" json:"id"`
	PluginID  string    `gorm:"not null;index" json:"plugin_id"` // FK to Plugin.PluginID (string)
	HookName  string    `gorm:"not null" json:"hook_name"`       // file_scanned, metadata_extracted, etc.
	Handler   string    `gorm:"not null" json:"handler"`         // Function/endpoint to call
	Priority  int       `gorm:"default:100" json:"priority"`     // Execution order (lower = earlier)
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PluginAdminPage represents admin pages provided by plugins
type PluginAdminPage struct {
	ID        uint32    `gorm:"primaryKey" json:"id"`
	PluginID  string    `gorm:"not null;index" json:"plugin_id"` // FK to Plugin.PluginID (string)
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
	ID          uint32    `gorm:"primaryKey" json:"id"`
	PluginID    string    `gorm:"not null;index" json:"plugin_id"` // FK to Plugin.PluginID (string)
	ComponentID string    `gorm:"not null" json:"component_id"`
	Name        string    `gorm:"not null" json:"name"`
	Type        string    `gorm:"not null" json:"type"`   // widget, modal, page
	Props       string    `gorm:"type:text" json:"props"` // JSON-encoded props
	URL         string    `gorm:"not null" json:"url"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PluginConfiguration represents stored configuration for a plugin
type PluginConfiguration struct {
	ID           uint32    `gorm:"primaryKey" json:"id"`
	PluginID     string    `gorm:"uniqueIndex;not null" json:"plugin_id"` // FK to Plugin.PluginID (string)
	SchemaData   string    `gorm:"type:text" json:"schema_data"`          // JSON-encoded configuration schema
	SettingsData string    `gorm:"type:text" json:"settings_data"`        // JSON-encoded configuration values
	Version      string    `gorm:"not null" json:"version"`
	ModifiedBy   string    `json:"modified_by,omitempty"`
	Dependencies string    `gorm:"type:text" json:"dependencies"` // JSON array of dependency plugin IDs
	Permissions  string    `gorm:"type:text" json:"permissions"`  // JSON array of required permissions
	IsActive     bool      `gorm:"default:true" json:"is_active"` // Whether this configuration is active
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
