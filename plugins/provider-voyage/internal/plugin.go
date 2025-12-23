package internal

import (
	"context"
	"encoding/json"
	"log/slog"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// Plugin implements the PluginCore service.
type Plugin struct {
	pluginv1.UnimplementedPluginCoreServer

	provider *VoyageProvider
	logger   *slog.Logger
}

// NewPlugin creates a new plugin instance.
func NewPlugin(logger *slog.Logger) *Plugin {
	return &Plugin{
		provider: NewVoyageProvider(logger),
		logger:   logger,
	}
}

// Provider returns the Voyage provider implementation.
func (p *Plugin) Provider() *VoyageProvider {
	return p.provider
}

// Initialize is called when the plugin is loaded.
func (p *Plugin) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
	p.logger.Info("initializing Voyage AI provider plugin",
		"host_version", req.HostVersion,
		"data_dir", req.DataDir)

	return &pluginv1.InitResponse{Success: true}, nil
}

// Shutdown is called when the plugin is unloaded.
func (p *Plugin) Shutdown(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.Empty, error) {
	p.logger.Info("shutting down Voyage AI provider plugin")
	return &pluginv1.Empty{}, nil
}

// HealthCheck returns the plugin's health status.
func (p *Plugin) HealthCheck(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.HealthStatus, error) {
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

// GetRoutes returns HTTP routes for Voyage provider.
func (p *Plugin) GetRoutes(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.PluginRoutes, error) {
	return &pluginv1.PluginRoutes{
		Routes: []*pluginv1.PluginRoute{
			{
				Path:        "/health",
				Methods:     []string{"GET"},
				AdminOnly:   false,
				Description: "Check Voyage AI API connectivity",
			},
		},
	}, nil
}

// GetSettingsSchema returns the JSON Schema for plugin settings.
func (p *Plugin) GetSettingsSchema(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.SettingsSchema, error) {
	schema := []byte(`{
		"type": "object",
		"title": "Voyage AI Settings",
		"x-viewra-meta": {
			"displayName": "Voyage AI",
			"description": "High-quality embeddings for semantic search",
			"tip": "Requires API key. Specialized for embedding generation.",
			"isLocal": false,
			"icon": "compass"
		},
		"required": ["api_key"],
		"properties": {
			"api_key": {
				"type": "string",
				"title": "API Key",
				"description": "Your Voyage AI API key",
				"format": "password"
			},
			"embedding_model": {
				"type": "string",
				"title": "Embedding Model",
				"description": "Model to use for generating embeddings",
				"default": "voyage-3-lite"
			}
		},
		"x-viewra-actions": [
			{
				"id": "test-connection",
				"type": "test",
				"title": "Test Connection",
				"endpoint": "/health"
			}
		]
	}`)

	return &pluginv1.SettingsSchema{
		JsonSchema: schema,
	}, nil
}

// Configure applies new settings to the plugin.
func (p *Plugin) Configure(ctx context.Context, req *pluginv1.Settings) (*pluginv1.ConfigureResponse, error) {
	p.logger.Debug("configure called", "settings_length", len(req.Json))

	var settings struct {
		APIKey         string `json:"api_key"`
		EmbeddingModel string `json:"embedding_model"`
	}

	if err := json.Unmarshal(req.Json, &settings); err != nil {
		return &pluginv1.ConfigureResponse{
			Success: false,
			Error:   "invalid settings JSON: " + err.Error(),
		}, nil
	}

	if err := p.provider.Configure(settings.APIKey); err != nil {
		return &pluginv1.ConfigureResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Store embedding model selection
	p.provider.SetEmbeddingModel(settings.EmbeddingModel)

	return &pluginv1.ConfigureResponse{Success: true}, nil
}

// HandleHTTP handles non-streaming HTTP requests.
func (p *Plugin) HandleHTTP(ctx context.Context, req *pluginv1.PluginHTTPRequest) (*pluginv1.PluginHTTPResponse, error) {
	p.logger.Debug("HandleHTTP", "path", req.Path, "method", req.Method)

	// GET /health - Check Voyage AI API connectivity
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

	return jsonError(404, "not found")
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
