package settings

import (
	"context"
	"strconv"

	"github.com/mantonx/viewra/internal/domain/ai"
	settingsDomain "github.com/mantonx/viewra/internal/domain/settings"
)

// AIConfigReader provides access to AI configuration from settings.
// It abstracts the settings service for use by AI infrastructure components.
type AIConfigReader struct {
	service *Service
}

// NewAIConfigReader creates a new AIConfigReader.
func NewAIConfigReader(service *Service) *AIConfigReader {
	return &AIConfigReader{service: service}
}

// IsEnabled returns whether AI features are enabled.
func (r *AIConfigReader) IsEnabled(ctx context.Context) bool {
	val, err := r.service.GetSystemValue(ctx, "ai.enabled")
	if err != nil {
		return false
	}
	enabled, ok := val.(bool)
	return ok && enabled
}

// GetProvider returns the configured AI provider.
func (r *AIConfigReader) GetProvider(ctx context.Context) ai.ProviderType {
	val, err := r.service.GetSystemValue(ctx, "ai.provider")
	if err != nil {
		return ai.ProviderOllama // default
	}
	provider, ok := val.(string)
	if !ok {
		return ai.ProviderOllama
	}
	return ai.ProviderType(provider)
}

// GetOllamaConfig returns Ollama-specific configuration.
func (r *AIConfigReader) GetOllamaConfig(ctx context.Context) (baseURL, model string) {
	baseURL = r.getString(ctx, "ai.ollama_url", "http://localhost:11434")
	model = r.getString(ctx, "ai.ollama_model", "nomic-embed-text")
	return
}

// GetOpenAIConfig returns OpenAI-specific configuration.
// The API key is decrypted automatically.
func (r *AIConfigReader) GetOpenAIConfig(ctx context.Context) (apiKey, model string) {
	apiKey = r.getString(ctx, "ai.openai_api_key", "")
	model = r.getString(ctx, "ai.openai_model", "text-embedding-3-small")
	return
}

// GetMaxResults returns the maximum search results setting.
func (r *AIConfigReader) GetMaxResults(ctx context.Context) int {
	val, err := r.service.GetSystemValue(ctx, "ai.max_results")
	if err != nil {
		return 20 // default
	}
	switch v := val.(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 20
}

// GetSimilarityThreshold returns the similarity threshold setting.
func (r *AIConfigReader) GetSimilarityThreshold(ctx context.Context) float64 {
	val, err := r.service.GetSystemValue(ctx, "ai.similarity_threshold")
	if err != nil {
		return 0.5 // default
	}
	switch v := val.(type) {
	case string:
		// Parse string to float
		if f, parseErr := strconv.ParseFloat(v, 64); parseErr == nil {
			return f
		}
	case float64:
		return v
	}
	return 0.5
}

// GetEmbeddingConfig returns the full embedding configuration based on provider.
func (r *AIConfigReader) GetEmbeddingConfig(ctx context.Context) ai.EmbeddingConfig {
	provider := r.GetProvider(ctx)

	switch provider {
	case ai.ProviderOpenAI:
		apiKey, model := r.GetOpenAIConfig(ctx)
		return ai.EmbeddingConfig{
			Provider: ai.ProviderOpenAI,
			Model:    model,
			APIKey:   apiKey,
		}
	case ai.ProviderOllama:
		fallthrough
	default:
		baseURL, model := r.GetOllamaConfig(ctx)
		return ai.EmbeddingConfig{
			Provider: ai.ProviderOllama,
			Model:    model,
			APIKey:   baseURL, // Ollama uses base URL instead of API key
		}
	}
}

// GetLLMConfig returns the full LLM configuration based on provider.
func (r *AIConfigReader) GetLLMConfig(ctx context.Context) ai.ProviderConfig {
	provider := r.GetProvider(ctx)

	switch provider {
	case ai.ProviderOpenAI:
		apiKey, model := r.GetOpenAIConfig(ctx)
		return ai.ProviderConfig{
			Type:   ai.ProviderOpenAI,
			Model:  model,
			APIKey: apiKey,
		}
	case ai.ProviderOllama:
		fallthrough
	default:
		baseURL, model := r.GetOllamaConfig(ctx)
		return ai.ProviderConfig{
			Type:    ai.ProviderOllama,
			Model:   model,
			BaseURL: baseURL,
		}
	}
}

// Helper to get string value with default
func (r *AIConfigReader) getString(ctx context.Context, key, defaultValue string) string {
	val, err := r.service.GetSystemValue(ctx, key)
	if err != nil {
		def := settingsDomain.GetSystemDefinition(key)
		if def != nil {
			if s, ok := def.Default.(string); ok {
				return s
			}
		}
		return defaultValue
	}
	if s, ok := val.(string); ok && s != "" {
		return s
	}
	return defaultValue
}
