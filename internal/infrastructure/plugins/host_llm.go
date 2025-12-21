package plugins

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/application/settings"
	"github.com/mantonx/viewra/internal/domain/ai"
	"github.com/mantonx/viewra/internal/infrastructure/ai/providers"
)

// HostLLMServer implements the HostLLM gRPC service.
// It provides plugins access to LLM providers for embeddings and chat.
type HostLLMServer struct {
	pluginv1.UnimplementedHostLLMServer

	factory *providers.Factory

	// Config reader for dynamic settings (optional)
	configReader *settings.AIConfigReader

	// Cached config with mutex for thread-safe access
	mu                       sync.RWMutex
	defaultEmbeddingProvider ai.ProviderType
	defaultEmbeddingModel    string
	defaultChatProvider      ai.ProviderType
	defaultChatModel         string
	ollamaBaseURL            string

	logger *slog.Logger
}

// HostLLMConfig configures the host LLM server.
type HostLLMConfig struct {
	DefaultEmbeddingProvider ai.ProviderType
	DefaultEmbeddingModel    string
	DefaultChatProvider      ai.ProviderType
	DefaultChatModel         string
	OllamaBaseURL            string
}

// NewHostLLMServer creates a new HostLLMServer.
func NewHostLLMServer(cfg HostLLMConfig, factory *providers.Factory, logger *slog.Logger) *HostLLMServer {
	return &HostLLMServer{
		factory:                  factory,
		defaultEmbeddingProvider: cfg.DefaultEmbeddingProvider,
		defaultEmbeddingModel:    cfg.DefaultEmbeddingModel,
		defaultChatProvider:      cfg.DefaultChatProvider,
		defaultChatModel:         cfg.DefaultChatModel,
		ollamaBaseURL:            cfg.OllamaBaseURL,
		logger:                   logger,
	}
}

// SetConfigReader sets the AI config reader for dynamic settings.
// When set, RefreshConfig will reload settings from the database.
func (s *HostLLMServer) SetConfigReader(reader *settings.AIConfigReader) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configReader = reader
}

// RefreshConfig reloads configuration from settings.
// Call this when AI settings are changed.
func (s *HostLLMServer) RefreshConfig(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.configReader == nil {
		return
	}

	// Get provider
	provider := s.configReader.GetProvider(ctx)
	s.defaultEmbeddingProvider = provider
	s.defaultChatProvider = provider

	// Get provider-specific config
	switch provider {
	case ai.ProviderOpenAI:
		_, model := s.configReader.GetOpenAIConfig(ctx)
		s.defaultEmbeddingModel = model
		s.defaultChatModel = model
	case ai.ProviderOllama:
		fallthrough
	default:
		baseURL, model := s.configReader.GetOllamaConfig(ctx)
		s.ollamaBaseURL = baseURL
		s.defaultEmbeddingModel = model
		s.defaultChatModel = model
	}

	s.logger.Info("AI config refreshed",
		"provider", provider,
		"embedding_model", s.defaultEmbeddingModel,
		"ollama_url", s.ollamaBaseURL)
}

// ListProviders returns available LLM providers.
func (s *HostLLMServer) ListProviders(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.LLMProviderList, error) {
	providerList := []*pluginv1.LLMProvider{
		{
			Id:                "ollama",
			Name:              "Ollama (Local)",
			Configured:        s.factory != nil,
			SupportsChat:      true,
			SupportsEmbedding: true,
		},
		{
			Id:                "openai",
			Name:              "OpenAI",
			Configured:        false,
			SupportsChat:      true,
			SupportsEmbedding: true,
		},
		{
			Id:                "anthropic",
			Name:              "Anthropic",
			Configured:        false,
			SupportsChat:      true,
			SupportsEmbedding: false,
		},
	}

	return &pluginv1.LLMProviderList{Providers: providerList}, nil
}

// ListModels returns available models for a provider.
func (s *HostLLMServer) ListModels(ctx context.Context, req *pluginv1.LLMProviderQuery) (*pluginv1.LLMModelList, error) {
	if s.factory == nil {
		return nil, errors.New("provider factory not configured")
	}

	providerType := ai.ProviderType(req.ProviderId)
	models, err := s.factory.ListAvailableModels(ctx, providerType, s.ollamaBaseURL)
	if err != nil {
		return nil, err
	}

	result := make([]*pluginv1.LLMModel, len(models))
	for i, m := range models {
		result[i] = &pluginv1.LLMModel{
			Id:            m.ID,
			Name:          m.Name,
			Provider:      req.ProviderId,
			IsEmbedding:   m.IsEmbedding,
			ContextLength: int32(m.ContextSize),
		}
	}

	return &pluginv1.LLMModelList{Models: result}, nil
}

