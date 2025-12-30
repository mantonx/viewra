// Package sdk provides vector storage utilities for ViewRA plugins.
//
// The VectorClient provides managed vector storage with automatic indexing.
// The host uses pgvector (PostgreSQL) or sqlite-vec (SQLite) under the hood,
// providing fast approximate nearest neighbor (ANN) search.
//
// # Usage
//
//	func (p *MyPlugin) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
//	    vec := p.storage.Vector()
//
//	    // Store an embedding
//	    err := vec.Store(ctx, sdk.Embedding{
//	        EntityType: "movie",
//	        EntityID:   123,
//	        Vector:     embedding,  // []float32 from your embedding model
//	        Text:       "The Matrix (1999) - A computer hacker learns...",
//	    })
//
//	    // Search for similar items
//	    results, err := vec.Search(ctx, sdk.VectorSearchRequest{
//	        QueryVector: queryEmbedding,
//	        Limit:       10,
//	    })
//	}
//
// # Entity Namespacing
//
// Embeddings are automatically namespaced per-plugin. Each plugin has its own
// isolated vector index. Entity types and IDs are scoped to the plugin.
//
// # Embedding Dimensions
//
// The host automatically detects the embedding dimensions from the first vector
// stored. All subsequent vectors must have the same dimensions.
// Common dimensions: 384 (MiniLM), 768 (BERT), 1536 (OpenAI ada-002), 3072 (OpenAI text-embedding-3-large)
package sdk

