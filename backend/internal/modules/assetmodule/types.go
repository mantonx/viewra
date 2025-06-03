package assetmodule

import (
	"time"

	"github.com/google/uuid"
)

// EntityType represents the main entity type for media assets
type EntityType string

const (
	EntityTypeArtist     EntityType = "artist"
	EntityTypeAlbum      EntityType = "album"
	EntityTypeTrack      EntityType = "track"
	EntityTypeMovie      EntityType = "movie"
	EntityTypeTVShow     EntityType = "tv_show"
	EntityTypeEpisode    EntityType = "episode"
	EntityTypeDirector   EntityType = "director"
	EntityTypeActor      EntityType = "actor"
	EntityTypeStudio     EntityType = "studio"
	EntityTypeLabel      EntityType = "label"
	EntityTypeNetwork    EntityType = "network"
	EntityTypeGenre      EntityType = "genre"
	EntityTypeCollection EntityType = "collection"
)

// AssetType represents the specific type of asset
type AssetType string

const (
	// Universal types
	AssetTypeLogo       AssetType = "logo"
	AssetTypePhoto      AssetType = "photo"
	AssetTypeBackground AssetType = "background"
	AssetTypeBanner     AssetType = "banner"
	AssetTypeThumb      AssetType = "thumb"
	AssetTypeFanart     AssetType = "fanart"

	// Artist specific
	AssetTypeClearart AssetType = "clearart"

	// Album/Collection specific
	AssetTypeCover   AssetType = "cover"
	AssetTypeDisc    AssetType = "disc"
	AssetTypeBooklet AssetType = "booklet"

	// Track specific
	AssetTypeWaveform    AssetType = "waveform"
	AssetTypeSpectrogram AssetType = "spectrogram"

	// Movie/TV specific
	AssetTypePoster AssetType = "poster"

	// TV Show specific
	AssetTypeNetworkLogo AssetType = "network_logo"

	// Episode specific
	AssetTypeScreenshot AssetType = "screenshot"

	// Actor/Director specific
	AssetTypeHeadshot  AssetType = "headshot"
	AssetTypePortrait  AssetType = "portrait"
	AssetTypeSignature AssetType = "signature"

	// Studio/Label specific
	AssetTypeHQPhoto AssetType = "hq_photo"

	// Genre specific
	AssetTypeIcon AssetType = "icon"
)

// AssetSource represents where the asset originated from
type AssetSource string

const (
	SourceLocal    AssetSource = "local"
	SourceUser     AssetSource = "user"
	SourceCore     AssetSource = "core"   // For core system plugins
	SourcePlugin   AssetSource = "plugin" // For external plugins
	SourceEmbedded AssetSource = "embedded"
)