// GenerateEmbedding generates an embedding for a single text.
func (s *HostLLMServer) GenerateEmbedding(ctx context.Context, req *pluginv1.EmbeddingRequest) (*pluginv1.EmbeddingResponse, error) {
	if s.factory == nil {
		return nil, errors.New("provider factory not configured")
	}

	providerType := ai.ProviderType(req.Provider)
	if providerType == "" {
		providerType = s.defaultEmbeddingProvider
	}
	if providerType == "" {
		providerType = ai.ProviderOllama
	}

	model := req.Model
	if model == "" {
		model = s.defaultEmbeddingModel
	}
	if model == "" {
		model = "nomic-embed-text"
	}

	provider, err := s.factory.CreateEmbeddingProvider(ai.EmbeddingConfig{
		Provider: providerType,
		Model:    model,
		APIKey:   s.ollamaBaseURL, // For Ollama, this is the base URL
	})
	if err != nil {
		return nil, err
	}

	resp, err := provider.Embed(ctx, ai.EmbeddingRequest{
		Texts: []string{req.Text},
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Embeddings) == 0 {
		return nil, errors.New("no embedding returned")
	}

	return &pluginv1.EmbeddingResponse{
		Embedding:  resp.Embeddings[0],
		Dimensions: int32(len(resp.Embeddings[0])),
		TokensUsed: int32(resp.Usage.TotalTokens),
	}, nil
}

// GenerateEmbeddingBatch generates embeddings for multiple texts.
func (s *HostLLMServer) GenerateEmbeddingBatch(ctx context.Context, req *pluginv1.EmbeddingBatchRequest) (*pluginv1.EmbeddingBatchResponse, error) {
	if s.factory == nil {
		return nil, errors.New("provider factory not configured")
	}

	providerType := ai.ProviderType(req.Provider)
	if providerType == "" {
		providerType = s.defaultEmbeddingProvider
	}
	if providerType == "" {
		providerType = ai.ProviderOllama
	}

	model := req.Model
	if model == "" {
		model = s.defaultEmbeddingModel
	}
	if model == "" {
		model = "nomic-embed-text"
	}

	provider, err := s.factory.CreateEmbeddingProvider(ai.EmbeddingConfig{
		Provider: providerType,
		Model:    model,
		APIKey:   s.ollamaBaseURL,
	})
	if err != nil {
		return nil, err
	}

	resp, err := provider.Embed(ctx, ai.EmbeddingRequest{
		Texts: req.Texts,
	})
	if err != nil {
		return nil, err
	}

	results := make([]*pluginv1.EmbeddingResult, len(resp.Embeddings))
	for i, emb := range resp.Embeddings {
		results[i] = &pluginv1.EmbeddingResult{
			Embedding:  emb,
			Dimensions: int32(len(emb)),
		}
	}

	return &pluginv1.EmbeddingBatchResponse{
		Embeddings:  results,
		TotalTokens: int32(resp.Usage.TotalTokens),
	}, nil
}

// Chat sends a chat completion request.
func (s *HostLLMServer) Chat(ctx context.Context, req *pluginv1.ChatRequest) (*pluginv1.ChatResponse, error) {
	if s.factory == nil {
		return nil, errors.New("provider factory not configured")
	}

	providerType := ai.ProviderType(req.Provider)
	if providerType == "" {
		providerType = s.defaultChatProvider
	}
	if providerType == "" {
		providerType = ai.ProviderOllama
	}

	model := req.Model
	if model == "" {
		model = s.defaultChatModel
	}
	if model == "" {
		model = "llama3.1:8b"
	}

	provider, err := s.factory.CreateLLMProvider(ai.ProviderConfig{
		Type:    providerType,
		Model:   model,
		BaseURL: s.ollamaBaseURL,
	})
	if err != nil {
		return nil, err
	}

	messages := make([]ai.Message, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = ai.Message{
			Role:    ai.Role(m.Role),
			Content: m.Content,
		}
	}

	resp, err := provider.Chat(ctx, ai.ChatRequest{
		Messages:    messages,
		Temperature: float64(req.Temperature),
		MaxTokens:   int(req.MaxTokens),
	})
	if err != nil {
		return nil, err
	}

	return &pluginv1.ChatResponse{
		Content:          resp.Content,
		FinishReason:     resp.FinishReason,
		PromptTokens:     int32(resp.Usage.PromptTokens),
		CompletionTokens: int32(resp.Usage.CompletionTokens),
	}, nil
}

// ChatStream sends a streaming chat completion request.
func (s *HostLLMServer) ChatStream(req *pluginv1.ChatRequest, stream pluginv1.HostLLM_ChatStreamServer) error {
	if s.factory == nil {
		return errors.New("provider factory not configured")
	}

	ctx := stream.Context()

	providerType := ai.ProviderType(req.Provider)
	if providerType == "" {
		providerType = s.defaultChatProvider
	}
	if providerType == "" {
		providerType = ai.ProviderOllama
	}

	model := req.Model
	if model == "" {
		model = s.defaultChatModel
	}
	if model == "" {
		model = "llama3.1:8b"
	}

	provider, err := s.factory.CreateLLMProvider(ai.ProviderConfig{
		Type:    providerType,
		Model:   model,
		BaseURL: s.ollamaBaseURL,
	})
	if err != nil {
		return err
	}

	messages := make([]ai.Message, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = ai.Message{
			Role:    ai.Role(m.Role),
			Content: m.Content,
		}
	}

	events, err := provider.ChatStream(ctx, ai.ChatRequest{
		Messages:    messages,
		Temperature: float64(req.Temperature),
		MaxTokens:   int(req.MaxTokens),
	})
	if err != nil {
		return err
	}

	for event := range events {
		if event.Error != nil {
			return event.Error
		}

		if err := stream.Send(&pluginv1.ChatStreamChunk{
			Content:      event.Content,
			Done:         event.Done,
			FinishReason: event.FinishReason,
		}); err != nil {
			return err
		}
	}

	return nil
}

// Ensure interface is implemented
var _ pluginv1.HostLLMServer = (*HostLLMServer)(nil)
