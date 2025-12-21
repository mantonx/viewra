// Package providers implements LLM and embedding providers.
package providers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ollama/ollama/api"

	"github.com/mantonx/viewra/internal/domain/ai"
)

const (
	defaultOllamaBaseURL = "http://localhost:11434"
	ollamaTimeout        = 120 * time.Second
)

// OllamaProvider implements LLMProvider and EmbeddingProvider using the official Ollama SDK.
type OllamaProvider struct {
	client  *api.Client
	model   string
	baseURL string
}

// NewOllamaProvider creates a new Ollama provider using the official SDK.
func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	if baseURL == "" {
		baseURL = defaultOllamaBaseURL
	}

	// Parse the base URL for the SDK client
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		parsedURL, _ = url.Parse(defaultOllamaBaseURL)
	}

	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: ollamaTimeout,
	}

	client := api.NewClient(parsedURL, httpClient)

	return &OllamaProvider{
		client:  client,
		model:   model,
		baseURL: baseURL,
	}
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string {
	return "Ollama"
}

// Model returns the current model name.
func (p *OllamaProvider) Model() string {
	return p.model
}

// Chat sends a chat completion request using the Ollama SDK.
func (p *OllamaProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	messages := make([]api.Message, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = api.Message{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	ollamaReq := &api.ChatRequest{
		Model:    p.model,
		Messages: messages,
		Stream:   boolPtr(false),
		Options: map[string]any{
			"temperature": req.Temperature,
			"num_predict": req.MaxTokens,
		},
	}

	var response api.ChatResponse
	err := p.client.Chat(ctx, ollamaReq, func(resp api.ChatResponse) error {
		response = resp
		return nil
	})
	if err != nil {
		return nil, p.mapError(err)
	}

	return &ai.ChatResponse{
		Content:      response.Message.Content,
		FinishReason: response.DoneReason,
		Usage: ai.TokenUsage{
			PromptTokens:     response.PromptEvalCount,
			CompletionTokens: response.EvalCount,
			TotalTokens:      response.PromptEvalCount + response.EvalCount,
		},
	}, nil
}

// ChatStream sends a streaming chat completion request using the Ollama SDK.
func (p *OllamaProvider) ChatStream(ctx context.Context, req ai.ChatRequest) (<-chan ai.ChatStreamEvent, error) {
	messages := make([]api.Message, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = api.Message{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	ollamaReq := &api.ChatRequest{
		Model:    p.model,
		Messages: messages,
		Stream:   boolPtr(true),
		Options: map[string]any{
			"temperature": req.Temperature,
			"num_predict": req.MaxTokens,
		},
	}

	events := make(chan ai.ChatStreamEvent)

	go func() {
		defer close(events)

		err := p.client.Chat(ctx, ollamaReq, func(resp api.ChatResponse) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case events <- ai.ChatStreamEvent{
				Content:      resp.Message.Content,
				Done:         resp.Done,
				FinishReason: resp.DoneReason,
			}:
				return nil
			}
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			select {
			case events <- ai.ChatStreamEvent{Error: p.mapError(err)}:
			case <-ctx.Done():
			}
		}
	}()

	return events, nil
}

// Embed generates embeddings using the Ollama SDK.
func (p *OllamaProvider) Embed(ctx context.Context, req ai.EmbeddingRequest) (*ai.EmbeddingResponse, error) {
	ollamaReq := &api.EmbedRequest{
		Model: p.model,
		Input: req.Texts,
	}

	resp, err := p.client.Embed(ctx, ollamaReq)
	if err != nil {
		return nil, p.mapError(err)
	}

	// Convert [][]float64 to [][]float32
	embeddings := make([][]float32, len(resp.Embeddings))
	for i, emb := range resp.Embeddings {
		embedding := make([]float32, len(emb))
		for j, v := range emb {
			embedding[j] = float32(v)
		}
		embeddings[i] = embedding
	}

	return &ai.EmbeddingResponse{
		Embeddings: embeddings,
		Usage:      ai.TokenUsage{}, // Ollama doesn't report token usage for embeddings
	}, nil
}

// Dimensions returns the embedding dimensions for the current model.
func (p *OllamaProvider) Dimensions() int {
	// Common embedding model dimensions
	switch {
	case strings.HasPrefix(p.model, "nomic-embed"):
		return 768
	case strings.HasPrefix(p.model, "all-minilm"):
		return 384
	case strings.HasPrefix(p.model, "mxbai-embed-large"):
		return 1024
	case strings.HasPrefix(p.model, "bge-base"):
		return 768
	case strings.HasPrefix(p.model, "bge-large"):
		return 1024
	default:
		return 768 // Default assumption
	}
}

// HealthCheck verifies Ollama is accessible using the SDK.
func (p *OllamaProvider) HealthCheck(ctx context.Context) error {
	_, err := p.client.List(ctx)
	if err != nil {
		return p.mapError(err)
	}
	return nil
}

// ListModels returns available models from Ollama using the SDK.
func (p *OllamaProvider) ListModels(ctx context.Context) ([]ai.ModelInfo, error) {
	resp, err := p.client.List(ctx)
	if err != nil {
		return nil, p.mapError(err)
	}

	models := make([]ai.ModelInfo, len(resp.Models))
	for i, m := range resp.Models {
		isEmbedding := isEmbeddingModel(m.Name)
		models[i] = ai.ModelInfo{
			ID:          m.Name,
			Name:        m.Name,
			IsChat:      !isEmbedding, // Embedding models are not chat models
			IsEmbedding: isEmbedding,
			CostTier:    ai.CostTierFree, // Ollama is always free (local)
		}
	}

	return models, nil
}

// Pull downloads a model from the Ollama registry with progress streaming.
func (p *OllamaProvider) Pull(ctx context.Context, modelName string) (<-chan ai.PullProgress, error) {
	progress := make(chan ai.PullProgress)

	go func() {
		defer close(progress)

		req := &api.PullRequest{
			Model:  modelName,
			Stream: boolPtr(true),
		}

		err := p.client.Pull(ctx, req, func(resp api.ProgressResponse) error {
			var percent float64
			if resp.Total > 0 {
				percent = float64(resp.Completed) / float64(resp.Total) * 100
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case progress <- ai.PullProgress{
				Status:    resp.Status,
				Digest:    resp.Digest,
				Total:     resp.Total,
				Completed: resp.Completed,
				Percent:   percent,
				Done:      resp.Status == "success",
			}:
				return nil
			}
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			select {
			case progress <- ai.PullProgress{Error: err.Error(), Done: true}:
			case <-ctx.Done():
			}
		}
	}()

	return progress, nil
}

// DeleteModel removes a model from the local Ollama installation.
func (p *OllamaProvider) DeleteModel(ctx context.Context, modelName string) error {
	req := &api.DeleteRequest{
		Model: modelName,
	}

	err := p.client.Delete(ctx, req)
	if err != nil {
		return p.mapError(err)
	}
	return nil
}

// mapError converts Ollama SDK errors to domain errors with rich details.
func (p *OllamaProvider) mapError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// Check for specific error patterns
	switch {
	case strings.Contains(errMsg, "connection refused"),
		strings.Contains(errMsg, "no such host"),
		strings.Contains(errMsg, "network is unreachable"):
		return ai.NewProviderError("ollama", "provider_unavailable",
			fmt.Sprintf("Cannot connect to Ollama at %s. Is Ollama running?", p.baseURL),
			503, err)

	case strings.Contains(errMsg, "model") && strings.Contains(errMsg, "not found"):
		return ai.NewProviderError("ollama", "model_not_found",
			fmt.Sprintf("Model '%s' not found. Try pulling it first with 'ollama pull %s'", p.model, p.model),
			404, err)

	case strings.Contains(errMsg, "context deadline exceeded"),
		strings.Contains(errMsg, "timeout"):
		return ai.NewProviderError("ollama", "timeout",
			"Request timed out. The model may be loading or the server is overloaded.",
			408, err)

	default:
		return ai.NewProviderError("ollama", "unknown_error", errMsg, 500, err)
	}
}

// isEmbeddingModel checks if a model name indicates an embedding model.
func isEmbeddingModel(name string) bool {
	embeddingPrefixes := []string{
		"nomic-embed",
		"all-minilm",
		"mxbai-embed",
		"bge-",
		"e5-",
		"embed",
	}
	nameLower := strings.ToLower(name)
	for _, prefix := range embeddingPrefixes {
		if strings.Contains(nameLower, prefix) {
			return true
		}
	}
	return false
}

// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool {
	return &b
}
