package scan

import (
	"log/slog"

	"github.com/mantonx/viewra/internal/domain/scanner"
)

// Config holds configuration for the scanner coordinator
type Config struct {
	// NumWorkers is the number of concurrent workers for file processing
	NumWorkers int
	// ResultBufferSize is the size of the result channel buffer
	ResultBufferSize int
	// EnableIncrementalScan enables smart file skipping based on ModTime
	EnableIncrementalScan bool
	// FileCache stores previously scanned file metadata
	FileCache map[string]*scanner.FileCacheEntry
	// Logger for diagnostic output (optional, uses default if nil)
	Logger *slog.Logger
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		NumWorkers:            4,
		ResultBufferSize:      100,
		EnableIncrementalScan: false,
		FileCache:             make(map[string]*scanner.FileCacheEntry),
	}
}
