// Package ai provides application services for AI functionality.
package ai

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync"

	"github.com/mantonx/viewra/internal/domain/ai"
)

const (
	// DefaultTargetDimensions is the default target dimension for embeddings.
	// 768 is a good balance - matches nomic-embed-text and bge-base.
	DefaultTargetDimensions = 768

	// MaxBatchSize is the maximum number of texts to embed in a single request.
	MaxBatchSize = 100
)

// EmbeddingService handles embedding generation with dimension normalization.
type EmbeddingService struct {
	provider         ai.EmbeddingProvider
	targetDimensions int
	logger           *slog.Logger
	mu               sync.RWMutex
}

// NewEmbeddingService creates a new embedding service.
func NewEmbeddingService(provider ai.EmbeddingProvider, targetDimensions int, logger *slog.Logger) *EmbeddingService {
	if targetDimensions <= 0 {
		targetDimensions = DefaultTargetDimensions
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &EmbeddingService{
		provider:         provider,
		targetDimensions: targetDimensions,
		logger:           logger,
	}
}

// SetProvider updates the embedding provider.
func (s *EmbeddingService) SetProvider(provider ai.EmbeddingProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.provider = provider
}

// GetProvider returns the current embedding provider.
func (s *EmbeddingService) GetProvider() ai.EmbeddingProvider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.provider
}

// TargetDimensions returns the target dimension for normalization.
func (s *EmbeddingService) TargetDimensions() int {
	return s.targetDimensions
}

// Embed generates embeddings for the given texts with dimension normalization.
func (s *EmbeddingService) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	s.mu.RLock()
	provider := s.provider
	s.mu.RUnlock()

	if provider == nil {
		return nil, ai.ErrProviderNotConfigured
	}

	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	// Process in batches if needed
	if len(texts) > MaxBatchSize {
		return s.embedBatched(ctx, provider, texts)
	}

	req := ai.EmbeddingRequest{Texts: texts}
	resp, err := provider.Embed(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("embed texts: %w", err)
	}

	// Normalize dimensions
	normalized := make([][]float32, len(resp.Embeddings))
	for i, embedding := range resp.Embeddings {
		normalized[i] = s.normalizeDimensions(embedding)
	}

	s.logger.Debug("generated embeddings",
		slog.Int("count", len(texts)),
		slog.Int("source_dims", provider.Dimensions()),
		slog.Int("target_dims", s.targetDimensions),
	)

	return normalized, nil
}

// EmbedSingle generates an embedding for a single text.
func (s *EmbeddingService) EmbedSingle(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := s.Embed(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

// embedBatched processes texts in batches.
func (s *EmbeddingService) embedBatched(ctx context.Context, provider ai.EmbeddingProvider, texts []string) ([][]float32, error) {
	result := make([][]float32, 0, len(texts))

	for i := 0; i < len(texts); i += MaxBatchSize {
		end := i + MaxBatchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		req := ai.EmbeddingRequest{Texts: batch}
		resp, err := provider.Embed(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("embed batch %d-%d: %w", i, end, err)
		}

		for _, embedding := range resp.Embeddings {
			result = append(result, s.normalizeDimensions(embedding))
		}
	}

	return result, nil
}

// normalizeDimensions adjusts embedding dimensions to the target size.
// - If source > target: truncate (works well with Matryoshka embeddings)
// - If source < target: pad with zeros
// - If source == target: return as-is
func (s *EmbeddingService) normalizeDimensions(embedding []float32) []float32 {
	sourceDims := len(embedding)

	if sourceDims == s.targetDimensions {
		return embedding
	}

	result := make([]float32, s.targetDimensions)

	if sourceDims > s.targetDimensions {
		// Truncate to target dimensions
		copy(result, embedding[:s.targetDimensions])
		// Re-normalize the vector after truncation
		s.l2Normalize(result)
	} else {
		// Pad with zeros (embedding is already normalized, zeros don't affect direction)
		copy(result, embedding)
	}

	return result
}

// l2Normalize normalizes a vector to unit length.
func (s *EmbeddingService) l2Normalize(v []float32) {
	var norm float64
	for _, val := range v {
		norm += float64(val) * float64(val)
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return
	}
	for i := range v {
		v[i] = float32(float64(v[i]) / norm)
	}
}

// HealthCheck verifies the embedding provider is accessible.
func (s *EmbeddingService) HealthCheck(ctx context.Context) error {
	s.mu.RLock()
	provider := s.provider
	s.mu.RUnlock()

	if provider == nil {
		return ai.ErrProviderNotConfigured
	}

	return provider.HealthCheck(ctx)
}

// ProviderInfo returns information about the current provider.
func (s *EmbeddingService) ProviderInfo() (name, model string, dimensions int) {
	s.mu.RLock()
	provider := s.provider
	s.mu.RUnlock()

	if provider == nil {
		return "", "", 0
	}

	return provider.Name(), provider.Model(), provider.Dimensions()
}
