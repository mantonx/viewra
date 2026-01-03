// Enricher plugin support for ViewRA metadata enricher plugins.
//
// This file provides the EnricherPlugin interface and ServeEnricher() helper
// for building enricher plugins (like TMDb, MusicBrainz).
//
// # Quick Start
//
// Create an enricher plugin that implements the EnricherPlugin interface:
//
//	type MyEnricher struct {
//	    sdk.Base
//	    storage *sdk.StorageClient
//	}
//
//	func (e *MyEnricher) GetCapabilities() sdk.EnricherCapabilities {
//	    return sdk.EnricherCapabilities{
//	        MediaTypes: []string{"movie", "tv"},
//	        Provides:   []string{"metadata", "artwork"},
//	    }
//	}
//
//	func (e *MyEnricher) Initialize(ctx context.Context, dataDir string, config []byte, services *sdk.HostServices) error {
//	    e.storage = services.Storage // Use host storage for caching
//	    return nil
//	}
//
//	// ... implement other methods
//
//	func main() {
//	    hclogger, logger := sdk.NewLogger("my-enricher")
//	    plugin := &MyEnricher{}
//	    plugin.SetLogger(logger)
//	    sdk.ServeEnricher(plugin, hclogger)
//	}
package sdk

import (
	"context"
	"io"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// EnricherPlugin is the interface that enricher plugins must implement.
// Plugin authors implement this interface and use ServeEnricher() to run the plugin.
//
// Plugin identity comes from plugin.yml manifest file, not code.
type EnricherPlugin interface {
	// mustEmbedBase ensures plugins embed sdk.Base
	mustEmbedBase()

	// GetCapabilities returns what this enricher provides and requires.
	GetCapabilities() EnricherCapabilities

	// Initialize is called when the plugin is loaded.
	// Config is the contents of config.yml passed by the host.
	// Services provides access to host services (storage, LLM, etc.) - may have nil fields.
	Initialize(ctx context.Context, dataDir string, config []byte, services *HostServices) error

	// Shutdown is called before the plugin is unloaded.
	// Use this to clean up any resources.
	Shutdown(ctx context.Context) error

	// Enrich processes a single media item.
	Enrich(ctx context.Context, req *EnrichRequest) (*EnrichResponse, error)
}

// ConfigurableEnricher is an optional interface for enrichers that support runtime configuration.
type ConfigurableEnricher interface {
	// GetSettingsSchema returns a JSON Schema describing the plugin's configurable settings.
	// Use Schema.Build() to generate this from your schema definition.
	GetSettingsSchema() ([]byte, error)

	// Configure applies new settings to the plugin.
	Configure(settings []byte) error
}

// HTTPEnricher is an optional interface for enrichers that expose HTTP routes.
type HTTPEnricher interface {
	// GetRoutes returns HTTP routes this enricher exposes.
	GetRoutes() []Route

	// HandleHTTP handles a non-streaming HTTP request.
	HandleHTTP(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error)
}

// HostServices provides access to host-provided services.
// Fields may be nil if the service is not available.
//
// For AI capabilities (embeddings, chat), use the Plugins client with capability-based
// discovery. Call Plugins.GetConnection("embeddings") or Plugins.GetConnection("chat")
// to get a connection to a provider plugin.
type HostServices struct {
	// Storage provides key-value, SQL, and vector storage for plugins
	Storage *StorageClient

	// Data provides access to media database
	Data *DataClient

	// Weather provides weather/location data
	Weather *WeatherClient

	// Plugins provides capability-based access to other plugins
	Plugins *PluginsClient

	// Ratings provides read-only access to user ratings (favorites, likes, dislikes)
	Ratings *RatingsClient
}

// EnricherCapabilities describes what an enricher provides and requires.
type EnricherCapabilities struct {
	MediaTypes []string // "movie", "tv", "music"
	Provides   []string // "metadata", "artwork", "external_ids"
	IsLocal    bool     // Local enrichers get high concurrency
	RateLimit  int      // Requests per minute (0 = unlimited)
	Requires   []string // External IDs required (e.g., ["imdb"])
	Priority   int      // Execution priority (lower = earlier)
}

// EnrichRequest contains all information needed to enrich a media item.
type EnrichRequest struct {
	MediaID     int64
	MediaType   string
	FilePath    string
	Title       string
	Year        int
	ExistingIDs map[string]string

	// TV-specific
	ShowTitle     string
	SeasonNumber  int
	EpisodeNumber int

	// Music-specific
	Artist      string
	Album       string
	TrackNumber int
}

// EnrichResponse contains the results of enrichment.
type EnrichResponse struct {
	Matched         bool
	Metadata        *EnrichedMetadata
	DiscoveredIDs   map[string]string
	Images          []EnrichedImage
	Skipped         bool
	SkipReason      string
	ConfidenceScore float32 // 0.0 to 1.0
}

// EnrichedMetadata contains metadata fields that can be updated.
type EnrichedMetadata struct {
	Title          *string
	OriginalTitle  *string
	SortTitle      *string
	Year           *int
	Plot           *string
	Tagline        *string
	Genres         []string
	Keywords       []Keyword
	ContentRating  *string
	RuntimeMinutes *int
	Rating         *float32
	RatingVotes    *int
	Directors      []string
	Writers        []string
	Cast           []CastMember
	Studios        []string

	// Music-specific fields (for tracks)
	Artist      *string
	Album       *string
	ReleaseDate *string

	// Music album metadata
	AlbumMetadata *AlbumMetadata

	// Music artist metadata
	ArtistMetadata *ArtistMetadata
}

// AlbumMetadata contains fields specific to music albums.
type AlbumMetadata struct {
	Title              *string
	AlbumArtist        *string
	Artist             *string
	Year               *int
	ReleaseDate        *string
	Genre              *string
	TotalTracks        *int
	TotalDiscs         *int
	RecordLabel        *string
	ReleaseType        *string
	Compilation        *bool
	MusicbrainzAlbumID *string
	SortTitle          *string
}

// ArtistMetadata contains fields specific to music artists.
type ArtistMetadata struct {
	Name                *string
	SortName            *string
	MusicbrainzArtistID *string
	Bio                 *string
	Country             *string
	FormedYear          *int
	Genre               *string
}

// Keyword represents a keyword/tag for a media item.
type Keyword struct {
	ID         int
	Name       string
	IsLocation bool
}

// CastMember represents an actor in the cast.
type CastMember struct {
	Name  string
	Role  string
	Thumb string
	Order int
}

// EnrichedImage represents an image discovered by an enricher.
type EnrichedImage struct {
	Type     string // "poster", "fanart", "banner", etc.
	Path     string // Local path or remote URL
	IsRemote bool
	Language string
	Width    int
	Height   int
	Rating   float32
}

// Skip creates a skipped response with the given reason.
func Skip(reason string) *EnrichResponse {
	return &EnrichResponse{
		Skipped:    true,
		SkipReason: reason,
	}
}

// NoMatch creates a response indicating no match was found.
func NoMatch() *EnrichResponse {
	return &EnrichResponse{
		Matched: false,
	}
}

// Match creates a successful match response.
func Match() *EnrichResponse {
	return &EnrichResponse{
		Matched:       true,
		DiscoveredIDs: make(map[string]string),
	}
}

// --- gRPC Server Implementation ---

// enricherGRPCServer wraps an EnricherPlugin to implement the gRPC service.
type enricherGRPCServer struct {
	pluginv1.UnimplementedPluginCoreServer
	pluginv1.UnimplementedEnricherServer
	impl     EnricherPlugin
	base     *Base
	broker   *plugin.GRPCBroker
	services *HostServices
}

func (s *enricherGRPCServer) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
	s.base.Init(req.DataDir)

	// Connect to host services
	s.services = &HostServices{}
	s.connectHostServices(req)

	if err := s.impl.Initialize(ctx, req.DataDir, req.Config, s.services); err != nil {
		return &pluginv1.InitResponse{Success: false, Error: err.Error()}, nil
	}
	return &pluginv1.InitResponse{Success: true}, nil
}

