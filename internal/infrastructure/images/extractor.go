package images

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mantonx/viewra/internal/domain/images"
)

// Extractor discovers and catalogs image files for media items
type Extractor struct {
	metadataExtractor  *MetadataExtractor
	embeddedExtractor  *EmbeddedExtractor
}

// NewExtractor creates a new image extractor
func NewExtractor() *Extractor {
	return &Extractor{
		metadataExtractor: NewMetadataExtractor(),
		embeddedExtractor: NewEmbeddedExtractor(),
	}
}

// ExtractMovieImages discovers all images for a movie file
// Supports Kodi/Plex naming conventions:
// - poster.jpg, movie-poster.jpg, folder.jpg (poster)
// - fanart.jpg, movie-fanart.jpg (fanart)
// - clearlogo.png (clearlogo)
// - landscape.jpg (landscape)
// - actor/*.jpg (actors)
// - extrathumbs/*.jpg (extra backdrops)
func (e *Extractor) ExtractMovieImages(movieFilePath string) (*ExtractedImages, error) {
	dir := filepath.Dir(movieFilePath)
	baseName := getBaseName(movieFilePath)

	var imageInfos []ImageInfo

	// Poster variations
	posterPatterns := []string{
		"poster.jpg", "poster.png",
		"folder.jpg", "folder.png",
		baseName + "-poster.jpg", baseName + "-poster.png",
		"movie.jpg", "movie.png",
	}
	if img := e.findFirstMatchingImage(dir, posterPatterns, images.ImageTypePoster, 0); img != nil {
		imageInfos = append(imageInfos, *img)
	}

	// Fanart variations
	fanartPatterns := []string{
		"fanart.jpg", "fanart.png",
		baseName + "-fanart.jpg", baseName + "-fanart.png",
		"backdrop.jpg", "backdrop.png",
	}
	if img := e.findFirstMatchingImage(dir, fanartPatterns, images.ImageTypeFanart, 0); img != nil {
		imageInfos = append(imageInfos, *img)
	}

	// Clear logo
	clearLogoPatterns := []string{
		"clearlogo.png",
		baseName + "-clearlogo.png",
		"logo.png",
	}
	if img := e.findFirstMatchingImage(dir, clearLogoPatterns, images.ImageTypeClearLogo, 0); img != nil {
		imageInfos = append(imageInfos, *img)
	}

	// Landscape
	landscapePatterns := []string{
		"landscape.jpg", "landscape.png",
		baseName + "-landscape.jpg", baseName + "-landscape.png",
	}
	if img := e.findFirstMatchingImage(dir, landscapePatterns, images.ImageTypeLandscape, 0); img != nil {
		imageInfos = append(imageInfos, *img)
	}

	// Banner
	bannerPatterns := []string{
		"banner.jpg", "banner.png",
		baseName + "-banner.jpg", baseName + "-banner.png",
	}
	if img := e.findFirstMatchingImage(dir, bannerPatterns, images.ImageTypeBanner, 0); img != nil {
		imageInfos = append(imageInfos, *img)
	}

	// Actor images (directory of actor headshots)
	actorDir := filepath.Join(dir, "actors")
	if actorImages := e.findImagesInDir(actorDir, images.ImageTypeActor); actorImages != nil {
		imageInfos = append(imageInfos, actorImages...)
	}

	// Extra thumbnails/backdrops
	extraThumbsDir := filepath.Join(dir, "extrathumbs")
	if extraImages := e.findImagesInDir(extraThumbsDir, images.ImageTypeExtraFanart); extraImages != nil {
		imageInfos = append(imageInfos, extraImages...)
	}

	return &ExtractedImages{Images: imageInfos}, nil
}

