package ai

import "context"

// EmbeddingRepository defines the interface for storing and querying embeddings.
type EmbeddingRepository interface {
	// Store saves or updates an embedding for an entity.
	Store(ctx context.Context, embedding *Embedding) error

	// StoreBatch saves multiple embeddings in a single transaction.
	StoreBatch(ctx context.Context, embeddings []*Embedding) error

	// Delete removes an embedding for an entity.
	Delete(ctx context.Context, entityType EntityType, entityID int64) error

	// DeleteByType removes all embeddings for a given entity type.
	DeleteByType(ctx context.Context, entityType EntityType) error

	// DeleteAll removes all embeddings (for re-indexing).
	DeleteAll(ctx context.Context) error

	// Get retrieves an embedding by entity type and ID.
	Get(ctx context.Context, entityType EntityType, entityID int64) (*Embedding, error)

	// Search performs a semantic similarity search.
	Search(ctx context.Context, req SemanticSearchRequest, queryVector []float32) (*SemanticSearchResponse, error)

	// Count returns the total number of embeddings.
	Count(ctx context.Context) (int64, error)

	// CountByType returns the number of embeddings for a given entity type.
	CountByType(ctx context.Context, entityType EntityType) (int64, error)

	// GetDimensions returns the dimension of stored embeddings (0 if empty).
	GetDimensions(ctx context.Context) (int, error)
}

// MoodTagRepository defines the interface for storing mood tags.
type MoodTagRepository interface {
	// Store saves mood tags for a media item.
	Store(ctx context.Context, mediaID int64, tags []MoodTag) error

	// Delete removes all mood tags for a media item.
	Delete(ctx context.Context, mediaID int64) error

	// GetByMediaID retrieves mood tags for a media item.
	GetByMediaID(ctx context.Context, mediaID int64) ([]MoodTag, error)

	// Search finds media items matching the given mood tags.
	Search(ctx context.Context, tags []string, limit int) ([]int64, error)
}

// AISettingsRepository defines the interface for AI configuration storage.
type AISettingsRepository interface {
	// GetLLMConfig retrieves the LLM provider configuration.
	GetLLMConfig(ctx context.Context) (*ProviderConfig, error)

	// SetLLMConfig saves the LLM provider configuration.
	SetLLMConfig(ctx context.Context, config *ProviderConfig) error

	// GetEmbeddingConfig retrieves the embedding provider configuration.
	GetEmbeddingConfig(ctx context.Context) (*EmbeddingConfig, error)

	// SetEmbeddingConfig saves the embedding provider configuration.
	SetEmbeddingConfig(ctx context.Context, config *EmbeddingConfig) error

	// GetUsageStats retrieves usage statistics for a user.
	GetUsageStats(ctx context.Context, userID string) (*UsageStats, error)

	// RecordUsage records token usage.
	RecordUsage(ctx context.Context, userID string, usage TokenUsage) error

	// GetIndexingStatus retrieves the current indexing status.
	GetIndexingStatus(ctx context.Context) (*IndexingStatus, error)

	// SetIndexingStatus updates the indexing status.
	SetIndexingStatus(ctx context.Context, status *IndexingStatus) error
}

// UsageStats tracks AI usage for a user.
type UsageStats struct {
	UserID            string
	DailyTokensUsed   int
	MonthlyTokensUsed int
	LastResetDaily    string // Date string YYYY-MM-DD
	LastResetMonthly  string // Date string YYYY-MM
	DailyTokenLimit   int
	MonthlyTokenLimit int
}

// IndexingStatus tracks the progress of embedding indexing.
type IndexingStatus struct {
	IsRunning     bool
	TotalItems    int
	IndexedItems  int
	CurrentEntity EntityType
	LastIndexedAt string // ISO timestamp
	Error         string
	StartedAt     string // ISO timestamp
}
