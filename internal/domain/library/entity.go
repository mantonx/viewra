package library

import (
	"strings"
	"time"

	"github.com/viewra/viewra/internal/pkg/validator"
)

// Library represents a media library containing movies, TV shows, or music
type Library struct {
	ID        int64
	Name      string
	Path      string
	Type      LibraryType
	CreatedAt time.Time
	UpdatedAt time.Time
}

// IsValid validates the library entity
func (l *Library) IsValid() error {
	if err := l.validateName(); err != nil {
		return err
	}

	if err := l.validatePath(); err != nil {
		return err
	}

	if err := l.validateType(); err != nil {
		return err
	}

	return nil
}

// validateName checks if the library name is valid
func (l *Library) validateName() error {
	name := strings.TrimSpace(l.Name)
	if name == "" {
		return ErrInvalidName
	}

	if len(name) > 100 {
		return ErrNameTooLong
	}

	l.Name = name
	return nil
}

// validatePath checks if the library path is valid
func (l *Library) validatePath() error {
	cleanPath, err := validator.ValidateLibraryPath(l.Path)
	if err != nil {
		// Map validator errors to domain errors
		switch err {
		case validator.ErrInvalidPath:
			return ErrInvalidPath
		case validator.ErrPathNotAbsolute:
			return ErrPathNotAbsolute
		case validator.ErrPathTraversal:
			return ErrPathTraversal
		default:
			return err
		}
	}

	l.Path = cleanPath
	return nil
}

// validateType checks if the library type is valid
func (l *Library) validateType() error {
	switch l.Type {
	case LibraryTypeMovies, LibraryTypeTV, LibraryTypeMusic:
		return nil
	default:
		return ErrInvalidType
	}
}
