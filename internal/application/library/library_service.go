package library

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/mantonx/viewra/internal/application/common"
	"github.com/mantonx/viewra/internal/application/library/scan/cleanup"
	domainImages "github.com/mantonx/viewra/internal/domain/images"
	"github.com/mantonx/viewra/internal/domain/library"
)

// LibraryService provides all CRUD operations for libraries
// Consolidates Create, Get, List, Update, and Delete use cases into a single service
type LibraryService struct {
	repo         library.Repository
	imageRepo    domainImages.Repository
	imageCleanup cleanup.ImageCleanupExecutor
	txManager    *common.TxManager
	logger       *slog.Logger
}

// NewLibraryService creates a new instance of LibraryService
func NewLibraryService(
	repo library.Repository,
	imageRepo domainImages.Repository,
	imageCleanup cleanup.ImageCleanupExecutor,
	txManager *common.TxManager,
	logger *slog.Logger,
) *LibraryService {
	return &LibraryService{
		repo:         repo,
		imageRepo:    imageRepo,
		imageCleanup: imageCleanup,
		txManager:    txManager,
		logger:       logger,
	}
}

// Create creates a new library with the given request data
func (s *LibraryService) Create(ctx context.Context, req CreateLibraryRequest) (LibraryResponse, error) {
	// Convert DTO to domain entity
	lib := &library.Library{
		Name:      req.Name,
		Path:      req.Path,
		Type:      library.LibraryType(req.Type),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Validate domain entity
	if err := lib.IsValid(); err != nil {
		return LibraryResponse{}, fmt.Errorf("validation failed: %w", err)
	}

	// Check if path exists on filesystem (before transaction)
	if _, err := os.Stat(lib.Path); os.IsNotExist(err) {
		return LibraryResponse{}, library.ErrPathNotFound
	}

	// Execute within transaction to ensure atomicity of check + create
	err := s.txManager.WithTransaction(ctx, func(tx *sql.Tx) error {
		// Check if library with this path already exists
		exists, err := s.repo.ExistsWithTx(ctx, tx, lib.Path)
		if err != nil {
			return fmt.Errorf("failed to check library existence: %w", err)
		}
		if exists {
			return library.ErrDuplicatePath
		}

		// Create the library
		if err := s.repo.CreateWithTx(ctx, tx, lib); err != nil {
			return fmt.Errorf("failed to create library: %w", err)
		}

		return nil
	})

	if err != nil {
		return LibraryResponse{}, err
	}

	return ToLibraryResponse(lib), nil
}

// Get retrieves a library by its ID
func (s *LibraryService) Get(ctx context.Context, id int64) (LibraryResponse, error) {
	lib, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return LibraryResponse{}, fmt.Errorf("failed to get library: %w", err)
	}

	return ToLibraryResponse(lib), nil
}

// List retrieves all libraries
func (s *LibraryService) List(ctx context.Context) (ListLibrariesResponse, error) {
	libs, err := s.repo.List(ctx)
	if err != nil {
		return ListLibrariesResponse{}, fmt.Errorf("failed to list libraries: %w", err)
	}

	return ToListLibrariesResponse(libs), nil
}

// Update updates an existing library with the given request data
func (s *LibraryService) Update(ctx context.Context, id int64, req UpdateLibraryRequest) (LibraryResponse, error) {
	// Get existing library
	lib, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return LibraryResponse{}, fmt.Errorf("failed to get library: %w", err)
	}

	// Update fields if provided
	if req.Name != "" {
		lib.Name = req.Name
	}
	if req.Path != "" {
		lib.Path = req.Path
	}
	if req.Type != "" {
		lib.Type = library.LibraryType(req.Type)
	}

	lib.UpdatedAt = time.Now()

	// Validate domain entity
	if err := lib.IsValid(); err != nil {
		return LibraryResponse{}, fmt.Errorf("validation failed: %w", err)
	}

	// Check if path exists on filesystem (if path was updated)
	if req.Path != "" {
		if _, err := os.Stat(lib.Path); os.IsNotExist(err) {
			return LibraryResponse{}, library.ErrPathNotFound
		}

		// Check if another library with this path already exists
		existing, err := s.repo.GetByPath(ctx, lib.Path)
		if err != nil && err != library.ErrLibraryNotFound {
			return LibraryResponse{}, fmt.Errorf("failed to check library path: %w", err)
		}
		if existing != nil && existing.ID != id {
			return LibraryResponse{}, library.ErrDuplicatePath
		}
	}

	// Update the library
	if err := s.repo.Update(ctx, lib); err != nil {
		return LibraryResponse{}, fmt.Errorf("failed to update library: %w", err)
	}

	return ToLibraryResponse(lib), nil
}

// Delete deletes a library by its ID
//
// IMPORTANT: This only removes database records, NOT actual media files on disk.
// Media files are user-managed. Only cached/generated files are cleaned up.
//
// What gets deleted:
// - Library database record
// - All media database records (CASCADE)
// - All image database records for media items (CASCADE via media_id FK)
// - Entity images (tv_show, tv_season, music_album, music_artist) - explicit cleanup
// - Image cache files (thumbnails, posters, etc.)
// - Transcode job records (CASCADE)
// - Watch progress records (CASCADE)
//
// What is preserved:
// - Original media files on disk (NEVER deleted)
func (s *LibraryService) Delete(ctx context.Context, id int64) error {
	// Execute within transaction to ensure atomicity of check + delete
	err := s.txManager.WithTransaction(ctx, func(tx *sql.Tx) error {
		// Verify library exists
		_, err := s.repo.GetByIDWithTx(ctx, tx, id)
		if err != nil {
			return fmt.Errorf("failed to get library: %w", err)
		}

		// Delete the library
		// This cascades to media table, which cascades to media_images table (ON DELETE CASCADE)
		if err := s.repo.DeleteWithTx(ctx, tx, id); err != nil {
			return fmt.Errorf("failed to delete library: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Clean up orphaned entity images (tv_show, tv_season, music_album, music_artist)
	// These use polymorphic entity_id references without FK constraints, so CASCADE doesn't delete them
	// Runs AFTER transaction commits (outside transaction)
	if s.imageRepo != nil {
		orphanCount, err := s.imageRepo.DeleteOrphanedEntityImages(ctx)
		if err != nil {
			s.logger.Warn("Failed to clean orphaned entity images after deleting library",
				"library_id", id,
				"error", err)
		} else if orphanCount > 0 {
			s.logger.Info("Cleaned orphaned entity images",
				"library_id", id,
				"deleted_count", orphanCount)
		}
	}

	// Trigger image cache cleanup to remove orphaned cache files immediately
	// This is a "best effort" cleanup - failures won't prevent library deletion
	// Runs AFTER transaction commits (outside transaction)
	if s.imageCleanup != nil {
		s.logger.Info("Triggering image cache cleanup after library deletion", "library_id", id)
		stats, err := s.imageCleanup.CleanOrphanedImages(ctx)
		if err != nil {
			// Log but don't fail - the scheduled cleanup will handle this later
			s.logger.Warn("Failed to clean image cache after deleting library",
				"library_id", id,
				"error", err)
		} else if stats != nil {
			s.logger.Info("Image cache cleanup completed",
				"library_id", id,
				"orphaned_files", stats.OrphanedFiles,
				"deleted_files", stats.DeletedFiles,
				"bytes_freed", stats.BytesFreed)
		}
	}

	return nil
}
