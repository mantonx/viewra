package plugins

import (
	"context"
	"errors"
	"log/slog"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/domain/ai"
)

// HostEmbeddingsServer implements the HostEmbeddings gRPC service.
// It provides plugins access to the embeddings storage.
type HostEmbeddingsServer struct {
	pluginv1.UnimplementedHostEmbeddingsServer

	repo   ai.EmbeddingRepository
	logger *slog.Logger
}

// NewHostEmbeddingsServer creates a new HostEmbeddingsServer.
func NewHostEmbeddingsServer(repo ai.EmbeddingRepository, logger *slog.Logger) *HostEmbeddingsServer {
	return &HostEmbeddingsServer{
		repo:   repo,
		logger: logger,
	}
}

// Store saves an embedding for an entity.
func (s *HostEmbeddingsServer) Store(ctx context.Context, req *pluginv1.StoreEmbeddingRequest) (*pluginv1.Empty, error) {
	if s.repo == nil {
		return nil, errors.New("embeddings repository not configured")
	}

	embedding := &ai.Embedding{
		EntityType: ai.EntityType(req.EntityType),
		EntityID:   req.EntityId,
		Vector:     req.Embedding,
		Text:       req.Text,
	}

	if err := s.repo.Store(ctx, embedding); err != nil {
		s.logger.Error("failed to store embedding", "entity_type", req.EntityType, "entity_id", req.EntityId, "error", err)
		return nil, err
	}

	return &pluginv1.Empty{}, nil
}

// StoreBatch saves multiple embeddings.
func (s *HostEmbeddingsServer) StoreBatch(ctx context.Context, req *pluginv1.StoreBatchRequest) (*pluginv1.Empty, error) {
	if s.repo == nil {
		return nil, errors.New("embeddings repository not configured")
	}

	embeddings := make([]*ai.Embedding, len(req.Embeddings))
	for i, e := range req.Embeddings {
		embeddings[i] = &ai.Embedding{
			EntityType: ai.EntityType(e.EntityType),
			EntityID:   e.EntityId,
			Vector:     e.Embedding,
			Text:       e.Text,
		}
	}

	if err := s.repo.StoreBatch(ctx, embeddings); err != nil {
		s.logger.Error("failed to store embedding batch", "count", len(embeddings), "error", err)
		return nil, err
	}

	return &pluginv1.Empty{}, nil
}

// Get retrieves an embedding by entity type and ID.
func (s *HostEmbeddingsServer) Get(ctx context.Context, req *pluginv1.EmbeddingQuery) (*pluginv1.StoredEmbedding, error) {
	if s.repo == nil {
		return nil, errors.New("embeddings repository not configured")
	}

	embedding, err := s.repo.Get(ctx, ai.EntityType(req.EntityType), req.EntityId)
	if err != nil {
		return nil, err
	}

	if embedding == nil {
		return &pluginv1.StoredEmbedding{Exists: false}, nil
	}

	return &pluginv1.StoredEmbedding{
		EntityType: string(embedding.EntityType),
		EntityId:   embedding.EntityID,
		Embedding:  embedding.Vector,
		Text:       embedding.Text,
		Exists:     true,
	}, nil
}

// Delete removes an embedding.
func (s *HostEmbeddingsServer) Delete(ctx context.Context, req *pluginv1.EmbeddingQuery) (*pluginv1.Empty, error) {
	if s.repo == nil {
		return nil, errors.New("embeddings repository not configured")
	}

	if err := s.repo.Delete(ctx, ai.EntityType(req.EntityType), req.EntityId); err != nil {
		return nil, err
	}

	return &pluginv1.Empty{}, nil
}

// DeleteByType removes all embeddings for an entity type.
func (s *HostEmbeddingsServer) DeleteByType(ctx context.Context, req *pluginv1.EntityTypeQuery) (*pluginv1.DeleteCountResponse, error) {
	if s.repo == nil {
		return nil, errors.New("embeddings repository not configured")
	}

	if err := s.repo.DeleteByType(ctx, ai.EntityType(req.EntityType)); err != nil {
		return nil, err
	}

	// We don't have a count from DeleteByType, so return 0
	return &pluginv1.DeleteCountResponse{Count: 0}, nil
}

