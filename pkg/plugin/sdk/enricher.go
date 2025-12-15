package sdk

import (
	"context"
	"time"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
)

// EnricherPlugin is the interface that enricher plugins must implement.
// Plugin authors implement this interface and use Serve() to run the plugin.
//
// Plugin identity comes from plugin.yml manifest file, not code.
type EnricherPlugin interface {
	// mustEmbedBase ensures plugins embed sdk.Base
	mustEmbedBase()

	// GetCapabilities returns what this enricher provides and requires.
	GetCapabilities() EnricherCapabilities

	// Initialize is called when the plugin is loaded.
	// Config is the contents of config.yml passed by the host.
	Initialize(ctx context.Context, dataDir string, config []byte) error

	// Shutdown is called before the plugin is unloaded.
	// Use this to clean up any resources.
	Shutdown(ctx context.Context) error

	// Enrich processes a single media item.
	Enrich(ctx context.Context, req *EnrichRequest) (*EnrichResponse, error)
}

// EnricherCapabilities describes what an enricher provides and requires.
type EnricherCapabilities struct {
	MediaTypes []string // "movie", "tv", "music"
	Provides   []string // "metadata", "artwork", "external_ids"
	IsLocal    bool     // Local enrichers get high concurrency
	RateLimit  int      // Requests per minute (0 = unlimited)
	Requires   []string // External IDs required (e.g., ["imdb"])
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
	ContentRating  *string
	RuntimeMinutes *int
	Rating         *float32
	RatingVotes    *int
	Directors      []string
	Writers        []string
	Cast           []CastMember
	Studios        []string
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
	impl EnricherPlugin
	base *Base
}

func (s *enricherGRPCServer) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
	s.base.Init(req.DataDir)

	if err := s.impl.Initialize(ctx, req.DataDir, req.Config); err != nil {
		return &pluginv1.InitResponse{Success: false, Error: err.Error()}, nil
	}
	return &pluginv1.InitResponse{Success: true}, nil
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
	// TODO: Support settings schema
	return &pluginv1.SettingsSchema{}, nil
}

func (s *enricherGRPCServer) Configure(ctx context.Context, settings *pluginv1.Settings) (*pluginv1.ConfigureResponse, error) {
	// TODO: Support configuration
	return &pluginv1.ConfigureResponse{Success: true}, nil
}

func (s *enricherGRPCServer) GetSubscriptions(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.EventSubscriptions, error) {
	return &pluginv1.EventSubscriptions{}, nil
}

func (s *enricherGRPCServer) OnEvent(ctx context.Context, event *pluginv1.Event) (*pluginv1.EventResponse, error) {
	return &pluginv1.EventResponse{Handled: false}, nil
}

func (s *enricherGRPCServer) GetCapabilities(ctx context.Context, _ *pluginv1.Empty) (*pluginv1.EnricherCapabilities, error) {
	caps := s.impl.GetCapabilities()
	return &pluginv1.EnricherCapabilities{
		MediaTypes: caps.MediaTypes,
		Provides:   caps.Provides,
		IsLocal:    caps.IsLocal,
		RateLimit:  int32(caps.RateLimit),
		Requires:   caps.Requires,
	}, nil
}

func (s *enricherGRPCServer) Enrich(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
	start := time.Now()

	// Convert proto request to SDK request
	sdkReq := protoToSDKRequest(req)

	// Call the plugin implementation
	resp, err := s.impl.Enrich(ctx, sdkReq)

	latency := time.Since(start)
	if err != nil {
		s.base.RecordError()
		return nil, err
	}
	s.base.RecordRequest(latency)

	// Convert SDK response to proto response
	return sdkToProtoResponse(resp), nil
}

// --- Conversion helpers ---

func protoToSDKRequest(req *pluginv1.EnrichRequest) *EnrichRequest {
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

func sdkToProtoResponse(resp *EnrichResponse) *pluginv1.EnrichResponse {
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
		result.Metadata = sdkToProtoMetadata(resp.Metadata)
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

func sdkToProtoMetadata(md *EnrichedMetadata) *pluginv1.EnrichedMetadata {
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

	return result
}

// --- go-plugin integration ---

// EnricherGRPCPlugin is the go-plugin implementation for enricher plugins.
type EnricherGRPCPlugin struct {
	plugin.Plugin
	Impl EnricherPlugin
}

func (p *EnricherGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	server := &enricherGRPCServer{
		impl: p.Impl,
		base: &Base{}, // Will be initialized in Initialize()
	}
	pluginv1.RegisterPluginCoreServer(s, server)
	pluginv1.RegisterEnricherServer(s, server)
	return nil
}

func (p *EnricherGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	// This is only used by the host, not by plugins
	return pluginv1.NewEnricherClient(c), nil
}
