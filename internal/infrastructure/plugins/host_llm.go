package plugins

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// ErrNoProvider is returned when no provider plugin is available.
var ErrNoProvider = errors.New("no AI provider plugin available")

// HostLLMServer implements the HostLLM gRPC service.
// It delegates all AI operations to registered provider plugins.
type HostLLMServer struct {
	pluginv1.UnimplementedHostLLMServer

	mu                       sync.RWMutex
	providerRegistry         *ProviderRegistry
	defaultEmbeddingProvider string
	defaultEmbeddingModel    string
	defaultChatProvider      string
	defaultChatModel         string
	logger                   *slog.Logger
}

// HostLLMConfig configures the host LLM server.
type HostLLMConfig struct {
	DefaultEmbeddingProvider string
	DefaultEmbeddingModel    string
	DefaultChatProvider      string
	DefaultChatModel         string
}

// NewHostLLMServer creates a new HostLLMServer.
func NewHostLLMServer(cfg HostLLMConfig, logger *slog.Logger) *HostLLMServer {
	return &HostLLMServer{
		defaultEmbeddingProvider: cfg.DefaultEmbeddingProvider,
		defaultEmbeddingModel:    cfg.DefaultEmbeddingModel,
		defaultChatProvider:      cfg.DefaultChatProvider,
		defaultChatModel:         cfg.DefaultChatModel,
		logger:                   logger,
	}
}

// SetProviderRegistry sets the provider registry.
func (s *HostLLMServer) SetProviderRegistry(registry *ProviderRegistry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providerRegistry = registry
}

// SetDefaults updates the default provider and model settings.
func (s *HostLLMServer) SetDefaults(embeddingProvider, embeddingModel, chatProvider, chatModel string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.defaultEmbeddingProvider = embeddingProvider
	s.defaultEmbeddingModel = embeddingModel
	s.defaultChatProvider = chatProvider
	s.defaultChatModel = chatModel
}

// getProvider returns the provider for the given ID, applying defaults.
func (s *HostLLMServer) getProvider(providerID string, useChat bool) (*RegisteredProvider, string, error) {
	s.mu.RLock()
	registry := s.providerRegistry
	defaultProvider := s.defaultEmbeddingProvider
	if useChat {
		defaultProvider = s.defaultChatProvider
	}
	s.mu.RUnlock()

	if registry == nil {
		return nil, "", ErrNoProvider
	}

	if providerID == "" {
		providerID = defaultProvider
	}
	if providerID == "" {
		return nil, "", errors.New("no provider specified and no default configured")
	}

	provider := registry.Get(providerID)
	if provider == nil {
		return nil, "", errors.New("provider not found: " + providerID)
	}

	return provider, providerID, nil
}

// ListProviders returns available providers.
func (s *HostLLMServer) ListProviders(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.LLMProviderList, error) {
	s.mu.RLock()
	registry := s.providerRegistry
	s.mu.RUnlock()

	var list []*pluginv1.LLMProvider
	if registry != nil {
		for _, p := range registry.List() {
			list = append(list, &pluginv1.LLMProvider{
				Id:                p.Capabilities.ProviderId,
				Name:              p.Capabilities.DisplayName,
				Configured:        true,
				SupportsChat:      p.Capabilities.SupportsChat,
				SupportsEmbedding: p.Capabilities.SupportsEmbeddings,
			})
		}
	}
	return &pluginv1.LLMProviderList{Providers: list}, nil
}

// ListModels returns available models for a provider.
func (s *HostLLMServer) ListModels(ctx context.Context, req *pluginv1.LLMProviderQuery) (*pluginv1.LLMModelList, error) {
	provider, _, err := s.getProvider(req.ProviderId, false)
	if err != nil {
		return nil, err
	}

	resp, err := provider.Client.ListModels(ctx, &pluginv1.Empty{})
	if err != nil {
		return nil, err
	}

	models := make([]*pluginv1.LLMModel, len(resp.Models))
	for i, m := range resp.Models {
		models[i] = &pluginv1.LLMModel{
			Id:            m.Id,
			Name:          m.Name,
			Provider:      provider.ProviderID,
			IsEmbedding:   m.IsEmbedding,
			ContextLength: m.ContextLength,
		}
	}
	return &pluginv1.LLMModelList{Models: models}, nil
}

