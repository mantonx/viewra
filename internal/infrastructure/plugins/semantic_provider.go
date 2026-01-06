package plugins

import (
	"context"
	"log/slog"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/application/movies"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/host"
	"github.com/mantonx/viewra/internal/infrastructure/plugins/registry"
)

// SemanticSearchProvider implements movies.SemanticSearchProvider using the plugin host server.
// It delegates semantic search requests to the semantic-search plugin via gRPC.
type SemanticSearchProvider struct {
	hostServer  *host.PluginsServer
	capRegistry *registry.CapabilityRegistry
	logger      *slog.Logger
}

// Compile-time check that SemanticSearchProvider implements the interface.
var _ movies.SemanticSearchProvider = (*SemanticSearchProvider)(nil)

// NewSemanticSearchProvider creates a new semantic search provider.
func NewSemanticSearchProvider(
	hostServer *host.PluginsServer,
	capRegistry *registry.CapabilityRegistry,
	logger *slog.Logger,
) *SemanticSearchProvider {
	return &SemanticSearchProvider{
		hostServer:  hostServer,
		capRegistry: capRegistry,
		logger:      logger.With("component", "semantic-search-provider"),
	}
}

// IsAvailable returns true if semantic search is currently available.
func (p *SemanticSearchProvider) IsAvailable() bool {
	if p.hostServer == nil {
		return false
	}

	// Check if any plugin provides the vector_search capability
	// Use HostPluginsServer.HasCapability which checks manifest-declared capabilities,
	// not CapabilityRegistry which only tracks HTTP route capabilities.
	return p.hostServer.HasCapability("vector_search")
}

// Search performs semantic search and returns matching entity IDs with similarity scores.
func (p *SemanticSearchProvider) Search(ctx context.Context, query string, entityTypes []string, limit int) ([]movies.SemanticSearchResult, error) {
	if !p.IsAvailable() {
		return nil, nil
	}

	// Build the search request
	searchReq := &pluginv1.SemanticSearchRequest{
		Query:       query,
		EntityTypes: entityTypes,
		Limit:       int32(limit),
	}

	// Invoke vector search through the host server
	resp, err := p.hostServer.InvokeVectorSearch(ctx, &pluginv1.VectorSearchInvokeRequest{
		Method: "Search",
		Search: searchReq,
	})
	if err != nil {
		p.logger.Debug("semantic search invocation failed", "error", err)
		return nil, nil // Return nil instead of error to allow fallback
	}

	if resp.Error != nil {
		p.logger.Debug("semantic search returned error",
			"code", resp.Error.Code,
			"message", resp.Error.Message)
		return nil, nil // Return nil instead of error to allow fallback
	}

	if resp.Response == nil || len(resp.Response.Results) == 0 {
		return nil, nil
	}

	// Convert to application layer types
	results := make([]movies.SemanticSearchResult, 0, len(resp.Response.Results))
	for _, r := range resp.Response.Results {
		results = append(results, movies.SemanticSearchResult{
			EntityID:   r.EntityId,
			EntityType: r.EntityType,
			Similarity: r.Similarity,
		})
	}

	p.logger.Debug("semantic search completed",
		"query", query,
		"results", len(results))

	return results, nil
}
