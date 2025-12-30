// Package internal implements the TMDb plugin logic.
package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
	"gopkg.in/yaml.v3"
)

// Config holds the TMDb plugin configuration.
type Config struct {
	APIKey        string `yaml:"api_key" json:"api_key"`
	RateLimit     int    `yaml:"rate_limit" json:"rate_limit"`           // requests per 10 seconds (default: 40)
	CacheTTLHours int    `yaml:"cache_ttl_hours" json:"cache_ttl_hours"` // cache duration (default: 24)
	Language      string `yaml:"language" json:"language"`               // preferred language (default: en-US)

	// Settings from UI
	IncludeAdult     bool     `yaml:"include_adult" json:"include_adult"`
	FetchImages      bool     `yaml:"fetch_images" json:"fetch_images"`
	ImageTypes       []string `yaml:"image_types" json:"image_types"`
	ImageSize        string   `yaml:"image_size" json:"image_size"`
	FetchActorPhotos bool     `yaml:"fetch_actor_photos" json:"fetch_actor_photos"`
	MaxActorPhotos   int      `yaml:"max_actor_photos" json:"max_actor_photos"`
}

// TMDbPlugin implements sdk.EnricherPlugin for TMDb.
type TMDbPlugin struct {
	sdk.Base

	logger  *slog.Logger
	dataDir string
	config  Config
	client  *Client
	storage *sdk.StorageClient

	mu sync.RWMutex

	// Stats for health reporting
	requestsTotal int64
	errorsTotal   int64
}

// NewTMDbPlugin creates a new TMDb plugin instance.
func NewTMDbPlugin(logger *slog.Logger) *TMDbPlugin {
	p := &TMDbPlugin{
		logger: logger,
	}
	p.SetLogger(logger)
	return p
}

// recordError increments the error counter (thread-safe).
func (p *TMDbPlugin) recordError() {
	p.mu.Lock()
	p.errorsTotal++
	p.mu.Unlock()
}

// --- sdk.EnricherPlugin implementation ---

func (p *TMDbPlugin) GetCapabilities() sdk.EnricherCapabilities {
	return sdk.EnricherCapabilities{
		MediaTypes: []string{"movie", "tv", "tv_show"},
		Provides:   []string{"metadata", "artwork", "external_ids"},
		IsLocal:    false,
		RateLimit:  40, // TMDb allows ~40 requests per 10 seconds
		Requires:   []string{},
		Priority:   50,
	}
}

func (p *TMDbPlugin) Initialize(ctx context.Context, dataDir string, config []byte, services *sdk.HostServices) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.dataDir = dataDir
	p.logger.Debug("initializing TMDb plugin", "data_dir", dataDir)

	// Store host storage client if available
	if services != nil && services.Storage != nil {
		p.storage = services.Storage
		p.logger.Debug("host storage service available")
	}

	// Parse config from YAML
	if len(config) == 0 {
		return fmt.Errorf("config.yml is required but was not provided")
	}

	if err := yaml.Unmarshal(config, &p.config); err != nil {
		return fmt.Errorf("failed to parse config.yml: %w", err)
	}

	// Validate required fields
	if p.config.APIKey == "" {
		return fmt.Errorf("api_key is required in config.yml")
	}

	// Apply defaults
	if p.config.RateLimit == 0 {
		p.config.RateLimit = 40
	}
	if p.config.CacheTTLHours == 0 {
		p.config.CacheTTLHours = 24
	}
	if p.config.Language == "" {
		p.config.Language = "en-US"
	}

	// Create the API client
	client, err := NewClient(ClientConfig{
		APIKey:        p.config.APIKey,
		CacheTTLHours: p.config.CacheTTLHours,
		Storage:       p.storage,
		Logger:        p.logger,
	})
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}
	p.client = client

	cacheStatus := "disabled (no host storage)"
	if p.storage != nil {
		cacheStatus = fmt.Sprintf("enabled (TTL: %dh)", p.config.CacheTTLHours)
	}
	p.logger.Debug("TMDb plugin initialized",
		"rate_limit", p.config.RateLimit,
		"cache", cacheStatus,
		"language", p.config.Language)

	return nil
}

func (p *TMDbPlugin) Shutdown(ctx context.Context) error {
	p.logger.Debug("shutting down TMDb plugin")
	if p.client != nil {
		p.client.Close()
	}
	return nil
}

// GetSettingsSchema returns the JSON Schema for plugin settings.
func (p *TMDbPlugin) GetSettingsSchema() ([]byte, error) {
	return SettingsSchema().Build()
}

