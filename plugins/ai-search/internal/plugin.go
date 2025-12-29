package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/pkg/plugin/sdk"
	"gopkg.in/yaml.v3"
)

// AISearchPlugin implements the AI Search plugin.
type AISearchPlugin struct {
	pluginv1.UnimplementedPluginCoreServer
	pluginv1.UnimplementedEnricherServer
	pluginv1.UnimplementedAISearchServer

	logger  *slog.Logger
	dataDir string
	config  Config

	// Host service clients (SDK wrappers)
	llm        *sdk.LLMClient
	embeddings *sdk.EmbeddingsClient
	data       *sdk.DataClient
	weather    *sdk.WeatherClient

	// Services
	embeddingService *EmbeddingService
	searchService    *SearchService
	indexingService  *IndexingService
	moodTagService   *MoodTagService
	contextEnricher  *ContextEnricher
	queryRewriter    *QueryRewriter

	mu sync.RWMutex

	// Stats for health reporting
	requestsTotal int64
	errorsTotal   int64
}

// NewAISearchPlugin creates a new AI Search plugin instance.
func NewAISearchPlugin(logger *slog.Logger) *AISearchPlugin {
	return &AISearchPlugin{
		logger: logger,
	}
}

// SetLLMClient sets the host LLM client.
func (p *AISearchPlugin) SetLLMClient(client *sdk.LLMClient) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.llm = client
}

// SetEmbeddingsClient sets the host embeddings client.
func (p *AISearchPlugin) SetEmbeddingsClient(client *sdk.EmbeddingsClient) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.embeddings = client
}

// SetDataClient sets the host data client.
func (p *AISearchPlugin) SetDataClient(client *sdk.DataClient) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.data = client
}

// SetWeatherClient sets the host weather client for location-based context enrichment.
func (p *AISearchPlugin) SetWeatherClient(client *sdk.WeatherClient) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.weather = client
}

// PluginCore implementation

func (p *AISearchPlugin) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.dataDir = req.DataDir
	p.logger.Debug("initializing AI Search plugin",
		"host_version", req.HostVersion,
		"data_dir", req.DataDir,
	)

	// Parse config from YAML
	if len(req.Config) == 0 {
		// Use defaults - provider settings come from host AI settings
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
				Enabled: false, // Disabled by default
			},
		}
	} else {
		if err := yaml.Unmarshal(req.Config, &p.config); err != nil {
			return &pluginv1.InitResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to parse config.yml: %v", err),
			}, nil
		}
	}

	// Initialize services once we have all the clients
	p.initializeServices()

	p.logger.Debug("AI Search plugin initialized",
		"auto_index", p.config.Indexing.AutoIndex,
		"mood_tags_enabled", p.config.MoodTags.Enabled,
	)

	return &pluginv1.InitResponse{Success: true}, nil
}

func (p *AISearchPlugin) initializeServices() {
	// Create embedding service - uses host's configured embedding provider
	if p.llm != nil {
		p.embeddingService = NewEmbeddingService(p.llm, p.logger)
	}

	// Create search service
	if p.embeddingService != nil && p.embeddings != nil {
		p.searchService = NewSearchService(
			p.embeddingService,
			p.embeddings,
			p.config.Search,
			p.logger,
		)
	}

	// Create indexing service
	if p.embeddingService != nil && p.embeddings != nil && p.data != nil {
		p.indexingService = NewIndexingService(
			p.embeddingService,
			p.embeddings,
			p.data,
			p.config.Indexing,
			p.logger,
		)
	}

	// Create mood tag service (if enabled) - uses host's configured chat provider
	if p.config.MoodTags.Enabled && p.llm != nil && p.data != nil {
		p.moodTagService = NewMoodTagService(
			p.llm,
			p.data,
			p.logger,
		)
	}

	// Create context enricher for location-aware search (optional)
	// Works without weather client, just won't include weather/location context
	p.contextEnricher = NewContextEnricher(p.weather, p.logger)

	// Create query rewriter for intent-based query understanding
	// Uses the LLM to rewrite queries like "feeling sad" -> "uplifting happy"
	p.queryRewriter = NewQueryRewriter(p.llm, p.logger)
}

