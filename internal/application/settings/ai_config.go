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

// GetEmbeddingProvider returns the configured embedding provider.
func (r *AIConfigReader) GetEmbeddingProvider(ctx context.Context) ai.ProviderType {
	val, err := r.service.GetSystemValue(ctx, "ai.embedding_provider")
	if err != nil {
		return ai.ProviderOllama // default
	}
	provider, ok := val.(string)
	if !ok {
		return ai.ProviderOllama
	}
	return ai.ProviderType(provider)
}

// GetChatProvider returns the configured chat provider.
func (r *AIConfigReader) GetChatProvider(ctx context.Context) ai.ProviderType {
	val, err := r.service.GetSystemValue(ctx, "ai.chat_provider")
	if err != nil {
		return ai.ProviderOllama // default
	}
	provider, ok := val.(string)
	if !ok {
		return ai.ProviderOllama
	}
	return ai.ProviderType(provider)
}

// GetOllamaURL returns the Ollama server URL.
func (r *AIConfigReader) GetOllamaURL(ctx context.Context) string {
	return r.getString(ctx, "ai.ollama_url", "http://localhost:11434")
}

// GetOllamaEmbeddingModel returns the Ollama embedding model.
func (r *AIConfigReader) GetOllamaEmbeddingModel(ctx context.Context) string {
	return r.getString(ctx, "ai.ollama_embedding_model", "nomic-embed-text")
}

// GetOllamaChatModel returns the Ollama chat model.
func (r *AIConfigReader) GetOllamaChatModel(ctx context.Context) string {
	return r.getString(ctx, "ai.ollama_chat_model", "llama3.2")
}

// GetOpenAIAPIKey returns the OpenAI API key (decrypted).
func (r *AIConfigReader) GetOpenAIAPIKey(ctx context.Context) string {
	return r.getString(ctx, "ai.openai_api_key", "")
}

// GetOpenAIEmbeddingModel returns the OpenAI embedding model.
func (r *AIConfigReader) GetOpenAIEmbeddingModel(ctx context.Context) string {
	return r.getString(ctx, "ai.openai_embedding_model", "text-embedding-3-small")
}

// GetOpenAIChatModel returns the OpenAI chat model.
func (r *AIConfigReader) GetOpenAIChatModel(ctx context.Context) string {
	return r.getString(ctx, "ai.openai_chat_model", "gpt-4o-mini")
}

// GetAnthropicAPIKey returns the Anthropic API key (decrypted).
func (r *AIConfigReader) GetAnthropicAPIKey(ctx context.Context) string {
	return r.getString(ctx, "ai.anthropic_api_key", "")
}

// GetAnthropicChatModel returns the Anthropic chat model.
func (r *AIConfigReader) GetAnthropicChatModel(ctx context.Context) string {
	return r.getString(ctx, "ai.anthropic_chat_model", "claude-sonnet-4-5-20250929")
}

// GetVoyageAPIKey returns the Voyage AI API key (decrypted).
func (r *AIConfigReader) GetVoyageAPIKey(ctx context.Context) string {
	return r.getString(ctx, "ai.voyage_api_key", "")
}

// GetVoyageEmbeddingModel returns the Voyage AI embedding model.
func (r *AIConfigReader) GetVoyageEmbeddingModel(ctx context.Context) string {
	return r.getString(ctx, "ai.voyage_embedding_model", "voyage-3-lite")
}

// GetOpenRouterAPIKey returns the OpenRouter API key (decrypted).
func (r *AIConfigReader) GetOpenRouterAPIKey(ctx context.Context) string {
	return r.getString(ctx, "ai.openrouter_api_key", "")
}

// GetOpenRouterChatModel returns the OpenRouter chat model.
func (r *AIConfigReader) GetOpenRouterChatModel(ctx context.Context) string {
	return r.getString(ctx, "ai.openrouter_chat_model", "anthropic/claude-3.5-sonnet")
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
	provider := r.GetEmbeddingProvider(ctx)

	switch provider {
	case ai.ProviderOpenAI:
		return ai.EmbeddingConfig{
			Provider: ai.ProviderOpenAI,
			Model:    r.GetOpenAIEmbeddingModel(ctx),
			APIKey:   r.GetOpenAIAPIKey(ctx),
		}
	case ai.ProviderVoyage:
		return ai.EmbeddingConfig{
			Provider: ai.ProviderVoyage,
			Model:    r.GetVoyageEmbeddingModel(ctx),
			APIKey:   r.GetVoyageAPIKey(ctx),
		}
	case ai.ProviderOllama:
		fallthrough
	default:
		return ai.EmbeddingConfig{
			Provider: ai.ProviderOllama,
			Model:    r.GetOllamaEmbeddingModel(ctx),
			APIKey:   r.GetOllamaURL(ctx), // Ollama uses base URL instead of API key
		}
	}
}

// GetChatConfig returns the full chat/LLM configuration based on provider.
func (r *AIConfigReader) GetChatConfig(ctx context.Context) ai.ProviderConfig {
	provider := r.GetChatProvider(ctx)

	switch provider {
	case ai.ProviderOpenAI:
		return ai.ProviderConfig{
			Type:   ai.ProviderOpenAI,
			Model:  r.GetOpenAIChatModel(ctx),
			APIKey: r.GetOpenAIAPIKey(ctx),
		}
	case ai.ProviderAnthropic:
		return ai.ProviderConfig{
			Type:   ai.ProviderAnthropic,
			Model:  r.GetAnthropicChatModel(ctx),
			APIKey: r.GetAnthropicAPIKey(ctx),
		}
	case ai.ProviderOpenRouter:
		return ai.ProviderConfig{
			Type:   ai.ProviderOpenRouter,
			Model:  r.GetOpenRouterChatModel(ctx),
			APIKey: r.GetOpenRouterAPIKey(ctx),
		}
	case ai.ProviderOllama:
		fallthrough
	default:
		return ai.ProviderConfig{
			Type:    ai.ProviderOllama,
			Model:   r.GetOllamaChatModel(ctx),
			BaseURL: r.GetOllamaURL(ctx),
		}
	}
}

// GetLLMConfig is an alias for GetChatConfig for backward compatibility.
// Deprecated: Use GetChatConfig instead.
func (r *AIConfigReader) GetLLMConfig(ctx context.Context) ai.ProviderConfig {
	return r.GetChatConfig(ctx)
}

// --- Legacy methods for backward compatibility ---

// GetProvider returns the embedding provider (for backward compatibility).
// Deprecated: Use GetEmbeddingProvider or GetChatProvider instead.
func (r *AIConfigReader) GetProvider(ctx context.Context) ai.ProviderType {
	return r.GetEmbeddingProvider(ctx)
}

// GetOllamaConfig returns Ollama embedding configuration (for backward compatibility).
// Deprecated: Use GetOllamaURL and GetOllamaEmbeddingModel instead.
func (r *AIConfigReader) GetOllamaConfig(ctx context.Context) (baseURL, model string) {
	return r.GetOllamaURL(ctx), r.GetOllamaEmbeddingModel(ctx)
}

// GetOpenAIConfig returns OpenAI embedding configuration (for backward compatibility).
// Deprecated: Use GetOpenAIAPIKey and GetOpenAIEmbeddingModel instead.
func (r *AIConfigReader) GetOpenAIConfig(ctx context.Context) (apiKey, model string) {
	return r.GetOpenAIAPIKey(ctx), r.GetOpenAIEmbeddingModel(ctx)
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
