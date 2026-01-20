package pipeline

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"log/slog"
	"os"
	"strings"

	pluginv1 "github.com/mantonx/viewra/api/proto/plugin"
	"github.com/mantonx/viewra/internal/domain/enrichment"
	"github.com/mantonx/viewra/internal/domain/images"
)

// ImageProcessor handles image discovery, metadata extraction, and storage.
// Phase 1 implementation with full processing pipeline:
// - Metadata extraction (dimensions, hash, MIME type)
// - Deduplication via file hash
// - WebP preset generation
// - Remote image downloading
// - SVG rasterization (converts vector graphics to raster)
type ImageProcessor struct {
	imageRepo         ImageRepository
	metadataExtractor MetadataExtractor
	transformer       ImageTransformer
	downloader        ImageDownloader
	svgRasterizer     SVGRasterizer
	logger            *slog.Logger
}

// NewImageProcessor creates a new ImageProcessor.
// All dependencies except imageRepo and logger are optional.
func NewImageProcessor(
	imageRepo ImageRepository,
	logger *slog.Logger,
	opts ...ImageProcessorOption,
) *ImageProcessor {
	p := &ImageProcessor{
		imageRepo: imageRepo,
		logger:    logger,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// ImageProcessorOption configures optional ImageProcessor dependencies.
type ImageProcessorOption func(*ImageProcessor)

// WithMetadataExtractor sets the metadata extractor.
func WithMetadataExtractor(e MetadataExtractor) ImageProcessorOption {
	return func(p *ImageProcessor) { p.metadataExtractor = e }
}

// WithTransformer sets the image transformer.
func WithTransformer(t ImageTransformer) ImageProcessorOption {
	return func(p *ImageProcessor) { p.transformer = t }
}

// WithDownloader sets the image downloader for remote images.
func WithDownloader(d ImageDownloader) ImageProcessorOption {
	return func(p *ImageProcessor) { p.downloader = d }
}

// WithSVGRasterizer sets the SVG rasterizer for converting vector graphics.
func WithSVGRasterizer(r SVGRasterizer) ImageProcessorOption {
	return func(p *ImageProcessor) { p.svgRasterizer = r }
}

// Process stores discovered images in the database with full processing.
func (p *ImageProcessor) Process(ctx context.Context, mediaID int64, mediaType enrichment.MediaType, enrichedImages []*pluginv1.EnrichedImage) error {
	if p.imageRepo == nil {
		return nil // No image repo configured, skip
	}

	// Map enrichment media type to images domain media type
	imgMediaType := mapToImageMediaType(mediaType)
	if imgMediaType == "" {
		return nil // Unknown media type for images
	}

	// Determine if this entity has a media table entry.
	// Only movie, tv (episode), and music (track) have media.id references.
	// Parent entities (tv_show, tv_season, music_album, music_artist) don't.
	hasMediaTableEntry := isMediaTableEntity(mediaType)

	for _, img := range enrichedImages {
		if img.IsRemote {
			if err := p.processRemoteImage(ctx, mediaID, imgMediaType, img, hasMediaTableEntry); err != nil {
				p.logger.Warn("failed to process remote image",
					slog.String("type", img.Type),
					slog.String("url", img.Path),
					slog.Any("error", err))
			}
			continue
		}

		if err := p.processLocalImage(ctx, mediaID, imgMediaType, img, hasMediaTableEntry); err != nil {
			p.logger.Warn("failed to process local image",
				slog.String("type", img.Type),
				slog.String("path", img.Path),
				slog.Any("error", err))
		}
	}

	return nil
}

// processLocalImage handles a local filesystem image with full pipeline.
// hasMediaTableEntry indicates if the entity has an entry in the media table (for FK constraint).
func (p *ImageProcessor) processLocalImage(ctx context.Context, mediaID int64, imgMediaType images.MediaType, img *pluginv1.EnrichedImage, hasMediaTableEntry bool) error {
	imgType := images.ImageType(img.Type)
	if !imgType.IsValid() {
		return fmt.Errorf("unknown image type: %s", img.Type)
	}

	// 1. Extract metadata if extractor is available
	var metadata *ImageMetadata
	if p.metadataExtractor != nil {
		var err error
		metadata, err = p.metadataExtractor.ExtractMetadata(img.Path)
		if err != nil {
			return fmt.Errorf("extract metadata: %w", err)
		}
	}

	// 2. Check for deduplication via hash
	if metadata != nil && metadata.FileHash != nil {
		isDup, err := p.checkDuplicate(ctx, *metadata.FileHash, mediaID, imgType)
		if err != nil {
			p.logger.Debug("deduplication check failed", slog.Any("error", err))
		} else if isDup {
			p.logger.Debug("skipping duplicate image",
				slog.String("hash", *metadata.FileHash),
				slog.String("type", img.Type))
			return nil
		}
	}

	// 3. Generate WebP presets if transformer is available
	var localCachePath *string
	if p.transformer != nil && metadata != nil && metadata.FileHash != nil {
		presetPaths, err := p.generatePresets(img.Path, *metadata.FileHash, imgType)
		if err != nil {
			p.logger.Warn("failed to generate presets",
				slog.String("path", img.Path),
				slog.Any("error", err))
			// Continue without presets - not a fatal error
		} else if medium, ok := presetPaths["medium"]; ok {
			localCachePath = &medium
		}
	}

	// 4. Build image record
	// Only set MediaID for entities that have entries in the media table.
	// Parent entities (tv_show, tv_season, music_album, music_artist) don't have media.id references.
	var mediaIDPtr *int
	if hasMediaTableEntry {
		mediaIDPtr = intPtr(int(mediaID))
	}

	image := &images.Image{
		MediaID:        mediaIDPtr,
		MediaType:      imgMediaType,
		EntityID:       int(mediaID),
		ImageType:      imgType,
		SourceType:     images.SourceTypeLocal,
		FilePath:       img.Path,
		LocalCachePath: localCachePath,
		Priority:       0, // Default priority for local images
	}

	// Add metadata if available
	if metadata != nil {
		image.Width = metadata.Width
		image.Height = metadata.Height
		image.FileSizeBytes = metadata.FileSizeBytes
		image.MimeType = metadata.MimeType
		image.FileHash = metadata.FileHash
	}

	// Add language if provided
	if img.Language != "" {
		image.Language = stringPtr(img.Language)
	}

	// 5. Store in database
	return p.imageRepo.Create(ctx, image)
}

// processRemoteImage handles a remote URL image with download and processing.
// hasMediaTableEntry indicates if the entity has an entry in the media table (for FK constraint).
func (p *ImageProcessor) processRemoteImage(ctx context.Context, mediaID int64, imgMediaType images.MediaType, img *pluginv1.EnrichedImage, hasMediaTableEntry bool) error {
	if p.downloader == nil {
		p.logger.Debug("skipping remote image (downloader not configured)",
			slog.String("type", img.Type),
			slog.String("url", img.Path))
		return nil
	}

	imgType := images.ImageType(img.Type)
	if !imgType.IsValid() {
		return fmt.Errorf("unknown image type: %s", img.Type)
	}

	// 1. Check if we already have this URL for this entity
	existing, err := p.imageRepo.GetByExternalURL(ctx, img.Path, mediaID)
	if err == nil && existing != nil {
		p.logger.Debug("skipping already-downloaded remote image",
			slog.String("url", img.Path))
		return nil
	}

	// 2. Download the image
	localPath, err := p.downloader.Download(ctx, img.Path)
	if err != nil {
		return fmt.Errorf("download image: %w", err)
	}

	// 3. Handle SVG files - rasterize to PNG for processing
	if isSVGFile(img.Path) || isSVGFile(localPath) {
		rasterizedPath, err := p.rasterizeSVG(localPath)
		if err != nil {
			os.Remove(localPath)
			return fmt.Errorf("rasterize svg: %w", err)
		}
		// Remove original SVG, use rasterized PNG
		os.Remove(localPath)
		localPath = rasterizedPath
	}

	// 4. Extract metadata from downloaded file
	var metadata *ImageMetadata
	if p.metadataExtractor != nil {
		metadata, err = p.metadataExtractor.ExtractMetadata(localPath)
		if err != nil {
			// Cleanup failed download
			os.Remove(localPath)
			return fmt.Errorf("extract metadata: %w", err)
		}
	}

	// 5. Generate presets
	var localCachePath *string
	if p.transformer != nil && metadata != nil && metadata.FileHash != nil {
		presetPaths, err := p.generatePresets(localPath, *metadata.FileHash, imgType)
		if err != nil {
			p.logger.Warn("failed to generate presets for remote image",
				slog.String("url", img.Path),
				slog.Any("error", err))
		} else if medium, ok := presetPaths["medium"]; ok {
			localCachePath = &medium
		}
	}

	// 6. Detect source type from URL
	sourceType := detectSourceFromURL(img.Path)

	// 7. Store image record with both external URL and local cache path
	// Only set MediaID for entities that have entries in the media table.
	var mediaIDPtr *int
	if hasMediaTableEntry {
		mediaIDPtr = intPtr(int(mediaID))
	}

	externalURL := img.Path
	image := &images.Image{
		MediaID:        mediaIDPtr,
		MediaType:      imgMediaType,
		EntityID:       int(mediaID),
		ImageType:      imgType,
		SourceType:     sourceType,
		ExternalURL:    &externalURL,
		FilePath:       localPath, // Downloaded original
		LocalCachePath: localCachePath,
		Priority:       10, // Remote images typically lower priority than local
	}

	// Add metadata if available
	if metadata != nil {
		image.Width = metadata.Width
		image.Height = metadata.Height
		image.FileSizeBytes = metadata.FileSizeBytes
		image.MimeType = metadata.MimeType
		image.FileHash = metadata.FileHash
	}

	// Add language if provided
	if img.Language != "" {
		image.Language = stringPtr(img.Language)
	}

	return p.imageRepo.Create(ctx, image)
}

// checkDuplicate checks if an image with the same hash already exists for this entity.
func (p *ImageProcessor) checkDuplicate(ctx context.Context, hash string, mediaID int64, imgType images.ImageType) (bool, error) {
	existing, err := p.imageRepo.GetByHash(ctx, hash)
	if err != nil {
		return false, err
	}

	for _, img := range existing {
		// Check if same entity and same type
		if img.MediaID != nil && int64(*img.MediaID) == mediaID && img.ImageType == imgType {
			return true, nil
		}
	}

	return false, nil
}

// generatePresets generates WebP presets for an image.
func (p *ImageProcessor) generatePresets(sourcePath, fileHash string, imgType images.ImageType) (map[string]string, error) {
	return p.transformer.TransformAllPresets(sourcePath, fileHash, imgType)
}

// detectSourceFromURL infers the source type from a remote image URL.
func detectSourceFromURL(url string) images.SourceType {
	switch {
	case strings.Contains(url, "themoviedb.org") || strings.Contains(url, "tmdb.org"):
		return images.SourceTypeTMDb
	case strings.Contains(url, "thetvdb.com"):
		return images.SourceTypeTVDb
	case strings.Contains(url, "fanart.tv"):
		return images.SourceTypeFanartTV
	case strings.Contains(url, "musicbrainz.org") || strings.Contains(url, "coverartarchive.org"):
		return images.SourceTypeMusicBrainz
	default:
		return images.SourceTypeLocal
	}
}

// isSVGFile checks if a path or URL points to an SVG file.
func isSVGFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".svg")
}

// rasterizeSVG converts an SVG file to a PNG file for further processing.
// Returns the path to the rasterized PNG file.
func (p *ImageProcessor) rasterizeSVG(svgPath string) (string, error) {
	if p.svgRasterizer == nil {
		return "", fmt.Errorf("SVG rasterizer not configured")
	}

	// Rasterize SVG to RGBA image
	img, err := p.svgRasterizer.RasterizeFile(svgPath)
	if err != nil {
		return "", fmt.Errorf("rasterize: %w", err)
	}

	// Save as PNG (preserves transparency, which WebP also supports)
	pngPath := strings.TrimSuffix(svgPath, ".svg") + ".png"
	pngPath = strings.TrimSuffix(pngPath, ".SVG") + ".png"

	f, err := os.Create(pngPath)
	if err != nil {
		return "", fmt.Errorf("create png file: %w", err)
	}
	defer f.Close()

	if err := encodePNG(f, img); err != nil {
		os.Remove(pngPath)
		return "", fmt.Errorf("encode png: %w", err)
	}

	return pngPath, nil
}

// encodePNG writes an image to a writer in PNG format.
func encodePNG(w io.Writer, img image.Image) error {
	return png.Encode(w, img)
}
