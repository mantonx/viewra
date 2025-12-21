package providers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/mantonx/viewra/internal/domain/ai"
)

// OpenAIProvider implements LLMProvider and EmbeddingProvider using the official OpenAI SDK.
type OpenAIProvider struct {
	client       openai.Client
	model        string
	providerType ai.ProviderType
}

// NewOpenAIProvider creates a new OpenAI provider using the official SDK.
func NewOpenAIProvider(apiKey, model string) *OpenAIProvider {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return &OpenAIProvider{
		client:       client,
		model:        model,
		providerType: ai.ProviderOpenAI,
	}
}

// NewOpenRouterProvider creates a new OpenRouter provider (OpenAI-compatible).
func NewOpenRouterProvider(apiKey, model string) *OpenAIProvider {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL("https://openrouter.ai/api/v1"),
		option.WithHeader("HTTP-Referer", "https://viewra.app"),
		option.WithHeader("X-Title", "Viewra"),
	)
	return &OpenAIProvider{
		client:       client,
		model:        model,
		providerType: ai.ProviderOpenRouter,
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

// Chat sends a chat completion request using the OpenAI SDK.
func (p *OpenAIProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	messages := make([]openai.ChatCompletionMessageParamUnion, len(req.Messages))
	for i, msg := range req.Messages {
		switch msg.Role {
		case ai.RoleSystem:
			messages[i] = openai.SystemMessage(msg.Content)
		case ai.RoleUser:
			messages[i] = openai.UserMessage(msg.Content)
		case ai.RoleAssistant:
			messages[i] = openai.AssistantMessage(msg.Content)
		default:
			messages[i] = openai.UserMessage(msg.Content)
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(p.model),
		Messages: messages,
	}

	if req.MaxTokens > 0 {
		params.MaxTokens = openai.Int(int64(req.MaxTokens))
	}
	if req.Temperature > 0 {
		params.Temperature = openai.Float(req.Temperature)
	}

	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, p.mapError(err)
	}

	if len(resp.Choices) == 0 {
		return nil, ai.NewProviderError(p.Name(), "no_choices",
			"No completion choices returned", 500, nil)
	}

	return &ai.ChatResponse{
		Content:      resp.Choices[0].Message.Content,
		FinishReason: string(resp.Choices[0].FinishReason),
		Usage: ai.TokenUsage{
			PromptTokens:     int(resp.Usage.PromptTokens),
			CompletionTokens: int(resp.Usage.CompletionTokens),
			TotalTokens:      int(resp.Usage.TotalTokens),
		},
	}, nil
}

// ChatStream sends a streaming chat completion request using the OpenAI SDK.
func (p *OpenAIProvider) ChatStream(ctx context.Context, req ai.ChatRequest) (<-chan ai.ChatStreamEvent, error) {
	messages := make([]openai.ChatCompletionMessageParamUnion, len(req.Messages))
	for i, msg := range req.Messages {
		switch msg.Role {
		case ai.RoleSystem:
			messages[i] = openai.SystemMessage(msg.Content)
		case ai.RoleUser:
			messages[i] = openai.UserMessage(msg.Content)
		case ai.RoleAssistant:
			messages[i] = openai.AssistantMessage(msg.Content)
		default:
			messages[i] = openai.UserMessage(msg.Content)
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(p.model),
		Messages: messages,
	}

	if req.MaxTokens > 0 {
		params.MaxTokens = openai.Int(int64(req.MaxTokens))
	}
	if req.Temperature > 0 {
		params.Temperature = openai.Float(req.Temperature)
	}

	stream := p.client.Chat.Completions.NewStreaming(ctx, params)

	events := make(chan ai.ChatStreamEvent)

	go func() {
		defer close(events)

		for stream.Next() {
			chunk := stream.Current()

			if len(chunk.Choices) == 0 {
				continue
			}

			choice := chunk.Choices[0]
			event := ai.ChatStreamEvent{
				Content:      choice.Delta.Content,
				Done:         choice.FinishReason != "",
				FinishReason: string(choice.FinishReason),
			}

			select {
			case events <- event:
			case <-ctx.Done():
				return
			}
		}

		if err := stream.Err(); err != nil && !errors.Is(err, context.Canceled) {
			select {
			case events <- ai.ChatStreamEvent{Error: p.mapError(err)}:
			case <-ctx.Done():
			}
		}
	}()

	return events, nil
}

// Embed generates embeddings using the OpenAI SDK.
func (p *OpenAIProvider) Embed(ctx context.Context, req ai.EmbeddingRequest) (*ai.EmbeddingResponse, error) {
	embeddingModel := p.model
	if !strings.Contains(embeddingModel, "embed") {
		embeddingModel = "text-embedding-3-small" // Default embedding model
	}

	params := openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(embeddingModel),
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: req.Texts,
		},
	}

	resp, err := p.client.Embeddings.New(ctx, params)
	if err != nil {
		return nil, p.mapError(err)
	}

	// Convert []float64 to [][]float32
	embeddings := make([][]float32, len(resp.Data))
	for i, data := range resp.Data {
		embedding := make([]float32, len(data.Embedding))
		for j, v := range data.Embedding {
			embedding[j] = float32(v)
		}
		embeddings[i] = embedding
	}

	return &ai.EmbeddingResponse{
		Embeddings: embeddings,
		Usage: ai.TokenUsage{
			PromptTokens: int(resp.Usage.PromptTokens),
			TotalTokens:  int(resp.Usage.TotalTokens),
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

// HealthCheck verifies the API is accessible using the SDK.
func (p *OpenAIProvider) HealthCheck(ctx context.Context) error {
	// List models to verify API access
	_, err := p.client.Models.List(ctx)
	if err != nil {
		return p.mapError(err)
	}
	return nil
}

// ListModels fetches available models from the API using the SDK.
func (p *OpenAIProvider) ListModels(ctx context.Context) ([]ai.ModelInfo, error) {
	resp, err := p.client.Models.List(ctx)
	if err != nil {
		return nil, p.mapError(err)
	}

	var models []ai.ModelInfo
	for _, model := range resp.Data {
		// Filter to relevant models
		isChat := strings.Contains(model.ID, "gpt") || strings.Contains(model.ID, "chat") ||
			strings.Contains(model.ID, "o1") || strings.Contains(model.ID, "o3")
		isEmbedding := strings.Contains(model.ID, "embed")

		if isChat || isEmbedding {
			costTier := ai.CostTierMedium
			if strings.Contains(model.ID, "mini") || strings.Contains(model.ID, "small") {
				costTier = ai.CostTierLow
			} else if strings.Contains(model.ID, "o1") || strings.Contains(model.ID, "o3") {
				costTier = ai.CostTierHigh
			}

			models = append(models, ai.ModelInfo{
				ID:          model.ID,
				Name:        model.ID,
				IsChat:      isChat,
				IsEmbedding: isEmbedding,
				CostTier:    costTier,
			})
		}
	}

	return models, nil
}

// mapError converts OpenAI SDK errors to domain errors with rich details.
func (p *OpenAIProvider) mapError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// The OpenAI SDK wraps errors, try to extract useful info
	switch {
	case strings.Contains(errMsg, "401"),
		strings.Contains(errMsg, "invalid_api_key"),
		strings.Contains(errMsg, "Incorrect API key"):
		return ai.NewProviderError(p.Name(), "invalid_api_key",
			"Invalid API key. Please check your API key in settings.",
			401, err)

	case strings.Contains(errMsg, "429"),
		strings.Contains(errMsg, "rate_limit"):
		pe := ai.NewProviderError(p.Name(), "rate_limit_exceeded",
			"Rate limit exceeded. Please wait and try again.",
			429, err)
		// Try to extract retry-after if present
		if strings.Contains(errMsg, "Please retry after") {
			// Extract the retry time if we can parse it
			pe.RetryAfter = "60s" // Default fallback
		}
		return pe

	case strings.Contains(errMsg, "404"),
		strings.Contains(errMsg, "model_not_found"):
		return ai.NewProviderError(p.Name(), "model_not_found",
			fmt.Sprintf("Model '%s' not found or not accessible with your API key.", p.model),
			404, err)

	case strings.Contains(errMsg, "context_length_exceeded"):
		return ai.NewProviderError(p.Name(), "context_length_exceeded",
			"Input too long for this model. Try reducing your message length.",
			400, err)

	case strings.Contains(errMsg, "insufficient_quota"):
		return ai.NewProviderError(p.Name(), "insufficient_quota",
			"Insufficient API quota. Please check your billing settings.",
			402, err)

	case strings.Contains(errMsg, "connection refused"),
		strings.Contains(errMsg, "no such host"),
		strings.Contains(errMsg, "network"):
		return ai.NewProviderError(p.Name(), "provider_unavailable",
			"Cannot connect to OpenAI. Please check your network connection.",
			503, err)

	default:
		return ai.NewProviderError(p.Name(), "unknown_error", errMsg, 500, err)
	}
}