// ExtractTVShowImages discovers show-level images
func (e *Extractor) ExtractTVShowImages(showDir string) (*ExtractedImages, error) {
	var imageInfos []ImageInfo

	// Show poster
	posterPatterns := []string{
		"poster.jpg", "poster.png",
		"folder.jpg", "folder.png",
		"tvshow-poster.jpg", "tvshow-poster.png",
	}
	if img := e.findFirstMatchingImage(showDir, posterPatterns, images.ImageTypePoster, 0); img != nil {
		imageInfos = append(imageInfos, *img)
	}

	// Show fanart
	fanartPatterns := []string{
		"fanart.jpg", "fanart.png",
		"tvshow-fanart.jpg", "tvshow-fanart.png",
	}
	if img := e.findFirstMatchingImage(showDir, fanartPatterns, images.ImageTypeFanart, 0); img != nil {
		imageInfos = append(imageInfos, *img)
	}

	// Show logo
	logoPatterns := []string{
		"clearlogo.png",
		"logo.png",
	}
	if img := e.findFirstMatchingImage(showDir, logoPatterns, images.ImageTypeClearLogo, 0); img != nil {
		imageInfos = append(imageInfos, *img)
	}

	// Banner
	bannerPatterns := []string{
		"banner.jpg", "banner.png",
	}
	if img := e.findFirstMatchingImage(showDir, bannerPatterns, images.ImageTypeBanner, 0); img != nil {
		imageInfos = append(imageInfos, *img)
	}

	return &ExtractedImages{Images: imageInfos}, nil
}

// ExtractTVSeasonImages discovers season-level images
func (e *Extractor) ExtractTVSeasonImages(showDir string, seasonNumber int) (*ExtractedImages, error) {
	var imageInfos []ImageInfo

	// Season poster: season01-poster.jpg, season-specials-poster.jpg
	var seasonPrefix string
	if seasonNumber == 0 {
		seasonPrefix = "season-specials"
	} else {
		seasonPrefix = fmt.Sprintf("season%02d", seasonNumber)
	}

	posterPatterns := []string{
		seasonPrefix + "-poster.jpg",
		seasonPrefix + "-poster.png",
		seasonPrefix + ".jpg",
		seasonPrefix + ".png",
	}
	if img := e.findFirstMatchingImage(showDir, posterPatterns, images.ImageTypePoster, 0); img != nil {
		imageInfos = append(imageInfos, *img)
	}

	// Season fanart
	fanartPatterns := []string{
		seasonPrefix + "-fanart.jpg",
		seasonPrefix + "-fanart.png",
	}
	if img := e.findFirstMatchingImage(showDir, fanartPatterns, images.ImageTypeFanart, 0); img != nil {
		imageInfos = append(imageInfos, *img)
	}

	return &ExtractedImages{Images: imageInfos}, nil
}

// ExtractTVEpisodeImages discovers episode-level images (thumbnails)
func (e *Extractor) ExtractTVEpisodeImages(episodeFilePath string) (*ExtractedImages, error) {
	dir := filepath.Dir(episodeFilePath)
	baseName := getBaseName(episodeFilePath)

	var imageInfos []ImageInfo

	// Episode thumbnail: S01E01-thumb.jpg or filename-thumb.jpg
	thumbPatterns := []string{
		baseName + "-thumb.jpg",
		baseName + "-thumb.png",
		baseName + ".jpg",
		baseName + ".png",
	}
	if img := e.findFirstMatchingImage(dir, thumbPatterns, images.ImageTypeThumb, 0); img != nil {
		imageInfos = append(imageInfos, *img)
	}

	return &ExtractedImages{Images: imageInfos}, nil
}

// ExtractMusicArtistImages discovers artist-level images
// Priority: Filesystem images first, then embedded artwork as fallback
func (e *Extractor) ExtractMusicArtistImages(artistDir string) (*ExtractedImages, error) {
	var imageInfos []ImageInfo

	// 1. Try filesystem images first (highest priority - explicit user choice)
	// Artist folder/poster
	posterPatterns := []string{
		"folder.jpg", "folder.png",
		"artist.jpg", "artist.png",
	}
	if img := e.findFirstMatchingImage(artistDir, posterPatterns, images.ImageTypeFolder, 0); img != nil {
		imageInfos = append(imageInfos, *img)
	} else {
		// 2. Fallback to embedded artwork from first audio file in artist directory
		if embeddedImg, err := e.embeddedExtractor.ExtractArtistArtFromFirstTrack(artistDir); err == nil && embeddedImg != nil {
			imageInfos = append(imageInfos, *embeddedImg)
		}
	}

	// Artist fanart (filesystem only - embedded tags don't typically have fanart)
	fanartPatterns := []string{
		"fanart.jpg", "fanart.png",
	}
	if img := e.findFirstMatchingImage(artistDir, fanartPatterns, images.ImageTypeFanart, 0); img != nil {
		imageInfos = append(imageInfos, *img)
	}

	// Artist logo (filesystem only - embedded tags don't typically have logos)
	logoPatterns := []string{
		"logo.png",
		"clearlogo.png",
	}
	if img := e.findFirstMatchingImage(artistDir, logoPatterns, images.ImageTypeLogo, 0); img != nil {
		imageInfos = append(imageInfos, *img)
	}

	return &ExtractedImages{Images: imageInfos}, nil
}

