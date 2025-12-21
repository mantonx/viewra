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
	voyageBaseURL = "https://api.voyageai.com/v1"
	voyageTimeout = 120 * time.Second
)

// VoyageProvider implements EmbeddingProvider for Voyage AI.
// Voyage AI is recommended by Anthropic for embeddings and specializes in
// high-quality embedding models optimized for retrieval.
type VoyageProvider struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewVoyageProvider creates a new Voyage AI embedding provider.
func NewVoyageProvider(apiKey, model string) *VoyageProvider {
	if model == "" {
		model = "voyage-3-lite" // Default to cost-effective model
	}
	return &VoyageProvider{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: voyageTimeout,
		},
	}
}

// Name returns the provider name.
func (p *VoyageProvider) Name() string {
	return "Voyage AI"
}

// Model returns the current model name.
func (p *VoyageProvider) Model() string {
	return p.model
}

// Embed generates embeddings using Voyage AI.
func (p *VoyageProvider) Embed(ctx context.Context, req ai.EmbeddingRequest) (*ai.EmbeddingResponse, error) {
	voyageReq := voyageEmbedRequest{
		Model:     p.model,
		Input:     req.Texts,
		InputType: "document", // Use "query" for search queries
	}

	body, err := json.Marshal(voyageReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, voyageBaseURL+"/embeddings", bytes.NewReader(body))
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

	var voyageResp voyageEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&voyageResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	embeddings := make([][]float32, len(voyageResp.Data))
	for i, data := range voyageResp.Data {
		embedding := make([]float32, len(data.Embedding))
		for j, v := range data.Embedding {
			embedding[j] = float32(v)
		}
		embeddings[i] = embedding
	}

	return &ai.EmbeddingResponse{
		Embeddings: embeddings,
		Usage: ai.TokenUsage{
			TotalTokens: voyageResp.Usage.TotalTokens,
		},
	}, nil
}

// EmbedQuery generates embeddings optimized for search queries.
// This uses input_type="query" which improves retrieval performance.
func (p *VoyageProvider) EmbedQuery(ctx context.Context, texts []string) (*ai.EmbeddingResponse, error) {
	voyageReq := voyageEmbedRequest{
		Model:     p.model,
		Input:     texts,
		InputType: "query",
	}

	body, err := json.Marshal(voyageReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, voyageBaseURL+"/embeddings", bytes.NewReader(body))
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

	var voyageResp voyageEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&voyageResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	embeddings := make([][]float32, len(voyageResp.Data))
	for i, data := range voyageResp.Data {
		embedding := make([]float32, len(data.Embedding))
		for j, v := range data.Embedding {
			embedding[j] = float32(v)
		}
		embeddings[i] = embedding
	}

	return &ai.EmbeddingResponse{
		Embeddings: embeddings,
		Usage: ai.TokenUsage{
			TotalTokens: voyageResp.Usage.TotalTokens,
		},
	}, nil
}

// Dimensions returns the embedding dimensions for the current model.
func (p *VoyageProvider) Dimensions() int {
	switch p.model {
	case "voyage-3":
		return 1024
	case "voyage-3-lite":
		return 512
	case "voyage-code-3":
		return 1024
	case "voyage-finance-2":
		return 1024
	case "voyage-law-2":
		return 1024
	case "voyage-multilingual-2":
		return 1024
	case "voyage-large-2-instruct":
		return 1024
	case "voyage-large-2":
		return 1536
	case "voyage-2":
		return 1024
	default:
		// Default for unknown models
		return 1024
	}
}

// HealthCheck verifies the API is accessible.
func (p *VoyageProvider) HealthCheck(ctx context.Context) error {
	// Voyage doesn't have a models endpoint, so we do a minimal embedding request
	voyageReq := voyageEmbedRequest{
		Model:     p.model,
		Input:     []string{"health check"},
		InputType: "document",
	}

	body, err := json.Marshal(voyageReq)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, voyageBaseURL+"/embeddings", bytes.NewReader(body))
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
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: status %d: %s", ai.ErrProviderUnavailable, resp.StatusCode, string(respBody))
	}

	return nil
}

func (p *VoyageProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
}

func (p *VoyageProvider) handleErrorResponse(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ai.ErrInvalidAPIKey
	case http.StatusTooManyRequests:
		return ai.ErrRateLimitExceeded
	case http.StatusBadRequest:
		// Parse error message for model not found
		if strings.Contains(string(body), "model") {
			return fmt.Errorf("%w: %s", ai.ErrModelNotFound, string(body))
		}
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	default:
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}
}

// GetAvailableModels returns the list of available Voyage embedding models.
// Note: Voyage doesn't have a models API, so we return a static list.
func GetVoyageEmbeddingModels() []ai.ModelInfo {
	return []ai.ModelInfo{
		{
			ID:          "voyage-3",
			Name:        "Voyage 3",
			Description: "Latest flagship model, optimized for general-purpose retrieval and RAG",
			Dimensions:  1024,
			IsEmbedding: true,
			CostTier:    ai.CostTierMedium,
			Recommended: true,
		},
		{
			ID:          "voyage-3-lite",
			Name:        "Voyage 3 Lite",
			Description: "Lightweight, cost-effective model for latency-sensitive applications",
			Dimensions:  512,
			IsEmbedding: true,
			CostTier:    ai.CostTierLow,
		},
		{
			ID:          "voyage-code-3",
			Name:        "Voyage Code 3",
			Description: "Optimized for code retrieval, supports 80+ programming languages",
			Dimensions:  1024,
			IsEmbedding: true,
			CostTier:    ai.CostTierMedium,
		},
		{
			ID:          "voyage-finance-2",
			Name:        "Voyage Finance 2",
			Description: "Optimized for financial domain retrieval",
			Dimensions:  1024,
			IsEmbedding: true,
			CostTier:    ai.CostTierMedium,
		},
		{
			ID:          "voyage-law-2",
			Name:        "Voyage Law 2",
			Description: "Optimized for legal document retrieval",
			Dimensions:  1024,
			IsEmbedding: true,
			CostTier:    ai.CostTierMedium,
		},
		{
			ID:          "voyage-multilingual-2",
			Name:        "Voyage Multilingual 2",
			Description: "Optimized for multilingual retrieval across 100+ languages",
			Dimensions:  1024,
			IsEmbedding: true,
			CostTier:    ai.CostTierMedium,
		},
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
