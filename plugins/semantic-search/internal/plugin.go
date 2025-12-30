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

// SemanticSearchPlugin implements the Semantic Search plugin.
// It embeds sdk.Base to satisfy the sdk.VectorSearchEnricherPlugin interface.
type SemanticSearchPlugin struct {
	sdk.Base // Provides mustEmbedBase(), logging, metrics, data dir

	config Config

	// Host service clients (set during Initialize via HostServices)
	plugins *sdk.PluginsClient // For capability broker (embedding provider)
	storage *sdk.StorageClient // For SQL and vector storage
	vector  *sdk.VectorClient  // For vector storage (convenience)
	sql     *sdk.SQLClient     // For SQL storage (mood tags, etc.)
	data    *sdk.DataClient    // For media data access
	weather *sdk.WeatherClient // For weather/location context

	// Services
	embeddingService *EmbeddingService
	searchService    *SearchService
	indexingService  *IndexingService
	moodTagService   *MoodTagService
	contextEnricher  *ContextEnricher
	queryRewriter    *QueryRewriter

	mu sync.RWMutex
}

// Compile-time check that SemanticSearchPlugin implements sdk.VectorSearchEnricherPlugin
var _ sdk.VectorSearchEnricherPlugin = (*SemanticSearchPlugin)(nil)

// NewSemanticSearchPlugin creates a new Semantic Search plugin instance.
func NewSemanticSearchPlugin(logger *slog.Logger) *SemanticSearchPlugin {
	p := &SemanticSearchPlugin{}
	p.SetLogger(logger)
	return p
}

// =============================================================================
// sdk.EnricherPlugin implementation
// =============================================================================

// GetCapabilities returns what this enricher provides and requires.
func (p *SemanticSearchPlugin) GetCapabilities() sdk.EnricherCapabilities {
	return sdk.EnricherCapabilities{
		MediaTypes: []string{"movie", "tv", "tv_show", "tv_episode", "music_artist", "music_album", "music_track"},
		Provides:   []string{"embedding"},
		IsLocal:    false,
		RateLimit:  0,                    // No rate limit for indexing
		Requires:   []string{"metadata"}, // Needs metadata first
		Priority:   99,                   // Run last
	}
}

// Initialize is called when the plugin is loaded.
func (p *SemanticSearchPlugin) Initialize(ctx context.Context, dataDir string, config []byte, services *sdk.HostServices) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Initialize base (sets dataDir)
	p.Base.Init(dataDir)

	p.Log().Debug("initializing Semantic Search plugin", "data_dir", dataDir)

	// Parse config from YAML
	if len(config) == 0 {
		// Use defaults
		p.config = Config{
			Indexing: IndexingConfig{
				BatchSize:     50,
				AutoIndex:     true,
				StagePosition: 99,
			},
			Search: SearchConfig{
				DefaultLimit:  20,
				MaxLimit:      100,
				MinSimilarity: 0.3,
			},
			MoodTags: MoodTagConfig{
				Enabled: false,
			},
		}
	} else {
		if err := yaml.Unmarshal(config, &p.config); err != nil {
			return fmt.Errorf("failed to parse config.yml: %w", err)
		}
	}

	// Store host service clients
	if services != nil {
		p.plugins = services.Plugins
		if services.Storage != nil {
			p.storage = services.Storage
			p.vector = services.Storage.Vector()
			p.sql = services.Storage.SQL()
		}
		p.data = services.Data
		p.weather = services.Weather
	}

	// Run database migrations for mood tags table
	if err := p.runMigrations(ctx); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Initialize services
	p.initializeServices()

	p.Log().Debug("Semantic Search plugin initialized",
		"auto_index", p.config.Indexing.AutoIndex,
		"mood_tags_enabled", p.config.MoodTags.Enabled,
	)

	return nil
}

// runMigrations runs database migrations for plugin-owned tables.
func (p *SemanticSearchPlugin) runMigrations(ctx context.Context) error {
	if p.sql == nil {
		p.Log().Warn("SQL client not available, skipping migrations")
		return nil
	}

	migrations := []sdk.Migration{
		{
			Version: 1,
			SQL: `CREATE TABLE IF NOT EXISTS mood_tags (
				id INTEGER PRIMARY KEY,
				entity_type TEXT NOT NULL,
				entity_id INTEGER NOT NULL,
				tag TEXT NOT NULL,
				confidence REAL DEFAULT 1.0,
				created_at TEXT DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(entity_type, entity_id, tag)
			);
			CREATE INDEX IF NOT EXISTS idx_mood_tags_entity ON mood_tags(entity_type, entity_id);
			CREATE INDEX IF NOT EXISTS idx_mood_tags_tag ON mood_tags(tag)`,
		},
	}

	if err := p.sql.Migrate(ctx, migrations); err != nil {
		return fmt.Errorf("migrate mood_tags: %w", err)
	}

	p.Log().Debug("database migrations completed")
	return nil
}