func (p *AISearchPlugin) Shutdown(ctx context.Context, req *pluginv1.Empty) (*pluginv1.Empty, error) {
	p.logger.Debug("shutting down AI Search plugin")

	// Cancel any running operations
	p.mu.RLock()
	if p.indexingService != nil {
		p.indexingService.Cancel()
	}
	if p.moodTagService != nil {
		p.moodTagService.Cancel()
	}
	p.mu.RUnlock()

	return &pluginv1.Empty{}, nil
}

func (p *AISearchPlugin) HealthCheck(ctx context.Context, req *pluginv1.Empty) (*pluginv1.HealthStatus, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	status := pluginv1.HealthStatus_HEALTHY
	message := "operational"

	// Check if services are available
	if p.llm == nil {
		status = pluginv1.HealthStatus_DEGRADED
		message = "LLM client not connected"
	} else if p.embeddings == nil {
		status = pluginv1.HealthStatus_DEGRADED
		message = "Embeddings client not connected"
	}

	return &pluginv1.HealthStatus{
		Status:        status,
		Message:       message,
		RequestsTotal: p.requestsTotal,
		ErrorsTotal:   p.errorsTotal,
	}, nil
}

func (p *AISearchPlugin) GetSettingsSchema(ctx context.Context, req *pluginv1.Empty) (*pluginv1.SettingsSchema, error) {
	// JSON Schema for plugin-specific settings
	// Provider settings come from the host AI settings
	schema := `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "indexing": {
      "type": "object",
      "title": "Indexing",
      "properties": {
        "batch_size": {
          "type": "integer",
          "title": "Batch Size",
          "description": "Number of items to process in each batch",
          "default": 50,
          "minimum": 1,
          "maximum": 500
        },
        "auto_index": {
          "type": "boolean",
          "title": "Auto Index",
          "description": "Automatically index new media in enrichment pipeline",
          "default": true
        }
      }
    },
    "search": {
      "type": "object",
      "title": "Search",
      "properties": {
        "default_limit": {
          "type": "integer",
          "title": "Default Results Limit",
          "description": "Default number of search results to return",
          "default": 20,
          "minimum": 1,
          "maximum": 100
        },
        "min_similarity": {
          "type": "number",
          "title": "Minimum Similarity",
          "description": "Minimum similarity score (0.0-1.0) for results",
          "default": 0.3,
          "minimum": 0,
          "maximum": 1
        }
      }
    },
    "mood_tags": {
      "type": "object",
      "title": "Mood Tags",
      "properties": {
        "enabled": {
          "type": "boolean",
          "title": "Enable Mood Tags",
          "description": "Generate mood/vibe tags for media using LLM",
          "default": false
        }
      }
    }
  }
}`
	return &pluginv1.SettingsSchema{JsonSchema: []byte(schema)}, nil
}

func (p *AISearchPlugin) Configure(ctx context.Context, req *pluginv1.Settings) (*pluginv1.ConfigureResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(req.Json) == 0 {
		return &pluginv1.ConfigureResponse{Success: true}, nil
	}

	// Parse settings JSON into a partial config structure
	var settings struct {
		Indexing *struct {
			BatchSize *int  `json:"batch_size"`
			AutoIndex *bool `json:"auto_index"`
		} `json:"indexing"`
		Search *struct {
			DefaultLimit  *int     `json:"default_limit"`
			MinSimilarity *float32 `json:"min_similarity"`
		} `json:"search"`
		MoodTags *struct {
			Enabled *bool `json:"enabled"`
		} `json:"mood_tags"`
	}

	if err := json.Unmarshal(req.Json, &settings); err != nil {
		return &pluginv1.ConfigureResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid settings JSON: %v", err),
		}, nil
	}

	// Apply settings
	if settings.Indexing != nil {
		if settings.Indexing.BatchSize != nil {
			p.config.Indexing.BatchSize = *settings.Indexing.BatchSize
		}
		if settings.Indexing.AutoIndex != nil {
			p.config.Indexing.AutoIndex = *settings.Indexing.AutoIndex
		}
	}
	if settings.Search != nil {
		if settings.Search.DefaultLimit != nil {
			p.config.Search.DefaultLimit = *settings.Search.DefaultLimit
		}
		if settings.Search.MinSimilarity != nil {
			p.config.Search.MinSimilarity = *settings.Search.MinSimilarity
		}
	}
	if settings.MoodTags != nil {
		if settings.MoodTags.Enabled != nil {
			p.config.MoodTags.Enabled = *settings.MoodTags.Enabled
		}
	}

	// Reinitialize services with new config
	p.initializeServices()

	p.logger.Debug("AI Search plugin reconfigured",
		"batch_size", p.config.Indexing.BatchSize,
		"auto_index", p.config.Indexing.AutoIndex,
		"mood_tags_enabled", p.config.MoodTags.Enabled,
	)

	return &pluginv1.ConfigureResponse{Success: true}, nil
}