// Configure applies new settings to the plugin.
func (p *TMDbPlugin) Configure(settings []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var newSettings struct {
		Language         string   `json:"language"`
		IncludeAdult     bool     `json:"include_adult"`
		CacheTTLHours    int      `json:"cache_ttl_hours"`
		FetchImages      bool     `json:"fetch_images"`
		ImageTypes       []string `json:"image_types"`
		ImageSize        string   `json:"image_size"`
		FetchActorPhotos bool     `json:"fetch_actor_photos"`
		MaxActorPhotos   int      `json:"max_actor_photos"`
	}
	if err := json.Unmarshal(settings, &newSettings); err != nil {
		return fmt.Errorf("failed to parse settings: %w", err)
	}

	// Apply settings to config
	if newSettings.Language != "" {
		p.config.Language = newSettings.Language
	}
	p.config.IncludeAdult = newSettings.IncludeAdult
	if newSettings.CacheTTLHours > 0 {
		p.config.CacheTTLHours = newSettings.CacheTTLHours
	}
	p.config.FetchImages = newSettings.FetchImages
	if len(newSettings.ImageTypes) > 0 {
		p.config.ImageTypes = newSettings.ImageTypes
	}
	if newSettings.ImageSize != "" {
		p.config.ImageSize = newSettings.ImageSize
	}
	p.config.FetchActorPhotos = newSettings.FetchActorPhotos
	if newSettings.MaxActorPhotos > 0 {
		p.config.MaxActorPhotos = newSettings.MaxActorPhotos
	}

	p.logger.Debug("configuration updated",
		"language", p.config.Language,
		"include_adult", p.config.IncludeAdult,
		"cache_ttl_hours", p.config.CacheTTLHours,
		"fetch_images", p.config.FetchImages,
		"image_types", p.config.ImageTypes,
		"fetch_actor_photos", p.config.FetchActorPhotos,
	)

	return nil
}

func (p *TMDbPlugin) Enrich(ctx context.Context, req *sdk.EnrichRequest) (*sdk.EnrichResponse, error) {
	p.mu.Lock()
	p.requestsTotal++
	client := p.client
	p.mu.Unlock()

	// Check if configured
	if client == nil {
		p.mu.Lock()
		p.errorsTotal++
		p.mu.Unlock()
		return sdk.Skip("TMDb API key not configured"), nil
	}

	p.logger.Debug("enriching media",
		"media_id", req.MediaID,
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
		return sdk.Skip("unsupported media type: " + req.MediaType), nil
	}
}

// --- sdk.HTTPEnricher implementation ---

func (p *TMDbPlugin) GetRoutes() []sdk.Route {
	return []sdk.Route{
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
	}
}

func (p *TMDbPlugin) HandleHTTP(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	p.logger.Debug("handling HTTP request", "path", req.Path, "method", req.Method)

	switch req.Path {
	case "/enrich":
		return p.handleEnrich(ctx, req)
	case "/lookup":
		return p.handleLookup(ctx, req)
	default:
		return sdk.JSONError(http.StatusNotFound, "route not found: "+req.Path)
	}
}

func (p *TMDbPlugin) handleEnrich(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	if req.Method != "POST" {
		return sdk.JSONError(http.StatusMethodNotAllowed, "method not allowed")
	}

	var enrichReq struct {
		MediaID     int64             `json:"media_id"`
		MediaType   string            `json:"media_type"`
		Title       string            `json:"title"`
		Year        int               `json:"year"`
		ExternalIDs map[string]string `json:"external_ids,omitempty"`
	}
	if err := json.Unmarshal(req.Body, &enrichReq); err != nil {
		return sdk.JSONError(http.StatusBadRequest, "invalid JSON: "+err.Error())
	}

	if enrichReq.MediaType == "" {
		return sdk.JSONError(http.StatusBadRequest, "media_type is required")
	}
	if enrichReq.Title == "" && len(enrichReq.ExternalIDs) == 0 {
		return sdk.JSONError(http.StatusBadRequest, "title or external_ids is required")
	}

	// Build the SDK enrich request
	sdkReq := &sdk.EnrichRequest{
		MediaID:     enrichReq.MediaID,
		MediaType:   enrichReq.MediaType,
		Title:       enrichReq.Title,
		Year:        enrichReq.Year,
		ExistingIDs: enrichReq.ExternalIDs,
	}

	// Call the enrichment logic
	resp, err := p.Enrich(ctx, sdkReq)
	if err != nil {
		return sdk.JSONError(http.StatusInternalServerError, err.Error())
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
			"genres":     resp.Metadata.Genres,
			"cast_count": len(resp.Metadata.Cast),
		}
		if resp.Metadata.Title != nil {
			metaMap["title"] = *resp.Metadata.Title
		}
		if resp.Metadata.Year != nil {
			metaMap["year"] = *resp.Metadata.Year
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
					"id":          kw.ID,
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

	return sdk.JSONResponse(http.StatusOK, result)
}

func (p *TMDbPlugin) handleLookup(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	if req.Method != "GET" {
		return sdk.JSONError(http.StatusMethodNotAllowed, "method not allowed")
	}

	title := req.Query["title"]
	mediaType := req.Query["type"]
	if mediaType == "" {
		mediaType = "movie"
	}

	if title == "" {
		return sdk.JSONError(http.StatusBadRequest, "title query param is required")
	}

	p.mu.RLock()
	client := p.client
	p.mu.RUnlock()

	if client == nil {
		return sdk.JSONError(http.StatusServiceUnavailable, "TMDb client not initialized")
	}

	var results any
	var err error

	switch mediaType {
	case "movie":
		results, err = client.SearchMovies(ctx, title, 0)
	case "tv", "tv_show":
		results, err = client.SearchTV(ctx, title, 0)
	default:
		return sdk.JSONError(http.StatusBadRequest, "invalid type, must be 'movie' or 'tv'")
	}

	if err != nil {
		return sdk.JSONError(http.StatusInternalServerError, err.Error())
	}

	return sdk.JSONResponse(http.StatusOK, map[string]any{
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
