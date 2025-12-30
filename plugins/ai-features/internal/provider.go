// Package internal implements the AI Local plugin with Ollama provider.
package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ollama/ollama/api"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
)

const (
	defaultBaseURL    = "http://localhost:11434"
	defaultChatModel  = "llama3.2"
	defaultEmbedModel = "nomic-embed-text"
	requestTimeout    = 120 * time.Second

	GB = 1024 * 1024 * 1024
	MB = 1024 * 1024
)

// AIFeaturesPlugin implements the combined AI configuration and Ollama provider plugin.
type AIFeaturesPlugin struct {
	sdk.Base

	client         *api.Client
	baseURL        string
	embeddingModel string
	chatModel      string
	logger         *slog.Logger
	systemInfo     *sdk.SystemInfo
	pluginsClient  *sdk.PluginsClient

	// Master AI configuration
	enabled           bool
	embeddingProvider string
	chatProvider      string
}

// NewAIFeaturesPlugin creates a new AI Local plugin.
func NewAIFeaturesPlugin(logger *slog.Logger) *AIFeaturesPlugin {
	p := &AIFeaturesPlugin{
		baseURL:        defaultBaseURL,
		embeddingModel: defaultEmbedModel,
		chatModel:      defaultChatModel,
		logger:         logger,
	}
	p.SetLogger(logger)
	return p
}

// SetModels sets the models to use for embeddings and chat.
func (p *AIFeaturesPlugin) SetModels(embeddingModel, chatModel string) {
	if embeddingModel != "" {
		p.embeddingModel = embeddingModel
	}
	if chatModel != "" {
		p.chatModel = chatModel
	}
	p.logger.Debug("configured models", "embedding", p.embeddingModel, "chat", p.chatModel)
}

// ConfigureClient updates the provider configuration.
func (p *AIFeaturesPlugin) ConfigureClient(baseURL string) error {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	p.client = api.NewClient(parsedURL, &http.Client{Timeout: requestTimeout})
	p.baseURL = baseURL
	p.logger.Debug("configured Ollama client", "base_url", baseURL)
	return nil
}

// ensureClient ensures the client is configured.
func (p *AIFeaturesPlugin) ensureClient() error {
	if p.client == nil {
		return p.ConfigureClient(p.baseURL)
	}
	return nil
}

// SetPluginsClient sets the PluginsClient for capability broker access.
func (p *AIFeaturesPlugin) SetPluginsClient(client *sdk.PluginsClient) {
	p.pluginsClient = client
}

// --- sdk.ProviderPlugin implementation ---

func (p *AIFeaturesPlugin) GetProviderCapabilities() sdk.ProviderCapabilities {
	return sdk.ProviderCapabilities{
		ProviderID:            "ai-features",
		DisplayName:           "AI Features",
		Description:           "Local AI with Ollama",
		SupportsChat:          true,
		SupportsEmbedding:     true,
		SupportsStreaming:     true,
		RequiresAPIKey:        false,
		RequiresURL:           true,
		IsLocal:               true,
		DefaultChatModel:      p.chatModel,
		DefaultEmbeddingModel: p.embeddingModel,
	}
}

func (p *AIFeaturesPlugin) Initialize(ctx context.Context, dataDir string, config []byte, systemInfo *sdk.SystemInfo) error {
	p.logger.Debug("initializing AI Local plugin", "data_dir", dataDir)
	p.Init(dataDir)

	// Store system info for model recommendations
	p.systemInfo = systemInfo
	if systemInfo != nil {
		p.logger.Debug("received system info",
			"ram_bytes", systemInfo.RAMBytes,
			"vram_bytes", systemInfo.VRAMBytes,
			"has_gpu", systemInfo.HasGPU,
			"cpu_cores", systemInfo.CPUCores)
	}

	// Configure provider with default URL
	if err := p.ConfigureClient(""); err != nil {
		return err
	}

	return nil
}

func (p *AIFeaturesPlugin) Shutdown(ctx context.Context) error {
	p.logger.Debug("shutting down AI Local plugin")
	return nil
}

func (p *AIFeaturesPlugin) HealthCheck(ctx context.Context) (*sdk.ProviderHealth, error) {
	if err := p.ensureClient(); err != nil {
		return &sdk.ProviderHealth{
			Healthy: false,
			Message: "Not configured",
			Error:   err.Error(),
		}, nil
	}

	start := time.Now()
	_, err := p.client.List(ctx)
	latency := time.Since(start)

	if err != nil {
		return &sdk.ProviderHealth{
			Healthy: false,
			Message: "Cannot connect to Ollama",
			Latency: latency,
			Error:   err.Error(),
		}, nil
	}

	return &sdk.ProviderHealth{
		Healthy: true,
		Message: "Connected to Ollama",
		Latency: latency,
	}, nil
}

