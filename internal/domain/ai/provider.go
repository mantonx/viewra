package ai

import "context"

// HealthChecker is a minimal interface for providers that support health checks.
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

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
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Size        string   `json:"size,omitempty"`        // Model size for display (e.g., "7B", "3.8 GB")
	ContextSize int      `json:"contextSize,omitempty"` // For chat models
	Dimensions  int      `json:"dimensions,omitempty"`  // For embedding models
	IsChat      bool     `json:"isChat"`
	IsEmbedding bool     `json:"isEmbedding"`
	CostTier    CostTier `json:"costTier"`
	Recommended bool     `json:"recommended,omitempty"`
}

// CostTier represents relative pricing levels for models.
type CostTier string

const (
	CostTierFree   CostTier = "free"   // Local models (Ollama)
	CostTierLow    CostTier = "low"    // Budget-friendly (text-embedding-3-small)
	CostTierMedium CostTier = "medium" // Standard pricing
	CostTierHigh   CostTier = "high"   // Premium models (GPT-4, Claude Opus)
)

// ProviderInfo describes a provider's capabilities and available models.
type ProviderInfo struct {
	Type              ProviderType `json:"type"`
	Name              string       `json:"name"`
	Description       string       `json:"description"`
	SupportsEmbedding bool         `json:"supportsEmbedding"`
	SupportsChat      bool         `json:"supportsChat"`
	RequiresAPIKey    bool         `json:"requiresApiKey"`
	RequiresURL       bool         `json:"requiresUrl"`
	IsPlugin          bool         `json:"isPlugin"`
	EmbeddingModels   []ModelInfo  `json:"embeddingModels,omitempty"`
	ChatModels        []ModelInfo  `json:"chatModels,omitempty"`
}
