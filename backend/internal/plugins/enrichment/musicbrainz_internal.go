package enrichment

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/enrichmentmodule"
)

// =============================================================================
// MUSICBRAINZ INTERNAL PLUGIN
// =============================================================================
// This is a core internal plugin that enriches music metadata using the
// MusicBrainz database. It integrates directly with the centralized enrichment
// module for better performance and consistency.

// MusicBrainzInternalPlugin is a core plugin that enriches music metadata using MusicBrainz
type MusicBrainzInternalPlugin struct {
	enrichmentModule  *enrichmentmodule.Module
	config            *MusicBrainzConfig
	lastAPICall       *time.Time
	enabled           bool
}

// MusicBrainzConfig holds configuration for the MusicBrainz plugin
type MusicBrainzConfig struct {
	Enabled             bool    `json:"enabled"`
	APIRateLimit        float64 `json:"api_rate_limit"`        // Requests per second
	UserAgent           string  `json:"user_agent"`
	MatchThreshold      float64 `json:"match_threshold"`       // 0.0 - 1.0
	AutoEnrich          bool    `json:"auto_enrich"`
	OverwriteExisting   bool    `json:"overwrite_existing"`
}

// MusicBrainz API response structures
type MusicBrainzResponse struct {
	Created    string              `json:"created"`
	Count      int                 `json:"count"`
	Offset     int                 `json:"offset"`
	Recordings []MusicBrainzRecord `json:"recordings"`
}

type MusicBrainzRecord struct {
	ID           string                `json:"id"`
	Score        float64               `json:"score"`
	Title        string                `json:"title"`
	Length       int                   `json:"length"`
	ArtistCredit []MusicBrainzArtist   `json:"artist-credit"`
	Releases     []MusicBrainzRelease  `json:"releases"`
	Tags         []MusicBrainzTag      `json:"tags"`
}

