// Host service client wrappers for ViewRA plugins.
//
// This file provides type-safe wrappers around the host services available
// to plugins. These services are exposed by the host and allow plugins to
// access data, AI capabilities, storage, and more.
//
// # Available Services
//
// The host exposes these services to plugins:
//
//   - HostLLM: Generate embeddings and chat completions
//   - HostEmbeddings: Store and search vector embeddings
//   - HostData: Access media library data (read-only)
//   - HostStorage: Plugin-scoped key-value and SQLite storage
//   - HostUserMetadata: Per-user plugin data storage
//   - HostWeather: Weather and time context for search queries
//
// # Usage
//
// Plugins receive broker IDs in the InitRequest. Use these to connect:
//
//	func (p *MyPlugin) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
//	    if req.HostLlmBrokerId > 0 {
//	        conn, _ := broker.Dial(req.HostLlmBrokerId)
//	        p.llm = sdk.NewLLMClient(conn)
//	    }
//	    return &pluginv1.InitResponse{Success: true}, nil
//	}
//
// Then use the wrapped client:
//
//	embedding, err := p.llm.Embed(ctx, "movie about space exploration")
//	if err != nil {
//	    return err
//	}
package sdk

import (
	"context"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"google.golang.org/grpc"
)

// ============================================================================
// LLM Client - Generate embeddings and chat completions
// ============================================================================

// LLMClient wraps the HostLLM service for easier use.
type LLMClient struct {
	client pluginv1.HostLLMClient
}

// NewLLMClient creates a new LLM client from a gRPC connection.
func NewLLMClient(conn *grpc.ClientConn) *LLMClient {
	return &LLMClient{client: pluginv1.NewHostLLMClient(conn)}
}

// Embed generates an embedding for a single text.
// Uses the host's configured default embedding provider and model.
//
// Example:
//
//	embedding, err := llm.Embed(ctx, "A movie about space exploration")
func (c *LLMClient) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := c.client.GenerateEmbedding(ctx, &pluginv1.EmbeddingRequest{Text: text})
	if err != nil {
		return nil, err
	}
	return resp.Embedding, nil
}

// EmbedWithModel generates an embedding using a specific provider and model.
//
// Example:
//
//	embedding, err := llm.EmbedWithModel(ctx, "text", "ollama", "nomic-embed-text")
func (c *LLMClient) EmbedWithModel(ctx context.Context, text, provider, model string) ([]float32, error) {
	resp, err := c.client.GenerateEmbedding(ctx, &pluginv1.EmbeddingRequest{
		Text:     text,
		Provider: provider,
		Model:    model,
	})
	if err != nil {
		return nil, err
	}
	return resp.Embedding, nil
}

// EmbedBatch generates embeddings for multiple texts.
//
// Example:
//
//	embeddings, err := llm.EmbedBatch(ctx, []string{"text1", "text2"})
func (c *LLMClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	resp, err := c.client.GenerateEmbeddingBatch(ctx, &pluginv1.EmbeddingBatchRequest{Texts: texts})
	if err != nil {
		return nil, err
	}
	embeddings := make([][]float32, len(resp.Embeddings))
	for i, e := range resp.Embeddings {
		embeddings[i] = e.Embedding
	}
	return embeddings, nil
}

// Chat sends a chat completion request.
//
// Example:
//
//	resp, err := llm.Chat(ctx, []sdk.ChatMessage{
//	    {Role: "system", Content: "You are a helpful assistant."},
//	    {Role: "user", Content: "What's a good movie for a rainy day?"},
//	})
func (c *LLMClient) Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error) {
	protoMsgs := make([]*pluginv1.ChatMessage, len(messages))
	for i, m := range messages {
		protoMsgs[i] = &pluginv1.ChatMessage{Role: m.Role, Content: m.Content}
	}

	resp, err := c.client.Chat(ctx, &pluginv1.ChatRequest{Messages: protoMsgs})
	if err != nil {
		return nil, err
	}

	return &ChatResponse{
		Content:          resp.Content,
		FinishReason:     resp.FinishReason,
		PromptTokens:     int(resp.PromptTokens),
		CompletionTokens: int(resp.CompletionTokens),
	}, nil
}

