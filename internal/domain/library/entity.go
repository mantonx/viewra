package library

import (
	"path/filepath"
	"strings"
	"time"
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

// validatePath validates the library path field
func (l *Library) validatePath() error {
	path := strings.TrimSpace(l.Path)
	
	if path == "" {
		return ErrEmptyPath
	}

	// Path must be absolute
	if !filepath.IsAbs(path) {
		return ErrPathNotAbsolute
	}

	// Clean path to normalize
	cleanPath := filepath.Clean(path)

	// Check for path traversal attempts (.. after cleaning)
	if strings.Contains(cleanPath, "..") {
		return ErrPathTraversal
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
