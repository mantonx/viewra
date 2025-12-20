package builtin

import (
	"context"
	"log/slog"
	"path/filepath"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	appenrich "github.com/mantonx/viewra/internal/application/enrichment"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	infraimages "github.com/mantonx/viewra/internal/infrastructure/images"
)

// LocalImagesEnricher discovers local image files (posters, fanart, etc.)
// in the media directory structure using the infrastructure extractor.
type LocalImagesEnricher struct {
	extractor *infraimages.Extractor
	logger    *slog.Logger
}

// NewLocalImagesEnricher creates a new local images enricher.
func NewLocalImagesEnricher(extractor *infraimages.Extractor, logger *slog.Logger) *LocalImagesEnricher {
	if logger == nil {
		logger = slog.Default()
	}
	return &LocalImagesEnricher{
		extractor: extractor,
		logger:    logger.With(slog.String("enricher", "local_images")),
	}
}

// Stage returns the stage name for this enricher.
func (e *LocalImagesEnricher) Stage() string {
	return "local-images"
}

// Metadata returns display information about this enricher.
func (e *LocalImagesEnricher) Metadata() (name, version string) {
	return "Local Images", "1.0.0"
}

// Capabilities returns what this enricher provides.
func (e *LocalImagesEnricher) Capabilities() appenrich.EnricherCapabilities {
	return appenrich.NewCapabilitiesBuilder().
		WithMediaTypes(
			enrichment.MediaTypeMovie,
			enrichment.MediaTypeTV,
			enrichment.MediaTypeTVShow,
			enrichment.MediaTypeTVSeason,
			enrichment.MediaTypeMusic,
			enrichment.MediaTypeMusicAlbum,
			enrichment.MediaTypeMusicArtist,
		).
		WithProvides("artwork").
		AsLocal().
		Build()
}

// Enrich discovers local image files for a media item.
func (e *LocalImagesEnricher) Enrich(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
	resp := appenrich.Match()
	resp.Confidence = 1.0 // Local files are authoritative

	if e.extractor == nil {
		return appenrich.Skip("no image extractor configured"), nil
	}

	switch req.MediaType {
	case string(enrichment.MediaTypeMovie):
		e.extractMovieImages(req.FilePath, resp)
	case string(enrichment.MediaTypeTV):
		e.extractTVEpisodeImages(req.FilePath, resp)
	case string(enrichment.MediaTypeTVShow):
		e.extractTVShowImages(req.FilePath, resp)
	case string(enrichment.MediaTypeTVSeason):
		var seasonNum int32
		if req.Tv != nil {
			seasonNum = req.Tv.SeasonNumber
		}
		e.extractTVSeasonImages(req.FilePath, seasonNum, resp)
	case string(enrichment.MediaTypeMusic):
		e.extractMusicImages(req.FilePath, resp)
	case string(enrichment.MediaTypeMusicAlbum):
		e.extractMusicAlbumImages(req.FilePath, resp)
	case string(enrichment.MediaTypeMusicArtist):
		e.extractMusicArtistImages(req.FilePath, resp)
	default:
		e.logger.Warn("unknown media type for local images",
			slog.String("media_type", req.MediaType))
	}

	if len(resp.Images) == 0 {
		return appenrich.Skip("no local images found"), nil
	}

	return resp, nil
}

// extractMovieImages uses the infrastructure extractor for movies.
func (e *LocalImagesEnricher) extractMovieImages(filePath string, resp *pluginv1.EnrichResponse) {
	extracted, err := e.extractor.ExtractMovieImages(filePath)
	if err != nil {
		e.logger.Warn("failed to extract movie images",
			slog.String("path", filePath),
			slog.Any("error", err))
		return
	}

	for _, img := range extracted.Images {
		appenrich.AddImage(resp, &pluginv1.EnrichedImage{
			Type:     string(img.Type),
			Path:     img.Path,
			IsRemote: false,
		})
	}
}

// extractTVEpisodeImages uses the infrastructure extractor for TV episodes.
func (e *LocalImagesEnricher) extractTVEpisodeImages(filePath string, resp *pluginv1.EnrichResponse) {
	extracted, err := e.extractor.ExtractTVEpisodeImages(filePath)
	if err != nil {
		e.logger.Warn("failed to extract episode images",
			slog.String("path", filePath),
			slog.Any("error", err))
		return
	}

	for _, img := range extracted.Images {
		appenrich.AddImage(resp, &pluginv1.EnrichedImage{
			Type:     string(img.Type),
			Path:     img.Path,
			IsRemote: false,
		})
	}
}

