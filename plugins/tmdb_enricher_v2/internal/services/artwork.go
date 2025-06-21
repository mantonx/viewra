package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/mantonx/viewra/plugins/tmdb_enricher_v2/internal/api"
	"github.com/mantonx/viewra/plugins/tmdb_enricher_v2/internal/config"
	"github.com/mantonx/viewra/plugins/tmdb_enricher_v2/internal/models"
	"github.com/mantonx/viewra/plugins/tmdb_enricher_v2/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
	"gorm.io/gorm"
)

// ArtworkService handles artwork downloading and management
type ArtworkService struct {
	db            *gorm.DB
	config        *config.Config
	unifiedClient *plugins.UnifiedServiceClient
	logger        plugins.Logger
	httpClient    *http.Client
	apiClient     *api.APIClient
}

// NewArtworkService creates a new artwork service
func NewArtworkService(db *gorm.DB, cfg *config.Config, client *plugins.UnifiedServiceClient, logger plugins.Logger) *ArtworkService {
	return &ArtworkService{
		db:            db,
		config:        cfg,
		unifiedClient: client,
		logger:        logger,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.Artwork.AssetTimeoutSec) * time.Second,
		},
		apiClient: api.NewAPIClient(cfg, logger),
	}
}

// DownloadArtworkForEnrichment downloads artwork for a TMDb enrichment
func (a *ArtworkService) DownloadArtworkForEnrichment(mediaFileID string, enrichment *models.TMDbEnrichment) error {
	if !a.config.Features.EnableArtwork {
		a.logger.Debug("artwork downloads disabled", "media_file_id", mediaFileID)
		return nil
	}

	a.logger.Info("downloading artwork", "media_file_id", mediaFileID, "tmdb_id", enrichment.TMDbID, "type", enrichment.TMDbType)

	var downloadCount int
	var errors []string

	// Download based on content type
	switch enrichment.TMDbType {
	case "movie":
		if err := a.downloadMovieArtwork(mediaFileID, enrichment.TMDbID, &downloadCount, &errors); err != nil {
			a.logger.Warn("failed to download movie artwork", "error", err)
			errors = append(errors, fmt.Sprintf("movie artwork: %v", err))
		}

	case "tv":
		if err := a.downloadTVShowArtwork(mediaFileID, enrichment.TMDbID, &downloadCount, &errors); err != nil {
			a.logger.Warn("failed to download TV show artwork", "error", err)
			errors = append(errors, fmt.Sprintf("TV show artwork: %v", err))
		}

	case "episode":
		if enrichment.ShowTMDbID != nil && enrichment.SeasonNumber != nil && enrichment.EpisodeNumber != nil {
			if err := a.downloadEpisodeArtwork(mediaFileID, *enrichment.ShowTMDbID, *enrichment.SeasonNumber, *enrichment.EpisodeNumber, &downloadCount, &errors); err != nil {
				a.logger.Warn("failed to download episode artwork", "error", err)
				errors = append(errors, fmt.Sprintf("episode artwork: %v", err))
			}
		}
	}

	a.logger.Info("artwork download completed",
		"media_file_id", mediaFileID,
		"downloaded", downloadCount,
		"errors", len(errors))

	if len(errors) > 0 {
		return fmt.Errorf("artwork download errors: %v", strings.Join(errors, "; "))
	}

	return nil
}

// downloadMovieArtwork downloads artwork for movies
func (a *ArtworkService) downloadMovieArtwork(mediaFileID string, tmdbID int, successCount *int, errors *[]string) error {
	images, err := a.apiClient.GetMovieImages(tmdbID)
	if err != nil {
		return fmt.Errorf("failed to fetch movie images: %w", err)
	}

	if a.config.Artwork.DownloadPosters && len(images.Posters) > 0 {
		if err := a.downloadBestImage(mediaFileID, "movie", "poster", "", images.Posters); err != nil {
			*errors = append(*errors, fmt.Sprintf("poster: %v", err))
		} else {
			*successCount++
		}
	}

	return nil
}

// downloadTVShowArtwork downloads artwork for TV shows
func (a *ArtworkService) downloadTVShowArtwork(mediaFileID string, tmdbID int, successCount *int, errors *[]string) error {
	images, err := a.apiClient.GetTVImages(tmdbID)
	if err != nil {
		return fmt.Errorf("failed to fetch TV images: %w", err)
	}

	if a.config.Artwork.DownloadPosters && len(images.Posters) > 0 {
		if err := a.downloadBestImage(mediaFileID, "tv", "poster", "", images.Posters); err != nil {
			*errors = append(*errors, fmt.Sprintf("poster: %v", err))
		} else {
			*successCount++
		}
	}

	return nil
}

