package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/domain/enrichment"
)

// MetadataApplier handles type-specific metadata updates.
type MetadataApplier struct {
	typedRepos *TypedMediaRepos
	logger     *slog.Logger
}

// NewMetadataApplier creates a new MetadataApplier.
func NewMetadataApplier(typedRepos *TypedMediaRepos, logger *slog.Logger) *MetadataApplier {
	return &MetadataApplier{
		typedRepos: typedRepos,
		logger:     logger,
	}
}

// Apply applies enriched metadata to the appropriate entity based on media type.
func (a *MetadataApplier) Apply(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, metadata *pluginv1.EnrichedMetadata) error {
	if a.typedRepos == nil {
		return nil // No typed repos configured, skip metadata updates
	}

	switch mediaType {
	case enrichment.MediaTypeMovie:
		return a.applyMovieMetadata(ctx, mediaID, metadata)
	case enrichment.MediaTypeTV:
		return a.applyTVEpisodeMetadata(ctx, mediaID, metadata)
	case enrichment.MediaTypeTVShow:
		return a.applyTVShowMetadata(ctx, mediaID, metadata)
	case enrichment.MediaTypeTVSeason:
		// Seasons don't have separate metadata entities yet - handled via show
		return nil
	case enrichment.MediaTypeMusic:
		return a.applyMusicMetadata(ctx, mediaID, metadata)
	case enrichment.MediaTypeMusicAlbum:
		// TODO: Add album metadata update when MusicRepository supports it
		return nil
	case enrichment.MediaTypeMusicArtist:
		// TODO: Add artist metadata update when MusicRepository supports it
		return nil
	default:
		a.logger.Warn("unknown media type for metadata update",
			slog.String("media_type", string(mediaType)),
			slog.Int64("media_id", mediaID))
		return nil
	}
}

// applyMovieMetadata updates a movie with enriched metadata.
func (a *MetadataApplier) applyMovieMetadata(ctx context.Context, mediaID int64, metadata *pluginv1.EnrichedMetadata) error {
	if a.typedRepos.Movie == nil {
		return nil
	}

	movie, err := a.typedRepos.Movie.GetMovieByID(ctx, mediaID)
	if err != nil {
		return fmt.Errorf("get movie: %w", err)
	}

	updated := false

	// Apply optional fields only if present in the response
	if metadata.Title != nil && *metadata.Title != "" {
		movie.Title = *metadata.Title
		updated = true
	}
	if metadata.OriginalTitle != nil {
		movie.OriginalTitle = *metadata.OriginalTitle
		updated = true
	}
	if metadata.SortTitle != nil {
		movie.SortTitle = *metadata.SortTitle
		updated = true
	}
	if metadata.Year != nil {
		movie.Year = int(*metadata.Year)
		updated = true
	}
	if metadata.Plot != nil {
		movie.Plot = *metadata.Plot
		updated = true
	}
	if metadata.Tagline != nil {
		movie.Tagline = *metadata.Tagline
		updated = true
	}
	if len(metadata.Genres) > 0 {
		movie.Genre = metadata.Genres
		updated = true
	}
	if metadata.ContentRating != nil {
		movie.ContentRating = *metadata.ContentRating
		updated = true
	}
	if metadata.RuntimeMinutes != nil {
		movie.RuntimeMinutes = int(*metadata.RuntimeMinutes)
		updated = true
	}
	if len(metadata.Directors) > 0 {
		movie.Director = strings.Join(metadata.Directors, ", ")
		updated = true
	}
	if len(metadata.Cast) > 0 {
		castNames := make([]string, 0, len(metadata.Cast))
		for _, c := range metadata.Cast {
			castNames = append(castNames, c.Name)
		}
		movie.Cast = castNames
		updated = true
	}

	if !updated {
		return nil
	}

	return a.typedRepos.Movie.UpdateMovie(ctx, movie)
}

// applyTVEpisodeMetadata updates a TV episode with enriched metadata.
func (a *MetadataApplier) applyTVEpisodeMetadata(ctx context.Context, mediaID int64, metadata *pluginv1.EnrichedMetadata) error {
	if a.typedRepos.TV == nil {
		return nil
	}

	episode, err := a.typedRepos.TV.GetTVEpisodeByID(ctx, mediaID)
	if err != nil {
		return fmt.Errorf("get episode: %w", err)
	}

	updated := false

	if metadata.Title != nil && *metadata.Title != "" {
		episode.EpisodeTitle = *metadata.Title
		updated = true
	}
	if metadata.Plot != nil {
		episode.Description = *metadata.Plot
		updated = true
	}
	if metadata.Premiered != nil {
		episode.AirDate = *metadata.Premiered
		updated = true
	}

	if !updated {
		return nil
	}

	return a.typedRepos.TV.UpdateTVEpisode(ctx, episode)
}

// applyTVShowMetadata updates a TV show with enriched metadata.
func (a *MetadataApplier) applyTVShowMetadata(ctx context.Context, mediaID int64, metadata *pluginv1.EnrichedMetadata) error {
	if a.typedRepos.TV == nil {
		return nil
	}

	show, err := a.typedRepos.TV.GetTVShowByID(ctx, mediaID)
	if err != nil {
		return fmt.Errorf("get show: %w", err)
	}

	updated := false

	if metadata.Title != nil && *metadata.Title != "" {
		show.Title = *metadata.Title
		updated = true
	}
	if metadata.Year != nil {
		show.Year = int(*metadata.Year)
		updated = true
	}
	if metadata.Plot != nil {
		show.Plot = *metadata.Plot
		updated = true
	}
	if len(metadata.Genres) > 0 {
		show.Genre = metadata.Genres
		updated = true
	}
	if metadata.ContentRating != nil {
		show.ContentRating = *metadata.ContentRating
		updated = true
	}

	if !updated {
		return nil
	}

	return a.typedRepos.TV.UpdateTVShow(ctx, show)
}

// applyMusicMetadata updates a music track with enriched metadata.
func (a *MetadataApplier) applyMusicMetadata(ctx context.Context, mediaID int64, metadata *pluginv1.EnrichedMetadata) error {
	if a.typedRepos.Music == nil {
		return nil
	}

	track, err := a.typedRepos.Music.GetMusicTrackByID(ctx, mediaID)
	if err != nil {
		return fmt.Errorf("get track: %w", err)
	}

	updated := false

	if metadata.Title != nil && *metadata.Title != "" {
		track.Title = *metadata.Title
		updated = true
	}
	if len(metadata.Genres) > 0 {
		track.Genre = strings.Join(metadata.Genres, ", ")
		updated = true
	}
	if metadata.Year != nil {
		track.Year = int(*metadata.Year)
		updated = true
	}

	if !updated {
		return nil
	}

	return a.typedRepos.Music.UpdateMusicTrack(ctx, track)
}
