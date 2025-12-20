package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mantonx/viewra/internal/domain/ai"
)

const (
	defaultOpenAIBaseURL     = "https://api.openai.com/v1"
	defaultOpenRouterBaseURL = "https://openrouter.ai/api/v1"
	openAITimeout            = 120 * time.Second
)

// OpenAIProvider implements LLMProvider and EmbeddingProvider for OpenAI and compatible APIs.
type OpenAIProvider struct {
	baseURL      string
	apiKey       string
	model        string
	providerType ai.ProviderType
	httpClient   *http.Client
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	return &OpenAIProvider{
		baseURL:      defaultOpenAIBaseURL,
		apiKey:       apiKey,
		model:        model,
		providerType: ai.ProviderOpenAI,
		httpClient: &http.Client{
			Timeout: openAITimeout,
		},
	}
}

// NewOpenRouterProvider creates a new OpenRouter provider (OpenAI-compatible).
func NewOpenRouterProvider(apiKey, model string) *OpenAIProvider {
	return &OpenAIProvider{
		baseURL:      defaultOpenRouterBaseURL,
		apiKey:       apiKey,
		model:        model,
		providerType: ai.ProviderOpenRouter,
		httpClient: &http.Client{
			Timeout: openAITimeout,
		},
	}
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	if p.providerType == ai.ProviderOpenRouter {
		return "OpenRouter"
	}
	return "OpenAI"
}

// Model returns the current model name.
func (p *OpenAIProvider) Model() string {
	return p.model
}

// Chat sends a chat completion request.
func (p *OpenAIProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	openAIReq := openAIChatRequest{
		Model:       p.model,
		Messages:    make([]openAIMessage, len(req.Messages)),
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      false,
	}

	for i, msg := range req.Messages {
		openAIReq.Messages[i] = openAIMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	body, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ai.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()

	if err := p.handleErrorResponse(resp); err != nil {
		return nil, err
	}

	var openAIResp openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &ai.ChatResponse{
		Content:      openAIResp.Choices[0].Message.Content,
		FinishReason: openAIResp.Choices[0].FinishReason,
		Usage: ai.TokenUsage{
			PromptTokens:     openAIResp.Usage.PromptTokens,
			CompletionTokens: openAIResp.Usage.CompletionTokens,
			TotalTokens:      openAIResp.Usage.TotalTokens,
		},
	}, nil
}

// ChatStream sends a streaming chat completion request.
func (p *OpenAIProvider) ChatStream(ctx context.Context, req ai.ChatRequest) (<-chan ai.ChatStreamEvent, error) {
	openAIReq := openAIChatRequest{
		Model:       p.model,
		Messages:    make([]openAIMessage, len(req.Messages)),
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      true,
	}

	for i, msg := range req.Messages {
		openAIReq.Messages[i] = openAIMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
	}

	body, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ai.ErrProviderUnavailable, err)
	}

	if err := p.handleErrorResponse(resp); err != nil {
		resp.Body.Close()
		return nil, err
	}

	events := make(chan ai.ChatStreamEvent)
	go func() {
		defer resp.Body.Close()
		defer close(events)

		reader := resp.Body
		buf := make([]byte, 4096)
		var partial string

		for {
			n, err := reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					select {
					case events <- ai.ChatStreamEvent{Error: err}:
					case <-ctx.Done():
					}
				}
				return
			}

			partial += string(buf[:n])
			lines := strings.Split(partial, "\n")

			// Keep the last incomplete line
			partial = lines[len(lines)-1]
			lines = lines[:len(lines)-1]

			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || line == "data: [DONE]" {
					continue
				}
				if !strings.HasPrefix(line, "data: ") {
					continue
				}

				data := strings.TrimPrefix(line, "data: ")
				var chunk openAIStreamChunk
				if err := json.Unmarshal([]byte(data), &chunk); err != nil {
					continue
				}

				if len(chunk.Choices) == 0 {
					continue
				}

				event := ai.ChatStreamEvent{
					Content:      chunk.Choices[0].Delta.Content,
					Done:         chunk.Choices[0].FinishReason != "",
					FinishReason: chunk.Choices[0].FinishReason,
				}

				select {
				case events <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return events, nil
}

// HealthCheck verifies the API is accessible.
func (p *OpenAIProvider) HealthCheck(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/models", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%w: %v", ai.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return ai.ErrInvalidAPIKey
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status %d", ai.ErrProviderUnavailable, resp.StatusCode)
	}

	return nil
}

// Embed generates embeddings using OpenAI.
func (p *OpenAIProvider) Embed(ctx context.Context, req ai.EmbeddingRequest) (*ai.EmbeddingResponse, error) {
	embeddingModel := p.model
	if !strings.Contains(embeddingModel, "embed") {
		embeddingModel = "text-embedding-3-small" // Default embedding model
	}

	openAIReq := openAIEmbedRequest{
		Model: embeddingModel,
		Input: req.Texts,
	}

	body, err := json.Marshal(openAIReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ai.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()

	if err := p.handleErrorResponse(resp); err != nil {
		return nil, err
	}

	var openAIResp openAIEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	embeddings := make([][]float32, len(openAIResp.Data))
	for i, data := range openAIResp.Data {
		embedding := make([]float32, len(data.Embedding))
		for j, v := range data.Embedding {
			embedding[j] = float32(v)
		}
		embeddings[i] = embedding
	}

	return &ai.EmbeddingResponse{
		Embeddings: embeddings,
		Usage: ai.TokenUsage{
			PromptTokens: openAIResp.Usage.PromptTokens,
			TotalTokens:  openAIResp.Usage.TotalTokens,
		},
	}, nil
}

// Dimensions returns the embedding dimensions for the current model.
func (p *OpenAIProvider) Dimensions() int {
	switch {
	case strings.Contains(p.model, "text-embedding-3-large"):
		return 3072
	case strings.Contains(p.model, "text-embedding-3-small"):
		return 1536
	case strings.Contains(p.model, "text-embedding-ada"):
		return 1536
	default:
		return 1536
	}
}

func (p *OpenAIProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	if p.providerType == ai.ProviderOpenRouter {
		req.Header.Set("HTTP-Referer", "https://viewra.app")
		req.Header.Set("X-Title", "Viewra")
	}
}

func (p *OpenAIProvider) handleErrorResponse(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ai.ErrInvalidAPIKey
	case http.StatusTooManyRequests:
		return ai.ErrRateLimitExceeded
	default:
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}
}

// ListModels fetches available models from the API.
func (p *OpenAIProvider) ListModels(ctx context.Context) ([]ai.ModelInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ai.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()

	if err := p.handleErrorResponse(resp); err != nil {
		return nil, err
	}

	var modelsResp struct {
		Data []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	models := make([]ai.ModelInfo, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		// Filter to relevant models
		isChat := strings.Contains(m.ID, "gpt") || strings.Contains(m.ID, "chat")
		isEmbedding := strings.Contains(m.ID, "embed")

		if isChat || isEmbedding {
			models = append(models, ai.ModelInfo{
				ID:          m.ID,
				Name:        m.ID,
				IsChat:      isChat,
				IsEmbedding: isEmbedding,
			})
		}
	}

	return models, nil
}

// OpenAI API types

type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message      openAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

type openAIEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openAIEmbedResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}