// MediaAsset represents a media asset stored in the filesystem
type MediaAsset struct {
	ID uuid.UUID `gorm:"type:uuid;primary_key;default:(lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6))))" json:"id"`

	// Legacy fields for backward compatibility (required by database schema)
	MediaID   string `gorm:"type:varchar(36);not null;index" json:"media_id,omitempty"`
	MediaType string `gorm:"type:text;not null;index" json:"media_type,omitempty"`
	AssetType string `gorm:"not null;index" json:"asset_type,omitempty"`

	// New entity-based fields
	EntityType EntityType  `gorm:"not null;index:idx_media_assets_entity" json:"entity_type"`
	EntityID   uuid.UUID   `gorm:"type:uuid;not null;index:idx_media_assets_entity" json:"entity_id"`
	Type       AssetType   `gorm:"not null;index:idx_media_assets_type" json:"type"`
	Source     AssetSource `gorm:"not null;index:idx_media_assets_source" json:"source"`
	PluginID   string      `gorm:"index:idx_media_assets_plugin" json:"plugin_id,omitempty"` // Specific plugin identifier when source is plugin/core
	Path       string      `gorm:"not null" json:"path"`
	Width      int         `gorm:"default:0" json:"width"`
	Height     int         `gorm:"default:0" json:"height"`
	Format     string      `gorm:"not null" json:"format"` // MIME type
	Preferred  bool        `gorm:"default:false" json:"preferred"`
	Language   string      `gorm:"default:''" json:"language,omitempty"`

	// Compatibility fields from legacy schema
	SizeBytes  int64  `gorm:"default:0" json:"size_bytes"`
	IsDefault  bool   `gorm:"default:false" json:"is_default"`
	Resolution string `json:"resolution,omitempty"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName returns the table name for MediaAsset
func (MediaAsset) TableName() string {
	return "media_assets"
}

// AssetRequest represents a request to save a media asset
type AssetRequest struct {
	EntityType EntityType  `json:"entity_type" binding:"required"`
	EntityID   uuid.UUID   `json:"entity_id" binding:"required"`
	Type       AssetType   `json:"type" binding:"required"`
	Source     AssetSource `json:"source" binding:"required"`
	PluginID   string      `json:"plugin_id,omitempty"` // Specific plugin identifier when source is plugin/core
	Data       []byte      `json:"data" binding:"required"`
	Width      int         `json:"width,omitempty"`
	Height     int         `json:"height,omitempty"`
	Format     string      `json:"format" binding:"required"` // MIME type
	Preferred  bool        `json:"preferred,omitempty"`
	Language   string      `json:"language,omitempty"`
}

// AssetResponse represents the response when retrieving a media asset
type AssetResponse struct {
	ID         uuid.UUID   `json:"id"`
	EntityType EntityType  `json:"entity_type"`
	EntityID   uuid.UUID   `json:"entity_id"`
	Type       AssetType   `json:"type"`
	Source     AssetSource `json:"source"`
	PluginID   string      `json:"plugin_id,omitempty"`
	Path       string      `json:"path"`
	Width      int         `json:"width"`
	Height     int         `json:"height"`
	Format     string      `json:"format"`
	Preferred  bool        `json:"preferred"`
	Language   string      `json:"language,omitempty"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

// AssetFilter represents filters for querying assets
type AssetFilter struct {
	EntityType EntityType  `json:"entity_type,omitempty"`
	EntityID   uuid.UUID   `json:"entity_id,omitempty"`
	Type       AssetType   `json:"type,omitempty"`
	Source     AssetSource `json:"source,omitempty"`
	PluginID   string      `json:"plugin_id,omitempty"`
	Preferred  *bool       `json:"preferred,omitempty"`
	Language   string      `json:"language,omitempty"`
	Limit      int         `json:"limit,omitempty"`
	Offset     int         `json:"offset,omitempty"`
}

// AssetStats represents statistics about stored assets
type AssetStats struct {
	TotalAssets      int64                 `json:"total_assets"`
	TotalSize        int64                 `json:"total_size"`
	AssetsByEntity   map[EntityType]int64  `json:"assets_by_entity"`
	AssetsByType     map[AssetType]int64   `json:"assets_by_type"`
	AssetsBySource   map[AssetSource]int64 `json:"assets_by_source"`
	AverageSize      float64               `json:"average_size"`
	LargestAssetSize int64                 `json:"largest_asset_size"`
	PreferredAssets  int64                 `json:"preferred_assets"`
	SupportedFormats []string              `json:"supported_formats"`
}

// GetValidEntityTypes returns all valid entity types
func GetValidEntityTypes() []EntityType {
	return []EntityType{
		EntityTypeArtist,
		EntityTypeAlbum,
		EntityTypeTrack,
		EntityTypeMovie,
		EntityTypeTVShow,
		EntityTypeEpisode,
		EntityTypeDirector,
		EntityTypeActor,
		EntityTypeStudio,
		EntityTypeLabel,
		EntityTypeNetwork,
		EntityTypeGenre,
		EntityTypeCollection,
	}
}

// GetValidAssetTypes returns valid asset types for a given entity type
func GetValidAssetTypes(entityType EntityType) []AssetType {
	switch entityType {
	case EntityTypeArtist:
		return []AssetType{AssetTypeLogo, AssetTypePhoto, AssetTypeBackground, AssetTypeBanner, AssetTypeThumb, AssetTypeClearart, AssetTypeFanart}
	case EntityTypeAlbum:
		return []AssetType{AssetTypeCover, AssetTypeThumb, AssetTypeDisc, AssetTypeBackground, AssetTypeBooklet}
	case EntityTypeTrack:
		return []AssetType{AssetTypeWaveform, AssetTypeSpectrogram, AssetTypeCover}
	case EntityTypeMovie:
		return []AssetType{AssetTypePoster, AssetTypeLogo, AssetTypeBanner, AssetTypeBackground, AssetTypeThumb, AssetTypeFanart}
	case EntityTypeTVShow:
		return []AssetType{AssetTypePoster, AssetTypeLogo, AssetTypeBanner, AssetTypeBackground, AssetTypeNetworkLogo, AssetTypeThumb, AssetTypeFanart}
	case EntityTypeEpisode:
		return []AssetType{AssetTypeScreenshot, AssetTypeThumb, AssetTypePoster}
	case EntityTypeActor:
		return []AssetType{AssetTypeHeadshot, AssetTypePhoto, AssetTypeThumb, AssetTypeSignature}
	case EntityTypeDirector:
		return []AssetType{AssetTypePortrait, AssetTypeSignature, AssetTypeLogo}
	case EntityTypeStudio:
		return []AssetType{AssetTypeLogo, AssetTypeHQPhoto, AssetTypeBanner}
	case EntityTypeLabel:
		return []AssetType{AssetTypeLogo, AssetTypeHQPhoto, AssetTypeBanner}
	case EntityTypeNetwork:
		return []AssetType{AssetTypeLogo, AssetTypeBanner}
	case EntityTypeGenre:
		return []AssetType{AssetTypeIcon, AssetTypeBackground, AssetTypeBanner}
	case EntityTypeCollection:
		return []AssetType{AssetTypeCover, AssetTypeBackground, AssetTypeLogo}
	default:
		return []AssetType{}
	}
}

// GetValidSources returns all valid asset sources
func GetValidSources() []AssetSource {
	return []AssetSource{
		SourceLocal,
		SourceUser,
		SourceCore,
		SourcePlugin,
		SourceEmbedded,
	}
}

// IsSupportedImageFormat checks if the given MIME type is supported
func IsSupportedImageFormat(mimeType string) bool {
	supportedFormats := []string{
		"image/jpeg",
		"image/jpg",
		"image/png",
		"image/webp",
		"image/gif",
		"image/bmp",
		"image/tiff",
		"image/svg+xml",
	}

	for _, format := range supportedFormats {
		if format == mimeType {
			return true
		}
	}
	return false
}

// GetFileExtensionForMimeType returns the appropriate file extension for a MIME type
func GetFileExtensionForMimeType(mimeType string) string {
	switch mimeType {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/bmp":
		return ".bmp"
	case "image/tiff":
		return ".tiff"
	case "image/svg+xml":
		return ".svg"
	default:
		return ".jpg" // Default fallback
	}
}
