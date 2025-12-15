package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	appenrich "github.com/mantonx/viewra/internal/application/enrichment"
	"github.com/mantonx/viewra/internal/domain/enrichment"
)

// LocalImagesEnricher discovers local image files (posters, fanart, etc.)
// in the media directory structure.
type LocalImagesEnricher struct{}

// NewLocalImagesEnricher creates a new local images enricher.
func NewLocalImagesEnricher() *LocalImagesEnricher {
	return &LocalImagesEnricher{}
}

// Stage returns the stage name for this enricher.
func (e *LocalImagesEnricher) Stage() string {
	return "local_images"
}

// Capabilities returns what this enricher provides.
func (e *LocalImagesEnricher) Capabilities() appenrich.EnricherCapabilities {
	return appenrich.NewCapabilitiesBuilder().
		WithMediaTypes(enrichment.MediaTypeMovie, enrichment.MediaTypeTV, enrichment.MediaTypeMusic).
		WithProvides("artwork").
		AsLocal().
		Build()
}

// Enrich discovers local image files for a media item.
func (e *LocalImagesEnricher) Enrich(ctx context.Context, req *pluginv1.EnrichRequest) (*pluginv1.EnrichResponse, error) {
	dir := filepath.Dir(req.FilePath)

	resp := appenrich.Match()
	resp.Confidence = 1.0 // Local files are authoritative

	// Discover images based on common naming conventions
	// These patterns are used by Kodi, Plex, Jellyfin, etc.
	imagePatterns := getImagePatterns(req.MediaType)

	for imageType, patterns := range imagePatterns {
		for _, pattern := range patterns {
			// Check for pattern in media directory
			imagePath := filepath.Join(dir, pattern)
			if fileExists(imagePath) {
				appenrich.AddImage(resp, &pluginv1.EnrichedImage{
					Type:     imageType,
					Path:     imagePath,
					IsRemote: false,
				})
				break // Found one for this type, move on
			}
		}
	}

	// Also check for images matching the media filename
	// e.g., "Movie Name (2020)-poster.jpg" for "Movie Name (2020).mkv"
	baseNameWithoutExt := strings.TrimSuffix(filepath.Base(req.FilePath), filepath.Ext(req.FilePath))
	for imageType, suffixes := range getFilenameSuffixes() {
		for _, suffix := range suffixes {
			for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp"} {
				imagePath := filepath.Join(dir, baseNameWithoutExt+suffix+ext)
				if fileExists(imagePath) {
					appenrich.AddImage(resp, &pluginv1.EnrichedImage{
						Type:     imageType,
						Path:     imagePath,
						IsRemote: false,
					})
					break
				}
			}
		}
	}

	if len(resp.Images) == 0 {
		return appenrich.Skip("no local images found"), nil
	}

	return resp, nil
}

// getImagePatterns returns common image filenames for each type.
func getImagePatterns(mediaType string) map[string][]string {
	// Common patterns used by Kodi, Plex, Jellyfin
	common := map[string][]string{
		"poster": {
			"poster.jpg", "poster.png", "poster.webp",
			"folder.jpg", "folder.png",
			"cover.jpg", "cover.png",
		},
		"fanart": {
			"fanart.jpg", "fanart.png", "fanart.webp",
			"backdrop.jpg", "backdrop.png",
			"background.jpg", "background.png",
		},
		"banner": {
			"banner.jpg", "banner.png",
		},
		"clearlogo": {
			"clearlogo.png", "logo.png",
		},
		"thumb": {
			"thumb.jpg", "thumb.png",
			"landscape.jpg", "landscape.png",
		},
	}

	if mediaType == string(enrichment.MediaTypeMusic) {
		// Music has different conventions
		return map[string][]string{
			"cover": {
				"cover.jpg", "cover.png",
				"folder.jpg", "folder.png",
				"album.jpg", "album.png",
				"front.jpg", "front.png",
			},
			"fanart": {
				"fanart.jpg", "fanart.png",
				"artist.jpg", "artist.png",
			},
			"discart": {
				"disc.png", "cdart.png",
			},
		}
	}

	return common
}

// getFilenameSuffixes returns suffixes to append to media filename.
func getFilenameSuffixes() map[string][]string {
	return map[string][]string{
		"poster":    {"-poster", ".poster", "_poster"},
		"fanart":    {"-fanart", ".fanart", "_fanart", "-backdrop"},
		"thumb":     {"-thumb", ".thumb", "_thumb"},
		"banner":    {"-banner", ".banner"},
		"clearlogo": {"-clearlogo", "-logo"},
	}
}

// fileExists checks if a file exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
