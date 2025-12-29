// Package scan provides core types and sub-packages for library scanning.
// The main orchestration remains in the parent library package, but configuration,
// processing logic, and media handling are organized into focused sub-packages.
package scan

import (
	"time"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
)

// Scan behavior constants - thresholds and limits used by the scanner.
// These are package-level constants rather than configuration values because
// they represent safety boundaries and heuristics that shouldn't typically change.
const (
	// DefaultHashWorkers is the fallback number of concurrent hash workers
	// when no system profile is available. Conservative to avoid overwhelming
	// storage on unknown systems.
	DefaultHashWorkers = 8

	// DefaultHashBatchSize is the fallback batch size for checkpoint creation
	// when no system profile is available.
	DefaultHashBatchSize = 10

	// DefaultProcessingWorkers is the fallback number of file processing workers
	// when no system profile is available.
	DefaultProcessingWorkers = 4

	// StaleMediaThresholdPercent is the maximum percentage of library files that
	// can be marked stale before cleanup is aborted. This safety limit prevents
	// accidental mass deletion when scans fail due to permission or network errors.
	// If more than this percentage would be deleted, we assume a scan problem.
	StaleMediaThresholdPercent = 10.0

	// FileDropWarningThresholdPercent triggers a warning when a scan discovers
	// significantly fewer files than the previous completed scan. This may indicate
	// incomplete discovery due to network issues or permission changes.
	FileDropWarningThresholdPercent = 10.0

	// PermissionErrorWarningThreshold is the minimum number of permission errors
	// during discovery before a warning is logged. A few permission errors are
	// normal, but many suggest a systemic permissions problem.
	PermissionErrorWarningThreshold = 10

	// PreviousJobsToCompare is how many previous scan jobs to fetch when comparing
	// discovery results. We look for the most recent completed scan within this window.
	PreviousJobsToCompare = 5
)

// MediaRepositories bundles media-related repositories needed for scan operations.
type MediaRepositories struct {
	Library library.Repository
	Media   media.Repository
	Movie   media.MovieRepository
	TV      media.TVRepository
	Music   media.MusicRepository
}

// ScanRepositories bundles scan-related repositories needed for scan operations.
type ScanRepositories struct {
	ScanJob    scanner.ScanJobRepository
	Checkpoint scanner.CheckpointRepository
	ScanState  scanner.ScanStateRepository
}

// Config bundles scan configuration parameters.
type Config struct {
	// Core settings
	Timeout          time.Duration
	ParallelWalkers  int // Number of concurrent directory walkers (0 = sequential)
	ProgressInterval int // Log progress every N files (0 = disabled)

	// Checkpoint processing
	CheckpointBatchSize int           // Files per batch fetch from DB (default: 50)
	MaxRetries          int           // Failed file retry attempts (default: 3)
	WorkerTimeout       time.Duration // Absolute max time per file (default: 5m)
	RetryBackoffBase    time.Duration // Base duration for retry backoff (default: 1s, actual = base * 2^retryCount)

	// Timeouts
	BaseFileTimeout      time.Duration // Per-file processing timeout for local storage (default: 30s)
	RemoteStorageTimeout time.Duration // Per-file processing timeout for network storage (default: 60s)
	MaxExtraTimeout      time.Duration // Max additional timeout for large files (default: 120s)
	ProgressUpdateTick   time.Duration // How often to update progress (default: 2s)
	BatchWriteTimeout    time.Duration // Timeout for DB batch inserts (default: 500ms)

	// Buffer sizes
	DiscoveryBufferSize  int // Buffer size for file discovery channel (default: 100000)
	CheckpointBufferSize int // Buffer size for checkpoint channels (default: 100)
	HashProgressLogEvery int // Log hashing progress every N files (default: 5000)
	DiscoveryLogEvery    int // Log discovery progress every N files (default: 1000)
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Timeout:          24 * time.Hour,
		ParallelWalkers:  0,
		ProgressInterval: 0,

		CheckpointBatchSize: 50,
		MaxRetries:          3,
		WorkerTimeout:       5 * time.Minute,
		RetryBackoffBase:    time.Second,

		BaseFileTimeout:      30 * time.Second,
		RemoteStorageTimeout: 60 * time.Second,
		MaxExtraTimeout:      120 * time.Second,
		ProgressUpdateTick:   2 * time.Second,
		BatchWriteTimeout:    500 * time.Millisecond,

		DiscoveryBufferSize:  100000,
		CheckpointBufferSize: 100,
		HashProgressLogEvery: 5000,
		DiscoveryLogEvery:    1000,
	}
}

// WithDefaults returns a copy of the config with zero values replaced by defaults.
func (c Config) WithDefaults() Config {
	defaults := DefaultConfig()

	if c.Timeout == 0 {
		c.Timeout = defaults.Timeout
	}
	if c.CheckpointBatchSize == 0 {
		c.CheckpointBatchSize = defaults.CheckpointBatchSize
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = defaults.MaxRetries
	}
	if c.WorkerTimeout == 0 {
		c.WorkerTimeout = defaults.WorkerTimeout
	}
	if c.RetryBackoffBase == 0 {
		c.RetryBackoffBase = defaults.RetryBackoffBase
	}
	if c.BaseFileTimeout == 0 {
		c.BaseFileTimeout = defaults.BaseFileTimeout
	}
	if c.RemoteStorageTimeout == 0 {
		c.RemoteStorageTimeout = defaults.RemoteStorageTimeout
	}
	if c.MaxExtraTimeout == 0 {
		c.MaxExtraTimeout = defaults.MaxExtraTimeout
	}
	if c.ProgressUpdateTick == 0 {
		c.ProgressUpdateTick = defaults.ProgressUpdateTick
	}
	if c.BatchWriteTimeout == 0 {
		c.BatchWriteTimeout = defaults.BatchWriteTimeout
	}
	if c.DiscoveryBufferSize == 0 {
		c.DiscoveryBufferSize = defaults.DiscoveryBufferSize
	}
	if c.CheckpointBufferSize == 0 {
		c.CheckpointBufferSize = defaults.CheckpointBufferSize
	}
	if c.HashProgressLogEvery == 0 {
		c.HashProgressLogEvery = defaults.HashProgressLogEvery
	}
	if c.DiscoveryLogEvery == 0 {
		c.DiscoveryLogEvery = defaults.DiscoveryLogEvery
	}

	return c
}
