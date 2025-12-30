package internal

import (
	"context"
	"fmt"
	"log/slog"
	"math"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// EmbeddingService handles embedding generation via the AI capability.
// Uses sdk.AIClient to invoke embedding methods on a provider plugin.
type EmbeddingService struct {
	ai               *sdk.AIClient
	targetDimensions int
	logger           *slog.Logger
}

// NewEmbeddingService creates a new embedding service.
func NewEmbeddingService(plugins *sdk.PluginsClient, logger *slog.Logger) *EmbeddingService {
	return &EmbeddingService{
		ai:               sdk.NewAIClient(plugins),
		targetDimensions: 768, // Standard dimension for most embedding models
		logger:           logger,
	}
}

// EmbedSingle generates an embedding for a single text.
func (s *EmbeddingService) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	if s.ai == nil {
		return nil, fmt.Errorf("AI client not available")
	}

	embedding, err := s.ai.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	if len(embedding) != s.targetDimensions {
		embedding = s.normalizeEmbedding(embedding, s.targetDimensions)
	}

	return embedding, nil
}

// EmbedBatch generates embeddings for multiple texts.
func (s *EmbeddingService) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if s.ai == nil {
		return nil, fmt.Errorf("AI client not available")
	}

	embeddings, err := s.ai.GenerateEmbeddingBatch(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("generate embeddings: %w", err)
	}

	results := make([][]float32, len(embeddings))
	for i, embedding := range embeddings {
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