// GenerateEmbedding generates an embedding for a single text.
func (s *HostLLMServer) GenerateEmbedding(ctx context.Context, req *pluginv1.EmbeddingRequest) (*pluginv1.EmbeddingResponse, error) {
	provider, providerID, err := s.getProvider(req.Provider, false)
	if err != nil {
		return nil, err
	}
	if !provider.Capabilities.SupportsEmbeddings {
		return nil, errors.New("provider does not support embeddings: " + providerID)
	}

	model := s.resolveModel(req.Model, s.defaultEmbeddingModel, provider.Capabilities.DefaultEmbeddingModel)

	return provider.Client.GenerateEmbedding(ctx, &pluginv1.ProviderEmbeddingRequest{
		Text:  req.Text,
		Model: model,
	})
}

// GenerateEmbeddingBatch generates embeddings for multiple texts.
func (s *HostLLMServer) GenerateEmbeddingBatch(ctx context.Context, req *pluginv1.EmbeddingBatchRequest) (*pluginv1.EmbeddingBatchResponse, error) {
	provider, providerID, err := s.getProvider(req.Provider, false)
	if err != nil {
		return nil, err
	}
	if !provider.Capabilities.SupportsEmbeddings {
		return nil, errors.New("provider does not support embeddings: " + providerID)
	}

	model := s.resolveModel(req.Model, s.defaultEmbeddingModel, provider.Capabilities.DefaultEmbeddingModel)

	return provider.Client.GenerateEmbeddingBatch(ctx, &pluginv1.ProviderEmbeddingBatchRequest{
		Texts: req.Texts,
		Model: model,
	})
}

// Chat sends a chat completion request.
func (s *HostLLMServer) Chat(ctx context.Context, req *pluginv1.ChatRequest) (*pluginv1.ChatResponse, error) {
	provider, providerID, err := s.getProvider(req.Provider, true)
	if err != nil {
		return nil, err
	}
	if !provider.Capabilities.SupportsChat {
		return nil, errors.New("provider does not support chat: " + providerID)
	}

	model := s.resolveModel(req.Model, s.defaultChatModel, provider.Capabilities.DefaultChatModel)

	return provider.Client.Chat(ctx, &pluginv1.ProviderChatRequest{
		Messages:    req.Messages,
		Model:       model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	})
}

// ChatStream sends a streaming chat completion request.
func (s *HostLLMServer) ChatStream(req *pluginv1.ChatRequest, stream pluginv1.HostLLM_ChatStreamServer) error {
	provider, providerID, err := s.getProvider(req.Provider, true)
	if err != nil {
		return err
	}
	if !provider.Capabilities.SupportsChat {
		return errors.New("provider does not support chat: " + providerID)
	}

	model := s.resolveModel(req.Model, s.defaultChatModel, provider.Capabilities.DefaultChatModel)

	providerStream, err := provider.Client.ChatStream(stream.Context(), &pluginv1.ProviderChatRequest{
		Messages:    req.Messages,
		Model:       model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	})
	if err != nil {
		return err
	}

	for {
		chunk, err := providerStream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := stream.Send(chunk); err != nil {
			return err
		}
		if chunk.Done {
			return nil
		}
	}
}

// resolveModel returns the first non-empty model from the candidates.
// If no model is specified at the host level, returns empty to let the provider use its configured default.
func (s *HostLLMServer) resolveModel(requested, defaultModel, _ string) string {
	if requested != "" {
		return requested
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if defaultModel != "" {
		return defaultModel
	}
	// Don't use cached provider default - let the provider use its own configured model
	return ""
}

var _ pluginv1.HostLLMServer = (*HostLLMServer)(nil)
