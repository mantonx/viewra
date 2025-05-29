package mediaassetmodule

import (
	"time"

	"gorm.io/gorm"
)

// AssetType represents the main type of media
type AssetType string

const (
	AssetTypeMusic  AssetType = "music"
	AssetTypeMovie  AssetType = "movie"
	AssetTypeTV     AssetType = "tv"
	AssetTypePeople AssetType = "people"
	AssetTypeMeta   AssetType = "meta"
)

// AssetCategory represents the specific category within a type
type AssetCategory string

const (
	// Music categories
	CategoryAlbum  AssetCategory = "album"
	CategoryArtist AssetCategory = "artist"
	CategoryTrack  AssetCategory = "track"
	CategoryLabel  AssetCategory = "label"
	CategoryGenre  AssetCategory = "genre"
	
	// Movie categories
	CategoryPoster     AssetCategory = "poster"
	CategoryBackdrop   AssetCategory = "backdrop"
	CategoryLogo       AssetCategory = "logo"
	CategoryCollection AssetCategory = "collection"
	
	// TV categories  
	CategoryShow    AssetCategory = "show"
	CategorySeason  AssetCategory = "season"
	CategoryEpisode AssetCategory = "episode"
	// CategoryBackdrop is shared between movie and TV
	
	// People categories
	CategoryActor    AssetCategory = "actor"
	CategoryDirector AssetCategory = "director"
	CategoryCrew     AssetCategory = "crew"
	
	// Meta categories
	CategoryStudio  AssetCategory = "studio"
	CategoryNetwork AssetCategory = "network"
	CategoryRating  AssetCategory = "rating"
)

// AssetSubtype represents different variations of assets (deprecated - kept for compatibility)
type AssetSubtype string

const (
	SubtypeArtwork    AssetSubtype = "artwork"
	SubtypePoster     AssetSubtype = "poster"
	SubtypeBackdrop   AssetSubtype = "backdrop"
	SubtypeThumbnail  AssetSubtype = "thumbnail"
	SubtypeSubtitle   AssetSubtype = "subtitle"
	SubtypeLyrics     AssetSubtype = "lyrics"
)

// MediaAsset represents a media asset (artwork, poster, etc.) stored in the filesystem
type MediaAsset struct {
	ID           uint          `gorm:"primaryKey" json:"id"`
	MediaFileID  uint          `gorm:"not null;index:idx_media_assets_media_file_id" json:"media_file_id"`
	Type         AssetType     `gorm:"not null;index:idx_media_assets_type" json:"type"`
	Category     AssetCategory `gorm:"not null;index:idx_media_assets_category" json:"category"`
	Subtype      AssetSubtype  `gorm:"not null;index:idx_media_assets_subtype" json:"subtype"`
	RelativePath string        `gorm:"not null" json:"relative_path"`
	Hash         string        `gorm:"not null;index:idx_media_assets_hash" json:"hash"`
	MimeType     string        `gorm:"not null" json:"mime_type"`
	Size         int64         `gorm:"not null" json:"size"`
	Width        int           `json:"width,omitempty"`
	Height       int           `json:"height,omitempty"`
	Metadata     string        `gorm:"type:text" json:"metadata,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

// TableName returns the table name for MediaAsset
func (MediaAsset) TableName() string {
	return "media_assets"
}

// AssetRequest represents a request to save a media asset
type AssetRequest struct {
	MediaFileID uint                   `json:"media_file_id"`
	Type        AssetType              `json:"type"`
	Category    AssetCategory          `json:"category"`
	Subtype     AssetSubtype           `json:"subtype"`
	Data        []byte                 `json:"data"`
	MimeType    string                 `json:"mime_type"`
	Width       int                    `json:"width,omitempty"`
	Height      int                    `json:"height,omitempty"`
	Metadata    map[string]string      `json:"metadata,omitempty"` // Metadata about the asset source
}

// AssetResponse represents the response when retrieving a media asset
type AssetResponse struct {
	ID           uint                   `json:"id"`
	MediaFileID  uint                   `json:"media_file_id"`
	Type         AssetType              `json:"type"`
	Category     AssetCategory          `json:"category"`
	Subtype      AssetSubtype           `json:"subtype"`
	RelativePath string                 `json:"relative_path"`
	Hash         string                 `json:"hash"`
	MimeType     string                 `json:"mime_type"`
	Size         int64                  `json:"size"`
	Width        int                    `json:"width,omitempty"`
	Height       int                    `json:"height,omitempty"`
	Metadata     map[string]string      `json:"metadata,omitempty"` // Metadata about the asset source
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// AssetFilter represents filters for querying assets
type AssetFilter struct {
	MediaFileID uint          `json:"media_file_id,omitempty"`
	Type        AssetType     `json:"type,omitempty"`
	Category    AssetCategory `json:"category,omitempty"`
	Subtype     AssetSubtype  `json:"subtype,omitempty"`
	Limit       int           `json:"limit,omitempty"`
	Offset      int           `json:"offset,omitempty"`
}

// AssetStats represents statistics about stored assets
type AssetStats struct {
	TotalAssets     int64                      `json:"total_assets"`
	TotalSize       int64                      `json:"total_size"`
	AssetsByType    map[AssetType]int64        `json:"assets_by_type"`
	AssetsByCategory map[AssetCategory]int64   `json:"assets_by_category"`
	AssetsBySubtype map[AssetSubtype]int64     `json:"assets_by_subtype"`
	AverageSize     float64                    `json:"average_size"`
	LargestAsset    int64                      `json:"largest_asset"`
}

// BeforeCreate hook to set timestamps
func (a *MediaAsset) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	a.CreatedAt = now
	a.UpdatedAt = now
	return nil
}

// BeforeUpdate hook to update timestamp
func (a *MediaAsset) BeforeUpdate(tx *gorm.DB) error {
	a.UpdatedAt = time.Now()
	return nil
} 