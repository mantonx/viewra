package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

const (
	GB = 1024 * 1024 * 1024
	MB = 1024 * 1024
)

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

// Plugin implements the PluginCore service.
type Plugin struct {
	pluginv1.UnimplementedPluginCoreServer

	provider   *OllamaProvider
	logger     *slog.Logger
	systemInfo *pluginv1.SystemInfo
}

// NewPlugin creates a new plugin instance.
func NewPlugin(logger *slog.Logger) *Plugin {
	return &Plugin{
		provider: NewOllamaProvider(logger),
		logger:   logger,
	}
}

// Provider returns the Ollama provider implementation.
func (p *Plugin) Provider() *OllamaProvider {
	return p.provider
}

// Initialize is called when the plugin is loaded.
func (p *Plugin) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
	p.logger.Info("initializing Ollama provider plugin",
		"host_version", req.HostVersion,
		"data_dir", req.DataDir)

	// Store system info for model recommendations
	p.systemInfo = req.SystemInfo
	if p.systemInfo != nil {
		p.logger.Info("received system info",
			"ram_bytes", p.systemInfo.RamBytes,
			"vram_bytes", p.systemInfo.VramBytes,
			"has_gpu", p.systemInfo.HasGpu,
			"cpu_cores", p.systemInfo.CpuCores)
	}

	// Configure provider with default URL
	// TODO: Read from config when settings schema is implemented
	if err := p.provider.Configure(""); err != nil {
		return &pluginv1.InitResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pluginv1.InitResponse{Success: true}, nil
}

// Shutdown is called when the plugin is unloaded.
func (p *Plugin) Shutdown(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.Empty, error) {
	p.logger.Info("shutting down Ollama provider plugin")
	return &pluginv1.Empty{}, nil
}

// HealthCheck returns the plugin's health status.
func (p *Plugin) HealthCheck(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.HealthStatus, error) {
	// Check if we can connect to Ollama
	status, err := p.provider.HealthCheck(ctx, &pluginv1.Empty{})
	if err != nil {
		return &pluginv1.HealthStatus{
			Status:  pluginv1.HealthStatus_UNHEALTHY,
			Message: err.Error(),
		}, nil
	}

	if !status.Healthy {
		return &pluginv1.HealthStatus{
			Status:  pluginv1.HealthStatus_UNHEALTHY,
			Message: status.Error,
		}, nil
	}

	return &pluginv1.HealthStatus{
		Status:  pluginv1.HealthStatus_HEALTHY,
		Message: status.Message,
	}, nil
}

// GetRoutes returns HTTP routes for Ollama model management.
func (p *Plugin) GetRoutes(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.PluginRoutes, error) {
	return &pluginv1.PluginRoutes{
		Routes: []*pluginv1.PluginRoute{
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
		},
	}, nil
}