import (
	"context"
	"fmt"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// VectorClient provides managed vector storage for plugins.
// Embeddings are automatically indexed for fast similarity search.
type VectorClient struct {
	client pluginv1.HostStorageClient
}

// newVectorClient creates a new vector client. Called internally by StorageClient.
func newVectorClient(client pluginv1.HostStorageClient) *VectorClient {
	return &VectorClient{client: client}
}

// Embedding represents a stored embedding.
type Embedding struct {
	// EntityType identifies the type of entity (e.g., "movie", "tv_show", "track")
	EntityType string

	// EntityID is the unique identifier within the entity type
	EntityID int64

	// Vector is the embedding vector (float32)
	Vector []float32

	// Text is the original text that was embedded (optional, for debugging)
	Text string

	// Model is the model used to generate the embedding (optional, for tracking)
	Model string
}

// VectorSearchRequest specifies search parameters.
type VectorSearchRequest struct {
	// QueryVector is the embedding to search for similar items
	QueryVector []float32

	// EntityTypes filters results by type (empty = all types)
	EntityTypes []string

	// Limit is the maximum number of results (default: 20)
	Limit int

	// Offset for pagination
	Offset int

	// MinSimilarity filters results below this threshold (0.0-1.0)
	MinSimilarity float32
}

// VectorSearchResult represents a search result.
type VectorSearchResult struct {
	// EntityType of the matched embedding
	EntityType string

	// EntityID of the matched embedding
	EntityID int64

	// Similarity score (0.0-1.0, higher is more similar)
	Similarity float32

	// Text that was embedded
	Text string
}

// VectorSearchResponse contains search results.
type VectorSearchResponse struct {
	// Results ordered by similarity (highest first)
	Results []VectorSearchResult

	// TotalCount is the total number of matching embeddings
	TotalCount int
}

// Store saves or updates an embedding for an entity.
// If an embedding already exists for the entity, it is replaced.
//
// Example:
//
//	err := vec.Store(ctx, sdk.Embedding{
//	    EntityType: "movie",
//	    EntityID:   123,
//	    Vector:     embedding,
//	    Text:       "The Matrix (1999)",
//	    Model:      "nomic-embed-text",
//	})
func (c *VectorClient) Store(ctx context.Context, emb Embedding) error {
	_, err := c.client.VectorStoreEmbedding(ctx, &pluginv1.VectorStoreRequest{
		EntityType: emb.EntityType,
		EntityId:   emb.EntityID,
		Vector:     emb.Vector,
		Text:       emb.Text,
		Model:      emb.Model,
	})
	return err
}

// StoreBatch saves multiple embeddings in a single transaction.
// This is more efficient than calling Store multiple times.
//
// Example:
//
//	err := vec.StoreBatch(ctx, []sdk.Embedding{
//	    {EntityType: "movie", EntityID: 1, Vector: emb1},
//	    {EntityType: "movie", EntityID: 2, Vector: emb2},
//	})
func (c *VectorClient) StoreBatch(ctx context.Context, embeddings []Embedding) error {
	reqs := make([]*pluginv1.VectorStoreRequest, len(embeddings))
	for i, emb := range embeddings {
		reqs[i] = &pluginv1.VectorStoreRequest{
			EntityType: emb.EntityType,
			EntityId:   emb.EntityID,
			Vector:     emb.Vector,
			Text:       emb.Text,
			Model:      emb.Model,
		}
	}
	_, err := c.client.VectorStoreBatch(ctx, &pluginv1.VectorStoreBatchRequest{
		Embeddings: reqs,
	})
	return err
}

// Search performs similarity search using the query vector.
// Returns results ordered by similarity (highest first).
//
// Example:
//
//	results, err := vec.Search(ctx, sdk.VectorSearchRequest{
//	    QueryVector:   queryEmbedding,
//	    EntityTypes:   []string{"movie", "tv_show"},
//	    Limit:         10,
//	    MinSimilarity: 0.5,
//	})
//	for _, r := range results.Results {
//	    fmt.Printf("%s %d: %.2f - %s\n", r.EntityType, r.EntityID, r.Similarity, r.Text)
//	}
func (c *VectorClient) Search(ctx context.Context, req VectorSearchRequest) (*VectorSearchResponse, error) {
	if len(req.QueryVector) == 0 {
		return nil, fmt.Errorf("query vector is required")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}

	resp, err := c.client.VectorSearch(ctx, &pluginv1.VectorSearchRequest{
		QueryVector:   req.QueryVector,
		EntityTypes:   req.EntityTypes,
		Limit:         int32(limit),
		Offset:        int32(req.Offset),
		MinSimilarity: req.MinSimilarity,
	})
	if err != nil {
		return nil, err
	}

	results := make([]VectorSearchResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = VectorSearchResult{
			EntityType: r.EntityType,
			EntityID:   r.EntityId,
			Similarity: r.Similarity,
			Text:       r.Text,
		}
	}

	return &VectorSearchResponse{
		Results:    results,
		TotalCount: int(resp.TotalCount),
	}, nil
}

// Get retrieves an embedding by entity type and ID.
// Returns nil if the embedding doesn't exist.
//
// Example:
//
//	emb, err := vec.Get(ctx, "movie", 123)
//	if emb != nil {
//	    fmt.Printf("Found embedding with %d dimensions\n", len(emb.Vector))
//	}
func (c *VectorClient) Get(ctx context.Context, entityType string, entityID int64) (*Embedding, error) {
	resp, err := c.client.VectorGet(ctx, &pluginv1.VectorQuery{
		EntityType: entityType,
		EntityId:   entityID,
	})
	if err != nil {
		return nil, err
	}

	if !resp.Exists {
		return nil, nil
	}

	return &Embedding{
		EntityType: entityType,
		EntityID:   entityID,
		Vector:     resp.Vector,
		Text:       resp.Text,
		Model:      resp.Model,
	}, nil
}

// Delete removes an embedding.
//
// Example:
//
//	err := vec.Delete(ctx, "movie", 123)
func (c *VectorClient) Delete(ctx context.Context, entityType string, entityID int64) error {
	_, err := c.client.VectorDelete(ctx, &pluginv1.VectorQuery{
		EntityType: entityType,
		EntityId:   entityID,
	})
	return err
}

// DeleteByType removes all embeddings for an entity type.
// Returns the number of deleted embeddings.
//
// Example:
//
//	count, err := vec.DeleteByType(ctx, "movie")
//	fmt.Printf("Deleted %d embeddings\n", count)
func (c *VectorClient) DeleteByType(ctx context.Context, entityType string) (int64, error) {
	resp, err := c.client.VectorDeleteByType(ctx, &pluginv1.VectorTypeQuery{
		EntityType: entityType,
	})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}

// DeleteAll removes all embeddings for this plugin.
// Returns the number of deleted embeddings.
func (c *VectorClient) DeleteAll(ctx context.Context) (int64, error) {
	resp, err := c.client.VectorDeleteByType(ctx, &pluginv1.VectorTypeQuery{
		EntityType: "", // Empty = all types
	})
	if err != nil {
		return 0, err
	}
	return resp.DeletedCount, nil
}

// SearchText performs text-based search on embedding text field.
// This is useful for name/title searches where semantic search may fail.
//
// Example:
//
//	results, err := vec.SearchText(ctx, "Christopher Nolan", []string{"movie"}, 50)
//	for _, r := range results.Results {
//	    fmt.Printf("%s %d: %s\n", r.EntityType, r.EntityID, r.Text)
//	}
func (c *VectorClient) SearchText(ctx context.Context, query string, entityTypes []string, limit int) (*VectorSearchResponse, error) {
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	if limit <= 0 {
		limit = 100
	}

	resp, err := c.client.VectorSearchText(ctx, &pluginv1.VectorTextSearchRequest{
		Query:       query,
		EntityTypes: entityTypes,
		Limit:       int32(limit),
	})
	if err != nil {
		return nil, err
	}

	results := make([]VectorSearchResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = VectorSearchResult{
			EntityType: r.EntityType,
			EntityID:   r.EntityId,
			Similarity: r.Similarity,
			Text:       r.Text,
		}
	}

	return &VectorSearchResponse{
		Results:    results,
		TotalCount: int(resp.TotalCount),
	}, nil
}

// Count returns the number of embeddings.
// If entityType is empty, returns total count across all types.
//
// Example:
//
//	total, _ := vec.Count(ctx, "")
//	movies, _ := vec.Count(ctx, "movie")
func (c *VectorClient) Count(ctx context.Context, entityType string) (int64, error) {
	resp, err := c.client.VectorCount(ctx, &pluginv1.VectorTypeQuery{
		EntityType: entityType,
	})
	if err != nil {
		return 0, err
	}
	return resp.Count, nil
}