type MusicBrainzArtist struct {
	Name   string `json:"name"`
	Artist struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"artist"`
}

type MusicBrainzRelease struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Date  string `json:"date"`
	ReleaseGroup struct {
		PrimaryType string `json:"primary-type"`
	} `json:"release-group"`
}

type MusicBrainzTag struct {
	Count int    `json:"count"`
	Name  string `json:"name"`
}

// NewMusicBrainzInternalPlugin creates a new internal MusicBrainz plugin
func NewMusicBrainzInternalPlugin(enrichmentModule *enrichmentmodule.Module) *MusicBrainzInternalPlugin {
	return &MusicBrainzInternalPlugin{
		enrichmentModule: enrichmentModule,
		enabled:          true,
		config: &MusicBrainzConfig{
			Enabled:             true,
			APIRateLimit:        0.8, // 0.8 requests per second to be safe
			UserAgent:           "Viewra/2.0 (https://github.com/mantonx/viewra)",
			MatchThreshold:      0.75,
			AutoEnrich:          true,
			OverwriteExisting:   false,
		},
	}
}

// Initialize sets up the plugin
func (p *MusicBrainzInternalPlugin) Initialize() error {
	if !p.enabled {
		return nil
	}

	log.Printf("INFO: MusicBrainz internal plugin initialized (using centralized enrichment system)")
	return nil
}

// GetName returns the plugin name
func (p *MusicBrainzInternalPlugin) GetName() string {
	return "musicbrainz_internal"
}

// CanEnrich determines if this plugin can enrich the given media file
func (p *MusicBrainzInternalPlugin) CanEnrich(mediaFile *database.MediaFile) bool {
	if !p.enabled || !p.config.Enabled {
		return false
	}

	// Only handle audio files
	return mediaFile.MediaType == database.MediaTypeTrack
}

// EnrichMediaFile enriches a media file using MusicBrainz data
func (p *MusicBrainzInternalPlugin) EnrichMediaFile(mediaFile *database.MediaFile, existingMetadata map[string]string) error {
	if !p.CanEnrich(mediaFile) {
		return nil
	}

	title := existingMetadata["title"]
	artist := existingMetadata["artist"]
	album := existingMetadata["album"]

	if title == "" || artist == "" {
		log.Printf("DEBUG: Skipping MusicBrainz enrichment - missing title or artist for %s", mediaFile.Path)
		return nil
	}

	log.Printf("INFO: Enriching with MusicBrainz: %s by %s", title, artist)

	// Search MusicBrainz for matching recording
	recording, err := p.searchRecording(title, artist, album)
	if err != nil {
		return fmt.Errorf("failed to search MusicBrainz: %w", err)
	}

	if recording == nil {
		log.Printf("DEBUG: No MusicBrainz match found for: %s by %s", title, artist)
		return nil
	}

	if recording.Score < p.config.MatchThreshold {
		log.Printf("DEBUG: MusicBrainz match score too low: %.2f < %.2f", recording.Score, p.config.MatchThreshold)
		return nil
	}

	// Convert MusicBrainz data to enrichment format
	enrichments := p.convertToEnrichments(recording)
	
	// Calculate confidence based on match score
	confidence := recording.Score / 100.0 // MusicBrainz score is 0-100
	
	// Register enrichment data with the core enrichment module
	if err := p.enrichmentModule.RegisterEnrichmentData(
		mediaFile.ID, 
		"musicbrainz_internal", 
		enrichments, 
		confidence,
	); err != nil {
		return fmt.Errorf("failed to register enrichment data: %w", err)
	}

	log.Printf("INFO: Successfully registered MusicBrainz enrichment for %s (score: %.2f)", mediaFile.Path, recording.Score)
	return nil
}

// convertToEnrichments converts MusicBrainz recording data to enrichment map
func (p *MusicBrainzInternalPlugin) convertToEnrichments(recording *MusicBrainzRecord) map[string]interface{} {
	enrichments := make(map[string]interface{})

	// Title
	if recording.Title != "" {
		enrichments["title"] = recording.Title
	}

	// Artist
	if len(recording.ArtistCredit) > 0 {
		enrichments["artist_name"] = recording.ArtistCredit[0].Name
	}

	// Album and release info
	if len(recording.Releases) > 0 {
		release := recording.Releases[0]
		if release.Title != "" {
			enrichments["album_name"] = release.Title
		}
		
		// Release year
		if release.Date != "" && len(release.Date) >= 4 {
			enrichments["release_year"] = release.Date[:4]
		}
	}

	// Duration
	if recording.Length > 0 {
		enrichments["duration"] = strconv.Itoa(recording.Length / 1000) // Convert ms to seconds
	}

	// Genres from tags
	if len(recording.Tags) > 0 {
		var genres []string
		for _, tag := range recording.Tags {
			if tag.Count > 0 { // Only include tags with votes
				genres = append(genres, tag.Name)
			}
		}
		if len(genres) > 0 {
			enrichments["genres"] = strings.Join(genres, ",")
		}
	}

	// External IDs
	externalIDs := make(map[string]string)
	externalIDs["musicbrainz_recording_id"] = recording.ID
	
	if len(recording.ArtistCredit) > 0 {
		externalIDs["musicbrainz_artist_id"] = recording.ArtistCredit[0].Artist.ID
	}
	
	if len(recording.Releases) > 0 {
		externalIDs["musicbrainz_release_id"] = recording.Releases[0].ID
	}
	
	// Add external IDs to enrichment data
	enrichments["external_ids"] = externalIDs

	return enrichments
}

// searchRecording searches MusicBrainz for a recording (no caching in internal plugin)
func (p *MusicBrainzInternalPlugin) searchRecording(title, artist, album string) (*MusicBrainzRecord, error) {
	// Rate limiting
	if err := p.respectRateLimit(); err != nil {
		return nil, err
	}

	// Build search query
	query := p.buildSearchQuery(title, artist, album)
	
	// Make API request
	apiURL := fmt.Sprintf("https://musicbrainz.org/ws/2/recording?query=%s&fmt=json&limit=10&inc=releases+tags",
		url.QueryEscape(query))

	log.Printf("DEBUG: MusicBrainz API request: %s", apiURL)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", p.config.UserAgent)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Parse response
	var mbResponse MusicBrainzResponse
	if err := json.NewDecoder(resp.Body).Decode(&mbResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Find best match
	bestMatch := p.findBestMatch(mbResponse.Recordings, title, artist, album)
	
	return bestMatch, nil
}

// buildSearchQuery builds a MusicBrainz search query
func (p *MusicBrainzInternalPlugin) buildSearchQuery(title, artist, album string) string {
	var parts []string
	
	if title != "" {
		parts = append(parts, fmt.Sprintf("recording:\"%s\"", title))
	}
	if artist != "" {
		parts = append(parts, fmt.Sprintf("artist:\"%s\"", artist))
	}
	if album != "" {
		parts = append(parts, fmt.Sprintf("release:\"%s\"", album))
	}
	
	return strings.Join(parts, " AND ")
}

// findBestMatch finds the best matching recording from search results
func (p *MusicBrainzInternalPlugin) findBestMatch(recordings []MusicBrainzRecord, title, artist, album string) *MusicBrainzRecord {
	if len(recordings) == 0 {
		return nil
	}

	// For now, use the first result with the highest MusicBrainz score
	// In the future, we could implement our own scoring algorithm
	var bestMatch *MusicBrainzRecord
	for i := range recordings {
		if bestMatch == nil || recordings[i].Score > bestMatch.Score {
			bestMatch = &recordings[i]
		}
	}

	return bestMatch
}

// respectRateLimit ensures we don't exceed the API rate limit
func (p *MusicBrainzInternalPlugin) respectRateLimit() error {
	if p.lastAPICall != nil {
		elapsed := time.Since(*p.lastAPICall)
		minInterval := time.Duration(float64(time.Second) / p.config.APIRateLimit)
		if elapsed < minInterval {
			time.Sleep(minInterval - elapsed)
		}
	}
	now := time.Now()
	p.lastAPICall = &now
	return nil
}

// OnMediaFileScanned is called when a media file is scanned (plugin hook)
func (p *MusicBrainzInternalPlugin) OnMediaFileScanned(mediaFile *database.MediaFile, metadata map[string]string) error {
	if !p.config.AutoEnrich {
		return nil
	}

	return p.EnrichMediaFile(mediaFile, metadata)
} 