func (s *enricherGRPCServer) connectHostServices(req *pluginv1.InitRequest) {
	logger := s.base.Log()

	// Storage service
	if req.HostStorageBrokerId > 0 {
		conn, err := s.broker.Dial(req.HostStorageBrokerId)
		if err != nil {
			logger.Error("failed to dial host storage", "error", err)
		} else {
			s.services.Storage = &StorageClient{client: pluginv1.NewHostStorageClient(conn)}
			logger.Debug("connected to host storage service")
		}
	}

	// LLM service
	// Data service
	if req.HostDataBrokerId > 0 {
		conn, err := s.broker.Dial(req.HostDataBrokerId)
		if err != nil {
			logger.Error("failed to dial host data", "error", err)
		} else {
			s.services.Data = &DataClient{client: pluginv1.NewHostDataClient(conn)}
			logger.Debug("connected to host data service")
		}
	}

	// Weather service
	if req.HostWeatherBrokerId > 0 {
		conn, err := s.broker.Dial(req.HostWeatherBrokerId)
		if err != nil {
			logger.Error("failed to dial host weather", "error", err)
		} else {
			s.services.Weather = &WeatherClient{client: pluginv1.NewHostWeatherClient(conn)}
			logger.Debug("connected to host weather service")
		}
	}

	// Plugins service (for capability-based plugin discovery)
	if req.HostPluginsBrokerId > 0 {
		conn, err := s.broker.Dial(req.HostPluginsBrokerId)
		if err != nil {
			logger.Error("failed to dial host plugins", "error", err)
		} else {
			s.services.Plugins = NewPluginsClient(conn)
			logger.Debug("connected to host plugins service")
		}
	}

	// Ratings service (for user ratings access)
	if req.HostRatingsBrokerId > 0 {
		conn, err := s.broker.Dial(req.HostRatingsBrokerId)
		if err != nil {
			logger.Error("failed to dial host ratings", "error", err)
		} else {
			s.services.Ratings = NewRatingsClient(conn)
			logger.Debug("connected to host ratings service")
		}
	}
}