// GetSettingsSchema returns the JSON Schema for plugin settings.
func (p *SemanticSearchPlugin) GetSettingsSchema() ([]byte, error) {
	return SettingsSchema().Build()
}

// Configure applies new settings to the plugin.
func (p *SemanticSearchPlugin) Configure(settings []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Parse the flat settings from the UI
	var flat struct {
		AutoIndex               bool    `json:"auto_index"`
		ReindexOnMetadataChange bool    `json:"reindex_on_metadata_change"`
		BatchSize               int     `json:"batch_size"`
		DefaultLimit            int     `json:"default_limit"`
		MinSimilarity           float32 `json:"min_similarity"`
		MoodTagsEnabled         bool    `json:"mood_tags_enabled"`
	}
	if err := json.Unmarshal(settings, &flat); err != nil {
		return fmt.Errorf("failed to parse settings: %w", err)
	}

	// Apply settings to config
	p.config.Indexing.AutoIndex = flat.AutoIndex
	p.config.Indexing.ReindexOnMetadataChange = flat.ReindexOnMetadataChange
	if flat.BatchSize > 0 {
		p.config.Indexing.BatchSize = flat.BatchSize
	}
	if flat.DefaultLimit > 0 {
		p.config.Search.DefaultLimit = flat.DefaultLimit
	}
	if flat.MinSimilarity > 0 {
		p.config.Search.MinSimilarity = flat.MinSimilarity
	}
	p.config.MoodTags.Enabled = flat.MoodTagsEnabled

	p.Log().Debug("configuration updated",
		"auto_index", p.config.Indexing.AutoIndex,
		"reindex_on_metadata_change", p.config.Indexing.ReindexOnMetadataChange,
		"batch_size", p.config.Indexing.BatchSize,
		"default_limit", p.config.Search.DefaultLimit,
		"min_similarity", p.config.Search.MinSimilarity,
		"mood_tags_enabled", p.config.MoodTags.Enabled,
	)

	// Reinitialize services if mood tags setting changed
	// (mood tag service is only created when enabled)
	p.initializeServices()

	return nil
}

// Shutdown is called before the plugin is unloaded.
func (p *SemanticSearchPlugin) Shutdown(ctx context.Context) error {
	p.Log().Debug("shutting down Semantic Search plugin")

	p.mu.RLock()
	if p.indexingService != nil {
		p.indexingService.Cancel()
	}
	if p.moodTagService != nil {
		p.moodTagService.Cancel()
	}
	p.mu.RUnlock()

	return nil
}

// Enrich processes a single media item.
func (p *SemanticSearchPlugin) Enrich(ctx context.Context, req *sdk.EnrichRequest) (*sdk.EnrichResponse, error) {
	p.mu.Lock()
	p.RecordRequest(0) // We'll record actual latency at the end
	indexingService := p.indexingService
	autoIndex := p.config.Indexing.AutoIndex
	p.mu.Unlock()

	// Check if auto-indexing is enabled
	if !autoIndex {
		return sdk.Skip("auto-indexing disabled"), nil
	}

	// Check if indexing service is available
	if indexingService == nil {
		p.RecordError()
		return sdk.Skip("indexing service not initialized"), nil
	}

	p.Log().Debug("indexing media",
		"media_id", req.MediaID,
		"media_type", req.MediaType,
		"title", req.Title,
	)

	// Convert to our entity type
	entityType := EntityType(req.MediaType)

	// Index the media
	if err := indexingService.IndexSingle(ctx, entityType, req.MediaID, nil); err != nil {
		p.RecordError()
		p.Log().Warn("failed to index media", "id", req.MediaID, "error", err)
		return sdk.Skip(fmt.Sprintf("indexing failed: %v", err)), nil
	}

	return &sdk.EnrichResponse{
		Matched: true,
		// No metadata changes - we just indexed it
	}, nil
}

