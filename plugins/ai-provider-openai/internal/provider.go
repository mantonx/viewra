// Package internal implements the OpenAI provider plugin.
package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

const (
	defaultChatModel  = "gpt-4o-mini"
	defaultEmbedModel = "text-embedding-3-small"
)

// OpenAIProvider implements sdk.ProviderPlugin for OpenAI.
type OpenAIProvider struct {
	sdk.Base

	client         *openai.Client
	apiKey         string
	baseURL        string
	embeddingModel string
	chatModel      string
	logger         *slog.Logger
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(logger *slog.Logger) *OpenAIProvider {
	p := &OpenAIProvider{
		embeddingModel: defaultEmbedModel,
		chatModel:      defaultChatModel,
		logger:         logger,
	}
	p.SetLogger(logger)
	return p
}

// SetModels sets the models to use for embeddings and chat.
func (p *OpenAIProvider) SetModels(embeddingModel, chatModel string) {
	if embeddingModel != "" {
		p.embeddingModel = embeddingModel
	}
	if chatModel != "" {
		p.chatModel = chatModel
	}
	p.logger.Debug("configured models", "embedding", p.embeddingModel, "chat", p.chatModel)
}

// ConfigureClient updates the provider configuration.
func (p *OpenAIProvider) ConfigureClient(apiKey, baseURL string) error {
	if apiKey == "" {
		p.client = nil
		p.apiKey = ""
		p.baseURL = ""
		return nil
	}

	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}

	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := openai.NewClient(opts...)
	p.client = &client
	p.apiKey = apiKey
	p.baseURL = baseURL
	p.logger.Debug("configured OpenAI provider", "has_custom_url", baseURL != "")
	return nil
}

// ensureClient ensures the client is configured.
func (p *OpenAIProvider) ensureClient() error {
	if p.client == nil {
		return fmt.Errorf("OpenAI API key not configured")
	}
	return nil
}

// --- sdk.ProviderPlugin implementation ---

func (p *OpenAIProvider) GetProviderCapabilities() sdk.ProviderCapabilities {
	return sdk.ProviderCapabilities{
		ProviderID:            "openai",
		DisplayName:           "OpenAI",
		Description:           "OpenAI API for GPT models and embeddings",
		SupportsChat:          true,
		SupportsEmbedding:     true,
		SupportsStreaming:     true,
		RequiresAPIKey:        true,
		RequiresURL:           false,
		IsLocal:               false,
		DefaultChatModel:      defaultChatModel,
		DefaultEmbeddingModel: defaultEmbedModel,
	}
}

func (p *OpenAIProvider) Initialize(ctx context.Context, dataDir string, config []byte, systemInfo *sdk.SystemInfo) error {
	p.logger.Debug("initializing OpenAI provider plugin", "data_dir", dataDir)
	return nil
}

func (p *OpenAIProvider) Shutdown(ctx context.Context) error {
	p.logger.Debug("shutting down OpenAI provider plugin")
	return nil
}

func (p *OpenAIProvider) HealthCheck(ctx context.Context) (*sdk.ProviderHealth, error) {
	if err := p.ensureClient(); err != nil {
		return &sdk.ProviderHealth{
			Healthy: false,
			Message: "Not configured",
			Error:   err.Error(),
		}, nil
	}

	start := time.Now()
	_, err := p.client.Models.List(ctx)
	latency := time.Since(start)

	if err != nil {
		return &sdk.ProviderHealth{
			Healthy: false,
			Message: "Cannot connect to OpenAI",
			Latency: latency,
			Error:   p.mapError(err).Error(),
		}, nil
	}

	return &sdk.ProviderHealth{
		Healthy: true,
		Message: "Connected to OpenAI",
		Latency: latency,
	}, nil
}

