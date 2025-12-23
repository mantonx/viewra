// Package internal implements the AI Search plugin logic.
package internal

// Config holds the AI Search plugin configuration.
// Provider settings (embedding/chat) are configured in ViewRA's AI settings.
type Config struct {
	Indexing IndexingConfig `yaml:"indexing" json:"indexing"`
	Search   SearchConfig   `yaml:"search" json:"search"`
	MoodTags MoodTagConfig  `yaml:"mood_tags" json:"mood_tags"`
}

// MoodTagConfig configures mood tag generation.
type MoodTagConfig struct {
	Enabled bool `yaml:"enabled" json:"enabled"` // Whether mood tag generation is enabled
}

// IndexingConfig configures media indexing behavior.
type IndexingConfig struct {
	BatchSize     int  `yaml:"batch_size" json:"batch_size"`
	AutoIndex     bool `yaml:"auto_index" json:"auto_index"`
	StagePosition int  `yaml:"stage_position" json:"stage_position"`
}

// SearchConfig configures search behavior.
type SearchConfig struct {
	DefaultLimit  int     `yaml:"default_limit" json:"default_limit"`
	MaxLimit      int     `yaml:"max_limit" json:"max_limit"`
	MinSimilarity float32 `yaml:"min_similarity" json:"min_similarity"`
}

// EntityType represents a type of media entity.
type EntityType string

const (
	EntityMovie       EntityType = "movie"
	EntityTVShow      EntityType = "tv_show"
	EntityTVEpisode   EntityType = "tv_episode"
	EntityMusicArtist EntityType = "music_artist"
	EntityMusicAlbum  EntityType = "music_album"
	EntityMusicTrack  EntityType = "music_track"
)

// SearchResult represents a single semantic search result.
// Used internally with EntityType enum; converted to SDK types for API.
type SearchResult struct {
	EntityType EntityType
	EntityID   int64
	Similarity float32
	Text       string
}

// IndexingStatus contains status information for the search service.
// Used internally; converted to sdk.AISearchStatus for API responses.
type IndexingStatus struct {
	Stats map[EntityType]EntityStats
}

// EntityStats contains statistics for an entity type.
type EntityStats struct {
	Indexed int64
	Total   int64
}

// IndexingProgress tracks indexing operation progress.
// Used internally; converted to sdk.IndexingProgress for API responses.
type IndexingProgress struct {
	LibraryID   int64
	LibraryType string
	EntityType  EntityType
	Total       int64
	Processed   int64
	Failed      int64
	StartedAt   int64
	LastUpdated int64
	LastError   string
}
