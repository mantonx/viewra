// Package internal implements the Anthropic provider plugin.
package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

const (
	defaultChatModel = "claude-sonnet-4-5-20250929"
)

// AnthropicProvider implements sdk.ProviderPlugin for Anthropic.
type AnthropicProvider struct {
	sdk.Base

	client    *anthropic.Client
	apiKey    string
	chatModel string
	logger    *slog.Logger
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(logger *slog.Logger) *AnthropicProvider {
	p := &AnthropicProvider{
		chatModel: defaultChatModel,
		logger:    logger,
	}
	p.SetLogger(logger)
	return p
}

// SetChatModel sets the model to use for chat.
func (p *AnthropicProvider) SetChatModel(chatModel string) {
	if chatModel != "" {
		p.chatModel = chatModel
	}
	p.logger.Debug("configured chat model", "model", p.chatModel)
}

// ConfigureClient updates the provider configuration.
func (p *AnthropicProvider) ConfigureClient(apiKey string) error {
	if apiKey == "" {
		p.client = nil
		p.apiKey = ""
		return nil
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	p.client = &client
	p.apiKey = apiKey
	p.logger.Debug("configured Anthropic provider")
	return nil
}

// ensureClient ensures the client is configured.
func (p *AnthropicProvider) ensureClient() error {
	if p.client == nil {
		return fmt.Errorf("Anthropic API key not configured")
	}
	return nil
}

// --- sdk.ProviderPlugin implementation ---

func (p *AnthropicProvider) GetProviderCapabilities() sdk.ProviderCapabilities {
	return sdk.ProviderCapabilities{
		ProviderID:            "anthropic",
		DisplayName:           "Anthropic",
		Description:           "Anthropic API for Claude models",
		SupportsChat:          true,
		SupportsEmbedding:     false, // Anthropic doesn't have embeddings
		SupportsStreaming:     true,
		RequiresAPIKey:        true,
		RequiresURL:           false,
		IsLocal:               false,
		DefaultChatModel:      defaultChatModel,
		DefaultEmbeddingModel: "", // No embeddings support
	}
}

func (p *AnthropicProvider) Initialize(ctx context.Context, dataDir string, config []byte, systemInfo *sdk.SystemInfo) error {
	p.logger.Debug("initializing Anthropic provider plugin", "data_dir", dataDir)
	return nil
}

func (p *AnthropicProvider) Shutdown(ctx context.Context) error {
	p.logger.Debug("shutting down Anthropic provider plugin")
	return nil
}

func (p *AnthropicProvider) HealthCheck(ctx context.Context) (*sdk.ProviderHealth, error) {
	if err := p.ensureClient(); err != nil {
		return &sdk.ProviderHealth{
			Healthy: false,
			Message: "Not configured",
			Error:   err.Error(),
		}, nil
	}

	start := time.Now()
	_, err := p.client.Models.List(ctx, anthropic.ModelListParams{})
	latency := time.Since(start)

	if err != nil {
		return &sdk.ProviderHealth{
			Healthy: false,
			Message: "Cannot connect to Anthropic",
			Latency: latency,
			Error:   p.mapError(err).Error(),
		}, nil
	}

	return &sdk.ProviderHealth{
		Healthy: true,
		Message: "Connected to Anthropic",
		Latency: latency,
	}, nil
}

func (p *AnthropicProvider) ListModels(ctx context.Context) ([]sdk.ProviderModel, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	page, err := p.client.Models.List(ctx, anthropic.ModelListParams{})
	if err != nil {
		return nil, p.mapError(err)
	}

	var models []sdk.ProviderModel
	for _, model := range page.Data {
		models = append(models, sdk.ProviderModel{
			ID:          model.ID,
			Name:        formatModelName(model.ID),
			Description: model.DisplayName,
			IsChat:      true,
			IsEmbedding: false,
		})
	}

	return models, nil
}

func (p *AnthropicProvider) GenerateEmbedding(ctx context.Context, text, model string) ([]float32, error) {
	return nil, fmt.Errorf("Anthropic does not support embeddings")
}

func (p *AnthropicProvider) GenerateEmbeddingBatch(ctx context.Context, texts []string, model string) ([][]float32, error) {
	return nil, fmt.Errorf("Anthropic does not support embeddings")
}

func (p *AnthropicProvider) Chat(ctx context.Context, req *sdk.ChatRequest) (*sdk.ChatResponse, error) {
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

	return &sdk.ChatResponse{
		Content:          content,
		FinishReason:     string(resp.StopReason),
		PromptTokens:     int(resp.Usage.InputTokens),
		CompletionTokens: int(resp.Usage.OutputTokens),
	}, nil
}

func (p *AnthropicProvider) ChatStream(ctx context.Context, req *sdk.ChatRequest, chunks chan<- sdk.ChatChunk) error {
	if err := p.ensureClient(); err != nil {
		return err
	}

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
				chunks <- sdk.ChatChunk{
					Content: delta.Text,
					Done:    false,
				}
			}
		case anthropic.MessageStopEvent:
			chunks <- sdk.ChatChunk{
				Done: true,
			}
		}
	}

	if err := anthropicStream.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return p.mapError(err)
	}

	return nil
}

// --- sdk.ConfigurableProvider implementation ---

func (p *AnthropicProvider) GetSettingsSchema() ([]byte, error) {
	return SettingsSchema().Build()
}

func (p *AnthropicProvider) Configure(settings []byte) error {
	var cfg struct {
		APIKey    string `json:"api_key"`
		ChatModel string `json:"chat_model"`
	}

	if err := json.Unmarshal(settings, &cfg); err != nil {
		return fmt.Errorf("invalid settings JSON: %w", err)
	}

	if err := p.ConfigureClient(cfg.APIKey); err != nil {
		return err
	}

	p.SetChatModel(cfg.ChatModel)
	return nil
}

// --- sdk.HTTPProvider implementation ---

func (p *AnthropicProvider) GetRoutes() []sdk.Route {
	return []sdk.Route{
		{
			Path:        "/health",
			Methods:     []string{"GET"},
			AdminOnly:   false,
			Description: "Check Anthropic API connectivity",
		},
	}
}

func (p *AnthropicProvider) HandleHTTP(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	if req.Method == "GET" && req.Path == "/health" {
		health, err := p.HealthCheck(ctx)
		if err != nil {
			return sdk.JSONError(503, err.Error())
		}

		if !health.Healthy {
			return sdk.JSONResponse(503, map[string]any{
				"success": false,
				"error":   health.Error,
				"message": health.Message,
			})
		}

		return sdk.JSONResponse(200, map[string]any{
			"success": true,
			"message": health.Message,
		})
	}

	return sdk.JSONError(404, "not found")
}

func (p *AnthropicProvider) HandleHTTPStream(ctx context.Context, req *sdk.HTTPRequest, stream sdk.HTTPStreamWriter) error {
	return fmt.Errorf("streaming not supported")
}

// --- Helper functions ---

// buildMessageParams converts the SDK request to Anthropic SDK params.
func (p *AnthropicProvider) buildMessageParams(model string, req *sdk.ChatRequest) anthropic.MessageNewParams {
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
