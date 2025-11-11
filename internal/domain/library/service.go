package library

import (
	"context"
	"fmt"

	"github.com/viewra/viewra/internal/pkg/validator"
)

// Service handles library business logic
type Service struct {
	repo Repository
}

// NewService creates a new library service
func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// Create creates a new library after validation
func (s *Service) Create(ctx context.Context, library *Library) error {
	// Validate entity
	if err := library.IsValid(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Validate path exists and is accessible
	if err := s.validatePathExists(library.Path); err != nil {
		return err
	}

	// Check for duplicate path
	exists, err := s.repo.Exists(ctx, library.Path)
	if err != nil {
		return fmt.Errorf("failed to check for duplicate path: %w", err)
	}
	if exists {
		return ErrDuplicatePath
	}

	// Create library
	if err := s.repo.Create(ctx, library); err != nil {
		return fmt.Errorf("failed to create library: %w", err)
	}

	return nil
}

// GetByID retrieves a library by ID
func (s *Service) GetByID(ctx context.Context, id int64) (*Library, error) {
	library, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get library: %w", err)
	}
	return library, nil
}

// GetByPath retrieves a library by path
func (s *Service) GetByPath(ctx context.Context, path string) (*Library, error) {
	library, err := s.repo.GetByPath(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get library by path: %w", err)
	}
	return library, nil
}

// List retrieves all libraries
func (s *Service) List(ctx context.Context) ([]*Library, error) {
	libraries, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list libraries: %w", err)
	}
	return libraries, nil
}

// Update updates an existing library
func (s *Service) Update(ctx context.Context, library *Library) error {
	// Validate entity
	if err := library.IsValid(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Validate path exists and is accessible
	if err := s.validatePathExists(library.Path); err != nil {
		return err
	}

	// Check if library exists
	existing, err := s.repo.GetByID(ctx, library.ID)
	if err != nil {
		return fmt.Errorf("failed to get existing library: %w", err)
	}
	if existing == nil {
		return ErrLibraryNotFound
	}

	// If path changed, check for duplicates
	if existing.Path != library.Path {
		exists, err := s.repo.Exists(ctx, library.Path)
		if err != nil {
			return fmt.Errorf("failed to check for duplicate path: %w", err)
		}
		if exists {
			return ErrDuplicatePath
		}
	}

	// Update library
	if err := s.repo.Update(ctx, library); err != nil {
		return fmt.Errorf("failed to update library: %w", err)
	}

	return nil
}

// Delete deletes a library by ID
func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete library: %w", err)
	}
	return nil
}

// validatePathExists checks if the path exists and is a directory
func (s *Service) validatePathExists(path string) error {
	if err := validator.ValidateDirectoryExists(path); err != nil {
		// Map validator errors to domain errors
		switch {
		case err == validator.ErrPathNotDirectory:
			return ErrPathNotDirectory
		case err == validator.ErrPathNotReadable:
			return ErrPathNotAccessible
		default:
			return err
		}
	}
	return nil
}
