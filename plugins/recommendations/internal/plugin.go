package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
	"gopkg.in/yaml.v3"
)

// RecommendationsPlugin implements the recommendations plugin.
type RecommendationsPlugin struct {
	sdk.Base

	config Config

	// Host service clients
	data    *sdk.DataClient
	plugins *sdk.PluginsClient
	ratings *sdk.RatingsClient

	// Services
	recommendationsService *RecommendationsService

	mu sync.RWMutex
}

// Compile-time check that RecommendationsPlugin implements WidgetPlugin
var _ sdk.WidgetPlugin = (*RecommendationsPlugin)(nil)

// NewRecommendationsPlugin creates a new recommendations plugin instance.
func NewRecommendationsPlugin(logger *slog.Logger) *RecommendationsPlugin {
	p := &RecommendationsPlugin{
		config: DefaultConfig(),
	}
	p.SetLogger(logger)
	return p
}

// Initialize is called when the plugin is loaded.
func (p *RecommendationsPlugin) Initialize(ctx context.Context, dataDir string, config []byte, services *sdk.HostServices) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Base.Init(dataDir)

	p.Log().Debug("initializing Recommendations plugin", "data_dir", dataDir)

	// Parse config from YAML
	if len(config) > 0 {
		if err := yaml.Unmarshal(config, &p.config); err != nil {
			return fmt.Errorf("failed to parse config.yml: %w", err)
		}
	}

	// Store host service clients
	if services != nil {
		p.plugins = services.Plugins
		p.data = services.Data
		p.ratings = services.Ratings
	}

	// Initialize services
	p.initializeServices()

	p.Log().Debug("Recommendations plugin initialized", "enabled", p.config.Enabled)

	return nil
}

// initializeServices creates the internal service instances.
func (p *RecommendationsPlugin) initializeServices() {
	// Create recommendations service
	if p.data != nil && p.ratings != nil {
		p.recommendationsService = NewRecommendationsService(
			p.ratings,
			p.data,
			p.plugins,
			p.config,
			p.Log(),
		)
	}
}

// GetSettingsSchema returns the JSON Schema for plugin settings.
func (p *RecommendationsPlugin) GetSettingsSchema() ([]byte, error) {
	return SettingsSchema().Build()
}

// Configure applies new settings to the plugin.
func (p *RecommendationsPlugin) Configure(settings []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var cfg struct {
		Enabled            bool `json:"enabled"`
		MaxRecommendations int  `json:"max_recommendations"`
		SimilarWeight      int  `json:"similar_weight"`
		FavoriteWeight     int  `json:"favorite_weight"`
	}
	if err := json.Unmarshal(settings, &cfg); err != nil {
		return fmt.Errorf("failed to parse settings: %w", err)
	}

	p.config.Enabled = cfg.Enabled
	if cfg.MaxRecommendations > 0 {
		p.config.MaxRecommendations = cfg.MaxRecommendations
	}
	p.config.SimilarWeight = cfg.SimilarWeight
	p.config.FavoriteWeight = cfg.FavoriteWeight

	// Update recommendations service config
	if p.recommendationsService != nil {
		p.recommendationsService.UpdateConfig(p.config)
	}

	p.Log().Debug("configuration updated",
		"enabled", p.config.Enabled,
		"max_recommendations", p.config.MaxRecommendations,
	)

	return nil
}

// Shutdown is called before the plugin is unloaded.
func (p *RecommendationsPlugin) Shutdown(ctx context.Context) error {
	p.Log().Debug("shutting down Recommendations plugin")
	return nil
}

// GetRoutes returns the HTTP routes this plugin provides.
// Note: Ratings CRUD is handled by core's /api/ratings endpoints.
func (p *RecommendationsPlugin) GetRoutes() []sdk.Route {
	return []sdk.Route{
		// Read-only ratings access (for UI compatibility during transition)
		{
			Path:        "/ratings",
			Methods:     []string{"GET"},
			Description: "Get user's ratings (read-only, use core /api/ratings for writes)",
		},
		// Recommendations
		{
			Path:        "/recommendations/for-you",
			Methods:     []string{"GET"},
			Description: "Get personalized recommendations",
		},
		{
			Path:        "/recommendations/because-you-liked",
			Methods:     []string{"GET"},
			Description: "Get recommendations based on favorites",
		},
	}
}