// GetSettingsSchema returns the JSON Schema for plugin settings.
// The schema is generated dynamically to include installed models as enum options.
func (p *Plugin) GetSettingsSchema(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.SettingsSchema, error) {
	// Fetch installed models to populate dropdowns
	var embeddingModels, chatModels []string
	var defaultEmbedding, defaultChat string

	if models, err := p.provider.ListModels(ctx, &pluginv1.Empty{}); err == nil {
		for _, m := range models.Models {
			if m.IsEmbedding {
				embeddingModels = append(embeddingModels, m.Id)
			}
			if m.IsChat {
				chatModels = append(chatModels, m.Id)
			}
		}
		// Auto-select if only one model of each type
		if len(embeddingModels) == 1 {
			defaultEmbedding = embeddingModels[0]
		}
		if len(chatModels) == 1 {
			defaultChat = chatModels[0]
		}
	}

	// Build embedding model property
	embeddingProp := map[string]any{
		"type":        "string",
		"title":       "Embedding Model",
		"description": "Model to use for generating embeddings",
	}
	if len(embeddingModels) > 0 {
		embeddingProp["enum"] = embeddingModels
		if defaultEmbedding != "" {
			embeddingProp["default"] = defaultEmbedding
		}
	} else {
		embeddingProp["description"] = "No embedding models installed. Go to the Models tab to pull one."
	}

	// Build chat model property
	chatProp := map[string]any{
		"type":        "string",
		"title":       "Chat Model",
		"description": "Model to use for chat completions",
	}
	if len(chatModels) > 0 {
		chatProp["enum"] = chatModels
		if defaultChat != "" {
			chatProp["default"] = defaultChat
		}
	} else {
		chatProp["description"] = "No chat models installed. Go to the Models tab to pull one."
	}

	schema := map[string]any{
		"type":  "object",
		"title": "Ollama Settings",
		"x-viewra-meta": map[string]any{
			"displayName": "Ollama",
			"description": "Local AI inference using Ollama",
			"tip":         "Runs on your hardware. Larger models need more RAM/VRAM.",
			"isLocal":     true,
			"icon":        "hard-drive",
		},
		"properties": map[string]any{
			"base_url": map[string]any{
				"type":        "string",
				"title":       "Server URL",
				"description": "URL of the Ollama server",
				"default":     "http://localhost:11434",
			},
			"embedding_model": embeddingProp,
			"chat_model":      chatProp,
		},
		"x-viewra-actions": []any{
			map[string]any{
				"id":       "recommended-models",
				"type":     "list",
				"title":    "Recommended Models",
				"tabTitle": "Models",
				"source": map[string]any{
					"endpoint": "/models/recommended",
				},
				"showSystemInfo": true,
				"display": map[string]any{
					"primaryField":   "name",
					"secondaryField": "description",
					"badges": []any{
						map[string]any{"field": "isEmbedding", "value": true, "label": "Embedding", "color": "blue"},
						map[string]any{"field": "isChat", "value": true, "label": "Chat", "color": "green"},
						map[string]any{"field": "installed", "value": true, "label": "Installed", "color": "emerald"},
						map[string]any{"field": "canRun", "value": false, "label": "Insufficient Resources", "color": "red"},
					},
					"metadata": []string{"size", "minRam"},
				},
				"itemActions": []any{
					map[string]any{
						"id":        "pull",
						"type":      "action",
						"label":     "Pull",
						"endpoint":  "/models/pull",
						"streaming": true,
						"showWhen":  map[string]any{"field": "installed", "value": false},
					},
					map[string]any{
						"id":       "delete",
						"type":     "delete",
						"label":    "Delete",
						"endpoint": "/models/:id",
						"showWhen": map[string]any{"field": "installed", "value": true},
						"confirm": map[string]any{
							"title":   "Delete Model",
							"message": "Are you sure you want to delete this model?",
						},
					},
				},
			},
			map[string]any{
				"id":       "test-connection",
				"type":     "test",
				"title":    "Test Connection",
				"endpoint": "/health",
			},
		},
	}

	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal schema: %w", err)
	}

	return &pluginv1.SettingsSchema{
		JsonSchema: schemaBytes,
	}, nil
}