func (s *enricherGRPCServer) Shutdown(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.Empty, error) {
	if err := s.impl.Shutdown(ctx); err != nil {
		s.base.Log().Error("shutdown error", "error", err)
	}
	return &pluginv1.Empty{}, nil
}

func (s *enricherGRPCServer) HealthCheck(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.HealthStatus, error) {
	metrics := s.base.Metrics()
	return &pluginv1.HealthStatus{
		Status:        pluginv1.HealthStatus_HEALTHY,
		RequestsTotal: metrics.RequestsTotal,
		ErrorsTotal:   metrics.ErrorsTotal,
		AvgLatencyMs:  float64(metrics.AvgLatency.Milliseconds()),
	}, nil
}

func (s *enricherGRPCServer) GetSettingsSchema(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.SettingsSchema, error) {
	if configurable, ok := s.impl.(ConfigurableEnricher); ok {
		schema, err := configurable.GetSettingsSchema()
		if err != nil {
			return nil, err
		}
		return &pluginv1.SettingsSchema{JsonSchema: schema}, nil
	}
	return &pluginv1.SettingsSchema{}, nil
}

func (s *enricherGRPCServer) Configure(ctx context.Context, settings *pluginv1.Settings) (*pluginv1.ConfigureResponse, error) {
	if configurable, ok := s.impl.(ConfigurableEnricher); ok {
		if err := configurable.Configure(settings.Json); err != nil {
			return &pluginv1.ConfigureResponse{Success: false, Error: err.Error()}, nil
		}
		return &pluginv1.ConfigureResponse{Success: true}, nil
	}
	return &pluginv1.ConfigureResponse{Success: true}, nil
}

func (s *enricherGRPCServer) GetSubscriptions(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.EventSubscriptions, error) {
	return &pluginv1.EventSubscriptions{}, nil
}

func (s *enricherGRPCServer) OnEvent(ctx context.Context, event *pluginv1.Event) (*pluginv1.EventResponse, error) {
	return &pluginv1.EventResponse{Handled: false}, nil
}

