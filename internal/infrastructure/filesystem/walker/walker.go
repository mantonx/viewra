package walker

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/mantonx/viewra/internal/domain/scanner"
	pkgLogger "github.com/mantonx/viewra/internal/pkg/logger"
)

// New creates a new Walker with default filepath.WalkDir
func New(opts ...Option) *Walker {
	w := &Walker{
		WalkDirFunc:      filepath.WalkDir,
		parallelWorkers:  0,    // Sequential by default
		enableParallel:   false,
		progressInterval: 0,    // No progress logging by default
		logger:           pkgLogger.DefaultIfNil(nil),
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

// getLogger returns the walker's logger or default if nil
func (w *Walker) getLogger() *slog.Logger {
	return pkgLogger.DefaultIfNil(w.logger)
}

// Walk traverses a directory tree, calling walkFn for each file.
// Uses parallel walking if enabled, otherwise falls back to sequential.
// Tracks statistics in w.stats which can be retrieved via GetStats().
func (w *Walker) Walk(ctx context.Context, root string, walkFn scanner.WalkFunc) error {
	if root == "" {
		return scanner.ErrInvalidPath
	}

	w.stats = NewStats()

	if w.enableParallel {
		return w.walkParallel(ctx, root, walkFn)
	}

	return w.walkSequential(ctx, root, walkFn)
}

// GetStats returns the statistics from the last Walk operation.
// Returns nil if Walk has not been called yet.
func (w *Walker) GetStats() *Stats {
	return w.stats
}

// Count returns the total number of files that match the filter function.
// This is a fast count-only pass that reuses Walk logic but just increments a counter.
// Uses the same filtering logic as Walk to ensure accurate estimates.
func (w *Walker) Count(ctx context.Context, root string, shouldCount func(scanner.FileInfo) bool) (int64, error) {
	var count atomic.Int64

	err := w.Walk(ctx, root, func(fi scanner.FileInfo) error {
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
	return w.WalkDirFunc(root, func(path string, d fs.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			if d != nil && d.IsDir() {
				w.stats.DirsSkipped++
			} else {
				w.stats.FilesSkipped++
			}
			categorizeError(err, w.stats)
			w.stats.AddSkippedPath(path)

			w.getLogger().Warn("Walk error, skipping",
				"path", path,
				"error", err)
			return nil
		}

		if d.IsDir() {
			w.stats.DirsScanned++
		}

		fileInfo, err := toFileInfo(path, d)
		if err != nil {
			w.stats.FilesSkipped++
			categorizeError(err, w.stats)
			w.stats.AddSkippedPath(path)

			w.getLogger().Warn("Failed to stat file, skipping",
				"path", path,
				"error", err)
			return nil
		}

		return walkFn(fileInfo)
	})
}

// walkParallel performs parallel directory traversal optimized for network storage.
// It walks top-level directories concurrently to parallelize network I/O.
func (w *Walker) walkParallel(ctx context.Context, root string, walkFn scanner.WalkFunc) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	var topLevelFiles []fs.DirEntry
	var topLevelDirs []fs.DirEntry

	for _, entry := range entries {
		if entry.IsDir() {
			topLevelDirs = append(topLevelDirs, entry)
		} else {
			topLevelFiles = append(topLevelFiles, entry)
		}
	}

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
			w.stats.FilesSkipped++
			categorizeError(err, w.stats)
			w.stats.AddSkippedPath(path)
			w.getLogger().Warn("Failed to stat top-level file, skipping",
				"path", path,
				"error", err)
			continue
		}

		if err := walkFn(fileInfo); err != nil {
			return err
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
		semaphore <- struct{}{}

		go func(dirEntry fs.DirEntry) {
			defer wg.Done()
			defer func() { <-semaphore }()

			dirPath := filepath.Join(root, dirEntry.Name())

			walkErr := w.WalkDirFunc(dirPath, func(path string, d fs.DirEntry, err error) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				if err != nil {
					mu.Lock()
					if d != nil && d.IsDir() {
						w.stats.DirsSkipped++
					} else {
						w.stats.FilesSkipped++
					}
					categorizeError(err, w.stats)
					w.stats.AddSkippedPath(path)
					mu.Unlock()

					w.getLogger().Warn("Walk error, skipping",
						"path", path,
						"error", err)
					return nil
				}

				if d.IsDir() {
					mu.Lock()
					w.stats.DirsScanned++
					mu.Unlock()
				}

				fileInfo, err := toFileInfo(path, d)
				if err != nil {
					mu.Lock()
					w.stats.FilesSkipped++
					categorizeError(err, w.stats)
					w.stats.AddSkippedPath(path)
					mu.Unlock()

					w.getLogger().Warn("Failed to stat file, skipping",
						"path", path,
						"error", err)
					return nil
				}

				return walkFn(fileInfo)
			})

			if walkErr != nil && walkErr != context.Canceled {
				mu.Lock()
				if firstErr == nil {
					firstErr = walkErr
				}
				mu.Unlock()
			}
		}(dir)
	}

	wg.Wait()

	return firstErr
}

// toFileInfo converts fs.DirEntry to scanner.FileInfo
func toFileInfo(path string, entry fs.DirEntry) (scanner.FileInfo, error) {
	info, err := entry.Info()
	if err != nil {
		return scanner.FileInfo{}, err
	}

	return scanner.FileInfo{
		Path:      path,
		Size:      info.Size(),
		ModTime:   info.ModTime(),
		Extension: filepath.Ext(path),
		IsDir:     info.IsDir(),
	}, nil
}
