package internal

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// EmbeddingService handles embedding generation via capability broker.
// Uses the "embedding" capability to connect to a provider plugin (ollama, openai, etc.)
type EmbeddingService struct {
	plugins          *sdk.PluginsClient
	targetDimensions int
	logger           *slog.Logger

	// Cache the provider connection (lazy initialized)
	mu       sync.Mutex
	provider pluginv1.PluginProviderClient
}

// NewEmbeddingService creates a new embedding service.
// Uses the capability broker to connect to an embedding provider.
func NewEmbeddingService(plugins *sdk.PluginsClient, logger *slog.Logger) *EmbeddingService {
	return &EmbeddingService{
		plugins:          plugins,
		targetDimensions: 768, // Standard dimension for most embedding models
		logger:           logger,
	}
}

// getProvider returns a cached or new provider client.
func (s *EmbeddingService) getProvider(ctx context.Context) (pluginv1.PluginProviderClient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Return cached provider if available
	if s.provider != nil {
		return s.provider, nil
	}

	// Get connection via capability broker
	if s.plugins == nil {
		return nil, fmt.Errorf("plugins client not available")
	}

	conn, err := s.plugins.GetConnection(ctx, "embedding")
	if err != nil {
		return nil, fmt.Errorf("get embedding provider: %w", err)
	}

	// Create and cache the provider client
	s.provider = pluginv1.NewPluginProviderClient(conn)
	s.logger.Debug("connected to embedding provider")
	return s.provider, nil
}

// EmbedSingle generates an embedding for a single text.
func (s *EmbeddingService) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	provider, err := s.getProvider(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := provider.GenerateEmbedding(ctx, &pluginv1.ProviderEmbeddingRequest{
		Text: text,
	})
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	embedding := resp.Embedding
	if len(embedding) != s.targetDimensions {
		embedding = s.normalizeEmbedding(embedding, s.targetDimensions)
	}

	return embedding, nil
}

// EmbedBatch generates embeddings for multiple texts.
func (s *EmbeddingService) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	provider, err := s.getProvider(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := provider.GenerateEmbeddingBatch(ctx, &pluginv1.ProviderEmbeddingBatchRequest{
		Texts: texts,
	})
	if err != nil {
		return nil, fmt.Errorf("generate embeddings: %w", err)
	}

	results := make([][]float32, len(resp.Embeddings))
	for i, emb := range resp.Embeddings {
		embedding := emb.Embedding
		if len(embedding) != s.targetDimensions {
			embedding = s.normalizeEmbedding(embedding, s.targetDimensions)
		}
		results[i] = embedding
	}

	return results, nil
}

// normalizeEmbedding adjusts embedding to target dimensions.
// Uses truncation (if larger) or zero-padding (if smaller).
func (s *EmbeddingService) normalizeEmbedding(embedding []float32, targetDim int) []float32 {
	if len(embedding) == targetDim {
		return embedding
	}

	normalized := make([]float32, targetDim)

	if len(embedding) > targetDim {
		// Truncate and renormalize
		copy(normalized, embedding[:targetDim])
		s.l2Normalize(normalized)
	} else {
		// Zero-pad (embedding stays the same, zeros at the end)
		copy(normalized, embedding)
	}

	return normalized
}

// l2Normalize normalizes a vector to unit length.
func (s *EmbeddingService) l2Normalize(v []float32) {
	var sum float64
	for _, val := range v {
		sum += float64(val) * float64(val)
	}
	if sum == 0 {
		return
	}
	norm := float32(math.Sqrt(sum))
	for i := range v {
		v[i] /= norm
	}
}
