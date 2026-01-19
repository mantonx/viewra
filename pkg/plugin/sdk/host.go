// Host service client wrappers for ViewRA plugins.
//
// This file provides type-safe wrappers around the host services available
// to plugins. These services are exposed by the host and allow plugins to
// access data, storage, and more.
//
// # Available Services
//
// The host exposes these services to plugins:
//
//   - HostData: Access media library data (read-only)
//   - HostStorage: Plugin-scoped key-value, SQL, and vector storage
//   - HostPlugins: Capability-based plugin discovery and inter-plugin communication
//   - HostWeather: Weather and time context for search queries
//   - HostRatings: User ratings (favorites, likes, dislikes) for recommendations
//
// # AI Capabilities
//
// For AI functionality (embeddings, chat), use PluginsClient to discover and
// connect to provider plugins that offer "embeddings" or "chat" capabilities.
// See PluginsClient.GetConnection() for details.
//
// # Usage
//
// Plugins receive broker IDs in the InitRequest. Use these to connect:
//
//	func (p *MyPlugin) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
//	    if req.HostStorageBrokerId > 0 {
//	        conn, _ := broker.Dial(req.HostStorageBrokerId)
//	        p.storage = sdk.NewStorageClient(conn)
//	    }
//	    if req.HostPluginsBrokerId > 0 {
//	        conn, _ := broker.Dial(req.HostPluginsBrokerId)
//	        // PluginsClient provides capability invocation via host-proxied generic invoke
//	        p.plugins = sdk.NewPluginsClient(conn)
//	    }
//	    return &pluginv1.InitResponse{Success: true}, nil
//	}
package sdk

import (
	"context"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"google.golang.org/grpc"
)

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
// Includes plot, cast, genres, etc. for plugin indexing.
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

// ListMediaByGenre returns media items matching a genre pattern.
// Used for genre-based recommendations when semantic search is unavailable.
//
// Parameters:
//   - mediaType: "movie" or "tv_show"
//   - genre: genre pattern to match (e.g., "Action", "Comedy")
//   - libraryID: library to filter by (0 = all libraries)
//   - excludeIDs: entity IDs to exclude from results
//   - limit: maximum results (default: 20, max: 100)
//
// Example:
//
//	items, err := data.ListMediaByGenre(ctx, "movie", "Action", 0, []int64{1, 2}, 20)
//	for _, item := range items {
//	    fmt.Printf("Found: %s (%d)\n", item.Title, item.Year)
//	}
func (c *DataClient) ListMediaByGenre(ctx context.Context, mediaType, genre string, libraryID int64, excludeIDs []int64, limit int) ([]*Media, error) {
	resp, err := c.client.ListMediaByGenre(ctx, &pluginv1.ListMediaByGenreRequest{
		MediaType:  mediaType,
		Genre:      genre,
		LibraryId:  libraryID,
		ExcludeIds: excludeIDs,
		Limit:      int32(limit),
	})
	if err != nil {
		return nil, err
	}

	items := make([]*Media, len(resp.Items))
	for i, item := range resp.Items {
		items[i] = protoToMedia(item)
	}
	return items, nil
}

