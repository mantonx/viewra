package filesystem

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// ProgressCallback is called periodically during file discovery to report progress
type ProgressCallback func(filesDiscovered int64)

// WalkStats tracks statistics from directory walking
// This helps identify incomplete discoveries due to errors
type WalkStats struct {
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

// NewWalkStats creates a new WalkStats instance
func NewWalkStats() *WalkStats {
	return &WalkStats{
		SkippedPaths:      make([]string, 0),
		maxSkippedSamples: 100,
	}
}

// AddSkippedPath adds a path to the skipped samples (up to max)
func (ws *WalkStats) AddSkippedPath(path string) {
	if len(ws.SkippedPaths) < ws.maxSkippedSamples {
		ws.SkippedPaths = append(ws.SkippedPaths, path)
	}
}

// HasErrors returns true if any errors were encountered
func (ws *WalkStats) HasErrors() bool {
	return ws.DirsSkipped > 0 || ws.FilesSkipped > 0
}

// TotalErrors returns the total count of all errors
func (ws *WalkStats) TotalErrors() int64 {
	return ws.PermissionErrors + ws.NetworkErrors + ws.OtherErrors
}

// Walker implements scanner.FileWalker using filepath.WalkDir
type Walker struct {
	// walkDirFunc allows injection for testing
	walkDirFunc func(root string, fn fs.WalkDirFunc) error

	// Parallel walking configuration
	parallelWorkers  int  // Number of concurrent directory walkers (0 = sequential)
	enableParallel   bool // Enable parallel walking for network storage optimization
	progressInterval int  // Log progress every N files (0 = disabled)

	// Progress callback for real-time discovery updates
	progressCallback ProgressCallback

	// Logger for structured logging (optional, will use slog.Default if nil)
	logger *slog.Logger

	// Statistics tracking (set during Walk/Count operations)
	stats *WalkStats
}

// Filter implements scanner.FileFilter with smart file detection
type Filter struct {
	// Configuration
	skipHidden bool
}

// FileSystem wraps os operations for testability
type FileSystem interface {
	Stat(name string) (os.FileInfo, error)
	ReadDir(name string) ([]fs.DirEntry, error)
}

// DefaultFileSystem uses the standard os package
type DefaultFileSystem struct{}

// Stat wraps os.Stat
func (dfs *DefaultFileSystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// ReadDir wraps os.ReadDir
func (dfs *DefaultFileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(name)
}

// WalkerOption configures Walker behavior
type WalkerOption func(*Walker)

// WithParallelWalking enables parallel directory walking with specified worker count
// Recommended for network storage to parallelize I/O operations
func WithParallelWalking(workers int) WalkerOption {
	return func(w *Walker) {
		w.enableParallel = true
		w.parallelWorkers = workers
	}
}

// WithProgressLogging enables progress logging every N files discovered
func WithProgressLogging(interval int) WalkerOption {
	return func(w *Walker) {
		w.progressInterval = interval
	}
}

// WithProgressCallback sets a callback function that will be called periodically
// during file discovery to report the current count of discovered files
func WithProgressCallback(callback ProgressCallback) WalkerOption {
	return func(w *Walker) {
		w.progressCallback = callback
	}
}

// WithLogger sets a custom logger for the walker
func WithLogger(logger *slog.Logger) WalkerOption {
	return func(w *Walker) {
		w.logger = logger
	}
}

// categorizeError determines the type of error for statistics
func categorizeError(err error, stats *WalkStats) {
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
	if filepath.Base(errStr) == "no such host" ||
	   filepath.Base(errStr) == "network is unreachable" ||
	   filepath.Base(errStr) == "connection refused" ||
	   filepath.Base(errStr) == "connection reset" ||
	   filepath.Base(errStr) == "i/o timeout" ||
	   filepath.Base(errStr) == "stale file handle" {
		stats.NetworkErrors++
		return
	}

	// Other errors
	stats.OtherErrors++
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
