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

	provider *OpenAIProvider
	logger   *slog.Logger
}

// NewPlugin creates a new plugin instance.
func NewPlugin(logger *slog.Logger) *Plugin {
	return &Plugin{
		provider: NewOpenAIProvider(logger),
		logger:   logger,
	}
}

// Provider returns the OpenAI provider implementation.
func (p *Plugin) Provider() *OpenAIProvider {
	return p.provider
}

// Initialize is called when the plugin is loaded.
func (p *Plugin) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
	p.logger.Info("initializing OpenAI provider plugin",
		"host_version", req.HostVersion,
		"data_dir", req.DataDir)

	return &pluginv1.InitResponse{Success: true}, nil
}

// Shutdown is called when the plugin is unloaded.
func (p *Plugin) Shutdown(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.Empty, error) {
	p.logger.Info("shutting down OpenAI provider plugin")
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

// GetRoutes returns HTTP routes for OpenAI provider.
func (p *Plugin) GetRoutes(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.PluginRoutes, error) {
	return &pluginv1.PluginRoutes{
		Routes: []*pluginv1.PluginRoute{
			{
				Path:        "/health",
				Methods:     []string{"GET"},
				AdminOnly:   false,
				Description: "Check OpenAI API connectivity",
			},
		},
	}, nil
}

// GetSettingsSchema returns the JSON Schema for plugin settings.
func (p *Plugin) GetSettingsSchema(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.SettingsSchema, error) {
	schema := []byte(`{
		"type": "object",
		"title": "OpenAI Settings",
		"x-viewra-meta": {
			"displayName": "OpenAI",
			"description": "OpenAI API for embeddings and chat",
			"tip": "Requires API key. Usage is billed per token.",
			"isLocal": false,
			"icon": "cloud"
		},
		"required": ["api_key"],
		"properties": {
			"api_key": {
				"type": "string",
				"title": "API Key",
				"description": "Your OpenAI API key",
				"format": "password"
			},
			"base_url": {
				"type": "string",
				"title": "Base URL",
				"description": "Custom API base URL (optional, for OpenAI-compatible providers)",
				"default": ""
			},
			"embedding_model": {
				"type": "string",
				"title": "Embedding Model",
				"description": "Model to use for generating embeddings",
				"default": "text-embedding-3-small"
			},
			"chat_model": {
				"type": "string",
				"title": "Chat Model",
				"description": "Model to use for chat completions",
				"default": "gpt-4o-mini"
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

	if err := p.provider.Configure(settings.APIKey, settings.BaseURL); err != nil {
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

	// GET /health - Check OpenAI API connectivity
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
