package images

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/domain/images"
	infraImages "github.com/viewra/viewra/internal/infrastructure/images"
)

// ExtractMovieImagesUseCase handles extracting and cataloging movie images
type ExtractMovieImagesUseCase struct {
	repo              images.Repository
	extractor         *infraImages.Extractor
	metadataExtractor *infraImages.MetadataExtractor
}

// NewExtractMovieImagesUseCase creates a new instance
func NewExtractMovieImagesUseCase(repo images.Repository) *ExtractMovieImagesUseCase {
	return &ExtractMovieImagesUseCase{
		repo:              repo,
		extractor:         infraImages.NewExtractor(),
		metadataExtractor: infraImages.NewMetadataExtractor(),
	}
}

// Execute extracts images for a movie and stores them in the database
func (uc *ExtractMovieImagesUseCase) Execute(ctx context.Context, movieFilePath string, mediaType images.MediaType, entityID int, mediaID *int) error {
	// Extract image paths
	extracted, err := uc.extractor.ExtractMovieImages(movieFilePath)
	if err != nil {
		return fmt.Errorf("failed to extract movie images: %w", err)
	}

	// Process and save all extracted images
	return ProcessAndSaveImages(ctx, uc.repo, uc.metadataExtractor, extracted, mediaType, entityID, mediaID)
}

// ExtractTVEpisodeImagesUseCase handles extracting and cataloging TV episode images
type ExtractTVEpisodeImagesUseCase struct {
	repo              images.Repository
	extractor         *infraImages.Extractor
	metadataExtractor *infraImages.MetadataExtractor
}

// NewExtractTVEpisodeImagesUseCase creates a new instance
func NewExtractTVEpisodeImagesUseCase(repo images.Repository) *ExtractTVEpisodeImagesUseCase {
	return &ExtractTVEpisodeImagesUseCase{
		repo:              repo,
		extractor:         infraImages.NewExtractor(),
		metadataExtractor: infraImages.NewMetadataExtractor(),
	}
}

// Execute extracts images for a TV episode and stores them in the database
func (uc *ExtractTVEpisodeImagesUseCase) Execute(ctx context.Context, episodeFilePath string, mediaType images.MediaType, entityID int, mediaID *int) error {
	// Extract image paths
	extracted, err := uc.extractor.ExtractTVEpisodeImages(episodeFilePath)
	if err != nil {
		return fmt.Errorf("failed to extract episode images: %w", err)
	}

	// Process and save all extracted images
	return ProcessAndSaveImages(ctx, uc.repo, uc.metadataExtractor, extracted, mediaType, entityID, mediaID)
}

// ExtractMusicAlbumImagesUseCase handles extracting and cataloging music album images
type ExtractMusicAlbumImagesUseCase struct {
	repo              images.Repository
	extractor         *infraImages.Extractor
	metadataExtractor *infraImages.MetadataExtractor
}

// NewExtractMusicAlbumImagesUseCase creates a new instance
func NewExtractMusicAlbumImagesUseCase(repo images.Repository) *ExtractMusicAlbumImagesUseCase {
	return &ExtractMusicAlbumImagesUseCase{
		repo:              repo,
		extractor:         infraImages.NewExtractor(),
		metadataExtractor: infraImages.NewMetadataExtractor(),
	}
}

// Execute extracts images for a music album and stores them in the database
func (uc *ExtractMusicAlbumImagesUseCase) Execute(ctx context.Context, albumDir string, mediaType images.MediaType, entityID int) error {
	// Extract image paths
	extracted, err := uc.extractor.ExtractMusicAlbumImages(albumDir)
	if err != nil {
		return fmt.Errorf("failed to extract album images: %w", err)
	}

	// Process and save all extracted images (albums don't have media_id, so pass nil)
	return ProcessAndSaveImages(ctx, uc.repo, uc.metadataExtractor, extracted, mediaType, entityID, nil)
}
