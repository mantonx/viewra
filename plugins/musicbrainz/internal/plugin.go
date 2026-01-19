// Package internal implements the MusicBrainz plugin logic.
package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/mantonx/viewra/pkg/plugin/sdk"
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

// MusicBrainzPlugin implements sdk.EnricherPlugin for MusicBrainz.
type MusicBrainzPlugin struct {
	sdk.Base

	logger  *slog.Logger
	dataDir string
	config  Config
	client  *Client
	storage *sdk.StorageClient

	mu sync.RWMutex

	// Stats for health reporting
	requestsTotal int64
	errorsTotal   int64
}

// NewMusicBrainzPlugin creates a new MusicBrainz plugin instance.
func NewMusicBrainzPlugin(logger *slog.Logger) *MusicBrainzPlugin {
	p := &MusicBrainzPlugin{
		logger: logger,
	}
	p.SetLogger(logger)
	return p
}

// recordError increments the error counter (thread-safe).
func (p *MusicBrainzPlugin) recordError() {
	p.mu.Lock()
	p.errorsTotal++
	p.mu.Unlock()
}

// --- sdk.EnricherPlugin implementation ---

func (p *MusicBrainzPlugin) GetCapabilities() sdk.EnricherCapabilities {
	return sdk.EnricherCapabilities{
		MediaTypes: []string{"music", "music_album", "music_artist"},
		Provides:   []string{"metadata", "artwork", "external_ids"},
		IsLocal:    false,
		RateLimit:  60, // MusicBrainz allows ~1 request/second
		Requires:   []string{},
		Priority:   50,
	}
}

func (p *MusicBrainzPlugin) Initialize(ctx context.Context, dataDir string, config []byte, services *sdk.HostServices) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.dataDir = dataDir
	p.logger.Debug("initializing MusicBrainz plugin", "data_dir", dataDir)

	// Store host storage client if available
	if services != nil && services.Storage != nil {
		p.storage = services.Storage
		p.logger.Debug("host storage service available")
	}

	// Parse config from YAML
	if len(config) == 0 {
		return fmt.Errorf("config.yml is required but was not provided")
	}

	if err := yaml.Unmarshal(config, &p.config); err != nil {
		return fmt.Errorf("failed to parse config.yml: %w", err)
	}

	// Validate required fields
	if p.config.UserAgent == "" {
		return fmt.Errorf("user_agent is required in config.yml (MusicBrainz API policy)")
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
		return fmt.Errorf("failed to create API client: %w", err)
	}
	p.client = client

	cacheStatus := "disabled (no host storage)"
	if p.storage != nil {
		cacheStatus = fmt.Sprintf("enabled (TTL: %dh)", p.config.CacheTTLHours)
	}
	p.logger.Debug("MusicBrainz plugin initialized",
		"cache", cacheStatus,
		"min_confidence", p.config.MinConfidence,
		"fetch_cover_art", p.config.FetchCoverArt,
	)

	return nil
}

func (p *MusicBrainzPlugin) Shutdown(ctx context.Context) error {
	p.logger.Debug("shutting down MusicBrainz plugin")
	if p.client != nil {
		p.client.Close()
	}
	return nil
}

// GetSettingsSchema returns the JSON Schema for plugin settings.
func (p *MusicBrainzPlugin) GetSettingsSchema() ([]byte, error) {
	return SettingsSchema().Build()
}

// Configure applies new settings to the plugin.
func (p *MusicBrainzPlugin) Configure(settings []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var newSettings struct {
		ContactEmail      string  `json:"contact_email"`
		MinConfidence     float32 `json:"min_confidence"`
		CacheTTLHours     int     `json:"cache_ttl_hours"`
		FetchCoverArt     bool    `json:"fetch_cover_art"`
		CoverArtSize      string  `json:"cover_art_size"`
		FetchArtistImages bool    `json:"fetch_artist_images"`
	}
	if err := json.Unmarshal(settings, &newSettings); err != nil {
		return fmt.Errorf("failed to parse settings: %w", err)
	}

	// Apply settings to config
	if newSettings.ContactEmail != "" {
		p.config.ContactEmail = newSettings.ContactEmail
	}
	if newSettings.MinConfidence > 0 {
		p.config.MinConfidence = newSettings.MinConfidence
	}
	if newSettings.CacheTTLHours > 0 {
		p.config.CacheTTLHours = newSettings.CacheTTLHours
	}
	p.config.FetchCoverArt = newSettings.FetchCoverArt
	if newSettings.CoverArtSize != "" {
		p.config.CoverArtSize = newSettings.CoverArtSize
	}

	p.logger.Debug("configuration updated",
		"contact_email", p.config.ContactEmail,
		"min_confidence", p.config.MinConfidence,
		"cache_ttl_hours", p.config.CacheTTLHours,
		"fetch_cover_art", p.config.FetchCoverArt,
		"cover_art_size", p.config.CoverArtSize,
	)

	return nil
}

func (p *MusicBrainzPlugin) IsConfigured() bool {
	// MusicBrainz doesn't require an API key - always configured
	return true
}

func (p *MusicBrainzPlugin) Enrich(ctx context.Context, req *sdk.EnrichRequest) (*sdk.EnrichResponse, error) {
	p.mu.Lock()
	p.requestsTotal++
	client := p.client
	minConfidence := p.config.MinConfidence
	p.mu.Unlock()

	if client == nil {
		p.mu.Lock()
		p.errorsTotal++
		p.mu.Unlock()
		return sdk.Skip("MusicBrainz plugin not configured"), nil
	}

	p.logger.Debug("enriching media",
		"media_id", req.MediaID,
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
		return sdk.Skip("unsupported media type: " + req.MediaType), nil
	}
}