func (p *OpenAIProvider) ListModels(ctx context.Context) ([]sdk.ProviderModel, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	resp, err := p.client.Models.List(ctx)
	if err != nil {
		return nil, p.mapError(err)
	}

	var models []sdk.ProviderModel
	for _, model := range resp.Data {
		isChat := strings.Contains(model.ID, "gpt") || strings.Contains(model.ID, "chat") ||
			strings.Contains(model.ID, "o1") || strings.Contains(model.ID, "o3")
		isEmbedding := strings.Contains(model.ID, "embed")

		if isChat || isEmbedding {
			models = append(models, sdk.ProviderModel{
				ID:          model.ID,
				Name:        model.ID,
				Description: getModelDescription(model.ID),
				IsChat:      isChat,
				IsEmbedding: isEmbedding,
			})
		}
	}

	return models, nil
}

func (p *OpenAIProvider) GenerateEmbedding(ctx context.Context, text, model string) ([]float32, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	if model == "" {
		model = p.embeddingModel
	}

	params := openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(model),
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: []string{text},
		},
	}

	resp, err := p.client.Embeddings.New(ctx, params)
	if err != nil {
		return nil, p.mapError(err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	embedding := make([]float32, len(resp.Data[0].Embedding))
	for i, v := range resp.Data[0].Embedding {
		embedding[i] = float32(v)
	}

	return embedding, nil
}

func (p *OpenAIProvider) GenerateEmbeddingBatch(ctx context.Context, texts []string, model string) ([][]float32, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	if model == "" {
		model = p.embeddingModel
	}

	params := openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(model),
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: texts,
		},
	}

	resp, err := p.client.Embeddings.New(ctx, params)
	if err != nil {
		return nil, p.mapError(err)
	}

	embeddings := make([][]float32, len(resp.Data))
	for i, data := range resp.Data {
		embedding := make([]float32, len(data.Embedding))
		for j, v := range data.Embedding {
			embedding[j] = float32(v)
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}

func (p *OpenAIProvider) Chat(ctx context.Context, req *sdk.ChatRequest) (*sdk.ChatResponse, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	model := req.Model
	if model == "" {
		model = p.chatModel
	}

	messages := make([]openai.ChatCompletionMessageParamUnion, len(req.Messages))
	for i, msg := range req.Messages {
		switch msg.Role {
		case "system":
			messages[i] = openai.SystemMessage(msg.Content)
		case "assistant":
			messages[i] = openai.AssistantMessage(msg.Content)
		default:
			messages[i] = openai.UserMessage(msg.Content)
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(model),
		Messages: messages,
	}

	if req.MaxTokens > 0 {
		params.MaxTokens = openai.Int(int64(req.MaxTokens))
	}
	if req.Temperature > 0 {
		params.Temperature = openai.Float(float64(req.Temperature))
	}

	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, p.mapError(err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no completion choices returned")
	}

	return &sdk.ChatResponse{
		Content:          resp.Choices[0].Message.Content,
		FinishReason:     string(resp.Choices[0].FinishReason),
		PromptTokens:     int(resp.Usage.PromptTokens),
		CompletionTokens: int(resp.Usage.CompletionTokens),
	}, nil
}

func (p *OpenAIProvider) ChatStream(ctx context.Context, req *sdk.ChatRequest, chunks chan<- sdk.ChatChunk) error {
	if err := p.ensureClient(); err != nil {
		return err
	}

	model := req.Model
	if model == "" {
		model = p.chatModel
	}

	messages := make([]openai.ChatCompletionMessageParamUnion, len(req.Messages))
	for i, msg := range req.Messages {
		switch msg.Role {
		case "system":
			messages[i] = openai.SystemMessage(msg.Content)
		case "assistant":
			messages[i] = openai.AssistantMessage(msg.Content)
		default:
			messages[i] = openai.UserMessage(msg.Content)
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(model),
		Messages: messages,
	}

	if req.MaxTokens > 0 {
		params.MaxTokens = openai.Int(int64(req.MaxTokens))
	}
	if req.Temperature > 0 {
		params.Temperature = openai.Float(float64(req.Temperature))
	}

	openaiStream := p.client.Chat.Completions.NewStreaming(ctx, params)

	for openaiStream.Next() {
		chunk := openaiStream.Current()

		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		chunks <- sdk.ChatChunk{
			Content:      choice.Delta.Content,
			Done:         choice.FinishReason != "",
			FinishReason: string(choice.FinishReason),
		}
	}

	if err := openaiStream.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return p.mapError(err)
	}

	return nil
}

// --- sdk.ConfigurableProvider implementation ---

func (p *OpenAIProvider) GetSettingsSchema() ([]byte, error) {
	return SettingsSchema().Build()
}

func (p *OpenAIProvider) Configure(settings []byte) error {
	var cfg struct {
		APIKey         string `json:"api_key"`
		BaseURL        string `json:"base_url"`
		EmbeddingModel string `json:"embedding_model"`
		ChatModel      string `json:"chat_model"`
	}

	if err := json.Unmarshal(settings, &cfg); err != nil {
		return fmt.Errorf("invalid settings JSON: %w", err)
	}

	if err := p.ConfigureClient(cfg.APIKey, cfg.BaseURL); err != nil {
		return err
	}

	p.SetModels(cfg.EmbeddingModel, cfg.ChatModel)
	return nil
}

func (p *OpenAIProvider) IsConfigured() bool {
	return p.apiKey != ""
}

// --- sdk.HTTPProvider implementation ---

func (p *OpenAIProvider) GetRoutes() []sdk.Route {
	return []sdk.Route{
		{
			Path:        "/health",
			Methods:     []string{"GET"},
			AdminOnly:   false,
			Description: "Check OpenAI API connectivity",
		},
	}
}

func (p *OpenAIProvider) HandleHTTP(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
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

func (p *OpenAIProvider) HandleHTTPStream(ctx context.Context, req *sdk.HTTPRequest, stream sdk.HTTPStreamWriter) error {
	return fmt.Errorf("streaming not supported")
}

// --- Helper functions ---

// mapError converts OpenAI errors to descriptive messages.
func (p *OpenAIProvider) mapError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	switch {
	case strings.Contains(errMsg, "401"),
		strings.Contains(errMsg, "invalid_api_key"),
		strings.Contains(errMsg, "Incorrect API key"):
		return fmt.Errorf("invalid API key: please check your API key in settings")

	case strings.Contains(errMsg, "429"),
		strings.Contains(errMsg, "rate_limit"):
		return fmt.Errorf("rate limit exceeded: please wait and try again")

	case strings.Contains(errMsg, "404"),
		strings.Contains(errMsg, "model_not_found"):
		return fmt.Errorf("model not found or not accessible with your API key")

	case strings.Contains(errMsg, "context_length_exceeded"):
		return fmt.Errorf("input too long for this model: try reducing your message length")

	case strings.Contains(errMsg, "insufficient_quota"):
		return fmt.Errorf("insufficient API quota: please check your billing settings")

	case strings.Contains(errMsg, "connection refused"),
		strings.Contains(errMsg, "no such host"),
		strings.Contains(errMsg, "network"):
		return fmt.Errorf("cannot connect to OpenAI: please check your network connection")

	default:
		return err
	}
}

// getModelDescription returns a description for known models.
func getModelDescription(modelID string) string {
	switch {
	case strings.Contains(modelID, "gpt-4o-mini"):
		return "Fast and affordable GPT-4o variant"
	case strings.Contains(modelID, "gpt-4o"):
		return "Most capable GPT-4 model with vision"
	case strings.Contains(modelID, "gpt-4-turbo"):
		return "GPT-4 Turbo with 128k context"
	case strings.Contains(modelID, "gpt-4"):
		return "Most capable GPT-4 model"
	case strings.Contains(modelID, "gpt-3.5-turbo"):
		return "Fast and cost-effective"
	case strings.Contains(modelID, "text-embedding-3-large"):
		return "Best quality embeddings (3072 dimensions)"
	case strings.Contains(modelID, "text-embedding-3-small"):
		return "Efficient embeddings (1536 dimensions)"
	case strings.Contains(modelID, "o1"):
		return "Advanced reasoning model"
	case strings.Contains(modelID, "o3"):
		return "Latest reasoning model"
	default:
		return ""
	}
}
