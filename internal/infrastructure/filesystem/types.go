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
