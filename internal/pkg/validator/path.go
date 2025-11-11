package validator

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrInvalidPath is returned when a path is invalid
	ErrInvalidPath = errors.New("invalid path")
	
	// ErrPathNotAbsolute is returned when a path is not absolute
	ErrPathNotAbsolute = errors.New("path must be absolute")
	
	// ErrPathTraversal is returned when a path contains traversal attempts
	ErrPathTraversal = errors.New("path contains invalid traversal")
	
	// ErrPathNotReadable is returned when a path cannot be read
	ErrPathNotReadable = errors.New("path is not readable")
	
	// ErrPathNotDirectory is returned when a path is not a directory
	ErrPathNotDirectory = errors.New("path is not a directory")
)

// ValidateLibraryPath validates and cleans a library path
// Returns the cleaned path and any validation error
func ValidateLibraryPath(path string) (string, error) {
	if path == "" {
		return "", ErrInvalidPath
	}

	// Path must be absolute
	if !filepath.IsAbs(path) {
		return "", ErrPathNotAbsolute
	}

	// Clean path to normalize
	cleanPath := filepath.Clean(path)

	// Check for path traversal attempts (.. after cleaning)
	if strings.Contains(cleanPath, "..") {
		return "", ErrPathTraversal
	}

	return cleanPath, nil
}

// ValidateDirectoryExists checks if a path exists and is an accessible directory
func ValidateDirectoryExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: path does not exist", ErrInvalidPath)
		}
		return fmt.Errorf("%w: %v", ErrPathNotReadable, err)
	}

	if !info.IsDir() {
		return ErrPathNotDirectory
	}

	// Try to read directory to ensure it's accessible
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrPathNotReadable, err)
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err != nil && err.Error() != "EOF" {
		return fmt.Errorf("%w: cannot read directory", ErrPathNotReadable)
	}

	return nil
}
