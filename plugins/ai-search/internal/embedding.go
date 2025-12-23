package internal

import (
	"context"
	"fmt"
	"log/slog"
	"math"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

// EmbeddingService handles embedding generation via the host's LLM service.
// Provider and model selection is handled by the host based on AI settings.
type EmbeddingService struct {
	llm              *sdk.LLMClient
	targetDimensions int
	logger           *slog.Logger
}

// NewEmbeddingService creates a new embedding service.
// The host determines which embedding provider/model to use based on AI settings.
func NewEmbeddingService(llm *sdk.LLMClient, logger *slog.Logger) *EmbeddingService {
	return &EmbeddingService{
		llm:              llm,
		targetDimensions: 768, // Standard dimension for most embedding models
		logger:           logger,
	}
}

// EmbedSingle generates an embedding for a single text.
// Uses the embedding provider configured in the host's AI settings.
func (s *EmbeddingService) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	if s.llm == nil {
		return nil, fmt.Errorf("LLM client not available")
	}

	// Uses host defaults from AI settings
	embedding, err := s.llm.Embed(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	if len(embedding) != s.targetDimensions {
		embedding = s.normalizeEmbedding(embedding, s.targetDimensions)
	}

	return embedding, nil
}

// EmbedBatch generates embeddings for multiple texts.
// Uses the embedding provider configured in the host's AI settings.
func (s *EmbeddingService) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if s.llm == nil {
		return nil, fmt.Errorf("LLM client not available")
	}

	// Uses host defaults from AI settings
	embeddings, err := s.llm.EmbedBatch(ctx, texts)
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
