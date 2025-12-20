package ai

import "context"

// LLMProvider defines the interface for LLM providers (chat completions).
type LLMProvider interface {
	// Chat sends a chat completion request and returns the response.
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)

	// ChatStream sends a chat completion request and streams the response.
	ChatStream(ctx context.Context, req ChatRequest) (<-chan ChatStreamEvent, error)

	// Name returns the provider name for display/logging.
	Name() string

	// Model returns the current model name.
	Model() string

	// HealthCheck verifies the provider is accessible.
	HealthCheck(ctx context.Context) error
}

// ChatStreamEvent represents an event in a streaming chat response.
type ChatStreamEvent struct {
	Content      string
	Done         bool
	Error        error
	FinishReason string
}

// EmbeddingProvider defines the interface for embedding providers.
type EmbeddingProvider interface {
	// Embed generates embeddings for the given texts.
	Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)

	// Name returns the provider name for display/logging.
	Name() string

	// Model returns the current model name.
	Model() string

	// Dimensions returns the native embedding dimensions for this model.
	Dimensions() int

	// HealthCheck verifies the provider is accessible.
	HealthCheck(ctx context.Context) error
}

// ProviderFactory creates LLM and embedding providers from configuration.
type ProviderFactory interface {
	// CreateLLMProvider creates an LLM provider from the given config.
	CreateLLMProvider(config ProviderConfig) (LLMProvider, error)

	// CreateEmbeddingProvider creates an embedding provider from the given config.
	CreateEmbeddingProvider(config EmbeddingConfig) (EmbeddingProvider, error)

	// ListAvailableModels lists available models for a provider type.
	ListAvailableModels(ctx context.Context, providerType ProviderType, baseURL string) ([]ModelInfo, error)
}

// ModelInfo contains information about an available model.
type ModelInfo struct {
	ID          string
	Name        string
	Description string
	ContextSize int
	IsChat      bool
	IsEmbedding bool
}