// ListMediaByDirector returns media items directed by a specific person.
// Used for director-based recommendations and themed collections.
//
// Parameters:
//   - mediaType: "movie" or "tv_show"
//   - directorName: director name to match (e.g., "Christopher Nolan", "Spielberg")
//   - libraryID: library to filter by (0 = all libraries)
//   - excludeIDs: entity IDs to exclude from results
//   - limit: maximum results (default: 20, max: 100)
//
// Example:
//
//	items, err := data.ListMediaByDirector(ctx, "movie", "Christopher Nolan", 0, []int64{}, 20)
//	for _, item := range items {
//	    fmt.Printf("Found: %s (%d)\n", item.Title, item.Year)
//	}
func (c *DataClient) ListMediaByDirector(ctx context.Context, mediaType, directorName string, libraryID int64, excludeIDs []int64, limit int) ([]*Media, error) {
	resp, err := c.client.ListMediaByDirector(ctx, &pluginv1.ListMediaByDirectorRequest{
		MediaType:    mediaType,
		DirectorName: directorName,
		LibraryId:    libraryID,
		ExcludeIds:   excludeIDs,
		Limit:        int32(limit),
	})
	if err != nil {
		return nil, err
	}

	items := make([]*Media, len(resp.Items))
	for i, item := range resp.Items {
		items[i] = protoToMedia(item)
	}
	return items, nil
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

// MediaDetails contains full metadata for plugin indexing.
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
	LocationKeywords []string
	ThemeKeywords    []string
	Composers        []string // Music composers
	Cinematographers []string // Directors of Photography

	// Playback information for filtering by technical specs
	PlaybackInfo *PlaybackInfo

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

// PlaybackInfo contains technical playback metadata for filtering.
// Used for queries like "4K movies", "Dolby Vision content", "movies with subtitles".
type PlaybackInfo struct {
	// Video specs
	Width           int    // Video width in pixels
	Height          int    // Video height in pixels
	ResolutionLabel string // "SD", "720p", "1080p", "4K", "8K"
	HDRFormat       string // "SDR", "HDR10", "HDR10+", "Dolby Vision", "HLG"
	VideoCodec      string // "h264", "hevc", "av1", etc.
	Bitrate         int64  // Video bitrate in bits/second

	// Audio and subtitle tracks
	AudioTracks    []AudioTrack
	SubtitleTracks []SubtitleTrack
}

// AudioTrack represents an audio stream in a media file.
type AudioTrack struct {
	Codec         string // "aac", "ac3", "eac3", "truehd", "dts", "dts-hd", "flac"
	Channels      int    // Number of channels (2, 6, 8, etc.)
	ChannelLayout string // "stereo", "5.1", "7.1", "Atmos"
	Language      string // ISO 639-1/2 code, e.g., "en", "eng"
	IsDefault     bool
	IsCommentary  bool
}

// SubtitleTrack represents a subtitle stream in a media file.
type SubtitleTrack struct {
	Language   string // ISO 639-1/2 code
	Codec      string // "srt", "ass", "pgs", "vobsub"
	IsForced   bool
	IsSDH      bool // Subtitles for Deaf/Hard of Hearing
	IsExternal bool // External file vs embedded
}

// MediaList is a paginated list of media.
type MediaList struct {
	Items   []*MediaDetails
	Total   int
	HasMore bool
}

// MoodTag represents a generated mood tag.
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
		LocationKeywords: m.LocationKeywords,
		ThemeKeywords:    m.ThemeKeywords,
		Composers:        m.Composers,
		Cinematographers: m.Cinematographers,
		PlaybackInfo:     protoToPlaybackInfo(m.PlaybackInfo),
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

// protoToPlaybackInfo converts proto PlaybackInfo to SDK PlaybackInfo.
func protoToPlaybackInfo(p *pluginv1.PlaybackInfo) *PlaybackInfo {
	if p == nil {
		return nil
	}

	audioTracks := make([]AudioTrack, len(p.AudioTracks))
	for i, t := range p.AudioTracks {
		audioTracks[i] = AudioTrack{
			Codec:         t.Codec,
			Channels:      int(t.Channels),
			ChannelLayout: t.ChannelLayout,
			Language:      t.Language,
			IsDefault:     t.IsDefault,
			IsCommentary:  t.IsCommentary,
		}
	}

	subtitleTracks := make([]SubtitleTrack, len(p.SubtitleTracks))
	for i, t := range p.SubtitleTracks {
		subtitleTracks[i] = SubtitleTrack{
			Language:   t.Language,
			Codec:      t.Codec,
			IsForced:   t.IsForced,
			IsSDH:      t.IsSdh,
			IsExternal: t.IsExternal,
		}
	}

	return &PlaybackInfo{
		Width:           int(p.Width),
		Height:          int(p.Height),
		ResolutionLabel: p.ResolutionLabel,
		HDRFormat:       p.HdrFormat,
		VideoCodec:      p.VideoCodec,
		Bitrate:         p.Bitrate,
		AudioTracks:     audioTracks,
		SubtitleTracks:  subtitleTracks,
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
//
// Deprecated: Use SQL() instead for managed storage. Direct database access
// requires plugins to bundle their own SQLite driver and manage connections.
func (c *StorageClient) GetDatabasePath(ctx context.Context) (string, error) {
	resp, err := c.client.GetDatabasePath(ctx, &pluginv1.Empty{})
	if err != nil {
		return "", err
	}
	return resp.Path, nil
}

// SQL returns a client for managed SQL storage.
// All table names are automatically prefixed with plugin_{id}_ by the host.
//
// Example:
//
//	db := storage.SQL()
//	db.Exec(ctx, `CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`)
//	db.Exec(ctx, `INSERT INTO items (name) VALUES (?)`, "test")
//	rows, _ := db.Query(ctx, `SELECT id, name FROM items`)
func (c *StorageClient) SQL() *SQLClient {
	return newSQLClient(c.client)
}

// Vector returns a client for managed vector storage.
// Embeddings are automatically indexed for fast similarity search.
// Uses pgvector (Postgres) or sqlite-vec (SQLite) under the hood.
//
// Example:
//
//	vec := storage.Vector()
//	vec.Store(ctx, sdk.Embedding{EntityType: "movie", EntityID: 123, Vector: embedding})
//	results, _ := vec.Search(ctx, sdk.VectorSearchRequest{QueryVector: query, Limit: 10})
func (c *StorageClient) Vector() *VectorClient {
	return newVectorClient(c.client)
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