// downloadEpisodeArtwork downloads artwork for episodes
func (a *ArtworkService) downloadEpisodeArtwork(mediaFileID string, tmdbID, seasonNumber, episodeNumber int, successCount *int, errors *[]string) error {
	// Download season poster if enabled
	if a.config.Artwork.DownloadSeasonPosters {
		if err := a.downloadSeasonPoster(mediaFileID, tmdbID, seasonNumber); err != nil {
			*errors = append(*errors, fmt.Sprintf("season poster: %v", err))
		} else {
			*successCount++
		}
	}

	// Download episode still if enabled
	if a.config.Artwork.DownloadEpisodeStills {
		if err := a.downloadEpisodeStill(mediaFileID, tmdbID, seasonNumber, episodeNumber); err != nil {
			*errors = append(*errors, fmt.Sprintf("episode still: %v", err))
		} else {
			*successCount++
		}
	}

	return nil
}

// downloadBestImage downloads the best image from a list of images
func (a *ArtworkService) downloadBestImage(mediaFileID, category, artworkType, subtype string, images []types.ImageInfo) error {
	if len(images) == 0 {
		return fmt.Errorf("no images available")
	}

	// For simplicity, just use the first image
	image := &images[0]
	imageURL := a.buildImageURL(image.FilePath, artworkType)

	return a.downloadAndSaveImage(mediaFileID, category, artworkType, subtype, imageURL, image)
}

// downloadSeasonPoster downloads a poster for a TV season
func (a *ArtworkService) downloadSeasonPoster(mediaFileID string, tmdbID, seasonNumber int) error {
	season, err := a.fetchSeasonDetails(tmdbID, seasonNumber)
	if err != nil {
		return fmt.Errorf("failed to fetch season details: %w", err)
	}

	if season.PosterPath == "" {
		return fmt.Errorf("no season poster available")
	}

	imageURL := a.buildImageURL(season.PosterPath, "poster")
	return a.downloadAndSaveImage(mediaFileID, "season", "poster", fmt.Sprintf("season_%d", seasonNumber), imageURL, nil)
}

// downloadEpisodeStill downloads a still image for an episode
func (a *ArtworkService) downloadEpisodeStill(mediaFileID string, tmdbID, seasonNumber, episodeNumber int) error {
	episode, err := a.fetchEpisodeDetails(tmdbID, seasonNumber, episodeNumber)
	if err != nil {
		return fmt.Errorf("failed to fetch episode details: %w", err)
	}

	if episode.StillPath == "" {
		return fmt.Errorf("no episode still available")
	}

	imageURL := a.buildImageURL(episode.StillPath, "still")
	return a.downloadAndSaveImage(mediaFileID, "episode", "still", fmt.Sprintf("s%de%d", seasonNumber, episodeNumber), imageURL, nil)
}