func (s *enricherGRPCServer) GetRoutes(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.PluginRoutes, error) {
	if httpEnricher, ok := s.impl.(HTTPEnricher); ok {
		routes := httpEnricher.GetRoutes()
		protoRoutes := make([]*pluginv1.PluginRoute, len(routes))
		for i, r := range routes {
			protoRoutes[i] = &pluginv1.PluginRoute{
				Path:        r.Path,
				Methods:     r.Methods,
				AdminOnly:   r.AdminOnly,
				Description: r.Description,
				Streaming:   r.Streaming,
				AliasPath:   r.AliasPath,
			}
		}
		return &pluginv1.PluginRoutes{Routes: protoRoutes}, nil
	}
	return &pluginv1.PluginRoutes{}, nil
}

func (s *enricherGRPCServer) HandleHTTP(ctx context.Context, req *pluginv1.PluginHTTPRequest) (*pluginv1.PluginHTTPResponse, error) {
	if httpEnricher, ok := s.impl.(HTTPEnricher); ok {
		sdkReq := protoToHTTPRequest(req)
		resp, err := httpEnricher.HandleHTTP(ctx, sdkReq)
		if err != nil {
			return &pluginv1.PluginHTTPResponse{
				StatusCode:  500,
				ContentType: "application/json",
				Body:        []byte(`{"error":"` + err.Error() + `"}`),
			}, nil
		}
		return httpResponseToProto(resp), nil
	}
	return &pluginv1.PluginHTTPResponse{
		StatusCode:  404,
		ContentType: "application/json",
		Body:        []byte(`{"error":"not found"}`),
	}, nil
}

func (s *enricherGRPCServer) HandleHTTPStream(stream pluginv1.PluginCore_HandleHTTPStreamServer) error {
	// Enrichers typically don't need streaming HTTP, but we implement it for completeness
	// First, receive the request
	chunk, err := stream.Recv()
	if err != nil {
		return err
	}
	if chunk.Type != pluginv1.PluginHTTPChunk_REQUEST_START {
		return sendEnricherStreamError(stream, 400, "expected request start")
	}

	// Just return 404 - enrichers don't support streaming HTTP by default
	return sendEnricherStreamError(stream, 404, "streaming not supported")
}

func sendEnricherStreamError(stream pluginv1.PluginCore_HandleHTTPStreamServer, status int32, msg string) error {
	if err := stream.Send(&pluginv1.PluginHTTPChunk{
		Type:        pluginv1.PluginHTTPChunk_RESPONSE_START,
		StatusCode:  status,
		ContentType: "application/json",
	}); err != nil {
		return err
	}
	if err := stream.Send(&pluginv1.PluginHTTPChunk{
		Type: pluginv1.PluginHTTPChunk_RESPONSE_BODY,
		Data: []byte(`{"error":"` + msg + `"}`),
	}); err != nil {
		return err
	}
	return stream.Send(&pluginv1.PluginHTTPChunk{
		Type: pluginv1.PluginHTTPChunk_RESPONSE_END,
	})
}

func (s *enricherGRPCServer) GetCapabilities(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.EnricherCapabilities, error) {
	caps := s.impl.GetCapabilities()
	return &pluginv1.EnricherCapabilities{
		MediaTypes: caps.MediaTypes,
		Provides:   caps.Provides,
		IsLocal:    caps.IsLocal,
		RateLimit:  int32(caps.RateLimit),
		Requires:   caps.Requires,
		Priority:   int32(caps.Priority),
	}, nil
}

func (s *enricherGRPCServer) Enrich(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
	start := time.Now()

	// Convert proto request to SDK request
	sdkReq := protoToSDKEnrichRequest(req)

	// Call the plugin implementation
	resp, err := s.impl.Enrich(ctx, sdkReq)

	latency := time.Since(start)
	if err != nil {
		s.base.RecordError()
		return nil, err
	}
	s.base.RecordRequest(latency)

	// Convert SDK response to proto response
	return sdkToProtoEnrichResponse(resp), nil
}

