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

	case ai.ProviderVoyage:
		if config.APIKey == "" {
			return nil, fmt.Errorf("Voyage AI requires an API key for embeddings")
		}
		return NewVoyageProvider(config.APIKey, config.Model), nil

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
		if apiKeyOrURL == "" {
			// Return static fallback list if no API key
			return GetAnthropicChatModels(), nil
		}
		provider := NewAnthropicProvider(apiKeyOrURL, "")
		models, err := provider.ListModels(ctx)
		if err != nil {
			// Fall back to static list on error
			return GetAnthropicChatModels(), nil
		}
		return models, nil

	case ai.ProviderVoyage:
		// Return static list for Voyage (no models API)
		return GetVoyageEmbeddingModels(), nil

	default:
		return nil, fmt.Errorf("unknown provider type: %s", providerType)
	}
}

// GetAllProviders returns information about all available providers.
func GetAllProviders() []ai.ProviderInfo {
	return []ai.ProviderInfo{
		{
			Type:              ai.ProviderOllama,
			Name:              "Ollama",
			Description:       "Run AI models locally on your own hardware. Free, private, and requires no API keys.",
			SupportsEmbedding: true,
			SupportsChat:      true,
			RequiresAPIKey:    false,
			RequiresURL:       true,
			EmbeddingModels:   getOllamaEmbeddingModels(),
			ChatModels:        getOllamaChatModels(),
		},
		{
			Type:              ai.ProviderOpenAI,
			Name:              "OpenAI",
			Description:       "Industry-leading AI models including GPT-4 and text-embedding-3.",
			SupportsEmbedding: true,
			SupportsChat:      true,
			RequiresAPIKey:    true,
			RequiresURL:       false,
			EmbeddingModels:   getOpenAIEmbeddingModels(),
			ChatModels:        getOpenAIChatModels(),
		},
		{
			Type:              ai.ProviderVoyage,
			Name:              "Voyage AI",
			Description:       "Specialized embedding models recommended by Anthropic. Best-in-class for retrieval.",
			SupportsEmbedding: true,
			SupportsChat:      false,
			RequiresAPIKey:    true,
			RequiresURL:       false,
			EmbeddingModels:   GetVoyageEmbeddingModels(),
		},
		{
			Type:              ai.ProviderAnthropic,
			Name:              "Anthropic",
			Description:       "Claude models for intelligent chat and reasoning. Does not support embeddings.",
			SupportsEmbedding: false,
			SupportsChat:      true,
			RequiresAPIKey:    true,
			RequiresURL:       false,
			ChatModels:        GetAnthropicChatModels(),
		},
	}
}

// Static model lists for providers that don't have a models API

func getOllamaEmbeddingModels() []ai.ModelInfo {
	return []ai.ModelInfo{
		{
			ID:          "nomic-embed-text",
			Name:        "Nomic Embed Text",
			Description: "High-quality embeddings with 768 dimensions. Recommended for most use cases.",
			Dimensions:  768,
			IsEmbedding: true,
			CostTier:    ai.CostTierFree,
			Recommended: true,
		},
		{
			ID:          "mxbai-embed-large",
			Name:        "MixedBread Embed Large",
			Description: "Large embedding model with 1024 dimensions. Higher quality but slower.",
			Dimensions:  1024,
			IsEmbedding: true,
			CostTier:    ai.CostTierFree,
		},
		{
			ID:          "all-minilm",
			Name:        "All MiniLM",
			Description: "Fast, lightweight embeddings with 384 dimensions.",
			Dimensions:  384,
			IsEmbedding: true,
			CostTier:    ai.CostTierFree,
		},
	}
}

func getOllamaChatModels() []ai.ModelInfo {
	return []ai.ModelInfo{
		{
			ID:          "llama3.2",
			Name:        "Llama 3.2",
			Description: "Latest Meta Llama model. Fast and capable for most tasks.",
			ContextSize: 128000,
			IsChat:      true,
			CostTier:    ai.CostTierFree,
			Recommended: true,
		},
		{
			ID:          "mistral",
			Name:        "Mistral 7B",
			Description: "Efficient 7B parameter model with strong performance.",
			ContextSize: 32768,
			IsChat:      true,
			CostTier:    ai.CostTierFree,
		},
		{
			ID:          "qwen2.5",
			Name:        "Qwen 2.5",
			Description: "Alibaba's latest model with excellent multilingual support.",
			ContextSize: 32768,
			IsChat:      true,
			CostTier:    ai.CostTierFree,
		},
	}
}

func getOpenAIEmbeddingModels() []ai.ModelInfo {
	return []ai.ModelInfo{
		{
			ID:          "text-embedding-3-small",
			Name:        "Text Embedding 3 Small",
			Description: "Fast and cost-effective embeddings with 1536 dimensions.",
			Dimensions:  1536,
			IsEmbedding: true,
			CostTier:    ai.CostTierLow,
			Recommended: true,
		},
		{
			ID:          "text-embedding-3-large",
			Name:        "Text Embedding 3 Large",
			Description: "Highest quality embeddings with 3072 dimensions.",
			Dimensions:  3072,
			IsEmbedding: true,
			CostTier:    ai.CostTierMedium,
		},
		{
			ID:          "text-embedding-ada-002",
			Name:        "Text Embedding Ada 002",
			Description: "Legacy model. Consider using text-embedding-3 instead.",
			Dimensions:  1536,
			IsEmbedding: true,
			CostTier:    ai.CostTierLow,
		},
	}
}

func getOpenAIChatModels() []ai.ModelInfo {
	return []ai.ModelInfo{
		{
			ID:          "gpt-4o",
			Name:        "GPT-4o",
			Description: "Most capable model with multimodal support.",
			ContextSize: 128000,
			IsChat:      true,
			CostTier:    ai.CostTierMedium,
			Recommended: true,
		},
		{
			ID:          "gpt-4o-mini",
			Name:        "GPT-4o Mini",
			Description: "Faster and more affordable while maintaining quality.",
			ContextSize: 128000,
			IsChat:      true,
			CostTier:    ai.CostTierLow,
		},
		{
			ID:          "gpt-4-turbo",
			Name:        "GPT-4 Turbo",
			Description: "Previous generation GPT-4 with vision support.",
			ContextSize: 128000,
			IsChat:      true,
			CostTier:    ai.CostTierHigh,
		},
	}
}

// Verify interfaces are implemented
var (
	_ ai.LLMProvider       = (*OllamaProvider)(nil)
	_ ai.EmbeddingProvider = (*OllamaProvider)(nil)
	_ ai.LLMProvider       = (*OpenAIProvider)(nil)
	_ ai.EmbeddingProvider = (*OpenAIProvider)(nil)
	_ ai.LLMProvider       = (*AnthropicProvider)(nil)
	_ ai.EmbeddingProvider = (*VoyageProvider)(nil)
	_ ai.ProviderFactory   = (*Factory)(nil)
)
