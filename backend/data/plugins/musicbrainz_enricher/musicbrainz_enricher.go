package musicbrainz_enricher

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"musicbrainz_enricher/config"
	"musicbrainz_enricher/internal"
	"musicbrainz_enricher/musicbrainz"
)

// MusicBrainzEnricher implements the MetadataEnricher interface for MusicBrainz
type MusicBrainzEnricher struct {
	db      *gorm.DB
	config  *config.Config
	client  *musicbrainz.Client
	matcher *musicbrainz.Matcher
}

// NewMusicBrainzEnricher creates a new MusicBrainz enricher instance
func NewMusicBrainzEnricher(db *gorm.DB, cfg *config.Config) *MusicBrainzEnricher {
	client := musicbrainz.NewClient(cfg.UserAgent, cfg.APIRateLimit)
	matcher := musicbrainz.NewMatcher(cfg.MatchThreshold)

	return &MusicBrainzEnricher{
		db:      db,
		config:  cfg,
		client:  client,
		matcher: matcher,
	}
}

// GetID returns the unique identifier for this enricher
func (e *MusicBrainzEnricher) GetID() string {
	return PluginID
}

// GetName returns the human-readable name for this enricher
func (e *MusicBrainzEnricher) GetName() string {
	return PluginName
}

// GetSupportedMediaTypes returns the media types this enricher supports
func (e *MusicBrainzEnricher) GetSupportedMediaTypes() []string {
	return []string{MediaTypeMusic}
}

// CanEnrich determines if this enricher can process the given track
func (e *MusicBrainzEnricher) CanEnrich(track *Track) bool {
	if !e.config.Enabled {
		return false
	}

	// Check if it's an audio file
	if !internal.IsAudioFile(track.FilePath) {
		return false
	}

	// Need at least title and artist for meaningful enrichment
	return track.Title != "" && track.Artist != ""
}

// Enrich performs the actual enrichment operation
func (e *MusicBrainzEnricher) Enrich(ctx context.Context, request *EnrichmentRequest) (*EnrichmentResult, error) {
	track := request.Track
	
	// Check if we can enrich this track
	if !e.CanEnrich(track) {
		return &EnrichmentResult{
			Success: false,
			TrackID: track.ID,
			Source:  SourceMusicBrainz,
			Error:   "Track cannot be enriched by MusicBrainz enricher",
		}, nil
	}

	// Check if already enriched and not forcing update
	if !request.ForceUpdate && e.isAlreadyEnriched(track.ID) {
		return &EnrichmentResult{
			Success: false,
			TrackID: track.ID,
			Source:  SourceMusicBrainz,
			Error:   "Track already enriched (use force_update to override)",
		}, nil
	}

	// Search MusicBrainz
	recordings, err := e.client.SearchRecordings(ctx, track.Title, track.Artist, track.Album)
	if err != nil {
		return &EnrichmentResult{
			Success: false,
			TrackID: track.ID,
			Source:  SourceMusicBrainz,
			Error:   fmt.Sprintf("MusicBrainz search failed: %v", err),
		}, nil
	}

	if len(recordings) == 0 {
		return &EnrichmentResult{
			Success: false,
			TrackID: track.ID,
			Source:  SourceMusicBrainz,
			Error:   "No matches found in MusicBrainz",
		}, nil
	}

	// Find best match
	matchResult := e.matcher.FindBestMatch(recordings, track.Title, track.Artist, track.Album)
	if !matchResult.Matched {
		return &EnrichmentResult{
			Success:    false,
			TrackID:    track.ID,
			Source:     SourceMusicBrainz,
			MatchScore: matchResult.Score,
			Error:      fmt.Sprintf("No suitable match found (score: %.2f, threshold: %.2f)", matchResult.Score, e.config.MatchThreshold),
		}, nil
	}

	// Convert to enriched metadata
	enrichedMeta := musicbrainz.MapToEnrichedMetadata(matchResult.Recording, matchResult.Score)

	// Build result metadata
	metadata := map[string]interface{}{
		"title":          enrichedMeta.EnrichedTitle,
		"artist":         enrichedMeta.EnrichedArtist,
		"album":          enrichedMeta.EnrichedAlbum,
		"album_artist":   enrichedMeta.EnrichedAlbumArtist,
		"year":           enrichedMeta.EnrichedYear,
		"genre":          enrichedMeta.EnrichedGenre,
		"track_number":   enrichedMeta.EnrichedTrackNumber,
		"disc_number":    enrichedMeta.EnrichedDiscNumber,
	}

	// Build external IDs
	externalIDs := map[string]string{
		"musicbrainz_recording_id": enrichedMeta.MusicBrainzRecordingID,
		"musicbrainz_release_id":   enrichedMeta.MusicBrainzReleaseID,
		"musicbrainz_artist_id":    enrichedMeta.MusicBrainzArtistID,
	}

	// Handle artwork if enabled
	var artworkURL string
	if e.config.EnableArtwork && enrichedMeta.MusicBrainzReleaseID != "" {
		if coverArt, err := e.client.GetCoverArt(ctx, enrichedMeta.MusicBrainzReleaseID); err == nil {
			if bestImage := e.client.FindBestArtwork(coverArt, e.config.ArtworkQuality); bestImage != nil {
				artworkURL = bestImage.Image
			}
		}
	}

	// Save enrichment to database
	enrichment := &Enrichment{
		MediaFileID:            track.ID,
		MusicBrainzRecordingID: enrichedMeta.MusicBrainzRecordingID,
		MusicBrainzReleaseID:   enrichedMeta.MusicBrainzReleaseID,
		MusicBrainzArtistID:    enrichedMeta.MusicBrainzArtistID,
		EnrichedTitle:          enrichedMeta.EnrichedTitle,
		EnrichedArtist:         enrichedMeta.EnrichedArtist,
		EnrichedAlbum:          enrichedMeta.EnrichedAlbum,
		EnrichedAlbumArtist:    enrichedMeta.EnrichedAlbumArtist,
		EnrichedYear:           enrichedMeta.EnrichedYear,
		EnrichedGenre:          enrichedMeta.EnrichedGenre,
		EnrichedTrackNumber:    enrichedMeta.EnrichedTrackNumber,
		EnrichedDiscNumber:     enrichedMeta.EnrichedDiscNumber,
		MatchScore:             enrichedMeta.MatchScore,
		ArtworkURL:             artworkURL,
		EnrichedAt:             time.Now(),
	}

	if err := e.db.Save(enrichment).Error; err != nil {
		return &EnrichmentResult{
			Success: false,
			TrackID: track.ID,
			Source:  SourceMusicBrainz,
			Error:   fmt.Sprintf("Failed to save enrichment: %v", err),
		}, nil
	}

	return &EnrichmentResult{
		Success:     true,
		TrackID:     track.ID,
		Source:      SourceMusicBrainz,
		MatchScore:  matchResult.Score,
		EnrichedAt:  time.Now(),
		Metadata:    metadata,
		ExternalIDs: externalIDs,
		ArtworkURL:  artworkURL,
	}, nil
}

