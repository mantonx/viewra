// Package internal implements the Anthropic provider plugin.
package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

const (
	defaultChatModel = "claude-sonnet-4-5-20250929"
)

// AnthropicProvider implements the PluginProvider service for Anthropic.
type AnthropicProvider struct {
	pluginv1.UnimplementedPluginProviderServer

	client    *anthropic.Client
	apiKey    string
	chatModel string
	logger    *slog.Logger
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(logger *slog.Logger) *AnthropicProvider {
	return &AnthropicProvider{
		chatModel: defaultChatModel,
		logger:    logger,
	}
}

// SetChatModel sets the model to use for chat.
func (p *AnthropicProvider) SetChatModel(chatModel string) {
	if chatModel != "" {
		p.chatModel = chatModel
	}
	p.logger.Info("configured chat model", "model", p.chatModel)
}

// Configure updates the provider configuration.
func (p *AnthropicProvider) Configure(apiKey string) error {
	if apiKey == "" {
		p.client = nil
		p.apiKey = ""
		return nil
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	p.client = &client
	p.apiKey = apiKey
	p.logger.Info("configured Anthropic provider")
	return nil
}

// ensureClient ensures the client is configured.
func (p *AnthropicProvider) ensureClient() error {
	if p.client == nil {
		return fmt.Errorf("Anthropic API key not configured")
	}
	return nil
}

// GetCapabilities returns the provider's capabilities.
func (p *AnthropicProvider) GetCapabilities(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.ProviderCapabilities, error) {
	return &pluginv1.ProviderCapabilities{
		ProviderId:            "anthropic",
		DisplayName:           "Anthropic",
		Description:           "Anthropic API for Claude models",
		SupportsChat:          true,
		SupportsEmbeddings:    false, // Anthropic doesn't have embeddings
		SupportsStreaming:     true,
		RequiresApiKey:        true,
		RequiresUrl:           false,
		IsLocal:               false,
		DefaultChatModel:      defaultChatModel,
		DefaultEmbeddingModel: "", // No embeddings support
	}, nil
}

// ListModels returns available models from Anthropic.
func (p *AnthropicProvider) ListModels(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.ProviderModelList, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	page, err := p.client.Models.List(ctx, anthropic.ModelListParams{})
	if err != nil {
		return nil, p.mapError(err)
	}

	var models []*pluginv1.ProviderModel
	for _, model := range page.Data {
		models = append(models, &pluginv1.ProviderModel{
			Id:          model.ID,
			Name:        formatModelName(model.ID),
			Description: model.DisplayName,
			IsChat:      true,
			IsEmbedding: false,
		})
	}

	return &pluginv1.ProviderModelList{Models: models}, nil
}

// GenerateEmbedding is not supported by Anthropic.
func (p *AnthropicProvider) GenerateEmbedding(ctx context.Context, req *pluginv1.ProviderEmbeddingRequest) (*pluginv1.EmbeddingResponse, error) {
	return nil, fmt.Errorf("Anthropic does not support embeddings")
}

// GenerateEmbeddingBatch is not supported by Anthropic.
func (p *AnthropicProvider) GenerateEmbeddingBatch(ctx context.Context, req *pluginv1.ProviderEmbeddingBatchRequest) (*pluginv1.EmbeddingBatchResponse, error) {
	return nil, fmt.Errorf("Anthropic does not support embeddings")
}

// Chat sends a chat completion request.
func (p *AnthropicProvider) Chat(ctx context.Context, req *pluginv1.ProviderChatRequest) (*pluginv1.ChatResponse, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	model := req.Model
	if model == "" {
		model = p.chatModel
	}

	params := p.buildMessageParams(model, req)

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

	return &pluginv1.ChatResponse{
		Content:          content,
		FinishReason:     string(resp.StopReason),
		PromptTokens:     int32(resp.Usage.InputTokens),
		CompletionTokens: int32(resp.Usage.OutputTokens),
	}, nil
}

// ChatStream sends a streaming chat completion request.
func (p *AnthropicProvider) ChatStream(req *pluginv1.ProviderChatRequest, stream pluginv1.PluginProvider_ChatStreamServer) error {
	if err := p.ensureClient(); err != nil {
		return err
	}

	ctx := stream.Context()

	model := req.Model
	if model == "" {
		model = p.chatModel
	}

	params := p.buildMessageParams(model, req)

	anthropicStream := p.client.Messages.NewStreaming(ctx, params)

	for anthropicStream.Next() {
		event := anthropicStream.Current()

		switch variant := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			switch delta := variant.Delta.AsAny().(type) {
			case anthropic.TextDelta:
				if err := stream.Send(&pluginv1.ChatStreamChunk{
					Content: delta.Text,
					Done:    false,
				}); err != nil {
					return err
				}
			}
		case anthropic.MessageStopEvent:
			if err := stream.Send(&pluginv1.ChatStreamChunk{
				Done: true,
			}); err != nil {
				return err
			}
		}
	}

	if err := anthropicStream.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return p.mapError(err)
	}

	return nil
}

