package pipeline

import (
	"context"
	"fmt"
	"log/slog"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/domain/enrichment"
)

// RequestBuilder constructs EnrichRequest from queue jobs.
type RequestBuilder struct {
	deps        *Deps
	typedRepos  *TypedMediaRepos
	entityCache *EntityCache
	logger      *slog.Logger
}

// NewRequestBuilder creates a new RequestBuilder.
func NewRequestBuilder(deps *Deps, typedRepos *TypedMediaRepos, entityCache *EntityCache, logger *slog.Logger) *RequestBuilder {
	return &RequestBuilder{
		deps:        deps,
		typedRepos:  typedRepos,
		entityCache: entityCache,
		logger:      logger,
	}
}

// Build fetches media data and builds an EnrichRequest.
// Uses job.MediaType for explicit routing - no fallback lookups needed.
// This method fetches external IDs on-demand. For batch processing, use BuildWithExternalIDs.
func (b *RequestBuilder) Build(ctx context.Context, job *enrichment.QueueJob) (*pluginv1.EnrichRequest, enrichment.MediaType, error) {
	return b.BuildWithExternalIDs(ctx, job, nil)
}

// BuildWithExternalIDs fetches media data and builds an EnrichRequest.
// If prefetchedIDs is provided, it will be used instead of fetching from the database.
func (b *RequestBuilder) BuildWithExternalIDs(ctx context.Context, job *enrichment.QueueJob, prefetchedIDs map[string]string) (*pluginv1.EnrichRequest, enrichment.MediaType, error) {
	// Use prefetched IDs if available, otherwise fetch from database
	var existingIDs map[string]string
	if prefetchedIDs != nil {
		existingIDs = prefetchedIDs
	} else {
		existingIDs = make(map[string]string)
		if extIDs, err := b.deps.ExternalIDRepo.GetByMedia(ctx, job.MediaID); err != nil {
			// Non-fatal - continue without existing IDs
			b.logger.Warn("failed to get external IDs", slog.Int64("media_id", job.MediaID), slog.Any("error", err))
		} else {
			for _, extID := range extIDs {
				existingIDs[extID.Provider] = extID.ExternalID
			}
		}
	}

	// Route based on job.MediaType - this is now explicit, not inferred
	switch job.MediaType {
	case enrichment.MediaTypeTVShow:
		// Parent entity - TV show is in tv_shows table, not media table
		return b.buildTVShowRequest(ctx, job, existingIDs)

	case enrichment.MediaTypeTVSeason:
		// Parent entity - TV season (virtual entity for image extraction)
		return b.buildTVSeasonRequest(ctx, job, existingIDs)

	case enrichment.MediaTypeMusicAlbum:
		// Parent entity - Music album (virtual entity for image extraction)
		return b.buildMusicAlbumRequest(ctx, job, existingIDs)

	case enrichment.MediaTypeMusicArtist:
		// Parent entity - Music artist (virtual entity for image extraction)
		return b.buildMusicArtistRequest(ctx, job, existingIDs)

	case enrichment.MediaTypeMovie, enrichment.MediaTypeTV, enrichment.MediaTypeMusic:
		// Child entities - these are in the media table
		return b.buildMediaEntityRequest(ctx, job, existingIDs)

	default:
		return nil, "", fmt.Errorf("unsupported media type: %s", job.MediaType)
	}
}