// --- Conversion helpers ---

func protoToSDKEnrichRequest(req *pluginv1.EnrichRequest) *EnrichRequest {
	result := &EnrichRequest{
		MediaID:     req.MediaId,
		MediaType:   req.MediaType,
		FilePath:    req.FilePath,
		Title:       req.Title,
		Year:        int(req.Year),
		ExistingIDs: req.ExistingIds,
	}

	if req.Tv != nil {
		result.ShowTitle = req.Tv.ShowTitle
		result.SeasonNumber = int(req.Tv.SeasonNumber)
		result.EpisodeNumber = int(req.Tv.EpisodeNumber)
	}

	if req.Music != nil {
		result.Artist = req.Music.Artist
		result.Album = req.Music.Album
		result.TrackNumber = int(req.Music.TrackNumber)
	}

	return result
}

func sdkToProtoEnrichResponse(resp *EnrichResponse) *pluginv1.EnrichResponse {
	if resp == nil {
		return nil
	}

	result := &pluginv1.EnrichResponse{
		Matched:       resp.Matched,
		DiscoveredIds: resp.DiscoveredIDs,
		Skipped:       resp.Skipped,
		SkipReason:    resp.SkipReason,
		Confidence:    resp.ConfidenceScore,
	}

	if resp.Metadata != nil {
		result.Metadata = sdkToProtoEnrichMetadata(resp.Metadata)
	}

	for _, img := range resp.Images {
		result.Images = append(result.Images, &pluginv1.EnrichedImage{
			Type:     img.Type,
			Path:     img.Path,
			IsRemote: img.IsRemote,
			Language: img.Language,
			Width:    int32(img.Width),
			Height:   int32(img.Height),
			Rating:   img.Rating,
		})
	}

	return result
}

func sdkToProtoEnrichMetadata(md *EnrichedMetadata) *pluginv1.EnrichedMetadata {
	if md == nil {
		return nil
	}

	result := &pluginv1.EnrichedMetadata{
		Genres:    md.Genres,
		Directors: md.Directors,
		Writers:   md.Writers,
		Studios:   md.Studios,
	}

	if md.Title != nil {
		result.Title = md.Title
	}
	if md.OriginalTitle != nil {
		result.OriginalTitle = md.OriginalTitle
	}
	if md.SortTitle != nil {
		result.SortTitle = md.SortTitle
	}
	if md.Year != nil {
		year := int32(*md.Year)
		result.Year = &year
	}
	if md.Plot != nil {
		result.Plot = md.Plot
	}
	if md.Tagline != nil {
		result.Tagline = md.Tagline
	}
	if md.ContentRating != nil {
		result.ContentRating = md.ContentRating
	}
	if md.RuntimeMinutes != nil {
		runtime := int32(*md.RuntimeMinutes)
		result.RuntimeMinutes = &runtime
	}
	if md.Rating != nil {
		result.Rating = md.Rating
	}
	if md.RatingVotes != nil {
		votes := int32(*md.RatingVotes)
		result.RatingVotes = &votes
	}

	for _, c := range md.Cast {
		result.Cast = append(result.Cast, &pluginv1.CastMember{
			Name:  c.Name,
			Role:  c.Role,
			Thumb: c.Thumb,
			Order: int32(c.Order),
		})
	}

	for _, kw := range md.Keywords {
		result.Keywords = append(result.Keywords, &pluginv1.Keyword{
			Id:         int32(kw.ID),
			Name:       kw.Name,
			IsLocation: kw.IsLocation,
		})
	}

	// Music-specific fields
	if md.Artist != nil {
		result.Artist = md.Artist
	}
	if md.Album != nil {
		result.Album = md.Album
	}
	if md.ReleaseDate != nil {
		result.ReleaseDate = md.ReleaseDate
	}

	// Album metadata
	if md.AlbumMetadata != nil {
		result.AlbumMetadata = sdkToProtoAlbumMetadata(md.AlbumMetadata)
	}

	// Artist metadata
	if md.ArtistMetadata != nil {
		result.ArtistMetadata = sdkToProtoArtistMetadata(md.ArtistMetadata)
	}

	return result
}