// ChatWithOptions sends a chat completion request with options.
//
// Example:
//
//	resp, err := llm.ChatWithOptions(ctx, messages, sdk.ChatOptions{
//	    Model:       "llama3.1:8b",
//	    Temperature: 0.7,
//	    MaxTokens:   500,
//	})
func (c *LLMClient) ChatWithOptions(ctx context.Context, messages []ChatMessage, opts ChatOptions) (*ChatResponse, error) {
	protoMsgs := make([]*pluginv1.ChatMessage, len(messages))
	for i, m := range messages {
		protoMsgs[i] = &pluginv1.ChatMessage{Role: m.Role, Content: m.Content}
	}

	resp, err := c.client.Chat(ctx, &pluginv1.ChatRequest{
		Messages:    protoMsgs,
		Provider:    opts.Provider,
		Model:       opts.Model,
		Temperature: opts.Temperature,
		MaxTokens:   int32(opts.MaxTokens),
	})
	if err != nil {
		return nil, err
	}

	return &ChatResponse{
		Content:          resp.Content,
		FinishReason:     resp.FinishReason,
		PromptTokens:     int(resp.PromptTokens),
		CompletionTokens: int(resp.CompletionTokens),
	}, nil
}

// ChatOptions contains optional parameters for chat completions.
type ChatOptions struct {
	// Provider to use (uses default if empty)
	Provider string

	// Model to use (uses provider default if empty)
	Model string

	// Temperature controls randomness (0.0-2.0)
	Temperature float32

	// MaxTokens limits response length
	MaxTokens int
}

// ListProviders returns available LLM providers.
func (c *LLMClient) ListProviders(ctx context.Context) ([]LLMProvider, error) {
	resp, err := c.client.ListProviders(ctx, &pluginv1.Empty{})
	if err != nil {
		return nil, err
	}
	providers := make([]LLMProvider, len(resp.Providers))
	for i, p := range resp.Providers {
		providers[i] = LLMProvider{
			ID:                p.Id,
			Name:              p.Name,
			Configured:        p.Configured,
			SupportsChat:      p.SupportsChat,
			SupportsEmbedding: p.SupportsEmbedding,
		}
	}
	return providers, nil
}

// LLMProvider describes an available LLM provider.
type LLMProvider struct {
	ID                string
	Name              string
	Configured        bool
	SupportsChat      bool
	SupportsEmbedding bool
}

// ============================================================================
// Embeddings Client - Store and search vector embeddings
// ============================================================================

// EmbeddingsClient wraps the HostEmbeddings service for vector storage.
type EmbeddingsClient struct {
	client pluginv1.HostEmbeddingsClient
}

// NewEmbeddingsClient creates a new embeddings storage client.
func NewEmbeddingsClient(conn *grpc.ClientConn) *EmbeddingsClient {
	return &EmbeddingsClient{client: pluginv1.NewHostEmbeddingsClient(conn)}
}

// Store saves an embedding for an entity.
//
// Example:
//
//	err := embeddings.Store(ctx, "movie", 123, embedding, "The Matrix (1999)")
func (c *EmbeddingsClient) Store(ctx context.Context, entityType string, entityID int64, embedding []float32, text string) error {
	_, err := c.client.Store(ctx, &pluginv1.StoreEmbeddingRequest{
		EntityType: entityType,
		EntityId:   entityID,
		Embedding:  embedding,
		Text:       text,
	})
	return err
}

// Get retrieves an embedding by entity type and ID.
func (c *EmbeddingsClient) Get(ctx context.Context, entityType string, entityID int64) (*StoredEmbedding, error) {
	resp, err := c.client.Get(ctx, &pluginv1.EmbeddingQuery{
		EntityType: entityType,
		EntityId:   entityID,
	})
	if err != nil {
		return nil, err
	}
	return &StoredEmbedding{
		EntityType: resp.EntityType,
		EntityID:   resp.EntityId,
		Embedding:  resp.Embedding,
		Text:       resp.Text,
		Model:      resp.Model,
		Exists:     resp.Exists,
	}, nil
}

// Delete removes an embedding.
func (c *EmbeddingsClient) Delete(ctx context.Context, entityType string, entityID int64) error {
	_, err := c.client.Delete(ctx, &pluginv1.EmbeddingQuery{
		EntityType: entityType,
		EntityId:   entityID,
	})
	return err
}

