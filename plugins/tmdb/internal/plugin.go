// Package internal implements the TMDb plugin logic.
package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"gopkg.in/yaml.v3"
)

// Config holds the TMDb plugin configuration.
type Config struct {
	APIKey        string `yaml:"api_key" json:"api_key"`
	RateLimit     int    `yaml:"rate_limit" json:"rate_limit"`           // requests per 10 seconds (default: 40)
	CacheTTLHours int    `yaml:"cache_ttl_hours" json:"cache_ttl_hours"` // cache duration (default: 24)
	Language      string `yaml:"language" json:"language"`               // preferred language (default: en-US)
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
		RateLimit:  40,         // TMDb allows ~40 requests per 10 seconds
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

// ============================================================
// Plugin-defined HTTP routes for testing/debugging
// ============================================================

// GetRoutes returns the HTTP routes this plugin provides.
func (p *TMDbPlugin) GetRoutes(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.PluginRoutes, error) {
	return &pluginv1.PluginRoutes{
		Routes: []*pluginv1.PluginRoute{
			{
				Path:        "/enrich",
				Methods:     []string{"POST"},
				AdminOnly:   true,
				Description: "Trigger enrichment for a specific media item (for testing)",
			},
			{
				Path:        "/lookup",
				Methods:     []string{"GET"},
				AdminOnly:   false,
				Description: "Look up a title on TMDB without enriching",
			},
		},
	}, nil
}

// HandleHTTP handles HTTP requests to plugin routes.
func (p *TMDbPlugin) HandleHTTP(ctx context.Context, req *pluginv1.PluginHTTPRequest) (*pluginv1.PluginHTTPResponse, error) {
	p.logger.Debug("handling HTTP request", "path", req.Path, "method", req.Method)

	switch req.Path {
	case "/enrich":
		return p.handleEnrich(ctx, req)
	case "/lookup":
		return p.handleLookup(ctx, req)
	default:
		return jsonResponse(http.StatusNotFound, map[string]string{
			"error": "route not found",
			"path":  req.Path,
		})
	}
}

// handleEnrich handles POST /enrich - triggers enrichment for a specific media item.
// Request body: {"media_id": 123, "media_type": "movie", "title": "The Matrix", "year": 1999}
func (p *TMDbPlugin) handleEnrich(ctx context.Context, req *pluginv1.PluginHTTPRequest) (*pluginv1.PluginHTTPResponse, error) {
	if req.Method != "POST" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	var enrichReq struct {
		MediaID     int64             `json:"media_id"`
		MediaType   string            `json:"media_type"`
		Title       string            `json:"title"`
		Year        int32             `json:"year"`
		ExternalIDs map[string]string `json:"external_ids,omitempty"`
	}
	if err := json.Unmarshal(req.Body, &enrichReq); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
	}

	if enrichReq.MediaType == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "media_type is required"})
	}
	if enrichReq.Title == "" && len(enrichReq.ExternalIDs) == 0 {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "title or external_ids is required"})
	}

	// Build the enrich request
	protoReq := &pluginv1.EnrichRequest{
		MediaId:     enrichReq.MediaID,
		MediaType:   enrichReq.MediaType,
		Title:       enrichReq.Title,
		Year:        enrichReq.Year,
		ExistingIds: enrichReq.ExternalIDs,
	}

	// Call the enrichment logic
	resp, err := p.Enrich(ctx, protoReq)
	if err != nil {
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Build response with metadata summary
	result := map[string]any{
		"success": !resp.Skipped,
	}
	if resp.Skipped {
		result["skip_reason"] = resp.SkipReason
	}
	if resp.Metadata != nil {
		metaMap := map[string]any{
			"title":      resp.Metadata.Title,
			"year":       resp.Metadata.Year,
			"genres":     resp.Metadata.Genres,
			"cast_count": len(resp.Metadata.Cast),
		}
		if resp.Metadata.Plot != nil {
			metaMap["plot"] = truncate(*resp.Metadata.Plot, 200)
		}
		result["metadata"] = metaMap

		// Show keyword details if present
		if len(resp.Metadata.Keywords) > 0 {
			keywords := make([]map[string]any, 0, len(resp.Metadata.Keywords))
			locationCount := 0
			for _, kw := range resp.Metadata.Keywords {
				keywords = append(keywords, map[string]any{
					"id":          kw.Id,
					"name":        kw.Name,
					"is_location": kw.IsLocation,
				})
				if kw.IsLocation {
					locationCount++
				}
			}
			result["keywords"] = keywords
			result["location_keywords_count"] = locationCount
		}
	}

	return jsonResponse(http.StatusOK, result)
}

// handleLookup handles GET /lookup - looks up a title on TMDB without enriching.
// Query params: ?title=The+Matrix&year=1999&type=movie
func (p *TMDbPlugin) handleLookup(ctx context.Context, req *pluginv1.PluginHTTPRequest) (*pluginv1.PluginHTTPResponse, error) {
	if req.Method != "GET" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	title := req.Query["title"]
	mediaType := req.Query["type"]
	if mediaType == "" {
		mediaType = "movie"
	}

	if title == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "title query param is required"})
	}

	p.mu.RLock()
	client := p.client
	p.mu.RUnlock()

	if client == nil {
		return jsonResponse(http.StatusServiceUnavailable, map[string]string{"error": "TMDb client not initialized"})
	}

	var results any
	var err error

	switch mediaType {
	case "movie":
		results, err = client.SearchMovies(ctx, title, 0)
	case "tv", "tv_show":
		results, err = client.SearchTV(ctx, title, 0)
	default:
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "invalid type, must be 'movie' or 'tv'"})
	}

	if err != nil {
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return jsonResponse(http.StatusOK, map[string]any{
		"query":   title,
		"type":    mediaType,
		"results": results,
	})
}

// truncate shortens a string to maxLen characters, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// jsonResponse creates a JSON HTTP response.
func jsonResponse(statusCode int, data any) (*pluginv1.PluginHTTPResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return &pluginv1.PluginHTTPResponse{
			StatusCode:  http.StatusInternalServerError,
			ContentType: "application/json",
			Body:        []byte(`{"error":"failed to serialize response"}`),
		}, nil
	}

	return &pluginv1.PluginHTTPResponse{
		StatusCode:  int32(statusCode),
		ContentType: "application/json",
		Body:        body,
	}, nil
}