// downloadAndSaveImage downloads an image and saves it via the unified service
func (a *ArtworkService) downloadAndSaveImage(mediaFileID, category, artworkType, subtype, imageURL string, imageInfo *types.ImageInfo) error {
	// Check if artwork already exists
	if a.config.Artwork.SkipExistingAssets {
		if exists, err := a.artworkExists(mediaFileID, category, artworkType, subtype, imageURL); err != nil {
			a.logger.Warn("failed to check if artwork exists", "error", err)
		} else if exists {
			a.logger.Debug("artwork already exists, skipping", "url", imageURL)
			return nil
		}
	}

	// Download the image
	resp, err := a.httpClient.Get(imageURL)
	if err != nil {
		return fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error %d downloading image", resp.StatusCode)
	}

	// Read the image data
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read image data: %w", err)
	}

	// Check size limits
	if len(data) > a.config.Artwork.MaxAssetSizeMB*1024*1024 {
		return fmt.Errorf("image too large: %d bytes > %d MB limit", len(data), a.config.Artwork.MaxAssetSizeMB)
	}

	// Determine MIME type
	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		// Try to determine from file extension
		ext := strings.ToLower(filepath.Ext(imageURL))
		switch ext {
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".png":
			mimeType = "image/png"
		case ".webp":
			mimeType = "image/webp"
		default:
			mimeType = "application/octet-stream"
		}
	}

	// Create metadata
	metadata := map[string]string{
		"source":       "tmdb",
		"tmdb_url":     imageURL,
		"artwork_type": artworkType,
	}
	if imageInfo != nil {
		metadata["width"] = fmt.Sprintf("%d", imageInfo.Width)
		metadata["height"] = fmt.Sprintf("%d", imageInfo.Height)
		metadata["aspect_ratio"] = fmt.Sprintf("%.2f", imageInfo.AspectRatio)
		metadata["vote_average"] = fmt.Sprintf("%.1f", imageInfo.VoteAverage)
		metadata["vote_count"] = fmt.Sprintf("%d", imageInfo.VoteCount)
		if imageInfo.ISO639_1 != "" {
			metadata["language"] = imageInfo.ISO639_1
		}
	}

	a.logger.Debug("saving artwork via unified service",
		"media_file_id", mediaFileID,
		"category", category,
		"artwork_type", artworkType,
		"subtype", subtype,
		"size", len(data),
		"mime_type", mimeType)

	// Save via unified service
	if a.unifiedClient != nil {
		request := &plugins.SaveAssetRequest{
			MediaFileID: mediaFileID,
			AssetType:   category,
			Category:    category,
			Subtype:     artworkType,
			Data:        data,
			MimeType:    "image/jpeg",
			SourceURL:   imageURL,
			PluginID:    "tmdb_enricher_v2",
			Metadata:    metadata,
		}

		ctx := context.Background()
		response, err := a.unifiedClient.AssetService().SaveAsset(ctx, request)
		if err != nil {
			return fmt.Errorf("failed to save asset via unified service: %w", err)
		}

		if !response.Success {
			return fmt.Errorf("asset save failed: %s", response.Error)
		}

		// Record in our database
		artwork := &models.TMDbArtwork{
			MediaFileID:  mediaFileID,
			TMDbID:       0, // We'd need to pass this from the caller
			ArtworkType:  artworkType,
			Category:     category,
			Subtype:      subtype,
			OriginalURL:  imageURL,
			LocalPath:    response.RelativePath,
			FileName:     filepath.Base(imageURL),
			MimeType:     mimeType,
			FileSize:     int64(len(data)),
			FileHash:     response.Hash,
			Width:        imageInfo.Width,
			Height:       imageInfo.Height,
			AspectRatio:  imageInfo.AspectRatio,
			Language:     imageInfo.ISO639_1,
			VoteAverage:  imageInfo.VoteAverage,
			VoteCount:    imageInfo.VoteCount,
			SourcePlugin: "tmdb_enricher_v2",
		}

		if err := a.db.Create(artwork).Error; err != nil {
			a.logger.Warn("failed to record artwork in database", "error", err, "asset_id", response.AssetID)
		}

		a.logger.Info("artwork saved successfully",
			"asset_id", response.AssetID,
			"path", response.RelativePath,
			"hash", response.Hash)

		return nil
	}

	return fmt.Errorf("unified client not available")
}

// buildImageURL builds a full TMDb image URL
func (a *ArtworkService) buildImageURL(imagePath, artworkType string) string {
	if imagePath == "" {
		return ""
	}

	baseURL := "https://image.tmdb.org/t/p/"
	size := a.config.Artwork.PosterSize // Simplified for now

	return fmt.Sprintf("%s%s%s", baseURL, size, imagePath)
}

// artworkExists checks if artwork already exists
func (a *ArtworkService) artworkExists(mediaFileID, category, artworkType, subtype, sourceURL string) (bool, error) {
	var count int64
	query := a.db.Model(&models.TMDbArtwork{}).Where(
		"media_file_id = ? AND category = ? AND artwork_type = ? AND original_url = ?",
		mediaFileID, category, artworkType, sourceURL,
	)

	if subtype != "" {
		query = query.Where("subtype = ?", subtype)
	}

	err := query.Count(&count).Error
	return count > 0, err
}

// API methods using shared API client

func (a *ArtworkService) fetchSeasonDetails(tmdbID, seasonNumber int) (*types.TVSeasonDetails, error) {
	return a.apiClient.GetSeasonDetails(tmdbID, seasonNumber)
}

func (a *ArtworkService) fetchEpisodeDetails(tmdbID, seasonNumber, episodeNumber int) (*types.TVEpisodeDetails, error) {
	return a.apiClient.GetEpisodeDetails(tmdbID, seasonNumber, episodeNumber)
}

// UpdateConfiguration updates the artwork service configuration at runtime
func (s *ArtworkService) UpdateConfiguration(newConfig *config.Config) {
	s.config = newConfig
	s.logger.Debug("artwork service configuration updated",
		"download_posters", newConfig.Artwork.DownloadPosters,
		"download_backdrops", newConfig.Artwork.DownloadBackdrops,
		"poster_size", newConfig.Artwork.PosterSize,
		"max_asset_size_mb", newConfig.Artwork.MaxAssetSizeMB)
}
