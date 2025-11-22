package filesystem

import (
	"context"
	"io/fs"
	"path/filepath"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// NewWalker creates a new Walker with default filepath.WalkDir
func NewWalker() *Walker {
	return &Walker{
		walkDirFunc: filepath.WalkDir,
	}
}

// Walk traverses a directory tree, calling walkFn for each file
func (w *Walker) Walk(ctx context.Context, root string, walkFn scanner.WalkFunc) error {
	// Validate root path
	if root == "" {
		return scanner.ErrInvalidPath
	}

	return w.walkDirFunc(root, func(path string, d fs.DirEntry, err error) error {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Handle walk errors
		if err != nil {
			// Log but continue walking
			return nil
		}

		// Convert to FileInfo
		fileInfo, err := toFileInfo(path, d)
		if err != nil {
			// Skip files we can't stat
			return nil
		}

		// Call the walk function
		return walkFn(fileInfo)
	})
}
