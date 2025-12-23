// Package internal implements the Voyage AI provider plugin.
package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

const (
	voyageBaseURL     = "https://api.voyageai.com/v1"
	voyageTimeout     = 120 * time.Second
	defaultEmbedModel = "voyage-3-lite"
)

// VoyageProvider implements the PluginProvider service for Voyage AI.
type VoyageProvider struct {
	pluginv1.UnimplementedPluginProviderServer

	apiKey         string
	embeddingModel string
	httpClient     *http.Client
	logger         *slog.Logger
}

// NewVoyageProvider creates a new Voyage AI provider.
func NewVoyageProvider(logger *slog.Logger) *VoyageProvider {
	return &VoyageProvider{
		embeddingModel: defaultEmbedModel,
		httpClient:     &http.Client{Timeout: voyageTimeout},
		logger:         logger,
	}
}

// SetEmbeddingModel sets the model to use for embeddings.
func (p *VoyageProvider) SetEmbeddingModel(embeddingModel string) {
	if embeddingModel != "" {
		p.embeddingModel = embeddingModel
	}
	p.logger.Info("configured embedding model", "model", p.embeddingModel)
}

// Configure updates the provider configuration.
func (p *VoyageProvider) Configure(apiKey string) error {
	p.apiKey = apiKey
	if apiKey != "" {
		p.logger.Info("configured Voyage AI provider")
	}
	return nil
}

// ensureClient ensures the API key is configured.
func (p *VoyageProvider) ensureClient() error {
	if p.apiKey == "" {
		return fmt.Errorf("Voyage AI API key not configured")
	}
	return nil
}

// GetCapabilities returns the provider's capabilities.
func (p *VoyageProvider) GetCapabilities(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.ProviderCapabilities, error) {
	return &pluginv1.ProviderCapabilities{
		ProviderId:            "voyage",
		DisplayName:           "Voyage AI",
		Description:           "Voyage AI for high-quality embeddings - Anthropic's recommended embedding provider",
		SupportsChat:          false,
		SupportsEmbeddings:    true,
		SupportsStreaming:     false,
		RequiresApiKey:        true,
		RequiresUrl:           false,
		IsLocal:               false,
		DefaultChatModel:      "",
		DefaultEmbeddingModel: defaultEmbedModel,
	}, nil
}

// ListModels returns available models from Voyage AI.
// Note: Voyage doesn't have a models API, so we return a static list.
func (p *VoyageProvider) ListModels(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.ProviderModelList, error) {
	models := []*pluginv1.ProviderModel{
		{
			Id:          "voyage-3",
			Name:        "Voyage 3",
			Description: "Latest flagship model, optimized for general-purpose retrieval and RAG",
			IsChat:      false,
			IsEmbedding: true,
		},
		{
			Id:          "voyage-3-lite",
			Name:        "Voyage 3 Lite",
			Description: "Lightweight, cost-effective model for latency-sensitive applications",
			IsChat:      false,
			IsEmbedding: true,
		},
		{
			Id:          "voyage-code-3",
			Name:        "Voyage Code 3",
			Description: "Optimized for code retrieval, supports 80+ programming languages",
			IsChat:      false,
			IsEmbedding: true,
		},
		{
			Id:          "voyage-finance-2",
			Name:        "Voyage Finance 2",
			Description: "Optimized for financial domain retrieval",
			IsChat:      false,
			IsEmbedding: true,
		},
		{
			Id:          "voyage-law-2",
			Name:        "Voyage Law 2",
			Description: "Optimized for legal document retrieval",
			IsChat:      false,
			IsEmbedding: true,
		},
		{
			Id:          "voyage-multilingual-2",
			Name:        "Voyage Multilingual 2",
			Description: "Optimized for multilingual retrieval across 100+ languages",
			IsChat:      false,
			IsEmbedding: true,
		},
	}

	return &pluginv1.ProviderModelList{Models: models}, nil
}

