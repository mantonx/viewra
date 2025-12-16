// Package internal implements the TMDb plugin logic.
package internal

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"gopkg.in/yaml.v3"
)

// Config holds the TMDb plugin configuration.
type Config struct {
	APIKey        string `yaml:"api_key" json:"api_key"`
	RateLimit     int    `yaml:"rate_limit" json:"rate_limit"`         // requests per 10 seconds (default: 40)
	CacheTTLHours int    `yaml:"cache_ttl_hours" json:"cache_ttl_hours"` // cache duration (default: 24)
	Language      string `yaml:"language" json:"language"`             // preferred language (default: en-US)
}

// TMDbPlugin implements both PluginCore and Enricher interfaces.
type TMDbPlugin struct {
	pluginv1.UnimplementedPluginCoreServer
	pluginv1.UnimplementedEnricherServer

	logger  *slog.Logger
	dataDir string
	config  Config
	client  *Client
	storage pluginv1.HostStorageClient

	mu sync.RWMutex

	// Stats for health reporting
	requestsTotal int64
	errorsTotal   int64
}

// recordError increments the error counter (thread-safe).
func (p *TMDbPlugin) recordError() {
	p.mu.Lock()
	p.errorsTotal++
	p.mu.Unlock()
}

// NewTMDbPlugin creates a new TMDb plugin instance.
func NewTMDbPlugin(logger *slog.Logger) *TMDbPlugin {
	return &TMDbPlugin{
		logger: logger,
	}
}

// SetStorageClient sets the host storage client for caching.
// This should be called before Initialize if host storage is available.
func (p *TMDbPlugin) SetStorageClient(client pluginv1.HostStorageClient) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.storage = client
}

// PluginCore implementation
// Plugin identity comes from plugin.yml manifest.

func (p *TMDbPlugin) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.dataDir = req.DataDir
	p.logger.Info("initializing TMDb plugin",
		"host_version", req.HostVersion,
		"data_dir", req.DataDir,
	)

	// Parse config from YAML (source of truth is config.yml passed by host)
	if len(req.Config) == 0 {
		return &pluginv1.InitResponse{
			Success: false,
			Error:   "config.yml is required but was not provided",
		}, nil
	}

	if err := yaml.Unmarshal(req.Config, &p.config); err != nil {
		return &pluginv1.InitResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to parse config.yml: %v", err),
		}, nil
	}

	// Validate required fields
	if p.config.APIKey == "" {
		return &pluginv1.InitResponse{
			Success: false,
			Error:   "api_key is required in config.yml",
		}, nil
	}

	// Apply defaults
	if p.config.RateLimit == 0 {
		p.config.RateLimit = 40 // TMDb default: 40 requests per 10 seconds
	}
	if p.config.CacheTTLHours == 0 {
		p.config.CacheTTLHours = 24
	}
	if p.config.Language == "" {
		p.config.Language = "en-US"
	}

	// Create the API client with rate limiting
	// Note: Storage client will be set separately via SetStorageClient if available
	client, err := NewClient(ClientConfig{
		APIKey:        p.config.APIKey,
		CacheTTLHours: p.config.CacheTTLHours,
		Storage:       p.storage, // May be nil if host storage not available
		Logger:        p.logger,
	})
	if err != nil {
		return &pluginv1.InitResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to create API client: %v", err),
		}, nil
	}
	p.client = client

	cacheStatus := "disabled (no host storage)"
	if p.storage != nil {
		cacheStatus = fmt.Sprintf("enabled (TTL: %dh)", p.config.CacheTTLHours)
	}
	p.logger.Info("TMDb plugin initialized",
		"rate_limit", p.config.RateLimit,
		"cache", cacheStatus,
		"language", p.config.Language)

	return &pluginv1.InitResponse{Success: true}, nil
}

func (p *TMDbPlugin) Shutdown(ctx context.Context, req *pluginv1.Empty) (*pluginv1.Empty, error) {
	p.logger.Info("shutting down TMDb plugin")
	if p.client != nil {
		p.client.Close()
	}
	return &pluginv1.Empty{}, nil
}

func (p *TMDbPlugin) HealthCheck(ctx context.Context, req *pluginv1.Empty) (*pluginv1.HealthStatus, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	status := pluginv1.HealthStatus_HEALTHY
	message := "operational"

	// Check if API key is configured
	if p.config.APIKey == "" {
		status = pluginv1.HealthStatus_DEGRADED
		message = "API key not configured"
	}

	return &pluginv1.HealthStatus{
		Status:        status,
		Message:       message,
		RequestsTotal: p.requestsTotal,
		ErrorsTotal:   p.errorsTotal,
	}, nil
}

func (p *TMDbPlugin) GetSettingsSchema(ctx context.Context, req *pluginv1.Empty) (*pluginv1.SettingsSchema, error) {
	// Configuration is via config.yaml file, not runtime settings
	return &pluginv1.SettingsSchema{JsonSchema: []byte("{}")}, nil
}

func (p *TMDbPlugin) Configure(ctx context.Context, req *pluginv1.Settings) (*pluginv1.ConfigureResponse, error) {
	// Configuration is via config.yml file, not runtime settings
	return &pluginv1.ConfigureResponse{
		Success: false,
		Error:   "runtime configuration not supported; edit config.yml and restart the plugin",
	}, nil
}

func (p *TMDbPlugin) GetSubscriptions(ctx context.Context, req *pluginv1.Empty) (*pluginv1.EventSubscriptions, error) {
	// TMDb doesn't need any events
	return &pluginv1.EventSubscriptions{}, nil
}

func (p *TMDbPlugin) OnEvent(ctx context.Context, req *pluginv1.Event) (*pluginv1.EventResponse, error) {
	return &pluginv1.EventResponse{Handled: false}, nil
}

// Enricher implementation

func (p *TMDbPlugin) GetCapabilities(ctx context.Context, req *pluginv1.Empty) (*pluginv1.EnricherCapabilities, error) {
	return &pluginv1.EnricherCapabilities{
		MediaTypes: []string{"movie", "tv", "tv_show"},
		Provides:   []string{"metadata", "artwork", "external_ids"},
		IsLocal:    false,
		RateLimit:  40, // TMDb allows ~40 requests per 10 seconds
		Requires:   []string{}, // Can work from title/year, but prefers imdb/tmdb IDs
		Priority:   50,
	}, nil
}

func (p *TMDbPlugin) Enrich(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
	p.mu.Lock()
	p.requestsTotal++
	client := p.client
	p.mu.Unlock()

	// Check if configured
	if client == nil {
		p.mu.Lock()
		p.errorsTotal++
		p.mu.Unlock()
		return &pluginv1.EnrichResponse{
			Skipped:    true,
			SkipReason: "TMDb API key not configured",
		}, nil
	}

	p.logger.Debug("enriching media",
		"media_id", req.MediaId,
		"media_type", req.MediaType,
		"title", req.Title,
		"year", req.Year,
	)

	switch req.MediaType {
	case "movie":
		return p.enrichMovie(ctx, client, req)
	case "tv", "tv_show":
		return p.enrichTV(ctx, client, req)
	default:
		return &pluginv1.EnrichResponse{
			Skipped:    true,
			SkipReason: "unsupported media type: " + req.MediaType,
		}, nil
	}
}
