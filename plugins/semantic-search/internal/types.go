// Package internal implements the Semantic Search plugin logic.
package internal

// Config holds the Semantic Search plugin configuration.
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
	BatchSize               int  `yaml:"batch_size" json:"batch_size"`
	AutoIndex               bool `yaml:"auto_index" json:"auto_index"`
	ReindexOnMetadataChange bool `yaml:"reindex_on_metadata_change" json:"reindex_on_metadata_change"`
	StagePosition           int  `yaml:"stage_position" json:"stage_position"`
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
	Rating     float32 // 0-10 scale, extracted from text
	VoteCount  int64   // Number of votes, extracted from text
}

// IndexingStatus contains status information for the search service.
// Used internally; converted to sdk.VectorSearchStatus for API responses.
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

// ExplainResult provides detailed breakdown of how a search query was processed.
// Used by the /explain endpoint for debugging search behavior.
type ExplainResult struct {
	Query           string                 `json:"query"`
	NormalizedQuery string                 `json:"normalized_query"`
	DetectedIntents map[string]interface{} `json:"detected_intents"`
	SearchMode      string                 `json:"search_mode"`
	EmbeddingCached bool                   `json:"embedding_cached"`
	VectorSearch    *VectorSearchExplain   `json:"vector_search,omitempty"`
	BoostsApplied   []BoostExplain         `json:"boosts_applied"`
	FinalResults    int                    `json:"final_results"`
	TookMs          int64                  `json:"took_ms"`
	TopResults      []ResultExplain        `json:"top_results,omitempty"`
}

// VectorSearchExplain provides details about the vector search stage.
type VectorSearchExplain struct {
	MinSimilarity       float32 `json:"min_similarity"`
	ResultsBeforeFilter int     `json:"results_before_filter"`
	ResultsAfterFilter  int     `json:"results_after_filter"`
}

// BoostExplain describes a boost that was applied during search.
type BoostExplain struct {
	Type    string  `json:"type"`
	Target  string  `json:"target,omitempty"`
	Boost   float32 `json:"boost"`
	Matches int     `json:"matches"`
}

// ResultExplain provides detailed scoring breakdown for a single result.
type ResultExplain struct {
	Title             string             `json:"title"`
	Year              int                `json:"year,omitempty"`
	BaseSimilarity    float32            `json:"base_similarity"`
	BoostedSimilarity float32            `json:"boosted_similarity"`
	BoostBreakdown    map[string]float32 `json:"boost_breakdown"`
}

// RecoveryAction describes a filter relaxation applied during zero-result recovery.
type RecoveryAction struct {
	Type        string `json:"type"`               // "threshold", "decade", "genre", "filters"
	Description string `json:"description"`        // Human-readable explanation
	Original    string `json:"original,omitempty"` // Original constraint value
	Relaxed     string `json:"relaxed,omitempty"`  // Relaxed constraint value
}

// IntentChip represents a detected query intent as a user-facing chip.
// These chips show users what the system understood from their query.
type IntentChip struct {
	ID          string   `json:"id"`           // Unique ID for this chip (e.g., "chip_decade_90s")
	Type        string   `json:"type"`         // Type: "decade", "person", "genre", "language", "studio", "mood", "similar_to", "exclusion"
	Value       string   `json:"value"`        // Internal value (e.g., "1990s", "steven spielberg")
	Display     string   `json:"display"`      // User-facing display text (e.g., "90s", "Spielberg")
	Removable   bool     `json:"removable"`    // Can the user remove this chip?
	Refinements []string `json:"refinements,omitempty"` // Suggested refinements (e.g., ["80s", "2000s"])
	Role        string   `json:"role,omitempty"`        // For person chips: "director", "actor", "writer", etc.
}

// ResultReason explains why a search result was included.
// Shows users one primary reason per result for transparency.
type ResultReason struct {
	Type    string `json:"type"`    // Type: "similar_to", "person", "mood", "era_genre", "quality"
	Display string `json:"display"` // Human-readable explanation (e.g., "Directed by Spielberg")
}

// SearchResultWithRecovery wraps search results with recovery information.
type SearchResultWithRecovery struct {
	Results         []SearchResult   `json:"results"`
	Total           int              `json:"total"`
	RecoveryApplied []RecoveryAction `json:"recovery_applied,omitempty"`
	OriginalQuery   string           `json:"original_query,omitempty"`
	IntentChips     []IntentChip     `json:"intent_chips,omitempty"`     // Detected intents as chips
	ResultReasons   []ResultReason   `json:"result_reasons,omitempty"`   // Reasons for each result (same length as Results)
}

// PlaybackConstraints defines filters for playback-related attributes.
// Used for queries like "4K movies", "Dolby Vision content", "movies with subtitles".
type PlaybackConstraints struct {
	// Resolution filters
	MinResolution string `json:"min_resolution,omitempty"` // "SD", "720p", "1080p", "4K", "8K"
	MaxResolution string `json:"max_resolution,omitempty"`

	// HDR format filters
	HDRFormats []string `json:"hdr_formats,omitempty"` // ["HDR10", "HDR10+", "Dolby Vision", "HLG"]

	// Audio format filters
	AudioFormats []string `json:"audio_formats,omitempty"` // ["atmos", "truehd", "dts-hd", "aac"]
	MinChannels  int      `json:"min_channels,omitempty"`  // Minimum audio channels (e.g., 6 for 5.1)

	// Subtitle filters
	HasSubtitles     *bool  `json:"has_subtitles,omitempty"`     // Require subtitles
	SubtitleLanguage string `json:"subtitle_language,omitempty"` // Specific subtitle language

	// Bitrate filter
	MaxBitrate int64 `json:"max_bitrate,omitempty"` // Maximum bitrate in bits/second
}

// IsEmpty returns true if no playback constraints are set.
func (p *PlaybackConstraints) IsEmpty() bool {
	if p == nil {
		return true
	}
	return p.MinResolution == "" &&
		p.MaxResolution == "" &&
		len(p.HDRFormats) == 0 &&
		len(p.AudioFormats) == 0 &&
		p.MinChannels == 0 &&
		p.HasSubtitles == nil &&
		p.SubtitleLanguage == "" &&
		p.MaxBitrate == 0
}