// extractTVShowImages uses the infrastructure extractor for TV shows.
func (e *LocalImagesEnricher) extractTVShowImages(filePath string, resp *pluginv1.EnrichResponse) {
	// For TV shows, filePath is the show directory
	extracted, err := e.extractor.ExtractTVShowImages(filePath)
	if err != nil {
		e.logger.Warn("failed to extract show images",
			slog.String("path", filePath),
			slog.Any("error", err))
		return
	}

	for _, img := range extracted.Images {
		appenrich.AddImage(resp, &pluginv1.EnrichedImage{
			Type:     string(img.Type),
			Path:     img.Path,
			IsRemote: false,
		})
	}
}

// extractTVSeasonImages uses the infrastructure extractor for TV seasons.
func (e *LocalImagesEnricher) extractTVSeasonImages(showDir string, seasonNumber int32, resp *pluginv1.EnrichResponse) {
	extracted, err := e.extractor.ExtractTVSeasonImages(showDir, int(seasonNumber))
	if err != nil {
		e.logger.Warn("failed to extract season images",
			slog.String("path", showDir),
			slog.Int("season", int(seasonNumber)),
			slog.Any("error", err))
		return
	}

	for _, img := range extracted.Images {
		appenrich.AddImage(resp, &pluginv1.EnrichedImage{
			Type:     string(img.Type),
			Path:     img.Path,
			IsRemote: false,
		})
	}
}

// extractMusicImages extracts images for music tracks.
// Uses album directory for cover art with fallback to embedded artwork.
func (e *LocalImagesEnricher) extractMusicImages(filePath string, resp *pluginv1.EnrichResponse) {
	// For music tracks, first try album directory images
	albumDir := filepath.Dir(filePath)
	extracted, err := e.extractor.ExtractMusicAlbumImages(albumDir)
	if err != nil {
		e.logger.Warn("failed to extract album images",
			slog.String("path", albumDir),
			slog.Any("error", err))
	} else {
		for _, img := range extracted.Images {
			appenrich.AddImage(resp, &pluginv1.EnrichedImage{
				Type:     string(img.Type),
				Path:     img.Path,
				IsRemote: false,
			})
		}
	}

	// If no cover found, try embedded artwork from the track itself
	hasCover := false
	for _, img := range resp.Images {
		if img.Type == "cover" {
			hasCover = true
			break
		}
	}

	if !hasCover {
		trackExtracted, err := e.extractor.ExtractMusicTrackImages(filePath)
		if err != nil {
			e.logger.Debug("no embedded artwork in track",
				slog.String("path", filePath))
		} else {
			for _, img := range trackExtracted.Images {
				appenrich.AddImage(resp, &pluginv1.EnrichedImage{
					Type:     string(img.Type),
					Path:     img.Path,
					IsRemote: false,
				})
			}
		}
	}
}

// extractMusicAlbumImages uses the infrastructure extractor for music albums.
func (e *LocalImagesEnricher) extractMusicAlbumImages(albumDir string, resp *pluginv1.EnrichResponse) {
	extracted, err := e.extractor.ExtractMusicAlbumImages(albumDir)
	if err != nil {
		e.logger.Warn("failed to extract album images",
			slog.String("path", albumDir),
			slog.Any("error", err))
		return
	}

	for _, img := range extracted.Images {
		appenrich.AddImage(resp, &pluginv1.EnrichedImage{
			Type:     string(img.Type),
			Path:     img.Path,
			IsRemote: false,
		})
	}
}

// extractMusicArtistImages uses the infrastructure extractor for music artists.
func (e *LocalImagesEnricher) extractMusicArtistImages(artistDir string, resp *pluginv1.EnrichResponse) {
	extracted, err := e.extractor.ExtractMusicArtistImages(artistDir)
	if err != nil {
		e.logger.Warn("failed to extract artist images",
			slog.String("path", artistDir),
			slog.Any("error", err))
		return
	}

	for _, img := range extracted.Images {
		appenrich.AddImage(resp, &pluginv1.EnrichedImage{
			Type:     string(img.Type),
			Path:     img.Path,
			IsRemote: false,
		})
	}
}
