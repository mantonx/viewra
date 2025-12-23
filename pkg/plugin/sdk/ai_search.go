// AI Search plugin support for ViewRA semantic search plugins.
//
// This file provides the AISearchPlugin interface and ServeAISearchEnricher() helper
// for building AI-powered search plugins that provide semantic search, similarity matching,
// and embedding-based media indexing.
//
// # Quick Start
//
// Create a plugin that implements both EnricherPlugin and AISearchPlugin:
//
//	type MyAISearch struct {
//	    sdk.Base
//	    llm        *sdk.LLMClient
//	    embeddings *sdk.EmbeddingsClient
//	    data       *sdk.DataClient
//	}
//
//	func (s *MyAISearch) Search(ctx context.Context, req *sdk.SemanticSearchRequest) (*sdk.SemanticSearchResponse, error) {
//	    // Perform semantic search
//	}
//
//	// ... implement other AISearchPlugin and EnricherPlugin methods
//
//	func main() {
//	    hclogger, logger := sdk.NewLogger("my-ai-search")
//	    plugin := &MyAISearch{}
//	    plugin.SetLogger(logger)
//	    sdk.ServeAISearchEnricher(plugin, hclogger)
//	}
package sdk

import (
	"context"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// --- SDK Types for AI Search ---

// SemanticSearchRequest contains parameters for semantic search.
type SemanticSearchRequest struct {
	Query       string   // Natural language search query
	EntityTypes []string // Filter by entity types: "movie", "tv_show", "tv_episode", etc.
	Limit       int      // Maximum number of results
	UserID      string   // Optional user ID for context enrichment (location, time)
}

// SemanticSearchResponse contains search results.
type SemanticSearchResponse struct {
	Results []SemanticSearchResult
	Total   int
}

// SemanticSearchResult represents a single semantic search result.
type SemanticSearchResult struct {
	EntityType string  // "movie", "tv_show", etc.
	EntityID   int64   // Database ID of the entity
	Similarity float32 // Cosine similarity score (0.0 to 1.0)
	Text       string  // The indexed text that matched
}

// FindSimilarRequest contains parameters for finding similar items.
type FindSimilarRequest struct {
	EntityType string // Type of the source entity
	EntityID   int64  // ID of the source entity
	Limit      int    // Maximum number of similar items to return
}

// AISearchStatus contains the current indexing status.
type AISearchStatus struct {
	IsIndexing   bool              // Whether indexing is currently running
	Progress     *IndexingProgress // Current operation progress (if indexing)
	Stats        []EntityTypeStats // Per-entity-type statistics
	TotalIndexed int64             // Total embeddings stored
}

// IndexingProgress tracks indexing operation progress.
type IndexingProgress struct {
	EntityType  string // Entity type being indexed
	Total       int64  // Total items to process
	Processed   int64  // Items processed so far
	Failed      int64  // Items that failed
	LastError   string // Most recent error message
	StartedAt   int64  // Unix timestamp when started
	LastUpdated int64  // Unix timestamp of last update
}

// EntityTypeStats contains statistics for an entity type.
type EntityTypeStats struct {
	EntityType string // "movie", "tv_show", etc.
	Indexed    int64  // Number of indexed items
	Total      int64  // Total items in database
}

// IndexLibraryRequest contains parameters for indexing a library.
type IndexLibraryRequest struct {
	LibraryID   int64  // Library database ID
	LibraryType string // "movie", "tv", "music"
}

// IndexLibraryResponse contains the result of an indexing trigger.
type IndexLibraryResponse struct {
	Started bool   // Whether indexing was started
	Message string // Status message
}

// --- AISearchPlugin Interface ---

// AISearchPlugin is the interface that AI search plugins must implement.
// Plugins implementing this interface should also implement EnricherPlugin
// for auto-indexing support in the enrichment pipeline.
type AISearchPlugin interface {
	mustEmbedBase()

	// Search performs semantic search across indexed media.
	// The query is converted to an embedding and matched against stored embeddings.
	Search(ctx context.Context, req *SemanticSearchRequest) (*SemanticSearchResponse, error)

	// FindSimilar finds items similar to a given entity.
	// Uses the entity's stored embedding to find similar items.
	FindSimilar(ctx context.Context, req *FindSimilarRequest) (*SemanticSearchResponse, error)

	// GetStatus returns the current indexing status and statistics.
	GetStatus(ctx context.Context) (*AISearchStatus, error)

	// IndexLibrary triggers indexing for all media in a library.
	// Indexing runs in the background; use GetStatus to monitor progress.
	IndexLibrary(ctx context.Context, req *IndexLibraryRequest) (*IndexLibraryResponse, error)

	// CancelIndexing cancels any running indexing operation.
	CancelIndexing(ctx context.Context) error
}

// --- Proto Conversion Functions ---

func protoToSemanticSearchRequest(req *pluginv1.SemanticSearchRequest) *SemanticSearchRequest {
	return &SemanticSearchRequest{
		Query:       req.Query,
		EntityTypes: req.EntityTypes,
		Limit:       int(req.Limit),
		UserID:      req.UserId,
	}
}

func semanticSearchResponseToProto(resp *SemanticSearchResponse) *pluginv1.SemanticSearchResponse {
	results := make([]*pluginv1.SemanticSearchResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = &pluginv1.SemanticSearchResult{
			EntityType: r.EntityType,
			EntityId:   r.EntityID,
			Similarity: r.Similarity,
			Text:       r.Text,
		}
	}
	return &pluginv1.SemanticSearchResponse{
		Results: results,
		Total:   int32(resp.Total),
	}
}

func protoToFindSimilarRequest(req *pluginv1.FindSimilarRequest) *FindSimilarRequest {
	return &FindSimilarRequest{
		EntityType: req.EntityType,
		EntityID:   req.EntityId,
		Limit:      int(req.Limit),
	}
}

func aiSearchStatusToProto(status *AISearchStatus) *pluginv1.AISearchStatus {
	result := &pluginv1.AISearchStatus{
		IsIndexing:   status.IsIndexing,
		TotalIndexed: status.TotalIndexed,
	}

	if status.Progress != nil {
		result.Progress = &pluginv1.IndexingProgress{
			EntityType:  status.Progress.EntityType,
			Total:       status.Progress.Total,
			Processed:   status.Progress.Processed,
			Failed:      status.Progress.Failed,
			LastError:   status.Progress.LastError,
			StartedAt:   status.Progress.StartedAt,
			LastUpdated: status.Progress.LastUpdated,
		}
	}

	for _, s := range status.Stats {
		result.Stats = append(result.Stats, &pluginv1.EntityTypeStats{
			EntityType: s.EntityType,
			Indexed:    s.Indexed,
			Total:      s.Total,
		})
	}

	return result
}

func protoToIndexLibraryRequest(req *pluginv1.IndexLibraryRequest) *IndexLibraryRequest {
	return &IndexLibraryRequest{
		LibraryID:   req.LibraryId,
		LibraryType: req.LibraryType,
	}
}

func indexLibraryResponseToProto(resp *IndexLibraryResponse) *pluginv1.IndexLibraryResponse {
	return &pluginv1.IndexLibraryResponse{
		Started: resp.Started,
		Message: resp.Message,
	}
}

// --- gRPC Server Wrapper ---

// aiSearchGRPCServer wraps an AISearchPlugin to implement pluginv1.AISearchServer.
type aiSearchGRPCServer struct {
	pluginv1.UnimplementedAISearchServer
	impl AISearchPlugin
}

func (s *aiSearchGRPCServer) Search(ctx context.Context, req *pluginv1.SemanticSearchRequest) (*pluginv1.SemanticSearchResponse, error) {
	resp, err := s.impl.Search(ctx, protoToSemanticSearchRequest(req))
	if err != nil {
		return nil, err
	}
	return semanticSearchResponseToProto(resp), nil
}

func (s *aiSearchGRPCServer) FindSimilar(ctx context.Context, req *pluginv1.FindSimilarRequest) (*pluginv1.SemanticSearchResponse, error) {
	resp, err := s.impl.FindSimilar(ctx, protoToFindSimilarRequest(req))
	if err != nil {
		return nil, err
	}
	return semanticSearchResponseToProto(resp), nil
}

func (s *aiSearchGRPCServer) GetStatus(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.AISearchStatus, error) {
	status, err := s.impl.GetStatus(ctx)
	if err != nil {
		return nil, err
	}
	return aiSearchStatusToProto(status), nil
}

func (s *aiSearchGRPCServer) IndexLibrary(ctx context.Context, req *pluginv1.IndexLibraryRequest) (*pluginv1.IndexLibraryResponse, error) {
	resp, err := s.impl.IndexLibrary(ctx, protoToIndexLibraryRequest(req))
	if err != nil {
		return nil, err
	}
	return indexLibraryResponseToProto(resp), nil
}

func (s *aiSearchGRPCServer) CancelIndexing(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.Empty, error) {
	if err := s.impl.CancelIndexing(ctx); err != nil {
		return nil, err
	}
	return &pluginv1.Empty{}, nil
}

// --- go-plugin Integration ---

// AISearchGRPCPlugin implements plugin.GRPCPlugin for the AISearch service.
type AISearchGRPCPlugin struct {
	plugin.Plugin
	Impl AISearchPlugin
}

func (p *AISearchGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pluginv1.RegisterAISearchServer(s, &aiSearchGRPCServer{impl: p.Impl})
	return nil
}

func (p *AISearchGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewAISearchClient(c), nil
}

// --- Serve Helper ---

// AISearchEnricherPlugin combines EnricherPlugin and AISearchPlugin interfaces.
// Plugins should implement this combined interface for full functionality.
type AISearchEnricherPlugin interface {
	EnricherPlugin
	AISearchPlugin
}

// ServeAISearchEnricher starts a plugin server for plugins that implement both
// EnricherPlugin (for auto-indexing in enrichment pipeline) and AISearchPlugin
// (for semantic search, similarity matching, and manual indexing).
//
// This is the recommended way to serve AI search plugins:
//
//	func main() {
//	    hclogger, logger := sdk.NewLogger("ai-search")
//	    plugin := NewAISearchPlugin(logger)
//	    sdk.ServeAISearchEnricher(plugin, hclogger)
//	}
func ServeAISearchEnricher(impl AISearchEnricherPlugin, logger hclog.Logger) {
	base := &Base{}
	plugins := map[string]plugin.Plugin{
		"core":            &EnricherCoreGRPCPlugin{Impl: impl, base: base},
		"enricher":        &EnricherGRPCPlugin{Impl: impl, base: base},
		"ai_search":       &AISearchGRPCPlugin{Impl: impl},
		"host_storage":    &HostStorageGRPCPlugin{},
		"host_llm":        &HostLLMGRPCPlugin{},
		"host_embeddings": &HostEmbeddingsGRPCPlugin{},
		"host_data":       &HostDataGRPCPlugin{},
		"host_weather":    &HostWeatherGRPCPlugin{},
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         plugins,
		GRPCServer:      plugin.DefaultGRPCServer,
		Logger:          logger,
	})
}