// GetPriority returns the priority of this enricher (higher = more priority)
func (e *MusicBrainzEnricher) GetPriority() int {
	// MusicBrainz is a high-quality source, so give it high priority
	return 80
}

// IsEnabled returns whether this enricher is currently enabled
func (e *MusicBrainzEnricher) IsEnabled() bool {
	return e.config.Enabled
}

// GetConfiguration returns the current configuration
func (e *MusicBrainzEnricher) GetConfiguration() map[string]interface{} {
	return map[string]interface{}{
		"enabled":              e.config.Enabled,
		"api_rate_limit":       e.config.APIRateLimit,
		"user_agent":           e.config.UserAgent,
		"enable_artwork":       e.config.EnableArtwork,
		"artwork_max_size":     e.config.ArtworkMaxSize,
		"artwork_quality":      e.config.ArtworkQuality,
		"match_threshold":      e.config.MatchThreshold,
		"auto_enrich":          e.config.AutoEnrich,
		"overwrite_existing":   e.config.OverwriteExisting,
		"cache_duration_hours": e.config.CacheDurationHours,
	}
}

// Initialize sets up the enricher and creates necessary database tables
func (e *MusicBrainzEnricher) Initialize() error {
	// Auto-migrate plugin tables
	if err := e.db.AutoMigrate(&Cache{}, &Enrichment{}, &Stats{}); err != nil {
		return fmt.Errorf("failed to migrate plugin tables: %w", err)
	}

	return nil
}

// isAlreadyEnriched checks if a track has already been enriched
func (e *MusicBrainzEnricher) isAlreadyEnriched(trackID uint) bool {
	var count int64
	e.db.Model(&Enrichment{}).Where("media_file_id = ?", trackID).Count(&count)
	return count > 0
}

// ConvertToTrack converts a metadata map to a Track struct
func ConvertToTrack(mediaFileID uint, filePath string, metadata map[string]interface{}) *Track {
	return &Track{
		ID:          mediaFileID,
		FilePath:    filePath,
		Title:       internal.ExtractMetadataString(metadata, "title"),
		Artist:      internal.ExtractMetadataString(metadata, "artist"),
		Album:       internal.ExtractMetadataString(metadata, "album"),
		AlbumArtist: internal.ExtractMetadataString(metadata, "album_artist"),
		Year:        internal.ExtractMetadataInt(metadata, "year"),
		TrackNumber: internal.ExtractMetadataInt(metadata, "track_number"),
		DiscNumber:  internal.ExtractMetadataInt(metadata, "disc_number"),
		Genre:       internal.ExtractMetadataString(metadata, "genre"),
		Duration:    internal.ExtractMetadataInt(metadata, "duration"),
	}
} 