// buildMediaEntityRequest builds a request for entities in the media table (movies, episodes, tracks).
func (b *RequestBuilder) buildMediaEntityRequest(ctx context.Context, job *enrichment.QueueJob, existingIDs map[string]string) (*pluginv1.EnrichRequest, enrichment.MediaType, error) {
	media, err := b.deps.MediaRepo.GetByID(ctx, job.MediaID)
	if err != nil {
		return nil, "", fmt.Errorf("media ID %d not found: %w", job.MediaID, err)
	}

	req := &pluginv1.EnrichRequest{
		MediaId:     job.MediaID,
		MediaType:   string(job.MediaType),
		FilePath:    media.FilePath,
		Title:       media.Title,
		ExistingIds: existingIDs,
	}

	// Populate type-specific fields
	switch job.MediaType {
	case enrichment.MediaTypeMovie:
		if b.typedRepos != nil && b.typedRepos.Movie != nil {
			movie, err := b.typedRepos.Movie.GetMovieByID(ctx, job.MediaID)
			if err == nil && movie != nil {
				req.Year = int32(movie.Year)
			}
		}

	case enrichment.MediaTypeTV:
		if b.typedRepos != nil && b.typedRepos.TV != nil {
			episode, err := b.typedRepos.TV.GetTVEpisodeByID(ctx, job.MediaID)
			if err == nil && episode != nil {
				req.Tv = &pluginv1.TVMetadata{
					ShowTitle:     episode.ShowTitle,
					SeasonNumber:  int32(episode.Season),
					EpisodeNumber: int32(episode.Episode),
				}
			}
		}

	case enrichment.MediaTypeMusic:
		if b.typedRepos != nil && b.typedRepos.Music != nil {
			track, err := b.typedRepos.Music.GetMusicTrackByID(ctx, job.MediaID)
			if err == nil && track != nil {
				req.Music = &pluginv1.MusicMetadata{
					Artist:      track.Artist,
					Album:       track.Album,
					TrackNumber: int32(track.TrackNumber),
				}
			}
		}
	}

	return req, job.MediaType, nil
}

// buildTVShowRequest builds a request for TV shows (parent entities in tv_shows table).
func (b *RequestBuilder) buildTVShowRequest(ctx context.Context, job *enrichment.QueueJob, existingIDs map[string]string) (*pluginv1.EnrichRequest, enrichment.MediaType, error) {
	if b.typedRepos == nil || b.typedRepos.TV == nil {
		return nil, "", fmt.Errorf("TV repository not configured")
	}

	show, err := b.typedRepos.TV.GetTVShowByID(ctx, job.MediaID)
	if err != nil {
		return nil, "", fmt.Errorf("TV show ID %d not found: %w", job.MediaID, err)
	}

	// Cache the show for future episode/season lookups
	if b.entityCache != nil {
		b.entityCache.PutShow(&CachedTVShow{
			ID:          show.ID,
			Title:       show.Title,
			Year:        show.Year,
			Directory:   show.Directory,
			ExternalIDs: existingIDs,
		})
	}

	return &pluginv1.EnrichRequest{
		MediaId:     job.MediaID,
		MediaType:   string(enrichment.MediaTypeTVShow),
		FilePath:    show.Directory, // Use directory as file path for NFO lookup
		Title:       show.Title,
		Year:        int32(show.Year),
		ExistingIds: existingIDs,
	}, enrichment.MediaTypeTVShow, nil
}

// buildTVSeasonRequest builds a request for TV seasons (for image extraction).
// MediaID is the tv_seasons.id - we look up the season, then get the show for directory info.
func (b *RequestBuilder) buildTVSeasonRequest(ctx context.Context, job *enrichment.QueueJob, existingIDs map[string]string) (*pluginv1.EnrichRequest, enrichment.MediaType, error) {
	if b.typedRepos == nil || b.typedRepos.TV == nil {
		return nil, "", fmt.Errorf("TV repository not configured")
	}

	// job.MediaID is the tv_seasons.id
	season, err := b.typedRepos.TV.GetTVSeasonByID(ctx, job.MediaID)
	if err != nil {
		return nil, "", fmt.Errorf("TV season ID %d not found: %w", job.MediaID, err)
	}

	// Cache the season for future lookups
	if b.entityCache != nil {
		b.entityCache.PutSeason(&CachedTVSeason{
			ID:           season.ID,
			ShowID:       season.ShowID,
			SeasonNumber: season.SeasonNumber,
		})
	}

	// Try to get the parent show from cache first
	var showTitle, showDirectory string
	var showYear int
	if b.entityCache != nil {
		if cached := b.entityCache.GetShow(season.ShowID); cached != nil {
			showTitle = cached.Title
			showDirectory = cached.Directory
			showYear = cached.Year
		}
	}

	// If not cached, fetch from database
	if showTitle == "" {
		show, err := b.typedRepos.TV.GetTVShowByID(ctx, season.ShowID)
		if err != nil {
			return nil, "", fmt.Errorf("TV show ID %d not found for season %d: %w", season.ShowID, season.SeasonNumber, err)
		}
		showTitle = show.Title
		showDirectory = show.Directory
		showYear = show.Year

		// Cache the show for future lookups
		if b.entityCache != nil {
			b.entityCache.PutShow(&CachedTVShow{
				ID:        show.ID,
				Title:     show.Title,
				Year:      show.Year,
				Directory: show.Directory,
			})
		}
	}

	return &pluginv1.EnrichRequest{
		MediaId:     job.MediaID,
		MediaType:   string(enrichment.MediaTypeTVSeason),
		FilePath:    showDirectory, // Show directory - enricher finds season subdirs
		Title:       showTitle,
		Year:        int32(showYear),
		ExistingIds: existingIDs,
		Tv: &pluginv1.TVMetadata{
			ShowTitle:    showTitle,
			SeasonNumber: int32(season.SeasonNumber),
		},
	}, enrichment.MediaTypeTVSeason, nil
}

