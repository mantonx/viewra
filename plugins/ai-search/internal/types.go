// Package internal implements the AI Search plugin logic.
package internal

// Config holds the AI Search plugin configuration.
type Config struct {
	Embedding EmbeddingConfig `yaml:"embedding" json:"embedding"`
	Indexing  IndexingConfig  `yaml:"indexing" json:"indexing"`
	Search    SearchConfig    `yaml:"search" json:"search"`
	MoodTags  MoodTagConfig   `yaml:"mood_tags" json:"mood_tags"`
}

// MoodTagConfig configures mood tag generation.
type MoodTagConfig struct {
	Provider string `yaml:"provider" json:"provider"` // LLM provider for chat completions
	Model    string `yaml:"model" json:"model"`       // LLM model (e.g., "llama3.1:8b", "gpt-4o-mini")
	Enabled  bool   `yaml:"enabled" json:"enabled"`   // Whether mood tag generation is enabled
}

// EmbeddingConfig configures embedding generation.
type EmbeddingConfig struct {
	Provider         string `yaml:"provider" json:"provider"`
	Model            string `yaml:"model" json:"model"`
	TargetDimensions int    `yaml:"target_dimensions" json:"target_dimensions"`
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

// IndexingProgress tracks the progress of an indexing operation.
type IndexingProgress struct {
	EntityType  EntityType `json:"entity_type"`
	Total       int64      `json:"total"`
	Processed   int64      `json:"processed"`
	Failed      int64      `json:"failed"`
	LastError   string     `json:"last_error,omitempty"`
	StartedAt   int64      `json:"started_at"` // Unix timestamp
	LastUpdated int64      `json:"last_updated"`
}

// IndexingStatus represents the overall indexing status.
type IndexingStatus struct {
	IsIndexing bool                       `json:"is_indexing"`
	Progress   *IndexingProgress          `json:"progress,omitempty"`
	Stats      map[EntityType]EntityStats `json:"stats"`
}

// EntityStats represents stats for a single entity type.
type EntityStats struct {
	Indexed int64 `json:"indexed"`
	Total   int64 `json:"total"`
}

// SearchResult represents a semantic search result.
type SearchResult struct {
	EntityType EntityType `json:"entity_type"`
	EntityID   int64      `json:"entity_id"`
	Similarity float32    `json:"similarity"`
	Text       string     `json:"text,omitempty"`
}
