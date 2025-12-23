package settings

import (
	"context"

	"github.com/mantonx/viewra/internal/domain/ai"
)

// AIConfigReader provides access to core AI configuration from settings.
// Provider-specific configuration (API keys, models, URLs) is managed by each
// provider plugin via their own settings schemas.
type AIConfigReader struct {
	service *Service
}

// NewAIConfigReader creates a new AIConfigReader.
func NewAIConfigReader(service *Service) *AIConfigReader {
	return &AIConfigReader{service: service}
}

// IsEnabled returns whether AI features are enabled.
func (r *AIConfigReader) IsEnabled(ctx context.Context) bool {
	return r.service.GetSystemBool(ctx, "ai.enabled", false)
}

// GetEmbeddingProvider returns the configured embedding provider plugin ID.
func (r *AIConfigReader) GetEmbeddingProvider(ctx context.Context) ai.ProviderType {
	provider := r.service.GetSystemString(ctx, "ai.embedding_provider", string(ai.ProviderOllama))
	return ai.ProviderType(provider)
}

// GetChatProvider returns the configured chat provider plugin ID.
func (r *AIConfigReader) GetChatProvider(ctx context.Context) ai.ProviderType {
	provider := r.service.GetSystemString(ctx, "ai.chat_provider", string(ai.ProviderOllama))
	return ai.ProviderType(provider)
}

// GetMaxResults returns the maximum search results setting.
func (r *AIConfigReader) GetMaxResults(ctx context.Context) int {
	return r.service.GetSystemInt(ctx, "ai.max_results", 20)
}

// GetSimilarityThreshold returns the similarity threshold setting.
func (r *AIConfigReader) GetSimilarityThreshold(ctx context.Context) float64 {
	return r.service.GetSystemFloat64(ctx, "ai.similarity_threshold", 0.5)
}