// =============================================================================
// sdk.AISearchPlugin implementation
// =============================================================================

// Search performs semantic search across indexed media.
func (p *SemanticSearchPlugin) Search(ctx context.Context, req *sdk.SemanticSearchRequest) (*sdk.SemanticSearchResponse, error) {
	p.mu.Lock()
	searchService := p.searchService
	contextEnricher := p.contextEnricher
	queryRewriter := p.queryRewriter
	p.mu.Unlock()

	if searchService == nil {
		return nil, fmt.Errorf("search service not initialized")
	}

	// Start with the original query
	query := req.Query

	// Use LLM to rewrite query for better intent matching
	if queryRewriter != nil {
		rewrittenQuery := queryRewriter.Rewrite(ctx, query)
		if rewrittenQuery != query {
			p.Log().Debug("LLM rewrote query",
				"original", query,
				"rewritten", rewrittenQuery,
			)
			query = rewrittenQuery
		}
	}

	// Enrich the query with context if user ID is provided
	if contextEnricher != nil && req.UserID != "" {
		qc, err := contextEnricher.GetContext(ctx, req.UserID)
		if err != nil {
			p.Log().Debug("failed to get context for query enrichment", "error", err)
		} else {
			enrichedQuery := contextEnricher.EnrichQuery(query, qc)
			if enrichedQuery != query {
				p.Log().Debug("enriched search query",
					"original", query,
					"enriched", enrichedQuery,
					"has_weather", qc.Weather != nil && qc.Weather.Available,
					"season", qc.Season,
					"time_of_day", qc.TimeOfDay,
				)
				query = enrichedQuery
			}
		}
	}

	// Convert entity types
	entityTypes := make([]EntityType, len(req.EntityTypes))
	for i, t := range req.EntityTypes {
		entityTypes[i] = EntityType(t)
	}

	results, err := searchService.Search(ctx, SearchParams{
		Query:       query,
		EntityTypes: entityTypes,
		Limit:       req.Limit,
	})
	if err != nil {
		p.RecordError()
		return nil, err
	}

	// Convert results to SDK types
	sdkResults := make([]sdk.SemanticSearchResult, len(results))
	for i, r := range results {
		sdkResults[i] = sdk.SemanticSearchResult{
			EntityType: string(r.EntityType),
			EntityID:   r.EntityID,
			Similarity: r.Similarity,
			Text:       r.Text,
		}
	}

	return &sdk.SemanticSearchResponse{
		Results: sdkResults,
		Total:   len(sdkResults),
	}, nil
}

// FindSimilar finds items similar to a given entity.
func (p *SemanticSearchPlugin) FindSimilar(ctx context.Context, req *sdk.FindSimilarRequest) (*sdk.SemanticSearchResponse, error) {
	p.mu.RLock()
	searchService := p.searchService
	p.mu.RUnlock()

	if searchService == nil {
		return nil, fmt.Errorf("search service not initialized")
	}

	results, err := searchService.FindSimilar(ctx, EntityType(req.EntityType), req.EntityID, req.Limit)
	if err != nil {
		p.RecordError()
		return nil, err
	}

	// Convert results to SDK types
	sdkResults := make([]sdk.SemanticSearchResult, len(results))
	for i, r := range results {
		sdkResults[i] = sdk.SemanticSearchResult{
			EntityType: string(r.EntityType),
			EntityID:   r.EntityID,
			Similarity: r.Similarity,
			Text:       r.Text,
		}
	}

	return &sdk.SemanticSearchResponse{
		Results: sdkResults,
		Total:   len(sdkResults),
	}, nil
}

// GetStatus returns the current indexing status and statistics.
func (p *SemanticSearchPlugin) GetStatus(ctx context.Context) (*sdk.VectorSearchStatus, error) {
	p.mu.RLock()
	searchService := p.searchService
	indexingService := p.indexingService
	p.mu.RUnlock()

	status := &sdk.VectorSearchStatus{}

	// Get indexing progress
	if indexingService != nil {
		status.IsIndexing = indexingService.IsIndexing()
		if progress := indexingService.GetProgress(); progress != nil {
			status.Progress = &sdk.IndexingProgress{
				EntityType:  string(progress.EntityType),
				Total:       progress.Total,
				Processed:   progress.Processed,
				Failed:      progress.Failed,
				LastError:   progress.LastError,
				StartedAt:   progress.StartedAt,
				LastUpdated: progress.LastUpdated,
			}
		}
	}

	// Get stats per entity type
	if searchService != nil {
		indexStatus, err := searchService.GetStatus(ctx)
		if err == nil && indexStatus != nil {
			for entityType, stats := range indexStatus.Stats {
				status.Stats = append(status.Stats, sdk.EntityTypeStats{
					EntityType: string(entityType),
					Indexed:    stats.Indexed,
					Total:      stats.Total,
				})
				status.TotalIndexed += stats.Indexed
			}
		}
	}

	return status, nil
}

