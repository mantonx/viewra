package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Service handles media business logic
type Service struct {
	repo Repository
}

// NewService creates a new media service
func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// CreateMedia creates a new media item with validation
func (s *Service) CreateMedia(ctx context.Context, media *Media, libraryPath string) error {
	// Validate media entity
	if err := media.IsValid(); err != nil {
		return fmt.Errorf("invalid media: %w", err)
	}

	// Check if file exists in the filesystem
	fullPath := filepath.Join(libraryPath, media.FilePath)
	if err := s.validateFileExists(fullPath); err != nil {
		return err
	}

	// Get actual file size if not provided
	if media.FileSize == 0 {
		info, err := os.Stat(fullPath)
		if err == nil {
			media.FileSize = info.Size()
		}
	}

	// Check for duplicate file path in the library
	exists, err := s.repo.ExistsInLibrary(ctx, media.LibraryID, media.FilePath)
	if err != nil {
		return fmt.Errorf("failed to check for duplicate: %w", err)
	}
	if exists {
		return ErrDuplicateFilePath
	}

	// Create the media item
	if err := s.repo.Create(ctx, media); err != nil {
		return fmt.Errorf("failed to create media: %w", err)
	}

	return nil
}

// GetMediaByID retrieves a media item by ID
func (s *Service) GetMediaByID(ctx context.Context, id int64) (*Media, error) {
	media, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get media: %w", err)
	}
	return media, nil
}

// ListMediaByLibrary retrieves all media items in a library
func (s *Service) ListMediaByLibrary(ctx context.Context, libraryID int64) ([]*Media, error) {
	media, err := s.repo.ListByLibrary(ctx, libraryID)
	if err != nil {
		return nil, fmt.Errorf("failed to list media: %w", err)
	}
	return media, nil
}

// UpdateMedia updates an existing media item
func (s *Service) UpdateMedia(ctx context.Context, media *Media, libraryPath string) error {
	// Validate media entity
	if err := media.IsValid(); err != nil {
		return fmt.Errorf("invalid media: %w", err)
	}

	// Check if file exists in the filesystem
	fullPath := filepath.Join(libraryPath, media.FilePath)
	if err := s.validateFileExists(fullPath); err != nil {
		return err
	}

	// Update the media item
	if err := s.repo.Update(ctx, media); err != nil {
		return fmt.Errorf("failed to update media: %w", err)
	}

	return nil
}

// DeleteMedia removes a media item
func (s *Service) DeleteMedia(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete media: %w", err)
	}
	return nil
}

// validateFileExists checks if a file exists and is readable
func (s *Service) validateFileExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrEmptyFilePath // File doesn't exist
		}
		return fmt.Errorf("failed to access file: %w", err)
	}

	// Ensure it's a file, not a directory
	if info.IsDir() {
		return ErrEmptyFilePath // Path is a directory
	}

	return nil
}
