// Package internal implements the OpenAI provider plugin.
package internal

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

const (
	defaultChatModel  = "gpt-4o-mini"
	defaultEmbedModel = "text-embedding-3-small"
)

// OpenAIProvider implements the PluginProvider service for OpenAI.
type OpenAIProvider struct {
	pluginv1.UnimplementedPluginProviderServer

	client         *openai.Client
	apiKey         string
	baseURL        string
	embeddingModel string
	chatModel      string
	logger         *slog.Logger
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(logger *slog.Logger) *OpenAIProvider {
	return &OpenAIProvider{
		embeddingModel: defaultEmbedModel,
		chatModel:      defaultChatModel,
		logger:         logger,
	}
}

// SetModels sets the models to use for embeddings and chat.
func (p *OpenAIProvider) SetModels(embeddingModel, chatModel string) {
	if embeddingModel != "" {
		p.embeddingModel = embeddingModel
	}
	if chatModel != "" {
		p.chatModel = chatModel
	}
	p.logger.Info("configured models", "embedding", p.embeddingModel, "chat", p.chatModel)
}

// Configure updates the provider configuration.
func (p *OpenAIProvider) Configure(apiKey, baseURL string) error {
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
	p.logger.Info("configured OpenAI provider", "has_custom_url", baseURL != "")
	return nil
}

// ensureClient ensures the client is configured.
func (p *OpenAIProvider) ensureClient() error {
	if p.client == nil {
		return fmt.Errorf("OpenAI API key not configured")
	}
	return nil
}

// GetCapabilities returns the provider's capabilities.
func (p *OpenAIProvider) GetCapabilities(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.ProviderCapabilities, error) {
	return &pluginv1.ProviderCapabilities{
		ProviderId:            "openai",
		DisplayName:           "OpenAI",
		Description:           "OpenAI API for GPT models and embeddings",
		SupportsChat:          true,
		SupportsEmbeddings:    true,
		SupportsStreaming:     true,
		RequiresApiKey:        true,
		RequiresUrl:           false,
		IsLocal:               false,
		DefaultChatModel:      defaultChatModel,
		DefaultEmbeddingModel: defaultEmbedModel,
	}, nil
}

// ListModels returns available models from OpenAI.
func (p *OpenAIProvider) ListModels(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.ProviderModelList, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	resp, err := p.client.Models.List(ctx)
	if err != nil {
		return nil, p.mapError(err)
	}

	var models []*pluginv1.ProviderModel
	for _, model := range resp.Data {
		isChat := strings.Contains(model.ID, "gpt") || strings.Contains(model.ID, "chat") ||
			strings.Contains(model.ID, "o1") || strings.Contains(model.ID, "o3")
		isEmbedding := strings.Contains(model.ID, "embed")

		if isChat || isEmbedding {
			models = append(models, &pluginv1.ProviderModel{
				Id:          model.ID,
				Name:        model.ID,
				Description: getModelDescription(model.ID),
				IsChat:      isChat,
				IsEmbedding: isEmbedding,
			})
		}
	}

	return &pluginv1.ProviderModelList{Models: models}, nil
}

// GenerateEmbedding generates an embedding for a single text.
func (p *OpenAIProvider) GenerateEmbedding(ctx context.Context, req *pluginv1.ProviderEmbeddingRequest) (*pluginv1.EmbeddingResponse, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	model := req.Model
	if model == "" {
		model = p.embeddingModel
	}

	params := openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(model),
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: []string{req.Text},
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

	return &pluginv1.EmbeddingResponse{
		Embedding:  embedding,
		Dimensions: int32(len(embedding)),
		TokensUsed: int32(resp.Usage.PromptTokens),
	}, nil
}

// GenerateEmbeddingBatch generates embeddings for multiple texts.
func (p *OpenAIProvider) GenerateEmbeddingBatch(ctx context.Context, req *pluginv1.ProviderEmbeddingBatchRequest) (*pluginv1.EmbeddingBatchResponse, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	model := req.Model
	if model == "" {
		model = p.embeddingModel
	}

	params := openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(model),
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: req.Texts,
		},
	}

	resp, err := p.client.Embeddings.New(ctx, params)
	if err != nil {
		return nil, p.mapError(err)
	}

	results := make([]*pluginv1.EmbeddingResult, len(resp.Data))
	for i, data := range resp.Data {
		embedding := make([]float32, len(data.Embedding))
		for j, v := range data.Embedding {
			embedding[j] = float32(v)
		}
		results[i] = &pluginv1.EmbeddingResult{
			Embedding:  embedding,
			Dimensions: int32(len(embedding)),
		}
	}

	return &pluginv1.EmbeddingBatchResponse{
		Embeddings:  results,
		TotalTokens: int32(resp.Usage.TotalTokens),
	}, nil
}

// Chat sends a chat completion request.
func (p *OpenAIProvider) Chat(ctx context.Context, req *pluginv1.ProviderChatRequest) (*pluginv1.ChatResponse, error) {
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

	return &pluginv1.ChatResponse{
		Content:          resp.Choices[0].Message.Content,
		FinishReason:     string(resp.Choices[0].FinishReason),
		PromptTokens:     int32(resp.Usage.PromptTokens),
		CompletionTokens: int32(resp.Usage.CompletionTokens),
	}, nil
}

// ChatStream sends a streaming chat completion request.
func (p *OpenAIProvider) ChatStream(req *pluginv1.ProviderChatRequest, stream pluginv1.PluginProvider_ChatStreamServer) error {
	if err := p.ensureClient(); err != nil {
		return err
	}

	ctx := stream.Context()

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
		if err := stream.Send(&pluginv1.ChatStreamChunk{
			Content:      choice.Delta.Content,
			Done:         choice.FinishReason != "",
			FinishReason: string(choice.FinishReason),
		}); err != nil {
			return err
		}
	}

	if err := openaiStream.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return p.mapError(err)
	}

	return nil
}

// HealthCheck verifies OpenAI is accessible.
func (p *OpenAIProvider) HealthCheck(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.ProviderHealthStatus, error) {
	if err := p.ensureClient(); err != nil {
		return &pluginv1.ProviderHealthStatus{
			Healthy: false,
			Message: "Not configured",
			Error:   err.Error(),
		}, nil
	}

	start := time.Now()
	_, err := p.client.Models.List(ctx)
	latency := time.Since(start)

	if err != nil {
		return &pluginv1.ProviderHealthStatus{
			Healthy:   false,
			Message:   "Cannot connect to OpenAI",
			LatencyMs: latency.Milliseconds(),
			Error:     p.mapError(err).Error(),
		}, nil
	}

	return &pluginv1.ProviderHealthStatus{
		Healthy:   true,
		Message:   "Connected to OpenAI",
		LatencyMs: latency.Milliseconds(),
	}, nil
}

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
