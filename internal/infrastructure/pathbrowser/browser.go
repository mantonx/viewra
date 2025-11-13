package pathbrowser

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/viewra/viewra/internal/domain/library"
)

// Service implements the filesystem.Browser interface
type Service struct {
	validator       *PathValidator
	defaultBasePath string
}

// NewService creates a new browser service
func NewService(allowedPaths []string, defaultBasePath string) *Service {
	return &Service{
		validator:       NewPathValidator(allowedPaths),
		defaultBasePath: defaultBasePath,
	}
}

// Browse lists directories at the given path
func (s *Service) Browse(_ context.Context, path string) (*library.BrowseResult, error) {
	// Use default base path if no path provided
	targetPath := path
	if targetPath == "" {
		targetPath = s.defaultBasePath
	}

	// Validate the path
	if err := s.validator.Validate(targetPath); err != nil {
		return nil, err
	}

	// Read directory contents
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return nil, err
	}

	// Filter and collect directories
	var directories []library.Directory
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(targetPath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue // Skip entries we can't stat
		}

		// Check permissions
		readable := isReadable(fullPath)
		writable := isWritable(fullPath)

		directories = append(directories, library.Directory{
			Name:       entry.Name(),
			Path:       fullPath,
			Readable:   readable,
			Writable:   writable,
			ModifiedAt: info.ModTime(),
		})
	}

	// Sort directories by name
	sort.Slice(directories, func(i, j int) bool {
		return directories[i].Name < directories[j].Name
	})

	// Determine parent directory
	parent := filepath.Dir(targetPath)
	var parentPtr *string
	isRoot := false

	// Check if we're at a root allowed path
	for _, allowedPath := range s.validator.allowedBasePaths {
		if targetPath == allowedPath {
			isRoot = true
			break
		}
	}

	// Only set parent if we're not at root and parent is valid
	if !isRoot {
		// Verify parent is within allowed paths
		if err := s.validator.Validate(parent); err == nil {
			parentPtr = &parent
		}
	}

	return &library.BrowseResult{
		CurrentPath: targetPath,
		Parent:      parentPtr,
		IsRoot:      isRoot,
		Directories: directories,
	}, nil
}

// isWritable checks if a directory is writable by checking file mode
// NOTE: This is informational only - the browser never writes to directories
func isWritable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Check if owner has write permission (simplified check)
	// In production, you might want to use unix.Access() for accurate permission checking
	mode := info.Mode()
	return mode&0o200 != 0 // Owner write bit
}