// ExtractMusicAlbumImages discovers album-level images
// Priority: Filesystem images first, then embedded artwork as fallback
func (e *Extractor) ExtractMusicAlbumImages(albumDir string) (*ExtractedImages, error) {
	var imageInfos []ImageInfo

	// 1. Try filesystem images first (highest priority - explicit user choice)
	coverPatterns := []string{
		"cover.jpg", "cover.png",
		"folder.jpg", "folder.png",
		"album.jpg", "album.png",
		"front.jpg", "front.png",
	}
	if img := e.findFirstMatchingImage(albumDir, coverPatterns, images.ImageTypeCover, 0); img != nil {
		imageInfos = append(imageInfos, *img)
	} else {
		// 2. Fallback to embedded artwork from first audio file
		if embeddedImg, err := e.embeddedExtractor.ExtractAlbumArtFromFirstTrack(albumDir); err == nil && embeddedImg != nil {
			imageInfos = append(imageInfos, *embeddedImg)
		}
	}

	// Disc art (filesystem only - embedded tags don't typically have disc art)
	discArtPatterns := []string{
		"discart.png",
		"disc.png",
		"cd.png",
	}
	if img := e.findFirstMatchingImage(albumDir, discArtPatterns, images.ImageTypeDiscArt, 0); img != nil {
		imageInfos = append(imageInfos, *img)
	}

	return &ExtractedImages{Images: imageInfos}, nil
}

// ExtractMusicTrackImages extracts embedded album art from a specific music track file
func (e *Extractor) ExtractMusicTrackImages(trackFile string) (*ExtractedImages, error) {
	var imageInfos []ImageInfo

	// Extract embedded artwork from this specific track file
	if embeddedImg, err := e.embeddedExtractor.ExtractAlbumArtFromTrack(trackFile); err == nil && embeddedImg != nil {
		imageInfos = append(imageInfos, *embeddedImg)
	}

	return &ExtractedImages{Images: imageInfos}, nil
}

// findFirstMatchingImage searches for the first matching image from a list of patterns
// Returns nil if none found. This eliminates repetitive pattern-matching loops.
func (e *Extractor) findFirstMatchingImage(dir string, patterns []string, imageType images.ImageType, priority int) *ImageInfo {
	for _, pattern := range patterns {
		if img := e.findImage(dir, pattern, imageType, priority); img != nil {
			return img
		}
	}
	return nil
}

// findImage looks for an image file (case-insensitive) and returns ImageInfo
func (e *Extractor) findImage(dir, filename string, imageType images.ImageType, priority int) *ImageInfo {
	// Try exact match first
	path := filepath.Join(dir, filename)
	if fileExists(path) {
		return &ImageInfo{
			Path:     path,
			Type:     imageType,
			Priority: priority,
		}
	}

	// Try case-insensitive search
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	lowerFilename := strings.ToLower(filename)
	for _, entry := range entries {
		if !entry.IsDir() && strings.ToLower(entry.Name()) == lowerFilename {
			return &ImageInfo{
				Path:     filepath.Join(dir, entry.Name()),
				Type:     imageType,
				Priority: priority,
			}
		}
	}

	return nil
}

// findImagesInDir finds all image files in a directory
func (e *Extractor) findImagesInDir(dir string, imageType images.ImageType) []ImageInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var imageInfos []ImageInfo
	priority := 0
	for _, entry := range entries {
		if !entry.IsDir() && IsImageFile(entry.Name()) {
			imageInfos = append(imageInfos, ImageInfo{
				Path:     filepath.Join(dir, entry.Name()),
				Type:     imageType,
				Priority: priority,
			})
			priority++
		}
	}

	return imageInfos
}

// getBaseName returns the filename without extension
func getBaseName(filePath string) string {
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
