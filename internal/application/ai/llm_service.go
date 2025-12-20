package ai

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/mantonx/viewra/internal/domain/ai"
)

// LLMService handles LLM chat completions with cost tracking.
type LLMService struct {
	provider ai.LLMProvider
	logger   *slog.Logger
	mu       sync.RWMutex
}

// NewLLMService creates a new LLM service.
func NewLLMService(provider ai.LLMProvider, logger *slog.Logger) *LLMService {
	if logger == nil {
		logger = slog.Default()
	}
	return &LLMService{
		provider: provider,
		logger:   logger,
	}
}

// SetProvider updates the LLM provider.
func (s *LLMService) SetProvider(provider ai.LLMProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.provider = provider
}

// GetProvider returns the current LLM provider.
func (s *LLMService) GetProvider() ai.LLMProvider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.provider
}

// Chat sends a chat completion request.
func (s *LLMService) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	s.mu.RLock()
	provider := s.provider
	s.mu.RUnlock()

	if provider == nil {
		return nil, ai.ErrProviderNotConfigured
	}

	resp, err := provider.Chat(ctx, req)
	if err != nil {
		s.logger.Error("chat completion failed",
			slog.String("provider", provider.Name()),
			slog.String("model", provider.Model()),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("chat completion: %w", err)
	}

	s.logger.Debug("chat completion",
		slog.String("provider", provider.Name()),
		slog.String("model", provider.Model()),
		slog.Int("prompt_tokens", resp.Usage.PromptTokens),
		slog.Int("completion_tokens", resp.Usage.CompletionTokens),
	)

	return resp, nil
}

// ChatStream sends a streaming chat completion request.
func (s *LLMService) ChatStream(ctx context.Context, req ai.ChatRequest) (<-chan ai.ChatStreamEvent, error) {
	s.mu.RLock()
	provider := s.provider
	s.mu.RUnlock()

	if provider == nil {
		return nil, ai.ErrProviderNotConfigured
	}

	events, err := provider.ChatStream(ctx, req)
	if err != nil {
		s.logger.Error("chat stream failed",
			slog.String("provider", provider.Name()),
			slog.String("model", provider.Model()),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("chat stream: %w", err)
	}

	return events, nil
}

// SimpleChat sends a simple user message and returns the response.
func (s *LLMService) SimpleChat(ctx context.Context, userMessage string) (string, error) {
	req := ai.ChatRequest{
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: userMessage},
		},
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	resp, err := s.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

// ChatWithSystem sends a chat request with a system message.
func (s *LLMService) ChatWithSystem(ctx context.Context, systemMessage, userMessage string) (string, error) {
	req := ai.ChatRequest{
		Messages: []ai.Message{
			{Role: ai.RoleSystem, Content: systemMessage},
			{Role: ai.RoleUser, Content: userMessage},
		},
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	resp, err := s.Chat(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

// HealthCheck verifies the LLM provider is accessible.
func (s *LLMService) HealthCheck(ctx context.Context) error {
	s.mu.RLock()
	provider := s.provider
	s.mu.RUnlock()

	if provider == nil {
		return ai.ErrProviderNotConfigured
	}

	return provider.HealthCheck(ctx)
}

// ProviderInfo returns information about the current provider.
func (s *LLMService) ProviderInfo() (name, model string) {
	s.mu.RLock()
	provider := s.provider
	s.mu.RUnlock()

	if provider == nil {
		return "", ""
	}

	return provider.Name(), provider.Model()
}

// IsConfigured returns true if an LLM provider is configured.
func (s *LLMService) IsConfigured() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.provider != nil
}
