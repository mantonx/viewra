// Package filesystem provides file system operations for media scanning.
//
// The package is organized into sub-packages by concern:
//   - walker: Directory traversal with parallel walking support
//   - filter: Media file detection and filtering
//   - hash: Fast file hashing for duplicate detection
//   - subtitles: External subtitle discovery
//   - scan: Scan coordination and metadata extraction
package filesystem

import (
	"io/fs"
	"os"

	"github.com/mantonx/viewra/internal/infrastructure/filesystem/filter"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem/hash"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem/scan"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem/subtitles"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem/walker"
)

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

// Walker types and functions
type (
	Walker       = walker.Walker
	WalkerOption = walker.Option
	WalkStats    = walker.Stats
)

// Walker constructor and options
var (
	NewWalker            = walker.New
	NewWalkStats         = walker.NewStats
	WithParallelWalking  = walker.WithParallelWalking
	WithProgressLogging  = walker.WithProgressLogging
	WithProgressCallback = walker.WithProgressCallback
	WithLogger           = walker.WithLogger
	CalculateDirSize     = walker.CalculateDirSize
)

// Filter types and functions
type Filter = filter.Filter

var NewFilter = filter.New

// Hasher types and functions
type Hasher = hash.Hasher

var NewHasher = hash.NewHasher

// HashChunkSize is the size of data to read from start and end of file
const HashChunkSize = hash.ChunkSize

// Subtitle types and functions
type ExternalSubtitle = subtitles.External

var (
	DiscoverExternalSubtitles     = subtitles.DiscoverExternal
	DiscoverSubtitlesSubdirectory = subtitles.DiscoverInSubdirectory
	DiscoverAllExternalSubtitles  = subtitles.DiscoverAll
)

// Coordinator types and functions
type (
	Coordinator       = scan.Coordinator
	CoordinatorConfig = scan.Config
)

var (
	NewCoordinator           = scan.NewCoordinator
	DefaultCoordinatorConfig = scan.DefaultConfig
)
