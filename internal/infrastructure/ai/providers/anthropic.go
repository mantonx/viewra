package providers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/mantonx/viewra/internal/domain/ai"
)

// AnthropicProvider implements LLMProvider using the official Anthropic SDK.
type AnthropicProvider struct {
	client anthropic.Client
	model  string
}

// NewAnthropicProvider creates a new Anthropic provider using the official SDK.
func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &AnthropicProvider{
		client: client,
		model:  model,
	}
}

// Name returns the provider name.
func (p *AnthropicProvider) Name() string {
	return "Anthropic"
}

// Model returns the current model name.
func (p *AnthropicProvider) Model() string {
	return p.model
}

// Chat sends a chat completion request using the Anthropic SDK.
func (p *AnthropicProvider) Chat(ctx context.Context, req ai.ChatRequest) (*ai.ChatResponse, error) {
	params, err := p.buildMessageParams(req)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, p.mapError(err)
	}

	// Extract text content from response
	var content string
	for _, block := range resp.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			content += text.Text
		}
	}

	return &ai.ChatResponse{
		Content:      content,
		FinishReason: string(resp.StopReason),
		Usage: ai.TokenUsage{
			PromptTokens:     int(resp.Usage.InputTokens),
			CompletionTokens: int(resp.Usage.OutputTokens),
			TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		},
	}, nil
}

// ChatStream sends a streaming chat completion request using the Anthropic SDK.
func (p *AnthropicProvider) ChatStream(ctx context.Context, req ai.ChatRequest) (<-chan ai.ChatStreamEvent, error) {
	params, err := p.buildMessageParams(req)
	if err != nil {
		return nil, err
	}

	stream := p.client.Messages.NewStreaming(ctx, params)

	events := make(chan ai.ChatStreamEvent)

	go func() {
		defer close(events)

		for stream.Next() {
			event := stream.Current()

			switch variant := event.AsAny().(type) {
			case anthropic.ContentBlockDeltaEvent:
				switch delta := variant.Delta.AsAny().(type) {
				case anthropic.TextDelta:
					select {
					case events <- ai.ChatStreamEvent{Content: delta.Text}:
					case <-ctx.Done():
						return
					}
				}
			case anthropic.MessageStopEvent:
				select {
				case events <- ai.ChatStreamEvent{Done: true}:
				case <-ctx.Done():
					return
				}
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

// ListModels fetches available models from the Anthropic API.
func (p *AnthropicProvider) ListModels(ctx context.Context) ([]ai.ModelInfo, error) {
	page, err := p.client.Models.List(ctx, anthropic.ModelListParams{})
	if err != nil {
		return nil, p.mapError(err)
	}

	var models []ai.ModelInfo
	for _, model := range page.Data {
		// Determine cost tier based on model name
		costTier := ai.CostTierMedium
		if strings.Contains(model.ID, "haiku") {
			costTier = ai.CostTierLow
		} else if strings.Contains(model.ID, "opus") {
			costTier = ai.CostTierHigh
		}

		// Mark latest sonnet as recommended
		recommended := strings.Contains(model.ID, "sonnet-4-5") || strings.Contains(model.ID, "sonnet-4.5")

		models = append(models, ai.ModelInfo{
			ID:          model.ID,
			Name:        formatAnthropicModelName(model.ID),
			Description: model.DisplayName,
			ContextSize: 200000, // All Claude models support 200k context
			IsChat:      true,
			CostTier:    costTier,
			Recommended: recommended,
		})
	}

	return models, nil
}

// formatAnthropicModelName converts model ID to a display name.
func formatAnthropicModelName(modelID string) string {
	// Convert "claude-sonnet-4-5-20250929" to "Claude Sonnet 4.5"
	name := strings.ReplaceAll(modelID, "-", " ")
	name = strings.Title(name)

	// Remove date suffix for cleaner display
	parts := strings.Split(name, " ")
	var cleanParts []string
	for _, part := range parts {
		// Skip if it looks like a date (8 digits)
		if len(part) == 8 {
			allDigits := true
			for _, c := range part {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				continue
			}
		}
		cleanParts = append(cleanParts, part)
	}

	return strings.Join(cleanParts, " ")
}

// HealthCheck verifies the Anthropic API is accessible using the SDK.
func (p *AnthropicProvider) HealthCheck(ctx context.Context) error {
	// Use models list endpoint to verify API access
	_, err := p.client.Models.List(ctx, anthropic.ModelListParams{})
	if err != nil {
		mapped := p.mapError(err)
		var pe *ai.ProviderError
		if errors.As(mapped, &pe) && pe.Code == "invalid_api_key" {
			return mapped
		}
		// For other errors, we consider the service reachable
		return nil
	}
	return nil
}

// HealthCheckLegacy verifies the Anthropic API using a minimal chat request.
// This is a fallback if the models endpoint is not available.
func (p *AnthropicProvider) HealthCheckLegacy(ctx context.Context) error {
	_, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 1,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("hi")),
		},
	})
	if err != nil {
		// Return the error only if it's an API key issue
		mapped := p.mapError(err)
		var pe *ai.ProviderError
		if errors.As(mapped, &pe) && pe.Code == "invalid_api_key" {
			return mapped
		}
		// For other errors, we consider the service reachable
		return nil
	}
	return nil
}

