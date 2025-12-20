// Package internal implements the MusicBrainz plugin logic.
package internal

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"gopkg.in/yaml.v3"
)

// Config holds the MusicBrainz plugin configuration.
type Config struct {
	UserAgent     string  `yaml:"user_agent" json:"user_agent"`
	ContactEmail  string  `yaml:"contact_email" json:"contact_email"`
	CacheTTLHours int     `yaml:"cache_ttl_hours" json:"cache_ttl_hours"`
	MinConfidence float32 `yaml:"min_confidence" json:"min_confidence"`
	FetchCoverArt bool    `yaml:"fetch_cover_art" json:"fetch_cover_art"`
	CoverArtSize  string  `yaml:"cover_art_size" json:"cover_art_size"`
}

// MusicBrainzPlugin implements both PluginCore and Enricher interfaces.
type MusicBrainzPlugin struct {
	pluginv1.UnimplementedPluginCoreServer
	pluginv1.UnimplementedEnricherServer

	logger  *slog.Logger
	dataDir string
	config  Config
	client  *Client
	storage pluginv1.HostStorageClient

	mu sync.RWMutex

	// Stats for health reporting
	requestsTotal int64
	errorsTotal   int64
}

// recordError increments the error counter (thread-safe).
func (p *MusicBrainzPlugin) recordError() {
	p.mu.Lock()
	p.errorsTotal++
	p.mu.Unlock()
}

// NewMusicBrainzPlugin creates a new MusicBrainz plugin instance.
func NewMusicBrainzPlugin(logger *slog.Logger) *MusicBrainzPlugin {
	return &MusicBrainzPlugin{
		logger: logger,
	}
}

// SetStorageClient sets the host storage client for caching.
// This should be called before Initialize if host storage is available.
func (p *MusicBrainzPlugin) SetStorageClient(client pluginv1.HostStorageClient) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.storage = client
}

// PluginCore implementation

func (p *MusicBrainzPlugin) Initialize(ctx context.Context, req *pluginv1.InitRequest) (*pluginv1.InitResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.dataDir = req.DataDir
	p.logger.Info("initializing MusicBrainz plugin",
		"host_version", req.HostVersion,
		"data_dir", req.DataDir,
	)

	// Parse config from YAML
	if len(req.Config) == 0 {
		return &pluginv1.InitResponse{
			Success: false,
			Error:   "config.yml is required but was not provided",
		}, nil
	}

	if err := yaml.Unmarshal(req.Config, &p.config); err != nil {
		return &pluginv1.InitResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to parse config.yml: %v", err),
		}, nil
	}

	// Validate required fields
	if p.config.UserAgent == "" {
		return &pluginv1.InitResponse{
			Success: false,
			Error:   "user_agent is required in config.yml (MusicBrainz API policy)",
		}, nil
	}

	// Apply defaults
	if p.config.CacheTTLHours == 0 {
		p.config.CacheTTLHours = 168 // 1 week
	}
	if p.config.MinConfidence == 0 {
		p.config.MinConfidence = 0.7
	}
	if p.config.CoverArtSize == "" {
		p.config.CoverArtSize = "large"
	}

	// Create the API client
	client, err := NewClient(ClientConfig{
		UserAgent:     p.config.UserAgent,
		CacheTTLHours: p.config.CacheTTLHours,
		Storage:       p.storage,
		Logger:        p.logger,
		FetchCoverArt: p.config.FetchCoverArt,
		CoverArtSize:  p.config.CoverArtSize,
	})
	if err != nil {
		return &pluginv1.InitResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to create API client: %v", err),
		}, nil
	}
	p.client = client

	cacheStatus := "disabled (no host storage)"
	if p.storage != nil {
		cacheStatus = fmt.Sprintf("enabled (TTL: %dh)", p.config.CacheTTLHours)
	}
	p.logger.Info("MusicBrainz plugin initialized",
		"cache", cacheStatus,
		"min_confidence", p.config.MinConfidence,
		"fetch_cover_art", p.config.FetchCoverArt,
	)

	return &pluginv1.InitResponse{Success: true}, nil
}

func (p *MusicBrainzPlugin) Shutdown(ctx context.Context, req *pluginv1.Empty) (*pluginv1.Empty, error) {
	p.logger.Info("shutting down MusicBrainz plugin")
	if p.client != nil {
		p.client.Close()
	}
	return &pluginv1.Empty{}, nil
}

func (p *MusicBrainzPlugin) HealthCheck(ctx context.Context, req *pluginv1.Empty) (*pluginv1.HealthStatus, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	status := pluginv1.HealthStatus_HEALTHY
	message := "operational"

	if p.config.UserAgent == "" {
		status = pluginv1.HealthStatus_DEGRADED
		message = "user agent not configured"
	}

	return &pluginv1.HealthStatus{
		Status:        status,
		Message:       message,
		RequestsTotal: p.requestsTotal,
		ErrorsTotal:   p.errorsTotal,
	}, nil
}

func (p *MusicBrainzPlugin) GetSettingsSchema(ctx context.Context, req *pluginv1.Empty) (*pluginv1.SettingsSchema, error) {
	return &pluginv1.SettingsSchema{JsonSchema: []byte("{}")}, nil
}

func (p *MusicBrainzPlugin) Configure(ctx context.Context, req *pluginv1.Settings) (*pluginv1.ConfigureResponse, error) {
	return &pluginv1.ConfigureResponse{
		Success: false,
		Error:   "runtime configuration not supported; edit config.yml and restart the plugin",
	}, nil
}

func (p *MusicBrainzPlugin) GetSubscriptions(ctx context.Context, req *pluginv1.Empty) (*pluginv1.EventSubscriptions, error) {
	return &pluginv1.EventSubscriptions{}, nil
}

func (p *MusicBrainzPlugin) OnEvent(ctx context.Context, req *pluginv1.Event) (*pluginv1.EventResponse, error) {
	return &pluginv1.EventResponse{Handled: false}, nil
}

// Enricher implementation

func (p *MusicBrainzPlugin) GetCapabilities(ctx context.Context, req *pluginv1.Empty) (*pluginv1.EnricherCapabilities, error) {
	return &pluginv1.EnricherCapabilities{
		MediaTypes: []string{"music", "music_album", "music_artist"},
		Provides:   []string{"metadata", "artwork", "external_ids"},
		IsLocal:    false,
		RateLimit:  60, // MusicBrainz allows ~1 request/second
		Requires:   []string{}, // Can work from artist/album/track names
		Priority:   50,
	}, nil
}

func (p *MusicBrainzPlugin) Enrich(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
	p.mu.Lock()
	p.requestsTotal++
	client := p.client
	minConfidence := p.config.MinConfidence
	p.mu.Unlock()

	if client == nil {
		p.mu.Lock()
		p.errorsTotal++
		p.mu.Unlock()
		return &pluginv1.EnrichResponse{
			Skipped:    true,
			SkipReason: "MusicBrainz plugin not configured",
		}, nil
	}

	p.logger.Debug("enriching media",
		"media_id", req.MediaId,
		"media_type", req.MediaType,
		"title", req.Title,
	)

	switch req.MediaType {
	case "music":
		return p.enrichTrack(ctx, client, req, minConfidence)
	case "music_album":
		return p.enrichAlbum(ctx, client, req, minConfidence)
	case "music_artist":
		return p.enrichArtist(ctx, client, req, minConfidence)
	default:
		return &pluginv1.EnrichResponse{
			Skipped:    true,
			SkipReason: "unsupported media type: " + req.MediaType,
		}, nil
	}
}
