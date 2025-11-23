package library

import (
	"context"

	"github.com/mantonx/viewra/internal/application/images"
	domainImages "github.com/mantonx/viewra/internal/domain/images"
)

// ImageExtractorAdapter adapts the 6 legacy image extraction use cases
// into the new unified ImageExtractor interface
type ImageExtractorAdapter struct {
	movieExtractor   *images.ExtractMovieImagesUseCase
	episodeExtractor *images.ExtractTVEpisodeImagesUseCase
	showExtractor    *images.ExtractTVShowImagesUseCase
	seasonExtractor  *images.ExtractTVSeasonImagesUseCase
	albumExtractor   *images.ExtractMusicAlbumImagesUseCase
	artistExtractor  *images.ExtractMusicArtistImagesUseCase
}

// NewImageExtractorAdapter creates a new adapter that wraps the legacy extractors
func NewImageExtractorAdapter(
	movieExtractor *images.ExtractMovieImagesUseCase,
	episodeExtractor *images.ExtractTVEpisodeImagesUseCase,
	showExtractor    *images.ExtractTVShowImagesUseCase,
	seasonExtractor  *images.ExtractTVSeasonImagesUseCase,
	albumExtractor   *images.ExtractMusicAlbumImagesUseCase,
	artistExtractor  *images.ExtractMusicArtistImagesUseCase,
) *ImageExtractorAdapter {
	return &ImageExtractorAdapter{
		movieExtractor:   movieExtractor,
		episodeExtractor: episodeExtractor,
		showExtractor:    showExtractor,
		seasonExtractor:  seasonExtractor,
		albumExtractor:   albumExtractor,
		artistExtractor:  artistExtractor,
	}
}

// Extract implements the ImageExtractor interface by routing to the appropriate legacy extractor
func (a *ImageExtractorAdapter) Extract(ctx context.Context, opts ImageExtractionOptions) error {
	switch opts.MediaType {
	case domainImages.MediaTypeMovie:
		if a.movieExtractor == nil {
			return nil
		}
		return a.movieExtractor.Execute(ctx, opts.FilePath, opts.MediaType, opts.EntityID, opts.MediaID)

	case domainImages.MediaTypeTVEpisode:
		if a.episodeExtractor == nil {
			return nil
		}
		return a.episodeExtractor.Execute(ctx, opts.FilePath, opts.MediaType, opts.EntityID, opts.MediaID)

	case domainImages.MediaTypeTVShow:
		if a.showExtractor == nil {
			return nil
		}
		return a.showExtractor.Execute(ctx, opts.FilePath, opts.MediaType, opts.EntityID)

	case domainImages.MediaTypeTVSeason:
		if a.seasonExtractor == nil {
			return nil
		}
		return a.seasonExtractor.Execute(ctx, opts.FilePath, opts.SeasonNum, opts.MediaType, opts.EntityID)

	case domainImages.MediaTypeMusicAlbum:
		if a.albumExtractor == nil {
			return nil
		}
		return a.albumExtractor.Execute(ctx, opts.FilePath, opts.MediaType, opts.EntityID)

	case domainImages.MediaTypeMusicArtist:
		if a.artistExtractor == nil {
			return nil
		}
		return a.artistExtractor.Execute(ctx, opts.FilePath, opts.MediaType, opts.EntityID)

	default:
		// Unknown media type - silently ignore
		return nil
	}
}