func (p *AIFeaturesPlugin) ListModels(ctx context.Context) ([]sdk.ProviderModel, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	resp, err := p.client.List(ctx)
	if err != nil {
		return nil, p.mapError(err)
	}

	models := make([]sdk.ProviderModel, len(resp.Models))
	for i, m := range resp.Models {
		isEmbedding := isEmbeddingModel(m.Name)
		models[i] = sdk.ProviderModel{
			ID:          m.Name,
			Name:        m.Name,
			Description: fmt.Sprintf("Size: %s", formatSize(m.Size)),
			IsChat:      !isEmbedding,
			IsEmbedding: isEmbedding,
			Size:        formatSize(m.Size),
		}
	}

	return models, nil
}

func (p *AIFeaturesPlugin) GenerateEmbedding(ctx context.Context, text, model string) ([]float32, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	if model == "" {
		model = p.embeddingModel
	}

	ollamaReq := &api.EmbedRequest{
		Model: model,
		Input: []string{text},
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

	return embedding, nil
}

func (p *AIFeaturesPlugin) GenerateEmbeddingBatch(ctx context.Context, texts []string, model string) ([][]float32, error) {
	if err := p.ensureClient(); err != nil {
		return nil, err
	}

	if model == "" {
		model = p.embeddingModel
	}

	ollamaReq := &api.EmbedRequest{
		Model: model,
		Input: texts,
	}

	resp, err := p.client.Embed(ctx, ollamaReq)
	if err != nil {
		return nil, p.mapError(err)
	}

	embeddings := make([][]float32, len(resp.Embeddings))
	for i, emb := range resp.Embeddings {
		embedding := make([]float32, len(emb))
		for j, v := range emb {
			embedding[j] = float32(v)
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}

func (p *AIFeaturesPlugin) Chat(ctx context.Context, req *sdk.ChatRequest) (*sdk.ChatResponse, error) {
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

	return &sdk.ChatResponse{
		Content:          response.Message.Content,
		FinishReason:     response.DoneReason,
		PromptTokens:     int(response.PromptEvalCount),
		CompletionTokens: int(response.EvalCount),
	}, nil
}

func (p *AIFeaturesPlugin) ChatStream(ctx context.Context, req *sdk.ChatRequest, chunks chan<- sdk.ChatChunk) error {
	if err := p.ensureClient(); err != nil {
		return err
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
		Stream:   boolPtr(true),
		Options: map[string]any{
			"temperature": float64(req.Temperature),
			"num_predict": int(req.MaxTokens),
		},
	}

	return p.client.Chat(ctx, ollamaReq, func(resp api.ChatResponse) error {
		chunks <- sdk.ChatChunk{
			Content:      resp.Message.Content,
			Done:         resp.Done,
			FinishReason: resp.DoneReason,
		}
		return nil
	})
}

// --- sdk.ConfigurableProvider implementation ---

func (p *AIFeaturesPlugin) GetSettingsSchema() ([]byte, error) {
	// Fetch installed models to populate dropdowns
	ctx := context.Background()
	opts := ModelOptions{}

	if models, err := p.ListModels(ctx); err == nil {
		for _, m := range models {
			if m.IsEmbedding {
				opts.EmbeddingModels = append(opts.EmbeddingModels, m.ID)
			}
			if m.IsChat {
				opts.ChatModels = append(opts.ChatModels, m.ID)
			}
		}
		// Auto-select if only one model of each type
		if len(opts.EmbeddingModels) == 1 {
			opts.DefaultEmbedding = opts.EmbeddingModels[0]
		}
		if len(opts.ChatModels) == 1 {
			opts.DefaultChat = opts.ChatModels[0]
		}
	}

	return SettingsSchema(opts).Build()
}

// Settings represents the combined AI Local settings.
type Settings struct {
	// Master AI configuration
	Enabled           bool   `json:"enabled"`
	EmbeddingProvider string `json:"embedding_provider"`
	ChatProvider      string `json:"chat_provider"`

	// Ollama-specific settings
	BaseURL        string `json:"base_url"`
	EmbeddingModel string `json:"embedding_model"`
	ChatModel      string `json:"chat_model"`
}

func (p *AIFeaturesPlugin) Configure(settings []byte) error {
	var cfg Settings
	if err := json.Unmarshal(settings, &cfg); err != nil {
		return fmt.Errorf("invalid settings JSON: %w", err)
	}

	p.logger.Info("configuring AI Local",
		"enabled", cfg.Enabled,
		"embedding_provider", cfg.EmbeddingProvider,
		"chat_provider", cfg.ChatProvider,
		"base_url", cfg.BaseURL)

	// Store master config
	p.enabled = cfg.Enabled
	p.embeddingProvider = cfg.EmbeddingProvider
	p.chatProvider = cfg.ChatProvider

	// Configure Ollama client
	if err := p.ConfigureClient(cfg.BaseURL); err != nil {
		return err
	}
	p.SetModels(cfg.EmbeddingModel, cfg.ChatModel)

	// Set capability preferences via the broker
	if p.pluginsClient != nil {
		ctx := context.Background()

		if cfg.Enabled {
			// Set embedding provider preference
			if cfg.EmbeddingProvider != "" {
				if err := p.pluginsClient.SetCapabilityPreference(ctx, "embedding", cfg.EmbeddingProvider); err != nil {
					p.logger.Error("failed to set embedding preference", "error", err)
				} else {
					p.logger.Info("set embedding provider", "provider", cfg.EmbeddingProvider)
				}
			} else {
				if err := p.pluginsClient.ClearCapabilityPreference(ctx, "embedding"); err != nil {
					p.logger.Error("failed to clear embedding preference", "error", err)
				}
			}

			// Set chat provider preference
			if cfg.ChatProvider != "" {
				if err := p.pluginsClient.SetCapabilityPreference(ctx, "chat", cfg.ChatProvider); err != nil {
					p.logger.Error("failed to set chat preference", "error", err)
				} else {
					p.logger.Info("set chat provider", "provider", cfg.ChatProvider)
				}
			} else {
				if err := p.pluginsClient.ClearCapabilityPreference(ctx, "chat"); err != nil {
					p.logger.Error("failed to clear chat preference", "error", err)
				}
			}
		} else {
			// AI disabled - clear all preferences
			_ = p.pluginsClient.ClearCapabilityPreference(ctx, "embedding")
			_ = p.pluginsClient.ClearCapabilityPreference(ctx, "chat")
			p.logger.Info("AI features disabled, cleared all preferences")
		}
	} else {
		p.logger.Warn("plugins client not available, cannot set capability preferences")
	}

	return nil
}

// --- sdk.HTTPProvider implementation ---

func (p *AIFeaturesPlugin) GetRoutes() []sdk.Route {
	return []sdk.Route{
		{
			Path:        "/models",
			Methods:     []string{"GET"},
			AdminOnly:   false,
			Description: "List all installed models",
		},
		{
			Path:        "/models/recommended",
			Methods:     []string{"GET"},
			AdminOnly:   false,
			Description: "List recommended models based on system resources",
		},
		{
			Path:        "/models/pull",
			Methods:     []string{"POST"},
			AdminOnly:   true,
			Description: "Pull a model from the Ollama registry",
			Streaming:   true,
		},
		{
			Path:        "/models/:model",
			Methods:     []string{"DELETE"},
			AdminOnly:   true,
			Description: "Delete a model from Ollama",
		},
		{
			Path:        "/health",
			Methods:     []string{"GET"},
			AdminOnly:   false,
			Description: "Check Ollama server connectivity",
		},
	}
}

func (p *AIFeaturesPlugin) HandleHTTP(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	p.logger.Debug("HandleHTTP", "path", req.Path, "method", req.Method)

	// GET /models - List all installed models
	if req.Method == "GET" && req.Path == "/models" {
		models, err := p.ListModels(ctx)
		if err != nil {
			return sdk.JSONError(503, err.Error())
		}

		// Transform to schema-action compatible format
		items := make([]map[string]any, len(models))
		for i, m := range models {
			items[i] = map[string]any{
				"id":          m.ID,
				"name":        m.Name,
				"description": m.Description,
				"size":        m.Size,
				"isChat":      m.IsChat,
				"isEmbedding": m.IsEmbedding,
			}
		}

		return sdk.JSONResponse(200, map[string]any{
			"items": items,
		})
	}

	// GET /models/recommended - List recommended models based on system resources
	if req.Method == "GET" && req.Path == "/models/recommended" {
		modelType := req.Query["type"] // "embedding", "chat", or empty for all
		return p.handleRecommendedModels(ctx, modelType)
	}

	// GET /health - Check Ollama server connectivity
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

	// DELETE /models/:model - Delete a model
	if req.Method == "DELETE" && len(req.PathParams) > 0 {
		modelName := req.PathParams["model"]
		if modelName == "" {
			return sdk.JSONError(400, "model name required")
		}

		if err := p.DeleteModel(ctx, modelName); err != nil {
			return sdk.JSONError(500, err.Error())
		}

		return sdk.JSONResponse(200, map[string]any{
			"success": true,
			"message": "Model deleted",
		})
	}

	return sdk.JSONError(404, "not found")
}

func (p *AIFeaturesPlugin) HandleHTTPStream(ctx context.Context, req *sdk.HTTPRequest, stream sdk.HTTPStreamWriter) error {
	p.logger.Debug("HandleHTTPStream", "path", req.Path, "method", req.Method)

	// POST /models/pull - Pull a model with streaming progress
	if req.Method == "POST" && req.Path == "/models/pull" {
		return p.handlePullModelStream(ctx, req, stream)
	}

	// Unknown route
	if err := stream.WriteHeader(404, "application/json", nil); err != nil {
		return err
	}
	return stream.WriteChunk([]byte(`{"error":"not found"}`))
}

// handlePullModelStream handles the streaming model pull request.
func (p *AIFeaturesPlugin) handlePullModelStream(ctx context.Context, req *sdk.HTTPRequest, stream sdk.HTTPStreamWriter) error {
	// Parse request
	var pullReq struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(req.Body, &pullReq); err != nil {
		if writeErr := stream.WriteHeader(200, "text/event-stream", nil); writeErr != nil {
			return writeErr
		}
		errData, _ := json.Marshal(PullProgress{Error: "invalid request body: " + err.Error()})
		return stream.WriteChunk([]byte("data: " + string(errData) + "\n\n"))
	}

	if pullReq.Model == "" {
		if writeErr := stream.WriteHeader(200, "text/event-stream", nil); writeErr != nil {
			return writeErr
		}
		errData, _ := json.Marshal(PullProgress{Error: "model name required"})
		return stream.WriteChunk([]byte("data: " + string(errData) + "\n\n"))
	}

	// Send response headers for SSE
	if err := stream.WriteHeader(200, "text/event-stream", map[string]string{
		"Cache-Control": "no-cache",
		"Connection":    "keep-alive",
	}); err != nil {
		return err
	}

	// Pull model with progress streaming
	pullErr := p.PullModel(ctx, pullReq.Model, func(progress PullProgress) {
		data, _ := json.Marshal(progress)
		_ = stream.WriteChunk([]byte("data: " + string(data) + "\n\n"))
	})

	if pullErr != nil {
		// Send error as SSE event
		errData, _ := json.Marshal(PullProgress{Error: pullErr.Error()})
		_ = stream.WriteChunk([]byte("data: " + string(errData) + "\n\n"))
	} else {
		// Send final success event
		doneData, _ := json.Marshal(PullProgress{Status: "success", Done: true})
		_ = stream.WriteChunk([]byte("data: " + string(doneData) + "\n\n"))
	}

	return nil
}

// handleRecommendedModels returns recommended models based on system resources.
func (p *AIFeaturesPlugin) handleRecommendedModels(ctx context.Context, modelType string) (*sdk.HTTPResponse, error) {
	// Get installed models to check installation status
	installedModels := make(map[string]bool)
	if models, err := p.ListModels(ctx); err == nil {
		for _, m := range models {
			installedModels[m.ID] = true
			// Also match base name without tag suffix
			if idx := strings.Index(m.ID, ":"); idx > 0 {
				installedModels[m.ID[:idx]] = true
			}
		}
	}

	// Get system resources
	var ramBytes, vramBytes uint64
	var hasGPU bool
	hasSystemInfo := p.systemInfo != nil
	if hasSystemInfo {
		ramBytes = p.systemInfo.RAMBytes
		vramBytes = p.systemInfo.VRAMBytes
		hasGPU = p.systemInfo.HasGPU
	}

	// Select models based on type filter
	var specs []ModelSpec
	switch modelType {
	case "embedding":
		specs = embeddingModels
	case "chat":
		specs = chatModels
	default:
		// No filter - combine all models
		specs = append(embeddingModels, chatModels...)
	}

	// Build response items
	items := make([]map[string]any, 0, len(specs))
	for _, spec := range specs {
		canRun := !hasSystemInfo || (ramBytes >= spec.MinRAM && (spec.MinVRAM == 0 || vramBytes >= spec.MinVRAM))
		installed := installedModels[spec.ID]

		// Only filter if we have system info - skip models that can't run and aren't installed
		if hasSystemInfo && !canRun && !installed {
			continue
		}

		items = append(items, map[string]any{
			"id":          spec.ID,
			"name":        spec.Name,
			"size":        formatBytes(spec.SizeBytes),
			"sizeBytes":   spec.SizeBytes,
			"description": spec.Description,
			"isEmbedding": spec.IsEmbedding,
			"isChat":      spec.IsChat,
			"canRun":      canRun,
			"installed":   installed,
			"minRam":      formatBytes(spec.MinRAM),
			"minVram":     formatBytes(spec.MinVRAM),
		})
	}

	return sdk.JSONResponse(200, map[string]any{
		"items": items,
		"systemInfo": map[string]any{
			"ramBytes":      ramBytes,
			"ramFormatted":  formatBytes(ramBytes),
			"vramBytes":     vramBytes,
			"vramFormatted": formatBytes(vramBytes),
			"hasGpu":        hasGPU,
		},
	})
}

// --- Model management ---

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
func (p *AIFeaturesPlugin) PullModel(ctx context.Context, modelName string, progressFn func(PullProgress)) error {
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
func (p *AIFeaturesPlugin) DeleteModel(ctx context.Context, modelName string) error {
	if err := p.ensureClient(); err != nil {
		return err
	}

	p.logger.Info("deleting model", "model", modelName)

	req := &api.DeleteRequest{
		Model: modelName,
	}

	return p.client.Delete(ctx, req)
}

// --- Helper functions ---

// mapError converts Ollama errors to descriptive messages.
func (p *AIFeaturesPlugin) mapError(err error) error {
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

// formatBytes formats bytes as human-readable string.
func formatBytes(bytes uint64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
		tb = gb * 1024
	)

	switch {
	case bytes >= tb:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(tb))
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%d MB", bytes/mb)
	case bytes >= kb:
		return fmt.Sprintf("%d KB", bytes/kb)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func boolPtr(b bool) *bool {
	return &b
}

// --- Model specifications ---

// ModelSpec defines the requirements and metadata for a model.
type ModelSpec struct {
	ID          string
	Name        string
	SizeBytes   uint64
	Description string
	MinRAM      uint64 // Minimum RAM in bytes
	MinVRAM     uint64 // Minimum VRAM in bytes (0 = CPU-only is fine)
	IsEmbedding bool
	IsChat      bool
}

// embeddingModels defines available embedding models ordered by quality.
var embeddingModels = []ModelSpec{
	{
		ID:          "nomic-embed-text",
		Name:        "Nomic Embed",
		SizeBytes:   274 * MB,
		Description: "Best balance of quality and speed",
		MinRAM:      2 * GB,
		MinVRAM:     0,
		IsEmbedding: true,
	},
	{
		ID:          "mxbai-embed-large",
		Name:        "MixedBread Large",
		SizeBytes:   670 * MB,
		Description: "Higher quality embeddings, needs more resources",
		MinRAM:      4 * GB,
		MinVRAM:     4 * GB,
		IsEmbedding: true,
	},
	{
		ID:          "bge-base-en-v1.5",
		Name:        "BGE Base",
		SizeBytes:   134 * MB,
		Description: "Good quality, English-optimized",
		MinRAM:      2 * GB,
		MinVRAM:     0,
		IsEmbedding: true,
	},
	{
		ID:          "all-minilm",
		Name:        "MiniLM",
		SizeBytes:   46 * MB,
		Description: "Smallest and fastest, good for limited resources",
		MinRAM:      1 * GB,
		MinVRAM:     0,
		IsEmbedding: true,
	},
}

// chatModels defines available chat models ordered by quality.
var chatModels = []ModelSpec{
	{
		ID:          "llama3.1:8b",
		Name:        "Llama 3.1 8B",
		SizeBytes:   4_700 * MB,
		Description: "Best quality for most systems",
		MinRAM:      8 * GB,
		MinVRAM:     6 * GB,
		IsChat:      true,
	},
	{
		ID:          "gemma2:2b",
		Name:        "Gemma 2 2B",
		SizeBytes:   1_600 * MB,
		Description: "Fast and lightweight, good for basic tasks",
		MinRAM:      4 * GB,
		MinVRAM:     0,
		IsChat:      true,
	},
	{
		ID:          "phi3:mini",
		Name:        "Phi-3 Mini",
		SizeBytes:   2_300 * MB,
		Description: "Microsoft's compact model, good reasoning",
		MinRAM:      4 * GB,
		MinVRAM:     0,
		IsChat:      true,
	},
	{
		ID:          "qwen2:1.5b",
		Name:        "Qwen2 1.5B",
		SizeBytes:   935 * MB,
		Description: "Smallest option, basic capabilities",
		MinRAM:      2 * GB,
		MinVRAM:     0,
		IsChat:      true,
	},
}