// IndexLibrary triggers indexing for all media in a library.
func (p *SemanticSearchPlugin) IndexLibrary(ctx context.Context, req *sdk.IndexLibraryRequest) (*sdk.IndexLibraryResponse, error) {
	p.mu.RLock()
	indexingService := p.indexingService
	p.mu.RUnlock()

	if indexingService == nil {
		return &sdk.IndexLibraryResponse{
			Started: false,
			Message: "indexing service not initialized",
		}, nil
	}

	// Start indexing in background
	go func() {
		if err := indexingService.IndexLibrary(context.Background(), req.LibraryID, req.LibraryType); err != nil {
			p.Log().Error("library indexing failed", "library_id", req.LibraryID, "error", err)
		}
	}()

	return &sdk.IndexLibraryResponse{
		Started: true,
		Message: fmt.Sprintf("started indexing library %d", req.LibraryID),
	}, nil
}

// CancelIndexing cancels any running indexing operation.
func (p *SemanticSearchPlugin) CancelIndexing(ctx context.Context) error {
	p.mu.RLock()
	indexingService := p.indexingService
	p.mu.RUnlock()

	if indexingService != nil {
		indexingService.Cancel()
	}

	return nil
}

// =============================================================================
// sdk.HTTPEnricher implementation (optional)
// =============================================================================

// GetRoutes returns the HTTP routes this plugin provides.
func (p *SemanticSearchPlugin) GetRoutes() []sdk.Route {
	return []sdk.Route{
		{
			Path:        "/search",
			Methods:     []string{"POST"},
			AdminOnly:   false,
			Description: "Perform semantic search across indexed media",
			AliasPath:   "/api/search",
		},
		{
			Path:        "/similar",
			Methods:     []string{"POST"},
			AdminOnly:   false,
			Description: "Find items similar to a given entity",
		},
		{
			Path:        "/status",
			Methods:     []string{"GET"},
			AdminOnly:   false,
			Description: "Get current indexing status and stats",
		},
		{
			Path:        "/index",
			Methods:     []string{"POST"},
			AdminOnly:   true,
			Description: "Trigger indexing for a library",
		},
		{
			Path:        "/index/cancel",
			Methods:     []string{"POST"},
			AdminOnly:   true,
			Description: "Cancel running indexing operation",
		},
		{
			Path:        "/index/estimate",
			Methods:     []string{"GET"},
			AdminOnly:   false,
			Description: "Estimate cost for indexing a library",
		},
		{
			Path:        "/index/clear",
			Methods:     []string{"DELETE"},
			AdminOnly:   true,
			Description: "Clear all embeddings from the index",
		},
		{
			Path:        "/mood-tags/generate",
			Methods:     []string{"POST"},
			AdminOnly:   true,
			Description: "Generate mood tags for a library",
		},
		{
			Path:        "/mood-tags/status",
			Methods:     []string{"GET"},
			AdminOnly:   false,
			Description: "Get mood tag generation status",
		},
		{
			Path:        "/mood-tags/cancel",
			Methods:     []string{"POST"},
			AdminOnly:   true,
			Description: "Cancel mood tag generation",
		},
	}
}

// HandleHTTP handles a non-streaming HTTP request.
func (p *SemanticSearchPlugin) HandleHTTP(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	p.Log().Debug("handling HTTP request", "path", req.Path, "method", req.Method)

	switch req.Path {
	case "/search":
		return p.handleSearch(ctx, req)
	case "/similar":
		return p.handleSimilar(ctx, req)
	case "/status":
		return p.handleStatus(ctx, req)
	case "/index":
		return p.handleIndex(ctx, req)
	case "/index/cancel":
		return p.handleCancelIndex(ctx, req)
	case "/index/estimate":
		return p.handleEstimate(ctx, req)
	case "/index/clear":
		return p.handleClearIndex(ctx, req)
	case "/mood-tags/generate":
		return p.handleMoodTagsGenerate(ctx, req)
	case "/mood-tags/status":
		return p.handleMoodTagsStatus(ctx, req)
	case "/mood-tags/cancel":
		return p.handleMoodTagsCancel(ctx, req)
	default:
		return jsonResponse(http.StatusNotFound, map[string]string{
			"error": "route not found",
			"path":  req.Path,
		})
	}
}