// HandleHTTP handles HTTP requests to the plugin.
func (p *RecommendationsPlugin) HandleHTTP(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	p.Log().Debug("handling HTTP request", "path", req.Path, "method", req.Method)

	switch {
	case req.Path == "/ratings" && req.Method == "GET":
		return p.handleGetRatings(ctx, req)
	case req.Path == "/recommendations/for-you":
		return p.handleForYou(ctx, req)
	case req.Path == "/recommendations/because-you-liked":
		return p.handleBecauseYouLiked(ctx, req)
	default:
		return jsonResponse(http.StatusNotFound, map[string]string{
			"error": "route not found",
			"path":  req.Path,
		})
	}
}

// =============================================================================
// HTTP Handlers
// =============================================================================

func (p *RecommendationsPlugin) handleGetRatings(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	p.mu.RLock()
	ratingsClient := p.ratings
	p.mu.RUnlock()

	if ratingsClient == nil {
		return jsonResponse(http.StatusServiceUnavailable, map[string]string{"error": "ratings service not available"})
	}

	// Parse filters
	entityType := req.Query["entity_type"]
	ratingType := req.Query["rating"]

	ratings, err := ratingsClient.ListRatings(ctx, req.UserID, entityType, ratingType)
	if err != nil {
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return jsonResponse(http.StatusOK, map[string]any{
		"ratings": ratings,
		"total":   len(ratings),
	})
}

func (p *RecommendationsPlugin) handleForYou(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	p.mu.RLock()
	recService := p.recommendationsService
	config := p.config
	p.mu.RUnlock()

	if !config.Enabled {
		return jsonResponse(http.StatusOK, map[string]any{
			"title": "Recommended For You",
			"items": []any{},
		})
	}

	if recService == nil {
		return jsonResponse(http.StatusServiceUnavailable, map[string]string{"error": "recommendations service not available"})
	}

	limit := 20
	if limitStr := req.Query["limit"]; limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	items, err := recService.GetForYou(ctx, req.UserID, limit)
	if err != nil {
		p.Log().Warn("failed to get recommendations", "error", err)
		// Return empty list on error instead of failing
		items = []sdk.MediaItem{}
	}

	return jsonResponse(http.StatusOK, map[string]any{
		"title": "Recommended For You",
		"items": items,
	})
}

func (p *RecommendationsPlugin) handleBecauseYouLiked(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	p.mu.RLock()
	recService := p.recommendationsService
	config := p.config
	p.mu.RUnlock()

	if !config.Enabled {
		return jsonResponse(http.StatusOK, map[string]any{
			"title": "Because You Liked...",
			"items": []any{},
		})
	}

	if recService == nil {
		return jsonResponse(http.StatusServiceUnavailable, map[string]string{"error": "recommendations service not available"})
	}

	limit := 20
	if limitStr := req.Query["limit"]; limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	row, err := recService.GetBecauseYouLiked(ctx, req.UserID, limit)
	if err != nil {
		p.Log().Warn("failed to get 'because you liked' recommendations", "error", err)
		return jsonResponse(http.StatusOK, map[string]any{
			"title": "Because You Liked...",
			"items": []any{},
		})
	}

	return jsonResponse(http.StatusOK, row)
}

// =============================================================================
// Helpers
// =============================================================================

func jsonResponse(statusCode int, data any) (*sdk.HTTPResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return &sdk.HTTPResponse{
			StatusCode:  http.StatusInternalServerError,
			ContentType: "application/json",
			Body:        []byte(`{"error":"failed to serialize response"}`),
		}, nil
	}

	return &sdk.HTTPResponse{
		StatusCode:  statusCode,
		ContentType: "application/json",
		Body:        body,
	}, nil
}
