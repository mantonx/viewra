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

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

const (
	voyageBaseURL     = "https://api.voyageai.com/v1"
	voyageTimeout     = 120 * time.Second
	defaultEmbedModel = "voyage-3-lite"
)

// VoyageProvider implements sdk.ProviderPlugin for Voyage AI.
type VoyageProvider struct {
	sdk.Base

	apiKey         string
	embeddingModel string
	httpClient     *http.Client
	logger         *slog.Logger
}

// NewVoyageProvider creates a new Voyage AI provider.
func NewVoyageProvider(logger *slog.Logger) *VoyageProvider {
	p := &VoyageProvider{
		embeddingModel: defaultEmbedModel,
		httpClient:     &http.Client{Timeout: voyageTimeout},
		logger:         logger,
	}
	p.SetLogger(logger)
	return p
}

// SetEmbeddingModel sets the model to use for embeddings.
func (p *VoyageProvider) SetEmbeddingModel(embeddingModel string) {
	if embeddingModel != "" {
		p.embeddingModel = embeddingModel
	}
	p.logger.Info("configured embedding model", "model", p.embeddingModel)
}

// ConfigureClient updates the provider configuration.
func (p *VoyageProvider) ConfigureClient(apiKey string) error {
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

// --- sdk.ProviderPlugin implementation ---

func (p *VoyageProvider) GetProviderCapabilities() sdk.ProviderCapabilities {
	return sdk.ProviderCapabilities{
		ProviderID:            "voyage",
		DisplayName:           "Voyage AI",
		Description:           "Voyage AI for high-quality embeddings - Anthropic's recommended embedding provider",
		SupportsChat:          false,
		SupportsEmbedding:     true,
		SupportsStreaming:     false,
		RequiresAPIKey:        true,
		RequiresURL:           false,
		IsLocal:               false,
		DefaultChatModel:      "",
		DefaultEmbeddingModel: defaultEmbedModel,
	}
}

func (p *VoyageProvider) Initialize(ctx context.Context, dataDir string, config []byte, systemInfo *sdk.SystemInfo) error {
	p.logger.Info("initializing Voyage AI provider plugin", "data_dir", dataDir)
	return nil
}

func (p *VoyageProvider) Shutdown(ctx context.Context) error {
	p.logger.Info("shutting down Voyage AI provider plugin")
	return nil
}

func (p *VoyageProvider) HealthCheck(ctx context.Context) (*sdk.ProviderHealth, error) {
	if err := p.ensureClient(); err != nil {
		return &sdk.ProviderHealth{
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
		return &sdk.ProviderHealth{
			Healthy: false,
			Message: "Cannot connect to Voyage AI",
			Latency: latency,
			Error:   err.Error(),
		}, nil
	}

	return &sdk.ProviderHealth{
		Healthy: true,
		Message: "Connected to Voyage AI",
		Latency: latency,
	}, nil
}

func (p *VoyageProvider) ListModels(ctx context.Context) ([]sdk.ProviderModel, error) {
	// Voyage doesn't have a models API, so we return a static list
	return []sdk.ProviderModel{
		{
			ID:          "voyage-3",
			Name:        "Voyage 3",
			Description: "Latest flagship model, optimized for general-purpose retrieval and RAG",
			IsChat:      false,
			IsEmbedding: true,
		},
		{
			ID:          "voyage-3-lite",
			Name:        "Voyage 3 Lite",
			Description: "Lightweight, cost-effective model for latency-sensitive applications",
			IsChat:      false,
			IsEmbedding: true,
		},
		{
			ID:          "voyage-code-3",
			Name:        "Voyage Code 3",
			Description: "Optimized for code retrieval, supports 80+ programming languages",
			IsChat:      false,
			IsEmbedding: true,
		},
		{
			ID:          "voyage-finance-2",
			Name:        "Voyage Finance 2",
			Description: "Optimized for financial domain retrieval",
			IsChat:      false,
			IsEmbedding: true,
		},
		{
			ID:          "voyage-law-2",
			Name:        "Voyage Law 2",
			Description: "Optimized for legal document retrieval",
			IsChat:      false,
			IsEmbedding: true,
		},
		{
			ID:          "voyage-multilingual-2",
			Name:        "Voyage Multilingual 2",
			Description: "Optimized for multilingual retrieval across 100+ languages",
			IsChat:      false,
			IsEmbedding: true,
		},
	}, nil
}

func (p *VoyageProvider) GenerateEmbedding(ctx context.Context, text, model string) ([]float32, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	if model == "" {
		model = p.embeddingModel
	}

	voyageReq := voyageEmbedRequest{
		Model:     model,
		Input:     []string{text},
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

	return embedding, nil
}

func (p *VoyageProvider) GenerateEmbeddingBatch(ctx context.Context, texts []string, model string) ([][]float32, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	if model == "" {
		model = p.embeddingModel
	}

	voyageReq := voyageEmbedRequest{
		Model:     model,
		Input:     texts,
		InputType: "document",
	}

	resp, err := p.doEmbedRequest(ctx, voyageReq)
	if err != nil {
		return nil, err
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

func (p *VoyageProvider) Chat(ctx context.Context, req *sdk.ChatRequest) (*sdk.ChatResponse, error) {
	return nil, fmt.Errorf("Voyage AI does not support chat - use embeddings only")
}

func (p *VoyageProvider) ChatStream(ctx context.Context, req *sdk.ChatRequest, chunks chan<- sdk.ChatChunk) error {
	return fmt.Errorf("Voyage AI does not support chat - use embeddings only")
}

// --- sdk.ConfigurableProvider implementation ---

func (p *VoyageProvider) GetSettingsSchema() ([]byte, error) {
	return SettingsSchema().Build()
}

func (p *VoyageProvider) Configure(settings []byte) error {
	var cfg struct {
		APIKey         string `json:"api_key"`
		EmbeddingModel string `json:"embedding_model"`
	}

	if err := json.Unmarshal(settings, &cfg); err != nil {
		return fmt.Errorf("invalid settings JSON: %w", err)
	}

	if err := p.ConfigureClient(cfg.APIKey); err != nil {
		return err
	}

	p.SetEmbeddingModel(cfg.EmbeddingModel)
	return nil
}

// --- sdk.HTTPProvider implementation ---

func (p *VoyageProvider) GetRoutes() []sdk.Route {
	return []sdk.Route{
		{
			Path:        "/health",
			Methods:     []string{"GET"},
			AdminOnly:   false,
			Description: "Check Voyage AI API connectivity",
		},
	}
}

func (p *VoyageProvider) HandleHTTP(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
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

func (p *VoyageProvider) HandleHTTPStream(ctx context.Context, req *sdk.HTTPRequest, stream sdk.HTTPStreamWriter) error {
	return fmt.Errorf("streaming not supported")
}

// --- Helper functions ---

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