// =============================================================================
// Internal helper methods
// =============================================================================

func (p *SemanticSearchPlugin) initializeServices() {
	logger := p.Log()

	// Create embedding service (uses capability broker)
	if p.plugins != nil {
		p.embeddingService = NewEmbeddingService(p.plugins, logger)
	}

	// Create search service (uses vector storage)
	if p.embeddingService != nil && p.vector != nil {
		p.searchService = NewSearchService(
			p.embeddingService,
			p.vector,
			p.config.Search,
			logger,
		)
	}

	// Create indexing service (uses vector storage)
	if p.embeddingService != nil && p.vector != nil && p.data != nil {
		p.indexingService = NewIndexingService(
			p.embeddingService,
			p.vector,
			p.data,
			p.config.Indexing,
			logger,
		)
	}

	// Create mood tag service (if enabled)
	// Note: MoodTagService needs chat capability - disabled for now until refactored
	if p.config.MoodTags.Enabled && p.plugins != nil && p.data != nil {
		p.moodTagService = NewMoodTagService(
			p.plugins,
			p.data,
			logger,
		)
	}

	// Create context enricher for location-aware search
	p.contextEnricher = NewContextEnricher(p.weather, logger)

	// Create query rewriter (uses chat capability)
	// Note: QueryRewriter needs chat capability - disabled for now until refactored
	p.queryRewriter = NewQueryRewriter(p.plugins, logger)
}

// =============================================================================
// HTTP Handlers
// =============================================================================

func (p *SemanticSearchPlugin) handleSearch(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	if req.Method != "POST" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	var searchReq struct {
		Query       string   `json:"query"`
		EntityTypes []string `json:"entity_types,omitempty"`
		Limit       int      `json:"limit,omitempty"`
	}
	if err := json.Unmarshal(req.Body, &searchReq); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	resp, err := p.Search(ctx, &sdk.SemanticSearchRequest{
		Query:       searchReq.Query,
		EntityTypes: searchReq.EntityTypes,
		Limit:       searchReq.Limit,
		UserID:      req.UserID,
	})
	if err != nil {
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return jsonResponse(http.StatusOK, resp)
}

func (p *SemanticSearchPlugin) handleSimilar(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	if req.Method != "POST" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	var similarReq struct {
		EntityType string `json:"entity_type"`
		EntityID   int64  `json:"entity_id"`
		Limit      int    `json:"limit,omitempty"`
	}
	if err := json.Unmarshal(req.Body, &similarReq); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	resp, err := p.FindSimilar(ctx, &sdk.FindSimilarRequest{
		EntityType: similarReq.EntityType,
		EntityID:   similarReq.EntityID,
		Limit:      similarReq.Limit,
	})
	if err != nil {
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return jsonResponse(http.StatusOK, resp)
}

func (p *SemanticSearchPlugin) handleStatus(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	if req.Method != "GET" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	status, err := p.GetStatus(ctx)
	if err != nil {
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return jsonResponse(http.StatusOK, status)
}

func (p *SemanticSearchPlugin) handleIndex(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	if req.Method != "POST" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	var indexReq struct {
		LibraryID   int64  `json:"library_id"`
		LibraryType string `json:"library_type"`
	}
	if err := json.Unmarshal(req.Body, &indexReq); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	resp, err := p.IndexLibrary(ctx, &sdk.IndexLibraryRequest{
		LibraryID:   indexReq.LibraryID,
		LibraryType: indexReq.LibraryType,
	})
	if err != nil {
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	status := http.StatusOK
	if !resp.Started {
		status = http.StatusBadRequest
	}

	return jsonResponse(status, resp)
}

func (p *SemanticSearchPlugin) handleCancelIndex(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	if req.Method != "POST" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	if err := p.CancelIndexing(ctx); err != nil {
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return jsonResponse(http.StatusOK, map[string]string{"status": "cancelled"})
}

func (p *SemanticSearchPlugin) handleEstimate(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	if req.Method != "GET" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	p.mu.RLock()
	dataClient := p.data
	p.mu.RUnlock()

	if dataClient == nil {
		return jsonResponse(http.StatusServiceUnavailable, map[string]string{"error": "data client not available"})
	}

	libraryIDStr := req.Query["library_id"]
	if libraryIDStr == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "library_id is required"})
	}

	var libraryID int64
	if _, err := fmt.Sscanf(libraryIDStr, "%d", &libraryID); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "invalid library_id"})
	}

	library, err := dataClient.GetLibrary(ctx, libraryID)
	if err != nil {
		return jsonResponse(http.StatusNotFound, map[string]string{"error": "library not found"})
	}

	// Count items in the library
	var totalItems int64
	offset := 0
	limit := 1000
	for {
		mediaList, err := dataClient.ListMediaByLibrary(ctx, libraryID, limit, offset)
		if err != nil {
			break
		}
		totalItems += int64(len(mediaList.Items))
		if !mediaList.HasMore {
			break
		}
		offset += limit
	}

	tokensPerItem := estimateTokensPerItem(library.MediaType)
	estimatedTokens := totalItems * int64(tokensPerItem)
	estimatedCostUSD := float64(estimatedTokens) * 0.02 / 1_000_000

	response := map[string]any{
		"library_id":   libraryID,
		"library_name": library.Name,
		"library_type": library.MediaType,
		"items": map[string]int64{
			library.MediaType: totalItems,
		},
		"estimated_tokens": estimatedTokens,
		"estimated_cost": map[string]any{
			"embeddings_usd": estimatedCostUSD,
			"total_usd":      estimatedCostUSD,
			"disclaimer":     "Approximate estimate. Actual costs depend on provider configured in Settings > AI. Local providers (Ollama) are free.",
		},
		"provider": map[string]any{
			"note": "Provider is configured in ViewRA Settings > AI",
		},
	}

	return jsonResponse(http.StatusOK, response)
}

