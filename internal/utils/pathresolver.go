// Package utils provides file system utilities and media file handling functions.
// This package contains optimized file operations, hashing utilities, and media type detection
// designed for high-performance media scanning and processing.
package utils

import (
	"os"
	"path/filepath"
	"strings"
)

// PathResolver handles resolving file paths across different environments.
// It provides intelligent path resolution that works seamlessly between Docker containers
// and host systems, making development and testing easier across different environments.
type PathResolver struct {
	workspaceRoot string
}

// NewPathResolver creates a new path resolver instance.
// The resolver uses the current working directory as the workspace root
// for resolving relative paths.
func NewPathResolver() *PathResolver {
	pwd, _ := os.Getwd()
	return &PathResolver{
		workspaceRoot: pwd,
	}
}

// ResolvePath attempts to find a valid file path by trying multiple variants.
// This is useful when paths may differ between Docker containers (/app/) and
// local development environments. It tries the original path first, then
// attempts various transformations to find a valid file.
func (pr *PathResolver) ResolvePath(originalPath string) (string, error) {
	pathVariants := pr.generatePathVariants(originalPath)

	for _, path := range pathVariants {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", os.ErrNotExist
}

// ResolveDirectory attempts to find a valid directory path by trying multiple variants.
// Similar to ResolvePath but specifically validates that the resolved path is a directory.
// Returns os.ErrNotExist if no valid directory is found.
func (pr *PathResolver) ResolveDirectory(originalPath string) (string, error) {
	pathVariants := pr.generatePathVariants(originalPath)

	for _, path := range pathVariants {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path, nil
		}
	}

	return "", os.ErrNotExist
}

// generatePathVariants creates a list of possible path variants to try.
// The function handles:
//   - Docker to local path mappings (/app/ prefix)
//   - Current working directory resolution
//   - Relative path handling (./)
//   - Special test data path resolution
//
// Path variants are ordered from most specific to least specific.
func (pr *PathResolver) generatePathVariants(originalPath string) []string {
	variants := []string{originalPath}

	// Docker to local path mappings
	if strings.HasPrefix(originalPath, "/app/") {
		variants = append(variants, strings.TrimPrefix(originalPath, "/app"))
		variants = append(variants, filepath.Join(".", strings.TrimPrefix(originalPath, "/app")))
	} else {
		variants = append(variants, filepath.Join("/app", originalPath))
	}

	// Current working directory variants
	if pr.workspaceRoot != "" {
		variants = append(variants, filepath.Join(pr.workspaceRoot, originalPath))

		// Handle relative paths
		if strings.HasPrefix(originalPath, "./") {
			variants = append(variants, filepath.Join(pr.workspaceRoot, originalPath[2:]))
		}

		// Special handling for test data paths
		if strings.Contains(originalPath, "data/test-music") {
			parts := strings.Split(originalPath, "data/test-music")
			if len(parts) > 1 {
				relPath := "data/test-music" + parts[len(parts)-1]
				variants = append(variants, filepath.Join(pr.workspaceRoot, relPath))
			}
		}
	}

	return variants
}
