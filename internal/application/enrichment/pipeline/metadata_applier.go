package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/domain/enrichment"
)

// parseDate parses a date string in YYYY-MM-DD format.
func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

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
		return a.applyAlbumMetadata(ctx, mediaID, metadata)
	case enrichment.MediaTypeMusicArtist:
		return a.applyArtistMetadata(ctx, mediaID, metadata)
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

	// Ratings
	if metadata.Rating != nil {
		movie.Rating = *metadata.Rating
		updated = true
	}
	if metadata.RatingVotes != nil {
		movie.RatingVotes = int(*metadata.RatingVotes)
		updated = true
	}

	// Production details
	if metadata.Budget != nil {
		movie.Budget = *metadata.Budget
		updated = true
	}
	if metadata.Revenue != nil {
		movie.Revenue = *metadata.Revenue
		updated = true
	}
	if metadata.OriginalLanguage != nil {
		movie.OriginalLanguage = *metadata.OriginalLanguage
		updated = true
	}
	if metadata.CountryOfOrigin != nil {
		movie.CountryOfOrigin = *metadata.CountryOfOrigin
		updated = true
	}

	// Release date (use release_date field from proto)
	if metadata.ReleaseDate != nil {
		// Parse and set the release date
		if t, err := parseDate(*metadata.ReleaseDate); err == nil {
			movie.ReleaseDate = t
			updated = true
		}
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

	// Episode-specific fields
	if metadata.RuntimeMinutes != nil {
		episode.RuntimeMinutes = int(*metadata.RuntimeMinutes)
		updated = true
	}
	if metadata.Rating != nil {
		episode.Rating = *metadata.Rating
		updated = true
	}
	if metadata.RatingVotes != nil {
		episode.RatingVotes = int(*metadata.RatingVotes)
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
	if metadata.OriginalTitle != nil {
		show.OriginalTitle = *metadata.OriginalTitle
		updated = true
	}
	if metadata.SortTitle != nil {
		show.SortTitle = *metadata.SortTitle
		updated = true
	}
	if metadata.Year != nil {
		show.Year = int(*metadata.Year)
		updated = true
	}
	if metadata.Premiered != nil {
		show.FirstAirDate = *metadata.Premiered
		updated = true
	}
	if metadata.LastAirDate != nil {
		show.LastAirDate = *metadata.LastAirDate
		updated = true
	}
	if metadata.Plot != nil {
		show.Plot = *metadata.Plot
		updated = true
	}
	if metadata.Tagline != nil {
		show.Tagline = *metadata.Tagline
		updated = true
	}
	if len(metadata.Genres) > 0 {
		show.Genre = metadata.Genres
		updated = true
	}
	if metadata.Status != nil {
		show.Status = *metadata.Status
		updated = true
	}
	if metadata.ContentRating != nil {
		show.ContentRating = *metadata.ContentRating
		updated = true
	}
	if metadata.Network != nil {
		show.Network = *metadata.Network
		updated = true
	}

	// Ratings
	if metadata.Rating != nil {
		show.Rating = *metadata.Rating
		updated = true
	}
	if metadata.RatingVotes != nil {
		show.RatingVotes = int(*metadata.RatingVotes)
		updated = true
	}

	// Production details
	if metadata.OriginalLanguage != nil {
		show.OriginalLanguage = *metadata.OriginalLanguage
		updated = true
	}
	if metadata.CountryOfOrigin != nil {
		show.CountryOfOrigin = *metadata.CountryOfOrigin
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

	// Basic metadata
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

	// Music-specific fields
	if metadata.Artist != nil && *metadata.Artist != "" {
		track.Artist = *metadata.Artist
		updated = true
	}
	if metadata.Album != nil && *metadata.Album != "" {
		track.Album = *metadata.Album
		updated = true
	}
	if metadata.AlbumArtist != nil && *metadata.AlbumArtist != "" {
		track.AlbumArtist = *metadata.AlbumArtist
		updated = true
	}
	if metadata.TrackNumber != nil && *metadata.TrackNumber > 0 {
		track.TrackNumber = int(*metadata.TrackNumber)
		updated = true
	}
	if metadata.DiscNumber != nil && *metadata.DiscNumber > 0 {
		track.DiscNumber = int(*metadata.DiscNumber)
		updated = true
	}
	if metadata.TotalTracks != nil && *metadata.TotalTracks > 0 {
		track.TotalTracks = int(*metadata.TotalTracks)
		updated = true
	}
	if metadata.TotalDiscs != nil && *metadata.TotalDiscs > 0 {
		track.TotalDiscs = int(*metadata.TotalDiscs)
		updated = true
	}

	// Release date
	if metadata.ReleaseDate != nil && *metadata.ReleaseDate != "" {
		track.ReleaseDate = *metadata.ReleaseDate
		updated = true
	}

	if !updated {
		return nil
	}

	return a.typedRepos.Music.UpdateMusicTrack(ctx, track)
}

// applyAlbumMetadata updates a music album with enriched metadata.
func (a *MetadataApplier) applyAlbumMetadata(ctx context.Context, mediaID int64, metadata *pluginv1.EnrichedMetadata) error {
	if a.typedRepos.Music == nil {
		return nil
	}

	albumMeta := metadata.AlbumMetadata
	if albumMeta == nil {
		return nil // No album-specific metadata provided
	}

	album, err := a.typedRepos.Music.GetAlbumByID(ctx, mediaID)
	if err != nil {
		return fmt.Errorf("get album: %w", err)
	}

	updated := false

	if albumMeta.Title != nil && *albumMeta.Title != "" {
		album.Title = *albumMeta.Title
		updated = true
	}
	if albumMeta.AlbumArtist != nil && *albumMeta.AlbumArtist != "" {
		album.AlbumArtist = *albumMeta.AlbumArtist
		updated = true
	}
	if albumMeta.Artist != nil && *albumMeta.Artist != "" {
		album.Artist = *albumMeta.Artist
		updated = true
	}
	if albumMeta.Year != nil && *albumMeta.Year > 0 {
		album.Year = int(*albumMeta.Year)
		updated = true
	}
	if albumMeta.ReleaseDate != nil && *albumMeta.ReleaseDate != "" {
		album.ReleaseDate = *albumMeta.ReleaseDate
		updated = true
	}
	if albumMeta.Genre != nil && *albumMeta.Genre != "" {
		album.Genre = *albumMeta.Genre
		updated = true
	}
	if albumMeta.TotalTracks != nil && *albumMeta.TotalTracks > 0 {
		album.TotalTracks = int(*albumMeta.TotalTracks)
		updated = true
	}
	if albumMeta.TotalDiscs != nil && *albumMeta.TotalDiscs > 0 {
		album.TotalDiscs = int(*albumMeta.TotalDiscs)
		updated = true
	}
	if albumMeta.RecordLabel != nil && *albumMeta.RecordLabel != "" {
		album.RecordLabel = *albumMeta.RecordLabel
		updated = true
	}
	if albumMeta.ReleaseType != nil && *albumMeta.ReleaseType != "" {
		album.ReleaseType = *albumMeta.ReleaseType
		updated = true
	}
	if albumMeta.Compilation != nil {
		album.Compilation = *albumMeta.Compilation
		updated = true
	}
	if albumMeta.MusicbrainzAlbumId != nil && *albumMeta.MusicbrainzAlbumId != "" {
		album.MusicBrainzAlbumID = *albumMeta.MusicbrainzAlbumId
		updated = true
	}
	if albumMeta.SortTitle != nil && *albumMeta.SortTitle != "" {
		album.SortTitle = *albumMeta.SortTitle
		updated = true
	}

	if !updated {
		return nil
	}

	return a.typedRepos.Music.UpdateAlbum(ctx, album)
}

// applyArtistMetadata updates a music artist with enriched metadata.
func (a *MetadataApplier) applyArtistMetadata(ctx context.Context, mediaID int64, metadata *pluginv1.EnrichedMetadata) error {
	if a.typedRepos.Music == nil {
		return nil
	}

	artistMeta := metadata.ArtistMetadata
	if artistMeta == nil {
		return nil // No artist-specific metadata provided
	}

	artist, err := a.typedRepos.Music.GetArtistByID(ctx, mediaID)
	if err != nil {
		return fmt.Errorf("get artist: %w", err)
	}

	updated := false

	if artistMeta.Name != nil && *artistMeta.Name != "" {
		artist.Name = *artistMeta.Name
		updated = true
	}
	if artistMeta.SortName != nil && *artistMeta.SortName != "" {
		artist.SortName = *artistMeta.SortName
		updated = true
	}
	if artistMeta.MusicbrainzArtistId != nil && *artistMeta.MusicbrainzArtistId != "" {
		artist.MusicBrainzArtistID = *artistMeta.MusicbrainzArtistId
		updated = true
	}
	if artistMeta.Bio != nil && *artistMeta.Bio != "" {
		artist.Bio = *artistMeta.Bio
		updated = true
	}
	if artistMeta.Country != nil && *artistMeta.Country != "" {
		artist.Country = *artistMeta.Country
		updated = true
	}
	if artistMeta.FormedYear != nil && *artistMeta.FormedYear > 0 {
		artist.FormedYear = int(*artistMeta.FormedYear)
		updated = true
	}
	if artistMeta.Genre != nil && *artistMeta.Genre != "" {
		artist.Genre = *artistMeta.Genre
		updated = true
	}

	if !updated {
		return nil
	}

	return a.typedRepos.Music.UpdateArtist(ctx, artist)
}