func (p *SemanticSearchPlugin) handleClearIndex(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	if req.Method != "DELETE" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	p.mu.RLock()
	vectorClient := p.vector
	p.mu.RUnlock()

	if vectorClient == nil {
		return jsonResponse(http.StatusServiceUnavailable, map[string]string{"error": "vector client not available"})
	}

	entityTypes := []string{"movie", "tv", "tv_show", "tv_episode", "music_artist", "music_album", "music_track"}
	var totalDeleted int64

	for _, entityType := range entityTypes {
		count, err := vectorClient.DeleteByType(ctx, entityType)
		if err != nil {
			p.Log().Warn("failed to delete embeddings", "type", entityType, "error", err)
			continue
		}
		totalDeleted += count
	}

	return jsonResponse(http.StatusOK, map[string]any{
		"status":        "cleared",
		"total_deleted": totalDeleted,
	})
}

func (p *SemanticSearchPlugin) handleMoodTagsGenerate(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	if req.Method != "POST" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	p.mu.RLock()
	moodTagService := p.moodTagService
	embeddingService := p.embeddingService
	vectorClient := p.vector
	dataClient := p.data
	sqlClient := p.sql
	logger := p.Log()
	p.mu.RUnlock()

	if moodTagService == nil {
		return jsonResponse(http.StatusServiceUnavailable, map[string]string{
			"error":   "mood tag service not available",
			"details": "mood tag generation is disabled in config or chat provider not connected",
		})
	}

	var genReq struct {
		LibraryID int64 `json:"library_id"`
	}
	if err := json.Unmarshal(req.Body, &genReq); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if genReq.LibraryID == 0 {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "library_id is required"})
	}

	logger.Info("handleMoodTagsGenerate called", "library_id", genReq.LibraryID)
	libraryID := genReq.LibraryID

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic in mood tag generation", "panic", r, "library_id", libraryID)
			}
		}()

		logger.Info("mood tag generation goroutine started", "library_id", libraryID)

		callback := func(entityType EntityType, entityID int64, tags []string) error {
			if len(tags) == 0 {
				return nil
			}

			// Store mood tags in plugin-owned table
			if sqlClient != nil {
				ctx := context.Background()
				// Delete existing tags for this entity
				_, _, _ = sqlClient.Exec(ctx, `DELETE FROM mood_tags WHERE entity_type = ? AND entity_id = ?`,
					string(entityType), entityID)
				// Insert new tags
				for _, tag := range tags {
					_, _, err := sqlClient.Exec(ctx,
						`INSERT INTO mood_tags (entity_type, entity_id, tag, confidence) VALUES (?, ?, ?, ?)`,
						string(entityType), entityID, tag, 1.0)
					if err != nil {
						logger.Warn("failed to persist mood tag", "entity_id", entityID, "tag", tag, "error", err)
					}
				}
			}

			details, err := dataClient.GetMediaDetails(context.Background(), entityID, string(entityType))
			if err != nil {
				return fmt.Errorf("get media details: %w", err)
			}

			text := buildTextWithMoodTagsSDK(details, tags)

			embedding, err := embeddingService.EmbedSingle(context.Background(), text)
			if err != nil {
				return fmt.Errorf("generate embedding: %w", err)
			}

			return vectorClient.Store(context.Background(), sdk.Embedding{
				EntityType: string(entityType),
				EntityID:   entityID,
				Vector:     embedding,
				Text:       text,
			})
		}

		logger.Debug("calling GenerateForLibrary", "library_id", libraryID)
		if err := moodTagService.GenerateForLibrary(context.Background(), libraryID, callback); err != nil {
			logger.Error("mood tag generation failed", "library_id", libraryID, "error", err)
		}
		logger.Info("mood tag generation completed", "library_id", libraryID)
	}()

	return jsonResponse(http.StatusOK, map[string]any{
		"started":    true,
		"library_id": genReq.LibraryID,
		"message":    "mood tag generation started in background",
	})
}

