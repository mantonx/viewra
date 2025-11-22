package filesystem

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// Walker implements scanner.FileWalker using filepath.WalkDir
type Walker struct {
	// walkDirFunc allows injection for testing
	walkDirFunc func(root string, fn fs.WalkDirFunc) error

	// Parallel walking configuration
	parallelWorkers  int  // Number of concurrent directory walkers (0 = sequential)
	enableParallel   bool // Enable parallel walking for network storage optimization
	progressInterval int  // Log progress every N files (0 = disabled)
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
