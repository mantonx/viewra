package plugins

import (
	"os"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"gorm.io/gorm"
)

// MediaItem represents a processed media file with its metadata
type MediaItem struct {
	MediaFile *database.MediaFile `json:"media_file"`
	Metadata  interface{}         `json:"metadata"`
	Type      string               `json:"type"` // "music", "video", "image", etc.
}

// MediaAsset represents an asset associated with a media file
type MediaAsset struct {
	Type        string            `json:"type"`         // "artwork", "subtitle", "thumbnail", "preview"
	Data        []byte            `json:"data"`         // Asset binary data
	Path        string            `json:"path"`         // Original file path for reference
	Extension   string            `json:"extension"`    // File extension (.jpg, .png, .srt, etc.)
	MediaFileID uint              `json:"media_file_id"` // Associated media file ID
	MimeType    string            `json:"mime_type"`    // MIME type of the asset
	Size        int64             `json:"size"`         // Size in bytes
	Metadata    map[string]string `json:"metadata,omitempty"` // Metadata about the asset source
}

// MediaContext provides context for media processing operations
type MediaContext struct {
	DB        *gorm.DB                `json:"-"`
	MediaFile *database.MediaFile     `json:"media_file"`
	LibraryID uint                    `json:"library_id"`
	EventBus  events.EventBus         `json:"-"`
}

// CoreMediaPlugin defines the interface for core media plugins with the new architecture
type CoreMediaPlugin interface {
	// Basic plugin info
	GetName() string
	GetVersion() string
	GetDescription() string
	
	// Plugin lifecycle
	IsEnabled() bool
	Enable() error
	Disable() error
	Initialize() error
	Shutdown() error
	
	// Media processing - new architecture
	HandleFile(path string, info os.FileInfo, ctx MediaContext) (*MediaItem, []MediaAsset, error)
	GetMediaType() string
	GetSupportedExtensions() []string
	GetPriority() int
	
	// Legacy compatibility for MediaHandlerPlugin
	Match(path string, info os.FileInfo) bool
	HandleMediaFile(path string, info os.FileInfo) error
}

// MediaHandlerPlugin defines the interface for handling media files (legacy)
type MediaHandlerPlugin interface {
	GetName() string
	GetMediaType() string
	GetSupportedExtensions() []string
	Match(path string, info os.FileInfo) bool
	HandleMediaFile(path string, info os.FileInfo) error
}

// MediaScannerHook defines callbacks for media scanning events
type MediaScannerHook interface {
	OnMediaFileScanned(mediaFile *database.MediaFile, metadata interface{}) error
}

// MediaPluginInfo contains metadata about a media plugin
type MediaPluginInfo struct {
	Name            string   `json:"name"`
	MediaType       string   `json:"media_type"`
	Version         string   `json:"version"`
	Description     string   `json:"description"`
	SupportedExts   []string `json:"supported_extensions"`
	Enabled         bool     `json:"enabled"`
	IsCore          bool     `json:"is_core"`
} 