func (p *SemanticSearchPlugin) handleMoodTagsStatus(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	if req.Method != "GET" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	p.mu.RLock()
	moodTagService := p.moodTagService
	config := p.config
	p.mu.RUnlock()

	status := map[string]any{
		"enabled": config.MoodTags.Enabled,
	}

	if moodTagService == nil {
		status["available"] = false
		status["reason"] = "mood tag service not initialized"
	} else {
		status["available"] = true
		status["is_generating"] = moodTagService.IsGenerating()
		if progress := moodTagService.GetProgress(); progress != nil {
			status["progress"] = progress
		}
	}

	return jsonResponse(http.StatusOK, status)
}

func (p *SemanticSearchPlugin) handleMoodTagsCancel(ctx context.Context, req *sdk.HTTPRequest) (*sdk.HTTPResponse, error) {
	if req.Method != "POST" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	p.mu.RLock()
	moodTagService := p.moodTagService
	p.mu.RUnlock()

	if moodTagService != nil {
		moodTagService.Cancel()
	}

	return jsonResponse(http.StatusOK, map[string]string{"status": "cancelled"})
}

// =============================================================================
// Utility functions
// =============================================================================

func estimateTokensPerItem(mediaType string) int {
	switch mediaType {
	case "movie", "movies":
		return 250
	case "tv", "tv_show":
		return 200
	case "tv_episode":
		return 100
	case "music_artist":
		return 60
	case "music_album":
		return 80
	case "music_track", "music":
		return 50
	default:
		return 100
	}
}

func buildTextWithMoodTagsSDK(details *sdk.MediaDetails, tags []string) string {
	if details == nil {
		return ""
	}

	var text string
	switch details.MediaType {
	case "movie":
		text = fmt.Sprintf("Title: %s (%d)\n", details.Title, details.Year)
		if len(details.Genres) > 0 {
			text += fmt.Sprintf("Genre: %s\n", joinStrings(details.Genres, ", "))
		}
		if details.Plot != "" {
			text += fmt.Sprintf("Plot: %s\n", details.Plot)
		}
		if len(details.Directors) > 0 {
			text += fmt.Sprintf("Directed by: %s\n", joinStrings(details.Directors, ", "))
		}
	case "tv_show":
		text = fmt.Sprintf("Title: %s (%d)\n", details.Title, details.Year)
		if len(details.Genres) > 0 {
			text += fmt.Sprintf("Genre: %s\n", joinStrings(details.Genres, ", "))
		}
		if details.Plot != "" {
			text += fmt.Sprintf("Plot: %s\n", details.Plot)
		}
	default:
		text = fmt.Sprintf("Title: %s\n", details.Title)
		if details.Plot != "" {
			text += fmt.Sprintf("Description: %s\n", details.Plot)
		}
	}

	if len(tags) > 0 {
		text += fmt.Sprintf("Mood: %s\n", joinStrings(tags, ", "))
	}

	return text
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

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
