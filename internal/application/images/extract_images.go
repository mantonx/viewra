package images

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/images"
	infraImages "github.com/mantonx/viewra/internal/infrastructure/images"
)

// ExtractMovieImagesUseCase handles extracting and cataloging movie images
type ExtractMovieImagesUseCase struct {
	repo              images.Repository
	extractor         *infraImages.Extractor
	metadataExtractor *infraImages.MetadataExtractor
	cacheService      *infraImages.CacheService
	transformer       *infraImages.Transformer
}

// NewExtractMovieImagesUseCase creates a new instance
func NewExtractMovieImagesUseCase(repo images.Repository, cacheService *infraImages.CacheService, transformer *infraImages.Transformer) *ExtractMovieImagesUseCase {
	return &ExtractMovieImagesUseCase{
		repo:              repo,
		extractor:         infraImages.NewExtractor(),
		metadataExtractor: infraImages.NewMetadataExtractor(),
		cacheService:      cacheService,
		transformer:       transformer,
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
	return ProcessAndSaveImages(ctx, uc.repo, uc.metadataExtractor, uc.cacheService, uc.transformer, extracted, mediaType, entityID, mediaID)
}

// ExtractTVEpisodeImagesUseCase handles extracting and cataloging TV episode images
type ExtractTVEpisodeImagesUseCase struct {
	repo              images.Repository
	extractor         *infraImages.Extractor
	metadataExtractor *infraImages.MetadataExtractor
	cacheService      *infraImages.CacheService
	transformer       *infraImages.Transformer
}

// NewExtractTVEpisodeImagesUseCase creates a new instance
func NewExtractTVEpisodeImagesUseCase(repo images.Repository, cacheService *infraImages.CacheService, transformer *infraImages.Transformer) *ExtractTVEpisodeImagesUseCase {
	return &ExtractTVEpisodeImagesUseCase{
		repo:              repo,
		extractor:         infraImages.NewExtractor(),
		metadataExtractor: infraImages.NewMetadataExtractor(),
		cacheService:      cacheService,
		transformer:       transformer,
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
	return ProcessAndSaveImages(ctx, uc.repo, uc.metadataExtractor, uc.cacheService, uc.transformer, extracted, mediaType, entityID, mediaID)
}

// ExtractMusicAlbumImagesUseCase handles extracting and cataloging music album images
type ExtractMusicAlbumImagesUseCase struct {
	repo              images.Repository
	extractor         *infraImages.Extractor
	metadataExtractor *infraImages.MetadataExtractor
	cacheService      *infraImages.CacheService
	transformer       *infraImages.Transformer
}

// NewExtractMusicAlbumImagesUseCase creates a new instance
func NewExtractMusicAlbumImagesUseCase(repo images.Repository, cacheService *infraImages.CacheService, transformer *infraImages.Transformer) *ExtractMusicAlbumImagesUseCase {
	return &ExtractMusicAlbumImagesUseCase{
		repo:              repo,
		extractor:         infraImages.NewExtractor(),
		metadataExtractor: infraImages.NewMetadataExtractor(),
		cacheService:      cacheService,
		transformer:       transformer,
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
	return ProcessAndSaveImages(ctx, uc.repo, uc.metadataExtractor, uc.cacheService, uc.transformer, extracted, mediaType, entityID, nil)
}

// ExtractTVShowImagesUseCase handles extracting and cataloging TV show images
type ExtractTVShowImagesUseCase struct {
	repo              images.Repository
	extractor         *infraImages.Extractor
	metadataExtractor *infraImages.MetadataExtractor
	cacheService      *infraImages.CacheService
	transformer       *infraImages.Transformer
}

// NewExtractTVShowImagesUseCase creates a new instance
func NewExtractTVShowImagesUseCase(repo images.Repository, cacheService *infraImages.CacheService, transformer *infraImages.Transformer) *ExtractTVShowImagesUseCase {
	return &ExtractTVShowImagesUseCase{
		repo:              repo,
		extractor:         infraImages.NewExtractor(),
		metadataExtractor: infraImages.NewMetadataExtractor(),
		cacheService:      cacheService,
		transformer:       transformer,
	}
}

// Execute extracts images for a TV show and stores them in the database
func (uc *ExtractTVShowImagesUseCase) Execute(ctx context.Context, showDir string, mediaType images.MediaType, entityID int) error {
	// Extract image paths
	extracted, err := uc.extractor.ExtractTVShowImages(showDir)
	if err != nil {
		return fmt.Errorf("failed to extract show images: %w", err)
	}

	// Process and save all extracted images (shows don't have media_id, so pass nil)
	return ProcessAndSaveImages(ctx, uc.repo, uc.metadataExtractor, uc.cacheService, uc.transformer, extracted, mediaType, entityID, nil)
}

// ExtractTVSeasonImagesUseCase handles extracting and cataloging TV season images
type ExtractTVSeasonImagesUseCase struct {
	repo              images.Repository
	extractor         *infraImages.Extractor
	metadataExtractor *infraImages.MetadataExtractor
	cacheService      *infraImages.CacheService
	transformer       *infraImages.Transformer
}

// NewExtractTVSeasonImagesUseCase creates a new instance
func NewExtractTVSeasonImagesUseCase(repo images.Repository, cacheService *infraImages.CacheService, transformer *infraImages.Transformer) *ExtractTVSeasonImagesUseCase {
	return &ExtractTVSeasonImagesUseCase{
		repo:              repo,
		extractor:         infraImages.NewExtractor(),
		metadataExtractor: infraImages.NewMetadataExtractor(),
		cacheService:      cacheService,
		transformer:       transformer,
	}
}

// Execute extracts images for a TV season and stores them in the database
func (uc *ExtractTVSeasonImagesUseCase) Execute(ctx context.Context, showDir string, seasonNumber int, mediaType images.MediaType, entityID int) error {
	// Extract image paths
	extracted, err := uc.extractor.ExtractTVSeasonImages(showDir, seasonNumber)
	if err != nil {
		return fmt.Errorf("failed to extract season images: %w", err)
	}

	// Process and save all extracted images (seasons don't have media_id, so pass nil)
	return ProcessAndSaveImages(ctx, uc.repo, uc.metadataExtractor, uc.cacheService, uc.transformer, extracted, mediaType, entityID, nil)
}

// ExtractMusicArtistImagesUseCase handles extracting and cataloging music artist images
type ExtractMusicArtistImagesUseCase struct {
	repo              images.Repository
	extractor         *infraImages.Extractor
	metadataExtractor *infraImages.MetadataExtractor
	cacheService      *infraImages.CacheService
	transformer       *infraImages.Transformer
}

// NewExtractMusicArtistImagesUseCase creates a new instance
func NewExtractMusicArtistImagesUseCase(repo images.Repository, cacheService *infraImages.CacheService, transformer *infraImages.Transformer) *ExtractMusicArtistImagesUseCase {
	return &ExtractMusicArtistImagesUseCase{
		repo:              repo,
		extractor:         infraImages.NewExtractor(),
		metadataExtractor: infraImages.NewMetadataExtractor(),
		cacheService:      cacheService,
		transformer:       transformer,
	}
}

// Execute extracts images for a music artist and stores them in the database
func (uc *ExtractMusicArtistImagesUseCase) Execute(ctx context.Context, artistDir string, mediaType images.MediaType, entityID int) error {
	// Extract image paths
	extracted, err := uc.extractor.ExtractMusicArtistImages(artistDir)
	if err != nil {
		return fmt.Errorf("failed to extract artist images: %w", err)
	}

	// Process and save all extracted images (artists don't have media_id, so pass nil)
	return ProcessAndSaveImages(ctx, uc.repo, uc.metadataExtractor, uc.cacheService, uc.transformer, extracted, mediaType, entityID, nil)
}
