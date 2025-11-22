package filesystem

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// NewWalker creates a new Walker with default filepath.WalkDir
func NewWalker(opts ...WalkerOption) *Walker {
	w := &Walker{
		walkDirFunc:      filepath.WalkDir,
		parallelWorkers:  0,    // Sequential by default
		enableParallel:   false,
		progressInterval: 0,    // No progress logging by default
	}

	// Apply options
	for _, opt := range opts {
		opt(w)
	}

	return w
}

// Walk traverses a directory tree, calling walkFn for each file
// Uses parallel walking if enabled, otherwise falls back to sequential
func (w *Walker) Walk(ctx context.Context, root string, walkFn scanner.WalkFunc) error {
	// Validate root path
	if root == "" {
		return scanner.ErrInvalidPath
	}

	// Use parallel walking if enabled
	if w.enableParallel {
		return w.walkParallel(ctx, root, walkFn)
	}

	// Sequential walking (original behavior)
	return w.walkSequential(ctx, root, walkFn)
}

// walkSequential performs traditional sequential directory traversal
func (w *Walker) walkSequential(ctx context.Context, root string, walkFn scanner.WalkFunc) error {
	filesDiscovered := 0

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

		// Progress logging
		if w.progressInterval > 0 && !fileInfo.IsDir {
			filesDiscovered++
			if filesDiscovered%w.progressInterval == 0 {
				fmt.Printf("info: discovered %d files so far...\n", filesDiscovered)
			}
		}

		// Call the walk function
		return walkFn(fileInfo)
	})
}

// walkParallel performs parallel directory traversal optimized for network storage
// It walks top-level directories concurrently to parallelize network I/O
func (w *Walker) walkParallel(ctx context.Context, root string, walkFn scanner.WalkFunc) error {
	// Read top-level directories
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	// Separate files and directories
	var topLevelFiles []fs.DirEntry
	var topLevelDirs []fs.DirEntry

	for _, entry := range entries {
		if entry.IsDir() {
			topLevelDirs = append(topLevelDirs, entry)
		} else {
			topLevelFiles = append(topLevelFiles, entry)
		}
	}

	// Track progress across all workers
	var filesDiscovered atomic.Int64
	var mu sync.Mutex
	var firstErr error

	// Process top-level files first (quick, sequential)
	for _, entry := range topLevelFiles {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		path := filepath.Join(root, entry.Name())
		fileInfo, err := toFileInfo(path, entry)
		if err != nil {
			continue
		}

		if err := walkFn(fileInfo); err != nil {
			return err
		}

		if w.progressInterval > 0 {
			count := filesDiscovered.Add(1)
			if count%int64(w.progressInterval) == 0 {
				fmt.Printf("info: discovered %d files so far...\n", count)
			}
		}
	}

	// Walk subdirectories in parallel using worker pool
	semaphore := make(chan struct{}, w.parallelWorkers)
	var wg sync.WaitGroup

	for _, dir := range topLevelDirs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		wg.Add(1)
		semaphore <- struct{}{} // Acquire semaphore

		go func(dirEntry fs.DirEntry) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore

			dirPath := filepath.Join(root, dirEntry.Name())

			// Walk this directory tree
			walkErr := w.walkDirFunc(dirPath, func(path string, d fs.DirEntry, err error) error {
				// Check for context cancellation
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				// Handle walk errors
				if err != nil {
					return nil // Continue walking
				}

				// Convert to FileInfo
				fileInfo, err := toFileInfo(path, d)
				if err != nil {
					return nil // Skip files we can't stat
				}

				// Progress logging
				if w.progressInterval > 0 && !fileInfo.IsDir {
					count := filesDiscovered.Add(1)
					if count%int64(w.progressInterval) == 0 {
						fmt.Printf("info: discovered %d files so far...\n", count)
					}
				}

				// Call the walk function (thread-safe)
				return walkFn(fileInfo)
			})

			// Capture first error
			if walkErr != nil && walkErr != context.Canceled {
				mu.Lock()
				if firstErr == nil {
					firstErr = walkErr
				}
				mu.Unlock()
			}
		}(dir)
	}

	// Wait for all workers to complete
	wg.Wait()

	return firstErr
}