func sdkToProtoAlbumMetadata(am *AlbumMetadata) *pluginv1.AlbumMetadata {
	if am == nil {
		return nil
	}
	result := &pluginv1.AlbumMetadata{}
	result.Title = am.Title
	result.AlbumArtist = am.AlbumArtist
	result.Artist = am.Artist
	if am.Year != nil {
		y := int32(*am.Year)
		result.Year = &y
	}
	result.ReleaseDate = am.ReleaseDate
	result.Genre = am.Genre
	if am.TotalTracks != nil {
		t := int32(*am.TotalTracks)
		result.TotalTracks = &t
	}
	if am.TotalDiscs != nil {
		d := int32(*am.TotalDiscs)
		result.TotalDiscs = &d
	}
	result.RecordLabel = am.RecordLabel
	result.ReleaseType = am.ReleaseType
	result.Compilation = am.Compilation
	result.MusicbrainzAlbumId = am.MusicbrainzAlbumID
	result.SortTitle = am.SortTitle
	return result
}

func sdkToProtoArtistMetadata(am *ArtistMetadata) *pluginv1.ArtistMetadata {
	if am == nil {
		return nil
	}
	result := &pluginv1.ArtistMetadata{}
	result.Name = am.Name
	result.SortName = am.SortName
	result.MusicbrainzArtistId = am.MusicbrainzArtistID
	result.Bio = am.Bio
	result.Country = am.Country
	if am.FormedYear != nil {
		y := int32(*am.FormedYear)
		result.FormedYear = &y
	}
	result.Genre = am.Genre
	return result
}

// --- go-plugin integration ---

// EnricherCoreGRPCPlugin is the go-plugin for PluginCore service (enrichers).
type EnricherCoreGRPCPlugin struct {
	plugin.Plugin
	Impl   EnricherPlugin
	base   *Base
	broker *plugin.GRPCBroker
}

func (p *EnricherCoreGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	if p.base == nil {
		p.base = &Base{}
	}
	p.broker = broker
	server := &enricherGRPCServer{
		impl:   p.Impl,
		base:   p.base,
		broker: broker,
	}
	pluginv1.RegisterPluginCoreServer(s, server)
	return nil
}

func (p *EnricherCoreGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewPluginCoreClient(c), nil
}

// EnricherGRPCPlugin is the go-plugin for Enricher service.
type EnricherGRPCPlugin struct {
	plugin.Plugin
	Impl   EnricherPlugin
	base   *Base
	broker *plugin.GRPCBroker
}

func (p *EnricherGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	if p.base == nil {
		p.base = &Base{}
	}
	server := &enricherGRPCServer{
		impl:   p.Impl,
		base:   p.base,
		broker: broker,
	}
	pluginv1.RegisterEnricherServer(s, server)
	return nil
}

func (p *EnricherGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewEnricherClient(c), nil
}

// HostServiceGRPCPlugins are empty plugins that allow the host to serve services to plugins.
// These are registered by the plugin but served by the host.

type HostStorageGRPCPlugin struct{ plugin.Plugin }

func (p *HostStorageGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	return nil
}
func (p *HostStorageGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewHostStorageClient(c), nil
}

type HostDataGRPCPlugin struct{ plugin.Plugin }

func (p *HostDataGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	return nil
}
func (p *HostDataGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewHostDataClient(c), nil
}

type HostWeatherGRPCPlugin struct{ plugin.Plugin }

func (p *HostWeatherGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	return nil
}
func (p *HostWeatherGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewHostWeatherClient(c), nil
}

type HostPluginsGRPCPlugin struct{ plugin.Plugin }

func (p *HostPluginsGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	return nil
}
func (p *HostPluginsGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return pluginv1.NewHostPluginsClient(c), nil
}

// --- ServeEnricher ---