// buildMusicAlbumRequest builds a request for music albums (for image extraction).
func (b *RequestBuilder) buildMusicAlbumRequest(ctx context.Context, job *enrichment.QueueJob, existingIDs map[string]string) (*pluginv1.EnrichRequest, enrichment.MediaType, error) {
	if b.typedRepos == nil || b.typedRepos.Music == nil {
		return nil, "", fmt.Errorf("Music repository not configured")
	}

	// For music_album jobs, MediaID is the album ID (from music_albums table)
	album, err := b.typedRepos.Music.GetAlbumByID(ctx, job.MediaID)
	if err != nil {
		return nil, "", fmt.Errorf("album ID %d not found: %w", job.MediaID, err)
	}

	// Use the album's directory for enrichment (where cover art is stored)
	filePath := album.Directory
	if filePath == "" {
		return nil, "", fmt.Errorf("album ID %d has no directory path", job.MediaID)
	}

	// Cache the album for future track lookups
	if b.entityCache != nil {
		b.entityCache.PutAlbum(&CachedAlbum{
			ID:          album.ID,
			Title:       album.Title,
			AlbumArtist: album.AlbumArtist,
			Directory:   album.Directory,
			ExternalIDs: existingIDs,
		})
	}

	return &pluginv1.EnrichRequest{
		MediaId:     job.MediaID,
		MediaType:   string(enrichment.MediaTypeMusicAlbum),
		FilePath:    filePath,
		Title:       album.Title,
		ExistingIds: existingIDs,
		Music: &pluginv1.MusicMetadata{
			Artist: album.AlbumArtist,
			Album:  album.Title,
		},
	}, enrichment.MediaTypeMusicAlbum, nil
}

// buildMusicArtistRequest builds a request for music artists (for image extraction).
func (b *RequestBuilder) buildMusicArtistRequest(ctx context.Context, job *enrichment.QueueJob, existingIDs map[string]string) (*pluginv1.EnrichRequest, enrichment.MediaType, error) {
	if b.typedRepos == nil || b.typedRepos.Music == nil {
		return nil, "", fmt.Errorf("Music repository not configured")
	}

	// For music_artist jobs, MediaID is the artist ID (from music_artists table)
	artist, err := b.typedRepos.Music.GetArtistByID(ctx, job.MediaID)
	if err != nil {
		return nil, "", fmt.Errorf("artist ID %d not found: %w", job.MediaID, err)
	}

	// Use the artist's directory for enrichment (where artist images are stored)
	filePath := artist.Directory
	if filePath == "" {
		return nil, "", fmt.Errorf("artist ID %d has no directory path", job.MediaID)
	}

	// Cache the artist for future lookups
	if b.entityCache != nil {
		b.entityCache.PutArtist(&CachedArtist{
			ID:          artist.ID,
			Name:        artist.Name,
			ExternalIDs: existingIDs,
		})
	}

	return &pluginv1.EnrichRequest{
		MediaId:     job.MediaID,
		MediaType:   string(enrichment.MediaTypeMusicArtist),
		FilePath:    filePath,
		Title:       artist.Name,
		ExistingIds: existingIDs,
		Music: &pluginv1.MusicMetadata{
			Artist: artist.Name,
		},
	}, enrichment.MediaTypeMusicArtist, nil
}
