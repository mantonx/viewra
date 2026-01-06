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
	"github.com/mantonx/viewra/plugins/recommendations/internal/cf"
	"gopkg.in/yaml.v3"
)

// RecommendationsPlugin implements the recommendations plugin.
type RecommendationsPlugin struct {
	sdk.Base

	config Config

	// Host service clients
	data     *sdk.DataClient
	plugins  *sdk.PluginsClient
	ratings  *sdk.RatingsClient
	progress *sdk.ProgressClient

	// Services
	recommendationsService *RecommendationsService
	userEmbeddingService   *UserEmbeddingService
	sarService             *cf.SARService
	hybridScorer           *HybridScorer
	themedRecommendations  *ThemedRecommendations

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
		p.progress = services.Progress
	}

	// Initialize services
	p.initializeServices()

	// Build SAR model in background (non-blocking)
	if p.sarService != nil {
		go func() {
			if err := p.sarService.BuildModel(ctx); err != nil {
				p.Log().Warn("failed to build SAR model on startup", "error", err)
			} else {
				users, items := p.sarService.Stats()
				p.Log().Debug("SAR model ready", "users", users, "items", items)
			}
		}()
	}

	p.Log().Debug("Recommendations plugin initialized", "enabled", p.config.Enabled)

	return nil
}

