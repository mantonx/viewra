// Package internal implements the Ollama provider plugin.
package internal

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ollama/ollama/api"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

const (
	defaultBaseURL    = "http://localhost:11434"
	defaultChatModel  = "llama3.2"
	defaultEmbedModel = "nomic-embed-text"
	requestTimeout    = 120 * time.Second
)

// OllamaProvider implements the PluginProvider service for Ollama.
type OllamaProvider struct {
	pluginv1.UnimplementedPluginProviderServer

	client         *api.Client
	baseURL        string
	embeddingModel string
	chatModel      string
	logger         *slog.Logger
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(logger *slog.Logger) *OllamaProvider {
	return &OllamaProvider{
		baseURL:        defaultBaseURL,
		embeddingModel: defaultEmbedModel,
		chatModel:      defaultChatModel,
		logger:         logger,
	}
}

// SetModels sets the models to use for embeddings and chat.
func (p *OllamaProvider) SetModels(embeddingModel, chatModel string) {
	if embeddingModel != "" {
		p.embeddingModel = embeddingModel
	}
	if chatModel != "" {
		p.chatModel = chatModel
	}
	p.logger.Info("configured models", "embedding", p.embeddingModel, "chat", p.chatModel)
}

// Configure updates the provider configuration.
func (p *OllamaProvider) Configure(baseURL string) error {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	p.client = api.NewClient(parsedURL, &http.Client{Timeout: requestTimeout})
	p.baseURL = baseURL
	p.logger.Info("configured Ollama provider", "base_url", baseURL)
	return nil
}

// ensureClient ensures the client is configured.
func (p *OllamaProvider) ensureClient() error {
	if p.client == nil {
		return p.Configure(p.baseURL)
	}
	return nil
}

// GetCapabilities returns the provider's capabilities.
func (p *OllamaProvider) GetCapabilities(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.ProviderCapabilities, error) {
	return &pluginv1.ProviderCapabilities{
		ProviderId:            "ollama",
		DisplayName:           "Ollama (Local)",
		Description:           "Local AI inference using Ollama",
		SupportsChat:          true,
		SupportsEmbeddings:    true,
		SupportsStreaming:     true,
		RequiresApiKey:        false,
		RequiresUrl:           true,
		IsLocal:               true,
		DefaultChatModel:      defaultChatModel,
		DefaultEmbeddingModel: defaultEmbedModel,
	}, nil
}

// ListModels returns available models from Ollama.
func (p *OllamaProvider) ListModels(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.ProviderModelList, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	resp, err := p.client.List(ctx)
	if err != nil {
		return nil, p.mapError(err)
	}

	models := make([]*pluginv1.ProviderModel, len(resp.Models))
	for i, m := range resp.Models {
		isEmbedding := isEmbeddingModel(m.Name)
		models[i] = &pluginv1.ProviderModel{
			Id:          m.Name,
			Name:        m.Name,
			Description: fmt.Sprintf("Size: %s", formatSize(m.Size)),
			IsChat:      !isEmbedding,
			IsEmbedding: isEmbedding,
			Size:        formatSize(m.Size),
		}
	}

	return &pluginv1.ProviderModelList{Models: models}, nil
}

// GenerateEmbedding generates an embedding for a single text.
func (p *OllamaProvider) GenerateEmbedding(ctx context.Context, req *pluginv1.ProviderEmbeddingRequest) (*pluginv1.EmbeddingResponse, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	model := req.Model
	if model == "" {
		model = p.embeddingModel
	}

	ollamaReq := &api.EmbedRequest{
		Model: model,
		Input: []string{req.Text},
	}

	resp, err := p.client.Embed(ctx, ollamaReq)
	if err != nil {
		return nil, p.mapError(err)
	}

	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	// Convert []float64 to []float32
	embedding := make([]float32, len(resp.Embeddings[0]))
	for i, v := range resp.Embeddings[0] {
		embedding[i] = float32(v)
	}

	return &pluginv1.EmbeddingResponse{
		Embedding:  embedding,
		Dimensions: int32(len(embedding)),
		TokensUsed: 0, // Ollama doesn't report token usage for embeddings
	}, nil
}

// GenerateEmbeddingBatch generates embeddings for multiple texts.
func (p *OllamaProvider) GenerateEmbeddingBatch(ctx context.Context, req *pluginv1.ProviderEmbeddingBatchRequest) (*pluginv1.EmbeddingBatchResponse, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	model := req.Model
	if model == "" {
		model = p.embeddingModel
	}

	ollamaReq := &api.EmbedRequest{
		Model: model,
		Input: req.Texts,
	}

	resp, err := p.client.Embed(ctx, ollamaReq)
	if err != nil {
		return nil, p.mapError(err)
	}

	results := make([]*pluginv1.EmbeddingResult, len(resp.Embeddings))
	for i, emb := range resp.Embeddings {
		embedding := make([]float32, len(emb))
		for j, v := range emb {
			embedding[j] = float32(v)
		}
		results[i] = &pluginv1.EmbeddingResult{
			Embedding:  embedding,
			Dimensions: int32(len(embedding)),
		}
	}

	return &pluginv1.EmbeddingBatchResponse{
		Embeddings:  results,
		TotalTokens: 0,
	}, nil
}

// Chat sends a chat completion request.
func (p *OllamaProvider) Chat(ctx context.Context, req *pluginv1.ProviderChatRequest) (*pluginv1.ChatResponse, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	model := req.Model
	if model == "" {
		model = p.chatModel
	}

	messages := make([]api.Message, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = api.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	ollamaReq := &api.ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   boolPtr(false),
		Options: map[string]any{
			"temperature": float64(req.Temperature),
			"num_predict": int(req.MaxTokens),
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

	return &pluginv1.ChatResponse{
		Content:          response.Message.Content,
		FinishReason:     response.DoneReason,
		PromptTokens:     int32(response.PromptEvalCount),
		CompletionTokens: int32(response.EvalCount),
	}, nil
}

// ChatStream sends a streaming chat completion request.
func (p *OllamaProvider) ChatStream(req *pluginv1.ProviderChatRequest, stream pluginv1.PluginProvider_ChatStreamServer) error {
	if err := p.ensureClient(); err != nil {
		return err
	}

	ctx := stream.Context()

	model := req.Model
	if model == "" {
		model = p.chatModel
	}

	messages := make([]api.Message, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = api.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	ollamaReq := &api.ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   boolPtr(true),
		Options: map[string]any{
			"temperature": float64(req.Temperature),
			"num_predict": int(req.MaxTokens),
		},
	}

	return p.client.Chat(ctx, ollamaReq, func(resp api.ChatResponse) error {
		return stream.Send(&pluginv1.ChatStreamChunk{
			Content:      resp.Message.Content,
			Done:         resp.Done,
			FinishReason: resp.DoneReason,
		})
	})
}

// HealthCheck verifies Ollama is accessible.
func (p *OllamaProvider) HealthCheck(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.ProviderHealthStatus, error) {
	if err := p.ensureClient(); err != nil {
		return &pluginv1.ProviderHealthStatus{
			Healthy: false,
			Message: "Not configured",
			Error:   err.Error(),
		}, nil
	}

	start := time.Now()
	_, err := p.client.List(ctx)
	latency := time.Since(start)

	if err != nil {
		return &pluginv1.ProviderHealthStatus{
			Healthy:   false,
			Message:   "Cannot connect to Ollama",
			LatencyMs: latency.Milliseconds(),
			Error:     err.Error(),
		}, nil
	}

	return &pluginv1.ProviderHealthStatus{
		Healthy:   true,
		Message:   "Connected to Ollama",
		LatencyMs: latency.Milliseconds(),
	}, nil
}

// mapError converts Ollama errors to descriptive messages.
func (p *OllamaProvider) mapError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	switch {
	case strings.Contains(errMsg, "connection refused"),
		strings.Contains(errMsg, "no such host"),
		strings.Contains(errMsg, "network is unreachable"):
		return fmt.Errorf("cannot connect to Ollama at %s: is Ollama running?", p.baseURL)

	case strings.Contains(errMsg, "model") && strings.Contains(errMsg, "not found"):
		return fmt.Errorf("model not found: try pulling it with 'ollama pull <model>'")

	case strings.Contains(errMsg, "context deadline exceeded"),
		strings.Contains(errMsg, "timeout"):
		return fmt.Errorf("request timed out: the model may be loading")

	default:
		return err
	}
}

// isEmbeddingModel checks if a model name indicates an embedding model.
func isEmbeddingModel(name string) bool {
	prefixes := []string{"nomic-embed", "all-minilm", "mxbai-embed", "bge-", "e5-", "embed"}
	nameLower := strings.ToLower(name)
	for _, prefix := range prefixes {
		if strings.Contains(nameLower, prefix) {
			return true
		}
	}
	return false
}

// formatSize formats a byte size into a human-readable string.
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func boolPtr(b bool) *bool {
	return &b
}

// PullProgress represents the progress of a model pull operation.
type PullProgress struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
	Done      bool   `json:"done,omitempty"`
	Error     string `json:"error,omitempty"`
}

// PullModel pulls a model from Ollama with progress reporting.
func (p *OllamaProvider) PullModel(ctx context.Context, modelName string, progressFn func(PullProgress)) error {
	if err := p.ensureClient(); err != nil {
		return err
	}

	p.logger.Info("pulling model", "model", modelName)

	req := &api.PullRequest{
		Model: modelName,
	}

	return p.client.Pull(ctx, req, func(resp api.ProgressResponse) error {
		progress := PullProgress{
			Status:    resp.Status,
			Digest:    resp.Digest,
			Total:     resp.Total,
			Completed: resp.Completed,
		}

		// Check if pull is complete
		if resp.Status == "success" || (resp.Total > 0 && resp.Completed == resp.Total && resp.Digest == "") {
			progress.Done = true
		}

		if progressFn != nil {
			progressFn(progress)
		}

		return nil
	})
}

// DeleteModel deletes a model from Ollama.
func (p *OllamaProvider) DeleteModel(ctx context.Context, modelName string) error {
	if err := p.ensureClient(); err != nil {
		return err
	}

	p.logger.Info("deleting model", "model", modelName)

	req := &api.DeleteRequest{
		Model: modelName,
	}

	return p.client.Delete(ctx, req)
}