// Configure applies new settings to the plugin.
func (p *Plugin) Configure(ctx context.Context, req *pluginv1.Settings) (*pluginv1.ConfigureResponse, error) {
	p.logger.Debug("configure called", "settings_length", len(req.Json))

	var settings struct {
		BaseURL        string `json:"base_url"`
		EmbeddingModel string `json:"embedding_model"`
		ChatModel      string `json:"chat_model"`
	}

	if err := json.Unmarshal(req.Json, &settings); err != nil {
		return &pluginv1.ConfigureResponse{
			Success: false,
			Error:   "invalid settings JSON: " + err.Error(),
		}, nil
	}

	if err := p.provider.Configure(settings.BaseURL); err != nil {
		return &pluginv1.ConfigureResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Store model selections
	p.provider.SetModels(settings.EmbeddingModel, settings.ChatModel)

	return &pluginv1.ConfigureResponse{Success: true}, nil
}

// HandleHTTP handles non-streaming HTTP requests.
func (p *Plugin) HandleHTTP(ctx context.Context, req *pluginv1.PluginHTTPRequest) (*pluginv1.PluginHTTPResponse, error) {
	p.logger.Debug("HandleHTTP", "path", req.Path, "method", req.Method)

	// GET /models - List all installed models
	if req.Method == "GET" && req.Path == "/models" {
		models, err := p.provider.ListModels(ctx, &pluginv1.Empty{})
		if err != nil {
			return jsonError(503, err.Error())
		}

		// Transform to schema-action compatible format
		items := make([]map[string]any, len(models.Models))
		for i, m := range models.Models {
			items[i] = map[string]any{
				"id":          m.Id,
				"name":        m.Name,
				"description": m.Description,
				"size":        m.Size,
				"isChat":      m.IsChat,
				"isEmbedding": m.IsEmbedding,
			}
		}

		return jsonResponse(200, map[string]any{
			"items": items,
		})
	}

	// GET /models/recommended - List recommended models based on system resources
	if req.Method == "GET" && req.Path == "/models/recommended" {
		return p.handleRecommendedModels(ctx)
	}

	// GET /health - Check Ollama server connectivity
	if req.Method == "GET" && req.Path == "/health" {
		status, err := p.provider.HealthCheck(ctx, &pluginv1.Empty{})
		if err != nil {
			return jsonResponse(503, map[string]any{
				"success": false,
				"error":   err.Error(),
			})
		}

		if !status.Healthy {
			return jsonResponse(503, map[string]any{
				"success": false,
				"error":   status.Error,
				"message": status.Message,
			})
		}

		return jsonResponse(200, map[string]any{
			"success": true,
			"message": status.Message,
		})
	}

	// DELETE /models/:model - Delete a model
	if req.Method == "DELETE" && len(req.PathParams) > 0 {
		modelName := req.PathParams["model"]
		if modelName == "" {
			return jsonError(400, "model name required")
		}

		if err := p.provider.DeleteModel(ctx, modelName); err != nil {
			return jsonError(500, err.Error())
		}

		return jsonResponse(200, map[string]any{
			"success": true,
			"message": "Model deleted",
		})
	}

	return jsonError(404, "not found")
}

// HandleHTTPStream handles streaming HTTP requests (model pull with progress).
func (p *Plugin) HandleHTTPStream(stream pluginv1.PluginCore_HandleHTTPStreamServer) error {
	// First, receive the request
	chunk, err := stream.Recv()
	if err != nil {
		return err
	}

	if chunk.Type != pluginv1.PluginHTTPChunk_REQUEST_START {
		return stream.Send(&pluginv1.PluginHTTPChunk{
			Type:       pluginv1.PluginHTTPChunk_RESPONSE_START,
			StatusCode: 400,
			Headers:    map[string]string{"Content-Type": "application/json"},
		})
	}

	req := chunk.Request
	p.logger.Debug("HandleHTTPStream", "path", req.Path, "method", req.Method)

	// POST /models/pull - Pull a model with streaming progress
	if req.Method == "POST" && req.Path == "/models/pull" {
		return p.handlePullModel(stream, req)
	}

	// Unknown route
	if err := stream.Send(&pluginv1.PluginHTTPChunk{
		Type:        pluginv1.PluginHTTPChunk_RESPONSE_START,
		StatusCode:  404,
		ContentType: "application/json",
	}); err != nil {
		return err
	}

	body, _ := json.Marshal(map[string]string{"error": "not found"})
	if err := stream.Send(&pluginv1.PluginHTTPChunk{
		Type: pluginv1.PluginHTTPChunk_RESPONSE_BODY,
		Data: body,
	}); err != nil {
		return err
	}

	return stream.Send(&pluginv1.PluginHTTPChunk{
		Type: pluginv1.PluginHTTPChunk_RESPONSE_END,
	})
}

// handlePullModel handles the streaming model pull request.
func (p *Plugin) handlePullModel(stream pluginv1.PluginCore_HandleHTTPStreamServer, req *pluginv1.PluginHTTPRequest) error {
	// Read request body
	var body []byte
	for {
		chunk, err := stream.Recv()
		if err != nil {
			return err
		}
		if chunk.Type == pluginv1.PluginHTTPChunk_REQUEST_BODY {
			body = append(body, chunk.Data...)
		} else if chunk.Type == pluginv1.PluginHTTPChunk_REQUEST_END {
			break
		}
	}

	// Parse request
	var pullReq struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &pullReq); err != nil {
		return sendSSEError(stream, "invalid request body: "+err.Error())
	}

	if pullReq.Model == "" {
		return sendSSEError(stream, "model name required")
	}

	// Send response headers for SSE
	if err := stream.Send(&pluginv1.PluginHTTPChunk{
		Type:        pluginv1.PluginHTTPChunk_RESPONSE_START,
		StatusCode:  200,
		ContentType: "text/event-stream",
		Headers: map[string]string{
			"Cache-Control": "no-cache",
			"Connection":    "keep-alive",
		},
	}); err != nil {
		return err
	}

	// Pull model with progress streaming
	ctx := stream.Context()
	pullErr := p.provider.PullModel(ctx, pullReq.Model, func(progress PullProgress) {
		data, _ := json.Marshal(progress)
		sseData := []byte("data: " + string(data) + "\n\n")
		_ = stream.Send(&pluginv1.PluginHTTPChunk{
			Type: pluginv1.PluginHTTPChunk_RESPONSE_BODY,
			Data: sseData,
		})
	})

	if pullErr != nil {
		// Send error as SSE event
		errData, _ := json.Marshal(PullProgress{Error: pullErr.Error()})
		sseData := []byte("data: " + string(errData) + "\n\n")
		_ = stream.Send(&pluginv1.PluginHTTPChunk{
			Type: pluginv1.PluginHTTPChunk_RESPONSE_BODY,
			Data: sseData,
		})
	} else {
		// Send final success event
		doneData, _ := json.Marshal(PullProgress{Status: "success", Done: true})
		sseData := []byte("data: " + string(doneData) + "\n\n")
		_ = stream.Send(&pluginv1.PluginHTTPChunk{
			Type: pluginv1.PluginHTTPChunk_RESPONSE_BODY,
			Data: sseData,
		})
	}

	return stream.Send(&pluginv1.PluginHTTPChunk{
		Type: pluginv1.PluginHTTPChunk_RESPONSE_END,
	})
}

