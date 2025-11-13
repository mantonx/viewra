package library

import (
	"context"
	"time"
)

// LibraryType represents the type of media in a library
type LibraryType string

const (
	// LibraryTypeMovies represents a movie library
	LibraryTypeMovies LibraryType = "movies"

	// LibraryTypeTV represents a TV show library
	LibraryTypeTV LibraryType = "tv"

	// LibraryTypeMusic represents a music library
	LibraryTypeMusic LibraryType = "music"
)

// IsValid checks if the library type is valid
func (lt LibraryType) IsValid() bool {
	switch lt {
	case LibraryTypeMovies, LibraryTypeTV, LibraryTypeMusic:
		return true
	default:
		return false
	}
}

// String returns the string representation of the library type
func (lt LibraryType) String() string {
	return string(lt)
}

// Directory represents a directory in the filesystem for path selection
type Directory struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Readable   bool      `json:"readable"`
	Writable   bool      `json:"writable"`
	ModifiedAt time.Time `json:"modified_at"`
}

// BrowseResult represents the result of browsing a directory
type BrowseResult struct {
	CurrentPath string      `json:"current_path"`
	Parent      *string     `json:"parent"` // null if at root
	IsRoot      bool        `json:"is_root"`
	Directories []Directory `json:"directories"`
}

// PathBrowser defines the interface for browsing the filesystem during library path selection
type PathBrowser interface {
	// Browse lists directories at the given path
	// If path is empty, uses the default base path
	Browse(ctx context.Context, path string) (*BrowseResult, error)
}