// HealthCheck verifies Anthropic is accessible.
func (p *AnthropicProvider) HealthCheck(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.ProviderHealthStatus, error) {
	if err := p.ensureClient(); err != nil {
		return &pluginv1.ProviderHealthStatus{
			Healthy: false,
			Message: "Not configured",
			Error:   err.Error(),
		}, nil
	}

	start := time.Now()
	_, err := p.client.Models.List(ctx, anthropic.ModelListParams{})
	latency := time.Since(start)

	if err != nil {
		return &pluginv1.ProviderHealthStatus{
			Healthy:   false,
			Message:   "Cannot connect to Anthropic",
			LatencyMs: latency.Milliseconds(),
			Error:     p.mapError(err).Error(),
		}, nil
	}

	return &pluginv1.ProviderHealthStatus{
		Healthy:   true,
		Message:   "Connected to Anthropic",
		LatencyMs: latency.Milliseconds(),
	}, nil
}

// buildMessageParams converts the request to Anthropic SDK params.
func (p *AnthropicProvider) buildMessageParams(model string, req *pluginv1.ProviderChatRequest) anthropic.MessageNewParams {
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
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
		case "system":
			// Anthropic handles system messages as a separate parameter
			params.System = []anthropic.TextBlockParam{
				{Text: msg.Content},
			}
		case "assistant":
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)))
		default:
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		}
	}
	params.Messages = messages

	if req.Temperature > 0 {
		params.Temperature = anthropic.Float(float64(req.Temperature))
	}

	return params
}

// mapError converts Anthropic errors to descriptive messages.
func (p *AnthropicProvider) mapError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	switch {
	case strings.Contains(errMsg, "401"),
		strings.Contains(errMsg, "authentication_error"),
		strings.Contains(errMsg, "invalid x-api-key"):
		return fmt.Errorf("invalid API key: please check your Anthropic API key in settings")

	case strings.Contains(errMsg, "429"),
		strings.Contains(errMsg, "rate_limit"):
		return fmt.Errorf("rate limit exceeded: please wait and try again")

	case strings.Contains(errMsg, "404"),
		strings.Contains(errMsg, "model_not_found"),
		strings.Contains(errMsg, "not_found"):
		return fmt.Errorf("model not found: please select a valid model")

	case strings.Contains(errMsg, "context_length"),
		strings.Contains(errMsg, "max_tokens"):
		return fmt.Errorf("input too long for this model: try reducing your message length")

	case strings.Contains(errMsg, "overloaded"),
		strings.Contains(errMsg, "529"):
		return fmt.Errorf("Anthropic is currently overloaded: please try again later")

	case strings.Contains(errMsg, "connection refused"),
		strings.Contains(errMsg, "no such host"),
		strings.Contains(errMsg, "network"):
		return fmt.Errorf("cannot connect to Anthropic: please check your network connection")

	default:
		return err
	}
}

// formatModelName converts model ID to a display name.
func formatModelName(modelID string) string {
	// Convert "claude-sonnet-4-5-20250929" to "Claude Sonnet 4.5"
	name := strings.ReplaceAll(modelID, "-", " ")

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
		// Capitalize first letter
		if len(part) > 0 {
			part = strings.ToUpper(part[:1]) + part[1:]
		}
		cleanParts = append(cleanParts, part)
	}

	return strings.Join(cleanParts, " ")
}