// sendSSEError sends an error response in SSE format.
func sendSSEError(stream pluginv1.PluginCore_HandleHTTPStreamServer, errMsg string) error {
	if err := stream.Send(&pluginv1.PluginHTTPChunk{
		Type:        pluginv1.PluginHTTPChunk_RESPONSE_START,
		StatusCode:  200,
		ContentType: "text/event-stream",
	}); err != nil {
		return err
	}

	errData, _ := json.Marshal(PullProgress{Error: errMsg})
	sseData := []byte("data: " + string(errData) + "\n\n")
	if err := stream.Send(&pluginv1.PluginHTTPChunk{
		Type: pluginv1.PluginHTTPChunk_RESPONSE_BODY,
		Data: sseData,
	}); err != nil {
		return err
	}

	return stream.Send(&pluginv1.PluginHTTPChunk{
		Type: pluginv1.PluginHTTPChunk_RESPONSE_END,
	})
}

// jsonResponse creates a JSON HTTP response.
func jsonResponse(status int32, data any) (*pluginv1.PluginHTTPResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &pluginv1.PluginHTTPResponse{
		StatusCode:  status,
		ContentType: "application/json",
		Body:        body,
	}, nil
}

// jsonError creates a JSON error response.
func jsonError(status int32, message string) (*pluginv1.PluginHTTPResponse, error) {
	return jsonResponse(status, map[string]string{"error": message})
}

// handleRecommendedModels returns recommended models based on system resources.
func (p *Plugin) handleRecommendedModels(ctx context.Context) (*pluginv1.PluginHTTPResponse, error) {
	// Get installed models to check installation status
	installedModels := make(map[string]bool)
	if models, err := p.provider.ListModels(ctx, &pluginv1.Empty{}); err == nil {
		for _, m := range models.Models {
			installedModels[m.Id] = true
			// Also match base name without tag suffix
			if idx := strings.Index(m.Id, ":"); idx > 0 {
				installedModels[m.Id[:idx]] = true
			}
		}
	}

	// Get system resources
	var ramBytes, vramBytes uint64
	var hasGPU bool
	hasSystemInfo := p.systemInfo != nil
	if hasSystemInfo {
		ramBytes = p.systemInfo.RamBytes
		vramBytes = p.systemInfo.VramBytes
		hasGPU = p.systemInfo.HasGpu
	}

	// Combine all models
	allSpecs := append(embeddingModels, chatModels...)

	// Build response items
	// If we have system info, filter to models that can run or are installed
	// If we don't have system info, show all models (can't determine what can run)
	items := make([]map[string]any, 0, len(allSpecs))
	for _, spec := range allSpecs {
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

	return jsonResponse(200, map[string]any{
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