// buildMessageParams converts our domain ChatRequest to Anthropic SDK params.
func (p *AnthropicProvider) buildMessageParams(req ai.ChatRequest) (anthropic.MessageNewParams, error) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: int64(req.MaxTokens),
	}

	// Anthropic requires max_tokens
	if params.MaxTokens == 0 {
		params.MaxTokens = 4096
	}

	// Build messages, handling system message separately
	messages := make([]anthropic.MessageParam, 0, len(req.Messages))
	for _, msg := range req.Messages {
		switch msg.Role {
		case ai.RoleSystem:
			// Anthropic handles system messages as a separate parameter
			params.System = []anthropic.TextBlockParam{
				{Text: msg.Content},
			}
		case ai.RoleUser:
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		case ai.RoleAssistant:
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)))
		default:
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		}
	}
	params.Messages = messages

	if req.Temperature > 0 {
		params.Temperature = anthropic.Float(req.Temperature)
	}

	return params, nil
}

// mapError converts Anthropic SDK errors to domain errors with rich details.
func (p *AnthropicProvider) mapError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	switch {
	case strings.Contains(errMsg, "401"),
		strings.Contains(errMsg, "authentication_error"),
		strings.Contains(errMsg, "invalid x-api-key"):
		return ai.NewProviderError(p.Name(), "invalid_api_key",
			"Invalid API key. Please check your Anthropic API key in settings.",
			401, err)

	case strings.Contains(errMsg, "429"),
		strings.Contains(errMsg, "rate_limit"):
		pe := ai.NewProviderError(p.Name(), "rate_limit_exceeded",
			"Rate limit exceeded. Please wait and try again.",
			429, err)
		pe.RetryAfter = "60s" // Default fallback
		return pe

	case strings.Contains(errMsg, "404"),
		strings.Contains(errMsg, "model_not_found"),
		strings.Contains(errMsg, "not_found"):
		return ai.NewProviderError(p.Name(), "model_not_found",
			fmt.Sprintf("Model '%s' not found. Please select a valid model.", p.model),
			404, err)

	case strings.Contains(errMsg, "context_length"),
		strings.Contains(errMsg, "max_tokens"):
		return ai.NewProviderError(p.Name(), "context_length_exceeded",
			"Input too long for this model. Try reducing your message length.",
			400, err)

	case strings.Contains(errMsg, "overloaded"),
		strings.Contains(errMsg, "529"):
		return ai.NewProviderError(p.Name(), "provider_overloaded",
			"Anthropic is currently overloaded. Please try again later.",
			529, err)

	case strings.Contains(errMsg, "connection refused"),
		strings.Contains(errMsg, "no such host"),
		strings.Contains(errMsg, "network"):
		return ai.NewProviderError(p.Name(), "provider_unavailable",
			"Cannot connect to Anthropic. Please check your network connection.",
			503, err)

	default:
		return ai.NewProviderError(p.Name(), "unknown_error", errMsg, 500, err)
	}
}

// GetAnthropicChatModels returns a fallback list of Anthropic chat models.
// This is used when no API key is available or when the models API fails.
// Prefer using AnthropicProvider.ListModels() for dynamic fetching.
func GetAnthropicChatModels() []ai.ModelInfo {
	return []ai.ModelInfo{
		{
			ID:          "claude-sonnet-4-5-20250929",
			Name:        "Claude Sonnet 4.5",
			Description: "Latest and most intelligent model with excellent reasoning and coding",
			ContextSize: 200000,
			IsChat:      true,
			CostTier:    ai.CostTierMedium,
			Recommended: true,
		},
		{
			ID:          "claude-opus-4-5-20251101",
			Name:        "Claude Opus 4.5",
			Description: "Most capable model for complex reasoning and analysis",
			ContextSize: 200000,
			IsChat:      true,
			CostTier:    ai.CostTierHigh,
		},
		{
			ID:          "claude-sonnet-4-20250514",
			Name:        "Claude Sonnet 4",
			Description: "Strong reasoning and coding capabilities",
			ContextSize: 200000,
			IsChat:      true,
			CostTier:    ai.CostTierMedium,
		},
		{
			ID:          "claude-haiku-4-5-20251001",
			Name:        "Claude Haiku 4.5",
			Description: "Fast and cost-effective for simple tasks",
			ContextSize: 200000,
			IsChat:      true,
			CostTier:    ai.CostTierLow,
		},
		{
			ID:          "claude-3-5-haiku-20241022",
			Name:        "Claude 3.5 Haiku",
			Description: "Previous generation fast model",
			ContextSize: 200000,
			IsChat:      true,
			CostTier:    ai.CostTierLow,
		},
	}
}
