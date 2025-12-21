// Package providers implements LLM and embedding providers.
package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mantonx/viewra/internal/domain/ai"
)

const (
	defaultOllamaBaseURL = "http://localhost:11434"
	ollamaTimeout        = 120 * time.Second
)

// OllamaProvider implements LLMProvider and EmbeddingProvider for Ollama.
type OllamaProvider struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	if baseURL == "" {
		baseURL = defaultOllamaBaseURL
	}
	return &OllamaProvider{
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout: ollamaTimeout,
		},
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

// Chat sends a chat completion request.
func (p *OllamaProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	ollamaReq := ollamaChatRequest{
		Model:    p.model,
		Messages: make([]ollamaMessage, len(req.Messages)),
		Stream:   false,
		Options: ollamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
		},
	}

	for i, msg := range req.Messages {
		ollamaReq.Messages[i] = ollamaMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ai.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &ai.ChatResponse{
		Content:      ollamaResp.Message.Content,
		FinishReason: ollamaResp.DoneReason,
		Usage: ai.TokenUsage{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		},
	}, nil
}

// ChatStream sends a streaming chat completion request.
func (p *OllamaProvider) ChatStream(ctx context.Context, req ai.ChatRequest) (<-chan ai.ChatStreamEvent, error) {
	ollamaReq := ollamaChatRequest{
		Model:    p.model,
		Messages: make([]ollamaMessage, len(req.Messages)),
		Stream:   true,
		Options: ollamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
		},
	}

	for i, msg := range req.Messages {
		ollamaReq.Messages[i] = ollamaMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ai.ErrProviderUnavailable, err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(respBody))
	}

	events := make(chan ai.ChatStreamEvent)
	go func() {
		defer resp.Body.Close()
		defer close(events)

		decoder := json.NewDecoder(resp.Body)
		for {
			var chunk ollamaChatResponse
			if err := decoder.Decode(&chunk); err != nil {
				if err == io.EOF {
					return
				}
				select {
				case events <- ai.ChatStreamEvent{Error: err}:
				case <-ctx.Done():
				}
				return
			}

			event := ai.ChatStreamEvent{
				Content:      chunk.Message.Content,
				Done:         chunk.Done,
				FinishReason: chunk.DoneReason,
			}

			select {
			case events <- event:
			case <-ctx.Done():
				return
			}

			if chunk.Done {
				return
			}
		}
	}()

	return events, nil
}

// HealthCheck verifies Ollama is accessible.
func (p *OllamaProvider) HealthCheck(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%w: %v", ai.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", ai.ErrProviderUnavailable, resp.StatusCode)
	}

	return nil
}

// Embed generates embeddings using Ollama.
func (p *OllamaProvider) Embed(ctx context.Context, req ai.EmbeddingRequest) (*ai.EmbeddingResponse, error) {
	embeddings := make([][]float32, len(req.Texts))

	for i, text := range req.Texts {
		ollamaReq := ollamaEmbedRequest{
			Model:  p.model,
			Prompt: text,
		}

		body, err := json.Marshal(ollamaReq)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/embeddings", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := p.httpClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ai.ErrProviderUnavailable, err)
		}

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("ollama embedding error (status %d): %s", resp.StatusCode, string(respBody))
		}

		var ollamaResp ollamaEmbedResponse
		if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode response: %w", err)
		}
		resp.Body.Close()

		// Convert float64 to float32
		embedding := make([]float32, len(ollamaResp.Embedding))
		for j, v := range ollamaResp.Embedding {
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
	switch p.model {
	case "nomic-embed-text":
		return 768
	case "all-minilm", "all-minilm:latest":
		return 384
	case "mxbai-embed-large":
		return 1024
	case "bge-base", "bge-base-en-v1.5":
		return 768
	case "bge-large", "bge-large-en-v1.5":
		return 1024
	default:
		return 768 // Default assumption
	}
}

// Ollama API types

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  ollamaOptions   `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaChatResponse struct {
	Model           string        `json:"model"`
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	DoneReason      string        `json:"done_reason,omitempty"`
	PromptEvalCount int           `json:"prompt_eval_count,omitempty"`
	EvalCount       int           `json:"eval_count,omitempty"`
}

type ollamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbedResponse struct {
	Embedding []float64 `json:"embedding"`
}

// ListModels returns available models from Ollama.
func (p *OllamaProvider) ListModels(ctx context.Context) ([]ai.ModelInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ai.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama error: status %d", resp.StatusCode)
	}

	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]ai.ModelInfo, len(tagsResp.Models))
	for i, m := range tagsResp.Models {
		models[i] = ai.ModelInfo{
			ID:          m.Name,
			Name:        m.Name,
			IsChat:      true,
			IsEmbedding: isEmbeddingModel(m.Name),
		}
	}

	return models, nil
}

// isEmbeddingModel checks if a model name indicates an embedding model.
func isEmbeddingModel(name string) bool {
	embeddingModels := []string{
		"nomic-embed",
		"all-minilm",
		"mxbai-embed",
		"bge-",
		"e5-",
		"embed",
	}
	for _, prefix := range embeddingModels {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// Pull downloads a model from the Ollama registry with progress streaming.
// The progress channel receives updates during the download.
// The channel is closed when the pull is complete or on error.
func (p *OllamaProvider) Pull(ctx context.Context, modelName string) (<-chan ai.PullProgress, error) {
	pullReq := ollamaPullRequest{
		Name:   modelName,
		Stream: true,
	}

	body, err := json.Marshal(pullReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Use a longer timeout for model pulls (can take many minutes)
	client := &http.Client{
		Timeout: 0, // No timeout for streaming
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/pull", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ai.ErrProviderUnavailable, err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("ollama pull error (status %d): %s", resp.StatusCode, string(respBody))
	}

	progress := make(chan ai.PullProgress)
	go func() {
		defer resp.Body.Close()
		defer close(progress)

		decoder := json.NewDecoder(resp.Body)
		for {
			var pullResp ollamaPullResponse
			if err := decoder.Decode(&pullResp); err != nil {
				if err == io.EOF {
					return
				}
				select {
				case progress <- ai.PullProgress{Error: err.Error(), Done: true}:
				case <-ctx.Done():
				}
				return
			}

			// Calculate percentage if we have total/completed
			var percent float64
			if pullResp.Total > 0 {
				percent = float64(pullResp.Completed) / float64(pullResp.Total) * 100
			}

			event := ai.PullProgress{
				Status:    pullResp.Status,
				Digest:    pullResp.Digest,
				Total:     pullResp.Total,
				Completed: pullResp.Completed,
				Percent:   percent,
				Done:      pullResp.Status == "success",
			}

			select {
			case progress <- event:
			case <-ctx.Done():
				return
			}

			if pullResp.Status == "success" {
				return
			}
		}
	}()

	return progress, nil
}

// DeleteModel removes a model from the local Ollama installation.
func (p *OllamaProvider) DeleteModel(ctx context.Context, modelName string) error {
	deleteReq := ollamaDeleteRequest{
		Name: modelName,
	}

	body, err := json.Marshal(deleteReq)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, p.baseURL+"/api/delete", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%w: %v", ai.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama delete error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Ollama pull/delete API types

type ollamaPullRequest struct {
	Name   string `json:"name"`
	Stream bool   `json:"stream"`
}

type ollamaPullResponse struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
}

type ollamaDeleteRequest struct {
	Name string `json:"name"`
}