// Search finds similar embeddings using cosine similarity.
//
// Example:
//
//	results, err := embeddings.Search(ctx, queryEmbedding, []string{"movie", "tv_show"}, 10, 0.5)
func (c *EmbeddingsClient) Search(ctx context.Context, queryEmbedding []float32, entityTypes []string, limit int, minSimilarity float32) ([]SearchResult, error) {
	resp, err := c.client.Search(ctx, &pluginv1.EmbeddingSearchRequest{
		QueryEmbedding: queryEmbedding,
		EntityTypes:    entityTypes,
		Limit:          int32(limit),
		MinSimilarity:  minSimilarity,
	})
	if err != nil {
		return nil, err
	}
	results := make([]SearchResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = SearchResult{
			EntityType: r.EntityType,
			EntityID:   r.EntityId,
			Similarity: r.Similarity,
			Text:       r.Text,
		}
	}
	return results, nil
}

// SearchText finds embeddings where text contains the query keywords.
// Useful for name/title searches where semantic search fails.
func (c *EmbeddingsClient) SearchText(ctx context.Context, query string, entityTypes []string, limit int) ([]SearchResult, error) {
	resp, err := c.client.SearchText(ctx, &pluginv1.TextSearchRequest{
		Query:       query,
		EntityTypes: entityTypes,
		Limit:       int32(limit),
	})
	if err != nil {
		return nil, err
	}
	results := make([]SearchResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = SearchResult{
			EntityType: r.EntityType,
			EntityID:   r.EntityId,
			Similarity: r.Similarity,
			Text:       r.Text,
		}
	}
	return results, nil
}

// Count returns the number of embeddings for an entity type.
func (c *EmbeddingsClient) Count(ctx context.Context, entityType string) (int64, error) {
	resp, err := c.client.CountByType(ctx, &pluginv1.EntityTypeQuery{EntityType: entityType})
	if err != nil {
		return 0, err
	}
	return resp.Count, nil
}

// DeleteByType removes all embeddings for an entity type.
// Returns the count of deleted embeddings.
func (c *EmbeddingsClient) DeleteByType(ctx context.Context, entityType string) (int64, error) {
	resp, err := c.client.DeleteByType(ctx, &pluginv1.EntityTypeQuery{EntityType: entityType})
	if err != nil {
		return 0, err
	}
	return resp.Count, nil
}

// StoredEmbedding represents a stored embedding.
type StoredEmbedding struct {
	EntityType string
	EntityID   int64
	Embedding  []float32
	Text       string
	Model      string
	Exists     bool
}

// SearchResult represents a search result from the embeddings store.
type SearchResult struct {
	EntityType string
	EntityID   int64
	Similarity float32
	Text       string
}

// ============================================================================
// Data Client - Access media library data (read-only)
// ============================================================================

// DataClient wraps the HostData service for media access.
type DataClient struct {
	client pluginv1.HostDataClient
}

// NewDataClient creates a new data client.
func NewDataClient(conn *grpc.ClientConn) *DataClient {
	return &DataClient{client: pluginv1.NewHostDataClient(conn)}
}

// GetMedia retrieves a single media item by ID.
func (c *DataClient) GetMedia(ctx context.Context, mediaID int64, mediaType string) (*Media, error) {
	resp, err := c.client.GetMedia(ctx, &pluginv1.MediaQuery{
		MediaId:   mediaID,
		MediaType: mediaType,
	})
	if err != nil {
		return nil, err
	}
	return protoToMedia(resp), nil
}

// GetMediaDetails retrieves full metadata for a media item.
// Includes plot, cast, genres, mood tags, etc. for AI indexing.
func (c *DataClient) GetMediaDetails(ctx context.Context, mediaID int64, mediaType string) (*MediaDetails, error) {
	resp, err := c.client.GetMediaDetails(ctx, &pluginv1.MediaQuery{
		MediaId:   mediaID,
		MediaType: mediaType,
	})
	if err != nil {
		return nil, err
	}
	return protoToMediaDetails(resp), nil
}

