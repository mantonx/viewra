package walker

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// ProgressCallback is called periodically during file discovery to report progress
type ProgressCallback func(filesDiscovered int64)

// Stats tracks statistics from directory walking.
// This helps identify incomplete discoveries due to errors.
type Stats struct {
	FilesDiscovered   int64    // Total files discovered
	DirsScanned       int64    // Directories successfully scanned
	DirsSkipped       int64    // Directories that couldn't be read
	FilesSkipped      int64    // Files that couldn't be stat'd
	PermissionErrors  int64    // Permission denied errors
	NetworkErrors     int64    // Network/timeout errors
	OtherErrors       int64    // Other errors encountered
	SkippedPaths      []string // Sample of skipped paths (max 100)
	maxSkippedSamples int      // Internal: max samples to keep
}

// NewStats creates a new Stats instance
func NewStats() *Stats {
	return &Stats{
		SkippedPaths:      make([]string, 0),
		maxSkippedSamples: 100,
	}
}

// AddSkippedPath adds a path to the skipped samples (up to max)
func (s *Stats) AddSkippedPath(path string) {
	if len(s.SkippedPaths) < s.maxSkippedSamples {
		s.SkippedPaths = append(s.SkippedPaths, path)
	}
}

// HasErrors returns true if any errors were encountered
func (s *Stats) HasErrors() bool {
	return s.DirsSkipped > 0 || s.FilesSkipped > 0
}

// TotalErrors returns the total count of all errors
func (s *Stats) TotalErrors() int64 {
	return s.PermissionErrors + s.NetworkErrors + s.OtherErrors
}

// Walker implements scanner.FileWalker using filepath.WalkDir
type Walker struct {
	// walkDirFunc allows injection for testing
	WalkDirFunc func(root string, fn fs.WalkDirFunc) error

	// Parallel walking configuration
	parallelWorkers  int  // Number of concurrent directory walkers (0 = sequential)
	enableParallel   bool // Enable parallel walking for network storage optimization
	progressInterval int  // Log progress every N files (0 = disabled)

	// Progress callback for real-time discovery updates
	progressCallback ProgressCallback

	// Logger for structured logging (optional, will use slog.Default if nil)
	logger *slog.Logger

	// Statistics tracking (set during Walk/Count operations)
	stats *Stats
}

// Option configures Walker behavior
type Option func(*Walker)

// WithParallelWalking enables parallel directory walking with specified worker count.
// Recommended for network storage to parallelize I/O operations.
func WithParallelWalking(workers int) Option {
	return func(w *Walker) {
		w.enableParallel = true
		w.parallelWorkers = workers
	}
}

// WithProgressLogging enables progress logging every N files discovered
func WithProgressLogging(interval int) Option {
	return func(w *Walker) {
		w.progressInterval = interval
	}
}

// WithProgressCallback sets a callback function that will be called periodically
// during file discovery to report the current count of discovered files.
func WithProgressCallback(callback ProgressCallback) Option {
	return func(w *Walker) {
		w.progressCallback = callback
	}
}

// WithLogger sets a custom logger for the walker
func WithLogger(logger *slog.Logger) Option {
	return func(w *Walker) {
		w.logger = logger
	}
}

// categorizeError determines the type of error for statistics
func categorizeError(err error, stats *Stats) {
	if err == nil {
		return
	}

	errStr := err.Error()

	// Check for permission errors
	if os.IsPermission(err) ||
		filepath.Base(errStr) == "permission denied" ||
		filepath.Base(errStr) == "access denied" {
		stats.PermissionErrors++
		return
	}

	// Check for network errors (CIFS/NFS common errors)
	// Note: Use string contains for "i/o timeout" since it has a slash that breaks filepath.Base
	if filepath.Base(errStr) == "no such host" ||
		filepath.Base(errStr) == "network is unreachable" ||
		filepath.Base(errStr) == "connection refused" ||
		filepath.Base(errStr) == "connection reset" ||
		errStr == "i/o timeout" || // Direct comparison since filepath.Base breaks on slash
		filepath.Base(errStr) == "stale file handle" {
		stats.NetworkErrors++
		return
	}

	// Other errors
	stats.OtherErrors++
}
