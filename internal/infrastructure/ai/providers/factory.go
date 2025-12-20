package providers

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/ai"
)

// Factory creates LLM and embedding providers from configuration.
type Factory struct{}

// NewFactory creates a new provider factory.
func NewFactory() *Factory {
	return &Factory{}
}

// CreateLLMProvider creates an LLM provider from the given config.
func (f *Factory) CreateLLMProvider(config ai.ProviderConfig) (ai.LLMProvider, error) {
	switch config.Type {
	case ai.ProviderOllama:
		return NewOllamaProvider(config.BaseURL, config.Model), nil

	case ai.ProviderOpenAI:
		if config.APIKey == "" {
			return nil, fmt.Errorf("OpenAI requires an API key")
		}
		return NewOpenAIProvider(config.APIKey, config.Model), nil

	case ai.ProviderOpenRouter:
		if config.APIKey == "" {
			return nil, fmt.Errorf("OpenRouter requires an API key")
		}
		return NewOpenRouterProvider(config.APIKey, config.Model), nil

	case ai.ProviderAnthropic:
		if config.APIKey == "" {
			return nil, fmt.Errorf("Anthropic requires an API key")
		}
		return NewAnthropicProvider(config.APIKey, config.Model), nil

	default:
		return nil, fmt.Errorf("unknown provider type: %s", config.Type)
	}
}

// CreateEmbeddingProvider creates an embedding provider from the given config.
func (f *Factory) CreateEmbeddingProvider(config ai.EmbeddingConfig) (ai.EmbeddingProvider, error) {
	switch config.Provider {
	case ai.ProviderOllama:
		return NewOllamaProvider(config.APIKey, config.Model), nil // APIKey is used as baseURL for Ollama

	case ai.ProviderOpenAI:
		if config.APIKey == "" {
			return nil, fmt.Errorf("OpenAI requires an API key for embeddings")
		}
		return NewOpenAIProvider(config.APIKey, config.Model), nil

	default:
		return nil, fmt.Errorf("embedding provider type not supported: %s", config.Provider)
	}
}

// ListAvailableModels lists available models for a provider type.
// For cloud providers, this fetches from their APIs when possible.
// For Ollama, it fetches locally installed models.
func (f *Factory) ListAvailableModels(ctx context.Context, providerType ai.ProviderType, apiKeyOrURL string) ([]ai.ModelInfo, error) {
	switch providerType {
	case ai.ProviderOllama:
		provider := NewOllamaProvider(apiKeyOrURL, "")
		return provider.ListModels(ctx)

	case ai.ProviderOpenAI:
		if apiKeyOrURL == "" {
			return nil, fmt.Errorf("API key required to list OpenAI models")
		}
		provider := NewOpenAIProvider(apiKeyOrURL, "")
		return provider.ListModels(ctx)

	case ai.ProviderOpenRouter:
		if apiKeyOrURL == "" {
			return nil, fmt.Errorf("API key required to list OpenRouter models")
		}
		provider := NewOpenRouterProvider(apiKeyOrURL, "")
		return provider.ListModels(ctx)

	case ai.ProviderAnthropic:
		// Anthropic doesn't have a models list API, return nil
		// Users will need to specify model name directly
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown provider type: %s", providerType)
	}
}

// Verify interfaces are implemented
var (
	_ ai.LLMProvider       = (*OllamaProvider)(nil)
	_ ai.EmbeddingProvider = (*OllamaProvider)(nil)
	_ ai.LLMProvider       = (*OpenAIProvider)(nil)
	_ ai.EmbeddingProvider = (*OpenAIProvider)(nil)
	_ ai.LLMProvider       = (*AnthropicProvider)(nil)
	_ ai.ProviderFactory   = (*Factory)(nil)
)