// ListMediaByLibrary lists all media in a library with pagination.
func (c *DataClient) ListMediaByLibrary(ctx context.Context, libraryID int64, limit, offset int) (*MediaList, error) {
	resp, err := c.client.ListMediaByLibrary(ctx, &pluginv1.ListMediaRequest{
		LibraryId: libraryID,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		return nil, err
	}
	items := make([]*MediaDetails, len(resp.Items))
	for i, item := range resp.Items {
		items[i] = protoToMediaDetails(item)
	}
	return &MediaList{
		Items:   items,
		Total:   int(resp.Total),
		HasMore: resp.HasMore,
	}, nil
}

// SetMoodTags stores mood tags for a media item.
func (c *DataClient) SetMoodTags(ctx context.Context, mediaID int64, mediaType string, tags []MoodTag) error {
	protoTags := make([]*pluginv1.MoodTag, len(tags))
	for i, t := range tags {
		protoTags[i] = &pluginv1.MoodTag{Tag: t.Tag, Confidence: t.Confidence}
	}
	_, err := c.client.SetMoodTags(ctx, &pluginv1.SetMoodTagsRequest{
		MediaId:   mediaID,
		MediaType: mediaType,
		Tags:      protoTags,
	})
	return err
}

// GetLibrary retrieves library information by ID.
func (c *DataClient) GetLibrary(ctx context.Context, libraryID int64) (*Library, error) {
	resp, err := c.client.GetLibrary(ctx, &pluginv1.LibraryId{Id: libraryID})
	if err != nil {
		return nil, err
	}
	return &Library{
		ID:        resp.Id,
		Name:      resp.Name,
		Path:      resp.Path,
		MediaType: resp.MediaType,
	}, nil
}

// Library represents a media library.
type Library struct {
	ID        int64
	Name      string
	Path      string
	MediaType string // "movie", "tv", "music"
}

// Media represents a media item.
type Media struct {
	ID          int64
	MediaType   string
	Title       string
	Year        int
	FilePath    string
	LibraryID   int64
	ExternalIDs map[string]string
}

// MediaDetails contains full metadata for AI indexing.
type MediaDetails struct {
	ID          int64
	MediaType   string
	Title       string
	Year        int
	LibraryID   int64
	ExternalIDs map[string]string

	// Rich metadata
	Plot             string
	Tagline          string
	Genres           []string
	Directors        []string
	Writers          []string
	Cast             []CastMember
	Studios          []string
	ContentRating    string
	RuntimeMinutes   int
	OriginalLanguage string
	CountryOfOrigin  string
	Producers        []string
	MoodTags         []string
	LocationKeywords []string
	ThemeKeywords    []string

	// TV-specific
	ShowTitle     string
	SeasonNumber  int
	EpisodeNumber int

	// Music-specific
	ArtistName  string
	AlbumTitle  string
	Biography   string
	Country     string
	ReleaseType string
}

// MediaList is a paginated list of media.
type MediaList struct {
	Items   []*MediaDetails
	Total   int
	HasMore bool
}

// MoodTag represents an AI-generated mood tag.
type MoodTag struct {
	Tag        string
	Confidence float32
}

// Helper functions for proto conversion
func protoToMedia(m *pluginv1.Media) *Media {
	return &Media{
		ID:          m.Id,
		MediaType:   m.MediaType,
		Title:       m.Title,
		Year:        int(m.Year),
		FilePath:    m.FilePath,
		LibraryID:   m.LibraryId,
		ExternalIDs: m.ExternalIds,
	}
}

func protoToMediaDetails(m *pluginv1.MediaDetails) *MediaDetails {
	cast := make([]CastMember, len(m.Cast))
	for i, c := range m.Cast {
		cast[i] = CastMember{Name: c.Name, Role: c.Role}
	}
	return &MediaDetails{
		ID:               m.Id,
		MediaType:        m.MediaType,
		Title:            m.Title,
		Year:             int(m.Year),
		LibraryID:        m.LibraryId,
		ExternalIDs:      m.ExternalIds,
		Plot:             m.Plot,
		Tagline:          m.Tagline,
		Genres:           m.Genres,
		Directors:        m.Directors,
		Writers:          m.Writers,
		Cast:             cast,
		Studios:          m.Studios,
		ContentRating:    m.ContentRating,
		RuntimeMinutes:   int(m.RuntimeMinutes),
		OriginalLanguage: m.OriginalLanguage,
		CountryOfOrigin:  m.CountryOfOrigin,
		Producers:        m.Producers,
		MoodTags:         m.MoodTags,
		LocationKeywords: m.LocationKeywords,
		ThemeKeywords:    m.ThemeKeywords,
		ShowTitle:        m.ShowTitle,
		SeasonNumber:     int(m.SeasonNumber),
		EpisodeNumber:    int(m.EpisodeNumber),
		ArtistName:       m.ArtistName,
		AlbumTitle:       m.AlbumTitle,
		Biography:        m.Biography,
		Country:          m.Country,
		ReleaseType:      m.ReleaseType,
	}
}

// ============================================================================
// Storage Client - Plugin-scoped key-value storage
// ============================================================================

// StorageClient wraps the HostStorage service for plugin storage.
type StorageClient struct {
	client pluginv1.HostStorageClient
}

// NewStorageClient creates a new storage client.
func NewStorageClient(conn *grpc.ClientConn) *StorageClient {
	return &StorageClient{client: pluginv1.NewHostStorageClient(conn)}
}

// Get retrieves a value from the plugin's key-value store.
func (c *StorageClient) Get(ctx context.Context, key string) ([]byte, bool, error) {
	resp, err := c.client.KVGet(ctx, &pluginv1.KVKey{Key: key})
	if err != nil {
		return nil, false, err
	}
	return resp.Value, resp.Exists, nil
}

// Set stores a value in the plugin's key-value store.
// Use ttlSeconds=0 for no expiration.
func (c *StorageClient) Set(ctx context.Context, key string, value []byte, ttlSeconds int64) error {
	_, err := c.client.KVSet(ctx, &pluginv1.KVEntry{
		Key:        key,
		Value:      value,
		TtlSeconds: ttlSeconds,
	})
	return err
}

// Delete removes a value from the plugin's key-value store.
func (c *StorageClient) Delete(ctx context.Context, key string) error {
	_, err := c.client.KVDelete(ctx, &pluginv1.KVKey{Key: key})
	return err
}

// List lists keys with an optional prefix.
func (c *StorageClient) List(ctx context.Context, prefix string, limit int) ([]string, error) {
	resp, err := c.client.KVList(ctx, &pluginv1.KVListRequest{
		Prefix: prefix,
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, err
	}
	return resp.Keys, nil
}

// GetDatabasePath returns the path to the plugin's SQLite database.
// Plugins can use this for complex data storage needs.
func (c *StorageClient) GetDatabasePath(ctx context.Context) (string, error) {
	resp, err := c.client.GetDatabasePath(ctx, &pluginv1.Empty{})
	if err != nil {
		return "", err
	}
	return resp.Path, nil
}

// ============================================================================
// Weather Client - Weather and time context
// ============================================================================

// WeatherClient wraps the HostWeather service for context enrichment.
type WeatherClient struct {
	client pluginv1.HostWeatherClient
}

// NewWeatherClient creates a new weather client.
func NewWeatherClient(conn *grpc.ClientConn) *WeatherClient {
	return &WeatherClient{client: pluginv1.NewHostWeatherClient(conn)}
}

// GetWeather returns current weather for a user's location.
// Returns nil if the user hasn't enabled location sharing.
func (c *WeatherClient) GetWeather(ctx context.Context, userID string) (*Weather, error) {
	resp, err := c.client.GetCurrentWeather(ctx, &pluginv1.WeatherRequest{UserId: userID})
	if err != nil {
		return nil, err
	}
	if !resp.Available {
		return nil, nil
	}
	return &Weather{
		Temperature:   resp.Temperature,
		Humidity:      int(resp.Humidity),
		IsDay:         resp.IsDay,
		Precipitation: resp.Precipitation,
		CloudCover:    int(resp.CloudCover),
		WeatherCode:   int(resp.WeatherCode),
		Condition:     resp.Condition,
		TimeOfDay:     resp.TimeOfDay,
		Season:        resp.Season,
	}, nil
}

// Weather contains current weather and time context.
type Weather struct {
	Temperature   float32 // Celsius
	Humidity      int     // Percentage
	IsDay         bool
	Precipitation float32 // mm
	CloudCover    int     // Percentage
	WeatherCode   int     // WMO code
	Condition     string  // sunny, cloudy, rainy, snowy, stormy, foggy
	TimeOfDay     string  // morning, afternoon, evening, night
	Season        string  // spring, summer, fall, winter
}