func (p *AISearchPlugin) GetSubscriptions(ctx context.Context, req *pluginv1.Empty) (*pluginv1.EventSubscriptions, error) {
	// We might subscribe to media.added events in the future
	return &pluginv1.EventSubscriptions{}, nil
}

func (p *AISearchPlugin) OnEvent(ctx context.Context, req *pluginv1.Event) (*pluginv1.EventResponse, error) {
	return &pluginv1.EventResponse{Handled: false}, nil
}

// Enricher implementation

func (p *AISearchPlugin) GetCapabilities(ctx context.Context, req *pluginv1.Empty) (*pluginv1.EnricherCapabilities, error) {
	return &pluginv1.EnricherCapabilities{
		MediaTypes: []string{"movie", "tv", "tv_show", "tv_episode", "music_artist", "music_album", "music_track"},
		Provides:   []string{"embedding"},
		IsLocal:    false,
		RateLimit:  0,                    // No rate limit for indexing
		Requires:   []string{"metadata"}, // Needs metadata first
		Priority:   99,                   // Run last
	}, nil
}

func (p *AISearchPlugin) Enrich(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
	p.mu.Lock()
	p.requestsTotal++
	indexingService := p.indexingService
	autoIndex := p.config.Indexing.AutoIndex
	p.mu.Unlock()

	// Check if auto-indexing is enabled
	if !autoIndex {
		return &pluginv1.EnrichResponse{
			Skipped:    true,
			SkipReason: "auto-indexing disabled",
		}, nil
	}

	// Check if indexing service is available
	if indexingService == nil {
		p.mu.Lock()
		p.errorsTotal++
		p.mu.Unlock()
		return &pluginv1.EnrichResponse{
			Skipped:    true,
			SkipReason: "indexing service not initialized",
		}, nil
	}

	p.logger.Debug("indexing media",
		"media_id", req.MediaId,
		"media_type", req.MediaType,
		"title", req.Title,
	)

	// Convert to our entity type
	entityType := EntityType(req.MediaType)

	// Index the media (IndexSingle fetches details from host)
	if err := indexingService.IndexSingle(ctx, entityType, req.MediaId, nil); err != nil {
		p.mu.Lock()
		p.errorsTotal++
		p.mu.Unlock()
		p.logger.Warn("failed to index media", "id", req.MediaId, "error", err)
		return &pluginv1.EnrichResponse{
			Skipped:    true,
			SkipReason: fmt.Sprintf("indexing failed: %v", err),
		}, nil
	}

	return &pluginv1.EnrichResponse{
		Skipped: false,
		// No metadata changes - we just indexed it
	}, nil
}

// GetSearchService returns the search service for external use.
func (p *AISearchPlugin) GetSearchService() *SearchService {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.searchService
}

// GetIndexingService returns the indexing service for external use.
func (p *AISearchPlugin) GetIndexingService() *IndexingService {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.indexingService
}

// AISearch service implementation

