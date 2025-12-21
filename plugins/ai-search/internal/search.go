package internal

import (
	"context"
	"fmt"
	"log/slog"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// SearchService handles semantic search operations.
type SearchService struct {
	embeddingService *EmbeddingService
	embeddingsClient pluginv1.HostEmbeddingsClient
	defaultLimit     int
	maxLimit         int
	minSimilarity    float32
	logger           *slog.Logger
}

// NewSearchService creates a new search service.
func NewSearchService(
	embeddingService *EmbeddingService,
	embeddingsClient pluginv1.HostEmbeddingsClient,
	config SearchConfig,
	logger *slog.Logger,
) *SearchService {
	defaultLimit := config.DefaultLimit
	if defaultLimit <= 0 {
		defaultLimit = 20
	}
	maxLimit := config.MaxLimit
	if maxLimit <= 0 {
		maxLimit = 100
	}
	minSimilarity := config.MinSimilarity
	if minSimilarity <= 0 {
		minSimilarity = 0.3
	}

	return &SearchService{
		embeddingService: embeddingService,
		embeddingsClient: embeddingsClient,
		defaultLimit:     defaultLimit,
		maxLimit:         maxLimit,
		minSimilarity:    minSimilarity,
		logger:           logger,
	}
}

// SearchParams defines parameters for semantic search.
type SearchParams struct {
	Query       string
	EntityTypes []EntityType
	Limit       int
}

// Search performs semantic search using the query text.
func (s *SearchService) Search(ctx context.Context, params SearchParams) ([]SearchResult, error) {
	if s.embeddingService == nil || s.embeddingsClient == nil {
		return nil, fmt.Errorf("search service not properly initialized")
	}

	// Generate embedding for the query
	queryEmbedding, err := s.embeddingService.EmbedSingle(ctx, params.Query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	// Apply limits
	limit := params.Limit
	if limit <= 0 {
		limit = s.defaultLimit
	}
	if limit > s.maxLimit {
		limit = s.maxLimit
	}

	// Convert entity types to strings
	entityTypeStrs := make([]string, len(params.EntityTypes))
	for i, et := range params.EntityTypes {
		entityTypeStrs[i] = string(et)
	}

	// Search using the embeddings client
	resp, err := s.embeddingsClient.Search(ctx, &pluginv1.EmbeddingSearchRequest{
		QueryEmbedding: queryEmbedding,
		EntityTypes:    entityTypeStrs,
		Limit:          int32(limit),
		MinSimilarity:  s.minSimilarity,
	})
	if err != nil {
		return nil, fmt.Errorf("search embeddings: %w", err)
	}

	// Convert results
	results := make([]SearchResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = SearchResult{
			EntityType: EntityType(r.EntityType),
			EntityID:   r.EntityId,
			Similarity: r.Similarity,
			Text:       r.Text,
		}
	}

	return results, nil
}

// FindSimilar finds items similar to a given entity.
func (s *SearchService) FindSimilar(ctx context.Context, entityType EntityType, entityID int64, limit int) ([]SearchResult, error) {
	if s.embeddingsClient == nil {
		return nil, fmt.Errorf("embeddings client not available")
	}

	// Get the embedding for the source entity
	resp, err := s.embeddingsClient.Get(ctx, &pluginv1.EmbeddingQuery{
		EntityType: string(entityType),
		EntityId:   entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("get embedding: %w", err)
	}
	if !resp.Exists {
		return nil, fmt.Errorf("embedding not found for %s:%d", entityType, entityID)
	}

	// Apply limits
	if limit <= 0 {
		limit = s.defaultLimit
	}
	if limit > s.maxLimit {
		limit = s.maxLimit
	}

	// Search for similar items (of the same type by default)
	searchResp, err := s.embeddingsClient.Search(ctx, &pluginv1.EmbeddingSearchRequest{
		QueryEmbedding: resp.Embedding,
		EntityTypes:    []string{string(entityType)},
		Limit:          int32(limit + 1), // +1 to account for self
		MinSimilarity:  s.minSimilarity,
	})
	if err != nil {
		return nil, fmt.Errorf("search similar: %w", err)
	}

	// Filter out the source entity
	results := make([]SearchResult, 0, len(searchResp.Results))
	for _, r := range searchResp.Results {
		if r.EntityType == string(entityType) && r.EntityId == entityID {
			continue // Skip self
		}
		results = append(results, SearchResult{
			EntityType: EntityType(r.EntityType),
			EntityID:   r.EntityId,
			Similarity: r.Similarity,
			Text:       r.Text,
		})
		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

// GetStatus returns the current search/indexing status.
func (s *SearchService) GetStatus(ctx context.Context) (*IndexingStatus, error) {
	if s.embeddingsClient == nil {
		return nil, fmt.Errorf("embeddings client not available")
	}

	status := &IndexingStatus{
		Stats: make(map[EntityType]EntityStats),
	}

	// Get counts for each entity type
	for _, entityType := range []EntityType{
		EntityMovie, EntityTVShow, EntityTVEpisode,
		EntityMusicArtist, EntityMusicAlbum, EntityMusicTrack,
	} {
		resp, err := s.embeddingsClient.CountByType(ctx, &pluginv1.EntityTypeQuery{
			EntityType: string(entityType),
		})
		if err != nil {
			s.logger.Warn("failed to count embeddings", "type", entityType, "error", err)
			continue
		}
		status.Stats[entityType] = EntityStats{
			Indexed: resp.Count,
		}
	}

	return status, nil
}
