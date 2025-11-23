package filesystem

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// NewWalker creates a new Walker with default filepath.WalkDir
func NewWalker(opts ...WalkerOption) *Walker {
	w := &Walker{
		walkDirFunc:      filepath.WalkDir,
		parallelWorkers:  0,    // Sequential by default
		enableParallel:   false,
		progressInterval: 0,    // No progress logging by default
		logger:           slog.Default(), // Use default logger if not provided
	}

	// Apply options
	for _, opt := range opts {
		opt(w)
	}

	return w
}

// getLogger returns the walker's logger or slog.Default if nil
func (w *Walker) getLogger() *slog.Logger {
	if w.logger == nil {
		return slog.Default()
	}
	return w.logger
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

// Count returns the total number of files that match the filter function
// This is a fast count-only pass that reuses Walk logic but just increments a counter
// Uses the same filtering logic as Walk to ensure accurate estimates
func (w *Walker) Count(ctx context.Context, root string, shouldCount func(scanner.FileInfo) bool) (int64, error) {
	var count atomic.Int64

	err := w.Walk(ctx, root, func(fi scanner.FileInfo) error {
		// Only count non-directory files that pass the filter
		if !fi.IsDir && shouldCount(fi) {
			count.Add(1)
		}
		return nil
	})

	if err != nil {
		return 0, err
	}

	return count.Load(), nil
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

		// Progress tracking and logging
		if !fileInfo.IsDir {
			filesDiscovered++

			// Call progress callback if set
			if w.progressCallback != nil && filesDiscovered%w.progressInterval == 0 {
				w.progressCallback(int64(filesDiscovered))
			}

			// Log progress at intervals
			if w.progressInterval > 0 && filesDiscovered%w.progressInterval == 0 {
				w.getLogger().Info("File discovery progress",
					"files_discovered", filesDiscovered)
			}
		}

		// Call the walk function
		return walkFn(fileInfo)
	})
}

// walkParallel performs parallel directory traversal optimized for network storage
// It walks top-level directories concurrently to parallelize network I/O
// Includes timeout and progress monitoring to handle slow/hanging network shares
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

		count := filesDiscovered.Add(1)

		// Call progress callback if set
		if w.progressCallback != nil && count%int64(w.progressInterval) == 0 {
			w.progressCallback(count)
		}

		// Log progress at intervals
		if w.progressInterval > 0 && count%int64(w.progressInterval) == 0 {
			w.getLogger().Info("File discovery progress",
				"files_discovered", count)
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
			dirName := dirEntry.Name()
			startTime := time.Now()
			var lastProgress atomic.Int64
			lastProgress.Store(filesDiscovered.Load())

			// Create progress monitor goroutine
			monitorCtx, cancelMonitor := context.WithCancel(ctx)
			defer cancelMonitor()

			go func() {
				ticker := time.NewTicker(30 * time.Second)
				defer ticker.Stop()

				for {
					select {
					case <-monitorCtx.Done():
						return
					case <-ticker.C:
						current := filesDiscovered.Load()
						last := lastProgress.Load()
						elapsed := time.Since(startTime)

						if current == last && elapsed > 60*time.Second {
							// No progress in 30s and running for >60s = likely hung
							w.getLogger().Warn("Walker appears stuck",
								"directory", dirName,
								"elapsed_seconds", elapsed.Seconds(),
								"files", current)
						} else if current > last {
							// Progress detected
							w.getLogger().Info("Directory scan progress",
								"directory", dirName,
								"files_discovered", current,
								"elapsed_seconds", elapsed.Seconds())
							lastProgress.Store(current)
						}
					}
				}
			}()

			// Walk this directory tree with context (allows cancellation)
			walkErr := w.walkDirFunc(dirPath, func(path string, d fs.DirEntry, err error) error {
				// Check for context cancellation
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				// Handle walk errors - log but continue
				if err != nil {
					w.getLogger().Warn("Walk error, continuing",
						"path", path,
						"error", err)
					return nil // Continue walking
				}

				// Convert to FileInfo
				fileInfo, err := toFileInfo(path, d)
				if err != nil {
					return nil // Skip files we can't stat
				}

				// Progress tracking and logging
				if !fileInfo.IsDir {
					count := filesDiscovered.Add(1)

					// Call progress callback if set
					if w.progressCallback != nil && count%int64(w.progressInterval) == 0 {
						w.progressCallback(count)
					}

					// Log progress at intervals
					if w.progressInterval > 0 && count%int64(w.progressInterval) == 0 {
						w.getLogger().Info("File discovery progress",
							"files_discovered", count)
					}
				}

				// Call the walk function (thread-safe)
				return walkFn(fileInfo)
			})

			elapsed := time.Since(startTime)
			if elapsed > 5*time.Second {
				w.getLogger().Info("Completed directory scan",
					"directory", dirName,
					"elapsed_seconds", elapsed.Seconds(),
					"files_discovered", filesDiscovered.Load())
			}

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