// Search performs semantic search across indexed media.
// If a user ID is provided and the user has enabled location sharing,
// the query is enriched with contextual information (weather, time of day, season).
func (p *AISearchPlugin) Search(ctx context.Context, req *pluginv1.SemanticSearchRequest) (*pluginv1.SemanticSearchResponse, error) {
	p.mu.Lock()
	p.requestsTotal++
	searchService := p.searchService
	contextEnricher := p.contextEnricher
	p.mu.Unlock()

	if searchService == nil {
		return nil, fmt.Errorf("search service not initialized")
	}

	// Start with the original query
	query := req.Query

	// Use LLM to rewrite query for better intent matching
	// (e.g., "feeling sad need cheering up" -> "uplifting heartwarming feel-good")
	p.mu.RLock()
	queryRewriter := p.queryRewriter
	p.mu.RUnlock()

	if queryRewriter != nil {
		rewrittenQuery := queryRewriter.Rewrite(ctx, query)
		if rewrittenQuery != query {
			p.logger.Debug("LLM rewrote query",
				"original", query,
				"rewritten", rewrittenQuery,
			)
			query = rewrittenQuery
		}
	}

	// Enrich the query with context if user ID is provided and enricher is available
	// Privacy: The enricher only adds location-based context if the user has explicitly
	// opted in to location sharing. Time-based context (time of day, day of week) is
	// always safe as it uses server time.
	if contextEnricher != nil && req.UserId != "" {
		qc, err := contextEnricher.GetContext(ctx, req.UserId)
		if err != nil {
			p.logger.Debug("failed to get context for query enrichment", "error", err)
		} else {
			enrichedQuery := contextEnricher.EnrichQuery(query, qc)
			if enrichedQuery != query {
				p.logger.Debug("enriched search query",
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
		Limit:       int(req.Limit),
	})
	if err != nil {
		p.mu.Lock()
		p.errorsTotal++
		p.mu.Unlock()
		return nil, err
	}

	// Convert results to proto
	protoResults := make([]*pluginv1.SemanticSearchResult, len(results))
	for i, r := range results {
		protoResults[i] = &pluginv1.SemanticSearchResult{
			EntityType: string(r.EntityType),
			EntityId:   r.EntityID,
			Similarity: r.Similarity,
			Text:       r.Text,
		}
	}

	return &pluginv1.SemanticSearchResponse{
		Results: protoResults,
		Total:   int32(len(protoResults)),
	}, nil
}

// FindSimilar finds items similar to a given entity.
func (p *AISearchPlugin) FindSimilar(ctx context.Context, req *pluginv1.FindSimilarRequest) (*pluginv1.SemanticSearchResponse, error) {
	p.mu.Lock()
	p.requestsTotal++
	searchService := p.searchService
	p.mu.Unlock()

	if searchService == nil {
		return nil, fmt.Errorf("search service not initialized")
	}

	results, err := searchService.FindSimilar(ctx, EntityType(req.EntityType), req.EntityId, int(req.Limit))
	if err != nil {
		p.mu.Lock()
		p.errorsTotal++
		p.mu.Unlock()
		return nil, err
	}

	// Convert results to proto
	protoResults := make([]*pluginv1.SemanticSearchResult, len(results))
	for i, r := range results {
		protoResults[i] = &pluginv1.SemanticSearchResult{
			EntityType: string(r.EntityType),
			EntityId:   r.EntityID,
			Similarity: r.Similarity,
			Text:       r.Text,
		}
	}

	return &pluginv1.SemanticSearchResponse{
		Results: protoResults,
		Total:   int32(len(protoResults)),
	}, nil
}

// GetStatus returns the current indexing status.
func (p *AISearchPlugin) GetStatus(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.AISearchStatus, error) {
	p.mu.RLock()
	searchService := p.searchService
	indexingService := p.indexingService
	p.mu.RUnlock()

	status := &pluginv1.AISearchStatus{}

	// Get indexing progress
	if indexingService != nil {
		status.IsIndexing = indexingService.IsIndexing()
		if progress := indexingService.GetProgress(); progress != nil {
			status.Progress = &pluginv1.IndexingProgress{
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
				status.Stats = append(status.Stats, &pluginv1.EntityTypeStats{
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

// IndexLibrary triggers indexing for a library.
func (p *AISearchPlugin) IndexLibrary(ctx context.Context, req *pluginv1.IndexLibraryRequest) (*pluginv1.IndexLibraryResponse, error) {
	p.mu.RLock()
	indexingService := p.indexingService
	p.mu.RUnlock()

	if indexingService == nil {
		return &pluginv1.IndexLibraryResponse{
			Started: false,
			Message: "indexing service not initialized",
		}, nil
	}

	// Start indexing in background
	go func() {
		if err := indexingService.IndexLibrary(context.Background(), req.LibraryId, req.LibraryType); err != nil {
			p.logger.Error("library indexing failed", "library_id", req.LibraryId, "error", err)
		}
	}()

	return &pluginv1.IndexLibraryResponse{
		Started: true,
		Message: fmt.Sprintf("started indexing library %d", req.LibraryId),
	}, nil
}

// CancelIndexing cancels any running indexing operation.
func (p *AISearchPlugin) CancelIndexing(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.Empty, error) {
	p.mu.RLock()
	indexingService := p.indexingService
	p.mu.RUnlock()

	if indexingService != nil {
		indexingService.Cancel()
	}

	return &pluginv1.Empty{}, nil
}

// ============================================================
// Plugin-defined HTTP routes (new approach)
// ============================================================

// GetRoutes returns the HTTP routes this plugin provides.
func (p *AISearchPlugin) GetRoutes(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.PluginRoutes, error) {
	return &pluginv1.PluginRoutes{
		Routes: []*pluginv1.PluginRoute{
			{
				Path:        "/search",
				Methods:     []string{"POST"},
				AdminOnly:   false,
				Description: "Perform semantic search across indexed media",
				Capability:  "semantic_search",
			},
			{
				Path:        "/similar",
				Methods:     []string{"POST"},
				AdminOnly:   false,
				Description: "Find items similar to a given entity",
				Capability:  "similar_items",
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
		},
	}, nil
}

// HandleHTTP handles HTTP requests to plugin routes.
func (p *AISearchPlugin) HandleHTTP(ctx context.Context, req *pluginv1.PluginHTTPRequest) (*pluginv1.PluginHTTPResponse, error) {
	p.logger.Debug("handling HTTP request", "path", req.Path, "method", req.Method)

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

// handleSearch handles POST /search
func (p *AISearchPlugin) handleSearch(ctx context.Context, req *pluginv1.PluginHTTPRequest) (*pluginv1.PluginHTTPResponse, error) {
	if req.Method != "POST" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	// Parse request body
	var searchReq struct {
		Query       string   `json:"query"`
		EntityTypes []string `json:"entity_types,omitempty"`
		Limit       int32    `json:"limit,omitempty"`
	}
	if err := json.Unmarshal(req.Body, &searchReq); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Call the search method with user ID for context enrichment
	resp, err := p.Search(ctx, &pluginv1.SemanticSearchRequest{
		Query:       searchReq.Query,
		EntityTypes: searchReq.EntityTypes,
		Limit:       searchReq.Limit,
		UserId:      req.UserId, // Pass user ID for location-aware context
	})
	if err != nil {
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return jsonResponse(http.StatusOK, resp)
}

// handleSimilar handles POST /similar
func (p *AISearchPlugin) handleSimilar(ctx context.Context, req *pluginv1.PluginHTTPRequest) (*pluginv1.PluginHTTPResponse, error) {
	if req.Method != "POST" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	// Parse request body
	var similarReq struct {
		EntityType string `json:"entity_type"`
		EntityID   int64  `json:"entity_id"`
		Limit      int32  `json:"limit,omitempty"`
	}
	if err := json.Unmarshal(req.Body, &similarReq); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Call the similar method
	resp, err := p.FindSimilar(ctx, &pluginv1.FindSimilarRequest{
		EntityType: similarReq.EntityType,
		EntityId:   similarReq.EntityID,
		Limit:      similarReq.Limit,
	})
	if err != nil {
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return jsonResponse(http.StatusOK, resp)
}

// handleStatus handles GET /status
func (p *AISearchPlugin) handleStatus(ctx context.Context, req *pluginv1.PluginHTTPRequest) (*pluginv1.PluginHTTPResponse, error) {
	if req.Method != "GET" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	status, err := p.GetStatus(ctx, &pluginv1.Empty{})
	if err != nil {
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return jsonResponse(http.StatusOK, status)
}

// handleIndex handles POST /index
func (p *AISearchPlugin) handleIndex(ctx context.Context, req *pluginv1.PluginHTTPRequest) (*pluginv1.PluginHTTPResponse, error) {
	if req.Method != "POST" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	// Parse request body
	var indexReq struct {
		LibraryID   int64  `json:"library_id"`
		LibraryType string `json:"library_type"`
	}
	if err := json.Unmarshal(req.Body, &indexReq); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	resp, err := p.IndexLibrary(ctx, &pluginv1.IndexLibraryRequest{
		LibraryId:   indexReq.LibraryID,
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

// handleCancelIndex handles POST /index/cancel
func (p *AISearchPlugin) handleCancelIndex(ctx context.Context, req *pluginv1.PluginHTTPRequest) (*pluginv1.PluginHTTPResponse, error) {
	if req.Method != "POST" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	_, err := p.CancelIndexing(ctx, &pluginv1.Empty{})
	if err != nil {
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return jsonResponse(http.StatusOK, map[string]string{"status": "cancelled"})
}

// handleEstimate handles GET /index/estimate
func (p *AISearchPlugin) handleEstimate(ctx context.Context, req *pluginv1.PluginHTTPRequest) (*pluginv1.PluginHTTPResponse, error) {
	if req.Method != "GET" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	p.mu.RLock()
	dataClient := p.data
	p.mu.RUnlock()

	if dataClient == nil {
		return jsonResponse(http.StatusServiceUnavailable, map[string]string{"error": "data client not available"})
	}

	// Parse library_id from query params
	libraryIDStr := req.Query["library_id"]
	if libraryIDStr == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "library_id is required"})
	}

	var libraryID int64
	if _, err := fmt.Sscanf(libraryIDStr, "%d", &libraryID); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "invalid library_id"})
	}

	// Get library info
	library, err := dataClient.GetLibrary(ctx, libraryID)
	if err != nil {
		return jsonResponse(http.StatusNotFound, map[string]string{"error": "library not found"})
	}

	// Count items in the library by listing with pagination
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

	// Estimate tokens based on media type
	tokensPerItem := estimateTokensPerItem(library.MediaType)
	estimatedTokens := totalItems * int64(tokensPerItem)

	// Cost estimation - we don't know the provider here, so provide a generic estimate
	// Approximate: $0.02 per 1M tokens for cloud providers
	// Local providers (Ollama) are free but we can't determine that here
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

// handleClearIndex handles DELETE /index/clear
func (p *AISearchPlugin) handleClearIndex(ctx context.Context, req *pluginv1.PluginHTTPRequest) (*pluginv1.PluginHTTPResponse, error) {
	if req.Method != "DELETE" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	p.mu.RLock()
	embeddingsClient := p.embeddings
	p.mu.RUnlock()

	if embeddingsClient == nil {
		return jsonResponse(http.StatusServiceUnavailable, map[string]string{"error": "embeddings client not available"})
	}

	// Delete all embeddings for each entity type
	entityTypes := []string{"movie", "tv", "tv_show", "tv_episode", "music_artist", "music_album", "music_track"}
	var totalDeleted int64

	for _, entityType := range entityTypes {
		count, err := embeddingsClient.DeleteByType(ctx, entityType)
		if err != nil {
			p.logger.Warn("failed to delete embeddings", "type", entityType, "error", err)
			continue
		}
		totalDeleted += count
	}

	return jsonResponse(http.StatusOK, map[string]any{
		"status":        "cleared",
		"total_deleted": totalDeleted,
	})
}

// estimateTokensPerItem returns the estimated token count per media type.
// Based on the AI Assistant spec token estimates.
func estimateTokensPerItem(mediaType string) int {
	switch mediaType {
	case "movie", "movies":
		return 250 // Title + year + plot (~150) + cast (5×10) + genres + director
	case "tv", "tv_show":
		return 200 // Similar to movie, slightly shorter plots
	case "tv_episode":
		return 100 // Show reference + episode title + short plot
	case "music_artist":
		return 60 // Name + bio excerpt + genres + country
	case "music_album":
		return 80 // Artist + album + year + genres
	case "music_track", "music":
		return 50 // Track + artist + album + genre
	default:
		return 100 // Default estimate
	}
}

// handleMoodTagsGenerate handles POST /mood-tags/generate
func (p *AISearchPlugin) handleMoodTagsGenerate(
	ctx context.Context,
	req *pluginv1.PluginHTTPRequest,
) (*pluginv1.PluginHTTPResponse, error) {
	if req.Method != "POST" {
		return jsonResponse(http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}

	p.mu.RLock()
	moodTagService := p.moodTagService
	embeddingService := p.embeddingService
	embeddingsClient := p.embeddings
	dataClient := p.data
	p.mu.RUnlock()

	if moodTagService == nil {
		return jsonResponse(http.StatusServiceUnavailable, map[string]string{
			"error":   "mood tag service not available",
			"details": "mood tag generation is disabled in config or LLM client not connected",
		})
	}

	// Parse request body
	var genReq struct {
		LibraryID int64 `json:"library_id"`
	}
	if err := json.Unmarshal(req.Body, &genReq); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if genReq.LibraryID == 0 {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "library_id is required"})
	}

	p.logger.Info("handleMoodTagsGenerate called", "library_id", genReq.LibraryID)

	// Capture library ID before goroutine to avoid closure issues
	libraryID := genReq.LibraryID

	// Start generation in background
	go func() {
		// Recover from any panics
		defer func() {
			if r := recover(); r != nil {
				p.logger.Error("panic in mood tag generation", "panic", r, "library_id", libraryID)
			}
		}()

		p.logger.Info("mood tag generation goroutine started", "library_id", libraryID)

		// Callback to persist mood tags and reindex media
		callback := func(entityType EntityType, entityID int64, tags []string) error {
			if len(tags) == 0 {
				return nil
			}

			// Persist mood tags to the database via host data service
			sdkTags := make([]sdk.MoodTag, len(tags))
			for i, tag := range tags {
				sdkTags[i] = sdk.MoodTag{
					Tag:        tag,
					Confidence: 1.0,
				}
			}
			if err := dataClient.SetMoodTags(context.Background(), entityID, string(entityType), sdkTags); err != nil {
				p.logger.Warn("failed to persist mood tags",
					"entity_id", entityID,
					"error", err,
				)
				// Continue anyway - we can still update the embedding
			}

			// Fetch the media details again to rebuild text with mood tags
			details, err := dataClient.GetMediaDetails(context.Background(), entityID, string(entityType))
			if err != nil {
				return fmt.Errorf("get media details: %w", err)
			}

			// Build text with mood tags appended
			text := buildTextWithMoodTagsSDK(details, tags)

			// Generate new embedding
			embedding, err := embeddingService.EmbedSingle(context.Background(), text)
			if err != nil {
				return fmt.Errorf("generate embedding: %w", err)
			}

			// Store updated embedding
			return embeddingsClient.Store(context.Background(), string(entityType), entityID, embedding, text)
		}

		p.logger.Debug("calling GenerateForLibrary", "library_id", libraryID)
		if err := moodTagService.GenerateForLibrary(
			context.Background(),
			libraryID,
			callback,
		); err != nil {
			p.logger.Error("mood tag generation failed", "library_id", libraryID, "error", err)
		}
		p.logger.Info("mood tag generation completed", "library_id", libraryID)
	}()

	return jsonResponse(http.StatusOK, map[string]any{
		"started":    true,
		"library_id": genReq.LibraryID,
		"message":    "mood tag generation started in background",
	})
}

// handleMoodTagsStatus handles GET /mood-tags/status
func (p *AISearchPlugin) handleMoodTagsStatus(
	ctx context.Context,
	req *pluginv1.PluginHTTPRequest,
) (*pluginv1.PluginHTTPResponse, error) {
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

// handleMoodTagsCancel handles POST /mood-tags/cancel
func (p *AISearchPlugin) handleMoodTagsCancel(
	ctx context.Context,
	req *pluginv1.PluginHTTPRequest,
) (*pluginv1.PluginHTTPResponse, error) {
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

// buildTextWithMoodTagsSDK appends mood tags to the media text.
func buildTextWithMoodTagsSDK(details *sdk.MediaDetails, tags []string) string {
	if details == nil {
		return ""
	}

	// Use the indexing service's text builder (simplified version here)
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

	// Append mood tags
	if len(tags) > 0 {
		text += fmt.Sprintf("Mood: %s\n", joinStrings(tags, ", "))
	}

	return text
}

// joinStrings joins strings with a separator.
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