// ServeEnricher starts an enricher plugin server.
// Call this from your plugin's main() function.
//
// If the plugin also implements TrendingProvider, it will automatically
// register the trending_provider service.
//
// Example:
//
//	func main() {
//	    hclogger, logger := sdk.NewLogger("my-enricher")
//	    p := NewMyEnricher()
//	    p.SetLogger(logger)
//	    sdk.ServeEnricher(p, hclogger)
//	}
func ServeEnricher(impl EnricherPlugin, logger hclog.Logger) {
	base := &Base{}
	plugins := map[string]plugin.Plugin{
		"core":         &EnricherCoreGRPCPlugin{Impl: impl, base: base},
		"enricher":     &EnricherGRPCPlugin{Impl: impl, base: base},
		"host_storage": &HostStorageGRPCPlugin{},
		"host_data":    &HostDataGRPCPlugin{},
		"host_weather": &HostWeatherGRPCPlugin{},
		"host_plugins": &HostPluginsGRPCPlugin{},
	}

	// If the enricher also implements TrendingProvider, register it
	if tp, ok := impl.(TrendingProvider); ok {
		plugins["trending_provider"] = &TrendingProviderGRPCPlugin{Impl: tp}
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         plugins,
		GRPCServer:      plugin.DefaultGRPCServer,
		Logger:          logger,
	})
}

// ServeEnricherWithExtra starts an enricher plugin server with additional gRPC services.
// Use this when your plugin exposes additional interfaces beyond the standard enricher.
//
// If the plugin also implements TrendingProvider, it will automatically
// register the trending_provider service.
//
// Example (vector-search plugin):
//
//	func main() {
//	    hclogger, logger := sdk.NewLogger("semantic-search")
//	    p := NewVectorSearchPlugin()
//	    p.SetLogger(logger)
//	    sdk.ServeEnricherWithExtra(p, hclogger, map[string]plugin.Plugin{
//	        "vector_search": &VectorSearchGRPCPlugin{Impl: p},
//	    })
//	}
func ServeEnricherWithExtra(impl EnricherPlugin, logger hclog.Logger, extra map[string]plugin.Plugin) {
	base := &Base{}
	plugins := map[string]plugin.Plugin{
		"core":         &EnricherCoreGRPCPlugin{Impl: impl, base: base},
		"enricher":     &EnricherGRPCPlugin{Impl: impl, base: base},
		"host_storage": &HostStorageGRPCPlugin{},
		"host_data":    &HostDataGRPCPlugin{},
		"host_weather": &HostWeatherGRPCPlugin{},
		"host_plugins": &HostPluginsGRPCPlugin{},
	}

	// If the enricher also implements TrendingProvider, register it
	if tp, ok := impl.(TrendingProvider); ok {
		plugins["trending_provider"] = &TrendingProviderGRPCPlugin{Impl: tp}
	}

	// Add extra plugins
	for name, p := range extra {
		plugins[name] = p
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         plugins,
		GRPCServer:      plugin.DefaultGRPCServer,
		Logger:          logger,
	})
}

// Ensure HTTPStreamWriter interface is satisfied
var _ HTTPStreamWriter = (*enricherStreamWriter)(nil)

type enricherStreamWriter struct {
	stream  pluginv1.PluginCore_HandleHTTPStreamServer
	started bool
}

func (w *enricherStreamWriter) WriteHeader(statusCode int, contentType string, headers map[string]string) error {
	if headers == nil {
		headers = make(map[string]string)
	}
	w.started = true
	return w.stream.Send(&pluginv1.PluginHTTPChunk{
		Type:        pluginv1.PluginHTTPChunk_RESPONSE_START,
		StatusCode:  int32(statusCode),
		ContentType: contentType,
		Headers:     headers,
	})
}

func (w *enricherStreamWriter) WriteChunk(data []byte) error {
	return w.stream.Send(&pluginv1.PluginHTTPChunk{
		Type: pluginv1.PluginHTTPChunk_RESPONSE_BODY,
		Data: data,
	})
}

// Compile-time interface check
var _ io.Writer = (*enricherStreamWriter)(nil)

func (w *enricherStreamWriter) Write(p []byte) (n int, err error) {
	if err := w.WriteChunk(p); err != nil {
		return 0, err
	}
	return len(p), nil
}
