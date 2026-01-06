package internal

// Config holds the plugin configuration.
type Config struct {
	// Enabled controls whether recommendations are shown
	Enabled bool `yaml:"enabled" json:"enabled"`

	// MaxRecommendations is the max items per recommendation row
	MaxRecommendations int `yaml:"max_recommendations" json:"max_recommendations"`

	// SimilarWeight is the weight for similar item recommendations (0-100)
	// Deprecated: Use HybridWeights instead
	SimilarWeight int `yaml:"similar_weight" json:"similar_weight"`

	// FavoriteWeight is the weight for favorite-based recommendations (0-100)
	// Deprecated: Use HybridWeights instead
	FavoriteWeight int `yaml:"favorite_weight" json:"favorite_weight"`

	// UseHybridScoring enables the hybrid recommendation engine
	UseHybridScoring bool `yaml:"use_hybrid_scoring" json:"use_hybrid_scoring"`

	// CollaborativeWeight is the weight for collaborative filtering (0-100)
	CollaborativeWeight int `yaml:"collaborative_weight" json:"collaborative_weight"`

	// SemanticWeight is the weight for semantic/content-based similarity (0-100)
	SemanticWeight int `yaml:"semantic_weight" json:"semantic_weight"`

	// ExplorationWeight is the weight for discovery/exploration (0-100)
	ExplorationWeight int `yaml:"exploration_weight" json:"exploration_weight"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:             true,
		MaxRecommendations:  20,
		SimilarWeight:       50,
		FavoriteWeight:      50,
		UseHybridScoring:    true,
		CollaborativeWeight: 50,
		SemanticWeight:      30,
		ExplorationWeight:   20,
	}
}

// Recommendation represents a recommended item.
type Recommendation struct {
	EntityType string  `json:"entity_type"`
	EntityID   int64   `json:"entity_id"`
	Title      string  `json:"title"`
	Year       int     `json:"year,omitempty"`
	Poster     string  `json:"poster,omitempty"`
	Reason     string  `json:"reason,omitempty"`
	Score      float32 `json:"score,omitempty"`
}

// RecommendationRow represents a row of recommendations for the home screen.
type RecommendationRow struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Subtitle  string           `json:"subtitle,omitempty"`
	Items     []Recommendation `json:"items"`
	SeeAllURL string           `json:"see_all_url,omitempty"`
}