// GenerateEmbedding generates an embedding for a single text.
func (p *VoyageProvider) GenerateEmbedding(ctx context.Context, req *pluginv1.ProviderEmbeddingRequest) (*pluginv1.EmbeddingResponse, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	model := req.Model
	if model == "" {
		model = p.embeddingModel
	}

	voyageReq := voyageEmbedRequest{
		Model:     model,
		Input:     []string{req.Text},
		InputType: "document",
	}

	resp, err := p.doEmbedRequest(ctx, voyageReq)
	if err != nil {
		return nil, err
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
		TokensUsed: int32(resp.Usage.TotalTokens),
	}, nil
}

// GenerateEmbeddingBatch generates embeddings for multiple texts.
func (p *VoyageProvider) GenerateEmbeddingBatch(ctx context.Context, req *pluginv1.ProviderEmbeddingBatchRequest) (*pluginv1.EmbeddingBatchResponse, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	model := req.Model
	if model == "" {
		model = p.embeddingModel
	}

	voyageReq := voyageEmbedRequest{
		Model:     model,
		Input:     req.Texts,
		InputType: "document",
	}

	resp, err := p.doEmbedRequest(ctx, voyageReq)
	if err != nil {
		return nil, err
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

// Chat is not supported by Voyage AI.
func (p *VoyageProvider) Chat(ctx context.Context, req *pluginv1.ProviderChatRequest) (*pluginv1.ChatResponse, error) {
	return nil, fmt.Errorf("Voyage AI does not support chat - use embeddings only")
}

// ChatStream is not supported by Voyage AI.
func (p *VoyageProvider) ChatStream(req *pluginv1.ProviderChatRequest, stream pluginv1.PluginProvider_ChatStreamServer) error {
	return fmt.Errorf("Voyage AI does not support chat - use embeddings only")
}

// HealthCheck verifies Voyage AI is accessible.
func (p *VoyageProvider) HealthCheck(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.ProviderHealthStatus, error) {
	if err := p.ensureClient(); err != nil {
		return &pluginv1.ProviderHealthStatus{
			Healthy: false,
			Message: "Not configured",
			Error:   err.Error(),
		}, nil
	}

	start := time.Now()

	// Voyage doesn't have a models endpoint, so we do a minimal embedding request
	voyageReq := voyageEmbedRequest{
		Model:     defaultEmbedModel,
		Input:     []string{"health check"},
		InputType: "document",
	}

	_, err := p.doEmbedRequest(ctx, voyageReq)
	latency := time.Since(start)

	if err != nil {
		return &pluginv1.ProviderHealthStatus{
			Healthy:   false,
			Message:   "Cannot connect to Voyage AI",
			LatencyMs: latency.Milliseconds(),
			Error:     err.Error(),
		}, nil
	}

	return &pluginv1.ProviderHealthStatus{
		Healthy:   true,
		Message:   "Connected to Voyage AI",
		LatencyMs: latency.Milliseconds(),
	}, nil
}

// doEmbedRequest performs an embedding request to Voyage AI.
func (p *VoyageProvider) doEmbedRequest(ctx context.Context, req voyageEmbedRequest) (*voyageEmbedResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, voyageBaseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.handleErrorResponse(resp)
	}

	var voyageResp voyageEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&voyageResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &voyageResp, nil
}

// handleErrorResponse converts HTTP errors to descriptive messages.
func (p *VoyageProvider) handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("invalid API key: please check your Voyage AI API key in settings")
	case http.StatusTooManyRequests:
		return fmt.Errorf("rate limit exceeded: please wait and try again")
	case http.StatusBadRequest:
		if strings.Contains(string(body), "model") {
			return fmt.Errorf("model not found: %s", string(body))
		}
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	default:
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}
}

// Voyage AI API types

type voyageEmbedRequest struct {
	Model     string   `json:"model"`
	Input     []string `json:"input"`
	InputType string   `json:"input_type,omitempty"` // "document" or "query"
}

type voyageEmbedResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}