// Search finds similar embeddings using cosine similarity.
func (s *HostEmbeddingsServer) Search(ctx context.Context, req *pluginv1.EmbeddingSearchRequest) (*pluginv1.EmbeddingSearchResponse, error) {
	if s.repo == nil {
		return nil, errors.New("embeddings repository not configured")
	}

	// Convert entity types
	types := make([]ai.EntityType, len(req.EntityTypes))
	for i, t := range req.EntityTypes {
		types[i] = ai.EntityType(t)
	}

	searchReq := ai.SemanticSearchRequest{
		Types: types,
		Limit: int(req.Limit),
	}

	resp, err := s.repo.Search(ctx, searchReq, req.QueryEmbedding)
	if err != nil {
		return nil, err
	}

	results := make([]*pluginv1.EmbeddingSearchResult, len(resp.Results))
	for i, r := range resp.Results {
		// Filter by min similarity if specified
		if req.MinSimilarity > 0 && r.Score < req.MinSimilarity {
			continue
		}
		results[i] = &pluginv1.EmbeddingSearchResult{
			EntityType: string(r.EntityType),
			EntityId:   r.EntityID,
			Similarity: r.Score,
			Text:       r.Text,
		}
	}

	// Remove nil entries from filtering
	filteredResults := make([]*pluginv1.EmbeddingSearchResult, 0, len(results))
	for _, r := range results {
		if r != nil {
			filteredResults = append(filteredResults, r)
		}
	}

	return &pluginv1.EmbeddingSearchResponse{Results: filteredResults}, nil
}

// SearchText finds embeddings where text contains the query keywords.
func (s *HostEmbeddingsServer) SearchText(ctx context.Context, req *pluginv1.TextSearchRequest) (*pluginv1.EmbeddingSearchResponse, error) {
	if s.repo == nil {
		return nil, errors.New("embeddings repository not configured")
	}

	// Convert entity types
	types := make([]ai.EntityType, len(req.EntityTypes))
	for i, t := range req.EntityTypes {
		types[i] = ai.EntityType(t)
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}

	results, err := s.repo.SearchText(ctx, req.GetQuery(), types, limit)
	if err != nil {
		return nil, err
	}

	protoResults := make([]*pluginv1.EmbeddingSearchResult, len(results))
	for i, r := range results {
		protoResults[i] = &pluginv1.EmbeddingSearchResult{
			EntityType: string(r.EntityType),
			EntityId:   r.EntityID,
			Similarity: 1.0, // Text match = full similarity
			Text:       r.Text,
		}
	}

	return &pluginv1.EmbeddingSearchResponse{Results: protoResults}, nil
}

// CountByType returns the number of embeddings for an entity type.
func (s *HostEmbeddingsServer) CountByType(ctx context.Context, req *pluginv1.EntityTypeQuery) (*pluginv1.CountResponse, error) {
	if s.repo == nil {
		return nil, errors.New("embeddings repository not configured")
	}

	count, err := s.repo.CountByType(ctx, ai.EntityType(req.EntityType))
	if err != nil {
		return nil, err
	}

	return &pluginv1.CountResponse{Count: count}, nil
}

// CountAll returns the total number of embeddings.
func (s *HostEmbeddingsServer) CountAll(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.CountResponse, error) {
	if s.repo == nil {
		return nil, errors.New("embeddings repository not configured")
	}

	count, err := s.repo.Count(ctx)
	if err != nil {
		return nil, err
	}

	return &pluginv1.CountResponse{Count: count}, nil
}

// Ensure interface is implemented
var _ pluginv1.HostEmbeddingsServer = (*HostEmbeddingsServer)(nil)