// initializeServices creates the internal service instances.
func (p *RecommendationsPlugin) initializeServices() {
	// Create user embedding service for taste-based recommendations
	if p.plugins != nil {
		p.userEmbeddingService = NewUserEmbeddingService(
			p.ratings,
			p.progress,
			p.plugins,
			p.data,
			p.Log(),
		)
	}

	// Create SAR service for collaborative filtering
	if p.ratings != nil || p.progress != nil {
		p.sarService = cf.NewSARService(
			p.ratings,
			p.progress,
			p.data,
			p.Log(),
		)
	}

	// Create hybrid scorer
	p.hybridScorer = NewHybridScorer(
		p.sarService,
		p.userEmbeddingService,
		p.ratings,
		p.progress,
		p.data,
		p.plugins,
		HybridWeights{
			Collaborative: p.config.CollaborativeWeight,
			Semantic:      p.config.SemanticWeight,
			Exploration:   p.config.ExplorationWeight,
		},
		p.Log(),
	)

	// Create recommendations service
	if p.data != nil && p.ratings != nil {
		p.recommendationsService = NewRecommendationsService(
			p.ratings,
			p.data,
			p.plugins,
			p.userEmbeddingService,
			p.sarService,
			p.hybridScorer,
			p.config,
			p.Log(),
		)
	}

	// Create themed recommendations service
	if p.data != nil {
		p.themedRecommendations = NewThemedRecommendations(p.data, p.Log())
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
		Enabled             bool `json:"enabled"`
		MaxRecommendations  int  `json:"max_recommendations"`
		SimilarWeight       int  `json:"similar_weight"`
		FavoriteWeight      int  `json:"favorite_weight"`
		UseHybridScoring    bool `json:"use_hybrid_scoring"`
		CollaborativeWeight int  `json:"collaborative_weight"`
		SemanticWeight      int  `json:"semantic_weight"`
		ExplorationWeight   int  `json:"exploration_weight"`
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
	p.config.UseHybridScoring = cfg.UseHybridScoring
	p.config.CollaborativeWeight = cfg.CollaborativeWeight
	p.config.SemanticWeight = cfg.SemanticWeight
	p.config.ExplorationWeight = cfg.ExplorationWeight

	// Update recommendations service config
	if p.recommendationsService != nil {
		p.recommendationsService.UpdateConfig(p.config)
	}

	// Update hybrid scorer weights
	if p.hybridScorer != nil {
		p.hybridScorer.UpdateWeights(HybridWeights{
			Collaborative: cfg.CollaborativeWeight,
			Semantic:      cfg.SemanticWeight,
			Exploration:   cfg.ExplorationWeight,
		})
	}

	p.Log().Debug("configuration updated",
		"enabled", p.config.Enabled,
		"max_recommendations", p.config.MaxRecommendations,
		"use_hybrid_scoring", p.config.UseHybridScoring,
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
		// Themed recommendations (time-of-day, seasonal, curated)
		{
			Path:        "/themed",
			Methods:     []string{"GET"},
			Description: "Get active themed recommendation rows",
		},
		{
			Path:        "/themed/row",
			Methods:     []string{"GET"},
			Description: "Get items for a specific themed row",
		},
		// Debug endpoint
		{
			Path:        "/debug",
			Methods:     []string{"GET"},
			Description: "Debug information",
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
	case req.Path == "/themed":
		return p.handleThemedList(ctx, req)
	case req.Path == "/themed/row":
		return p.handleThemedRow(ctx, req)
	case req.Path == "/debug":
		return p.handleDebug(ctx, req)
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

func (p *RecommendationsPlugin) handleDebug(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	p.mu.RLock()
	config := p.config
	hasRatings := p.ratings != nil
	hasData := p.data != nil
	hasPlugins := p.plugins != nil
	hasProgress := p.progress != nil
	hasRecService := p.recommendationsService != nil
	hasHybrid := p.hybridScorer != nil
	p.mu.RUnlock()

	debug := map[string]any{
		"user_id":                 req.UserID,
		"config_enabled":          config.Enabled,
		"config_use_hybrid":       config.UseHybridScoring,
		"has_ratings_client":      hasRatings,
		"has_data_client":         hasData,
		"has_plugins_client":      hasPlugins,
		"has_progress_client":     hasProgress,
		"has_recommendations_svc": hasRecService,
		"has_hybrid_scorer":       hasHybrid,
	}

	// Test ratings client
	if p.ratings != nil {
		hasUserRatings, err := p.ratings.HasRatings(ctx, req.UserID)
		if err != nil {
			debug["ratings_has_error"] = err.Error()
		} else {
			debug["user_has_ratings"] = hasUserRatings
		}

		positiveIDs, err := p.ratings.GetPositivelyRatedIDs(ctx, req.UserID, "", 10)
		if err != nil {
			debug["positive_ids_error"] = err.Error()
		} else {
			debug["positive_ids"] = positiveIDs
		}
	}

	// Test data client
	if p.data != nil {
		items, err := p.data.ListMediaByGenre(ctx, "movie", "Action", 0, nil, 5)
		if err != nil {
			debug["list_genre_error"] = err.Error()
		} else {
			debug["list_genre_count"] = len(items)
			if len(items) > 0 {
				debug["list_genre_first"] = items[0].Title
			}
		}
	}

	return jsonResponse(http.StatusOK, debug)
}

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

func (p *RecommendationsPlugin) handleThemedList(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	p.mu.RLock()
	themedService := p.themedRecommendations
	config := p.config
	p.mu.RUnlock()

	if !config.Enabled {
		return jsonResponse(http.StatusOK, map[string]any{
			"rows": []any{},
		})
	}

	if themedService == nil {
		return jsonResponse(http.StatusServiceUnavailable, map[string]string{"error": "themed recommendations not available"})
	}

	// Get active themed rows based on current context (time, season, etc.)
	activeRows := themedService.GetActiveThemedRows(ctx)

	// Convert to response format
	rows := make([]map[string]any, 0, len(activeRows))
	for _, row := range activeRows {
		rows = append(rows, map[string]any{
			"id":       row.ID,
			"title":    row.Title,
			"subtitle": row.Subtitle,
			"priority": row.Priority,
		})
	}

	return jsonResponse(http.StatusOK, map[string]any{
		"rows": rows,
	})
}

func (p *RecommendationsPlugin) handleThemedRow(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	p.mu.RLock()
	themedService := p.themedRecommendations
	ratingsClient := p.ratings
	config := p.config
	p.mu.RUnlock()

	if !config.Enabled {
		return jsonResponse(http.StatusOK, map[string]any{
			"title": "",
			"items": []any{},
		})
	}

	if themedService == nil {
		return jsonResponse(http.StatusServiceUnavailable, map[string]string{"error": "themed recommendations not available"})
	}

	// Get row ID from query
	rowID := req.Query["id"]
	if rowID == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "id parameter is required"})
	}

	limit := 20
	if limitStr := req.Query["limit"]; limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	// Find the matching themed row
	activeRows := themedService.GetActiveThemedRows(ctx)
	var targetRow *ThemedRow

	// Handle special "mood" meta-row - picks the first time-of-day row
	if rowID == "mood" {
		// Time-of-day rows have IDs like "late-night", "morning-picks", "evening-watch"
		moodRowIDs := map[string]bool{
			"late-night":    true,
			"morning-picks": true,
			"evening-watch": true,
			"weekend-epics": true,
		}
		for _, row := range activeRows {
			if moodRowIDs[row.ID] {
				targetRow = &row
				break
			}
		}
	} else {
		// Direct row ID lookup
		for _, row := range activeRows {
			if row.ID == rowID {
				targetRow = &row
				break
			}
		}
	}

	if targetRow == nil {
		// Return empty result instead of error for missing contextual rows
		return jsonResponse(http.StatusOK, map[string]any{
			"title": "Picks For You",
			"items": []any{},
		})
	}

	// Build exclude set from downvoted items
	excludeSet := make(map[int64]bool)
	if ratingsClient != nil {
		downvotedIDs, _ := ratingsClient.GetDownvotedIDs(ctx, req.UserID, "", 100)
		for _, id := range downvotedIDs {
			excludeSet[id] = true
		}
	}

	// Get recommendations for this themed row
	items, err := themedService.GetThemedRecommendations(ctx, *targetRow, excludeSet, limit)
	if err != nil {
		p.Log().Warn("failed to get themed recommendations", "row_id", rowID, "error", err)
		return jsonResponse(http.StatusOK, map[string]any{
			"title":    targetRow.Title,
			"subtitle": targetRow.Subtitle,
			"items":    []any{},
		})
	}

	return jsonResponse(http.StatusOK, map[string]any{
		"title":    targetRow.Title,
		"subtitle": targetRow.Subtitle,
		"items":    items,
	})
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
