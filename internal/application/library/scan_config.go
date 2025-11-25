package library

import (
	"time"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/domain/scanner"
)

// MediaRepositories bundles media-related repositories
type MediaRepositories struct {
	Library library.Repository
	Media   media.Repository
	Movie   media.MovieRepository
	TV      media.TVRepository
	Music   media.MusicRepository
}

// ScanRepositories bundles scan-related repositories
type ScanRepositories struct {
	ScanJob    scanner.ScanJobRepository
	Checkpoint scanner.CheckpointRepository
	ScanState  scanner.ScanStateRepository
}

// ScanConfig bundles scan configuration parameters
type ScanConfig struct {
	// Core settings
	Timeout          time.Duration
	ParallelWalkers  int // Number of concurrent directory walkers (0 = sequential)
	ProgressInterval int // Log progress every N files (0 = disabled)

	// Checkpoint processing
	CheckpointBatchSize int           // Files per batch fetch from DB (default: 50)
	MaxRetries          int           // Failed file retry attempts (default: 3)
	WorkerTimeout       time.Duration // Absolute max time per file (default: 5m)

	// Timeouts
	BaseFileTimeout      time.Duration // Per-file processing timeout for local storage (default: 30s)
	RemoteStorageTimeout time.Duration // Per-file processing timeout for network storage (default: 60s)
	MaxExtraTimeout      time.Duration // Max additional timeout for large files (default: 120s)
	ProgressUpdateTick   time.Duration // How often to update progress (default: 2s)
	BatchWriteTimeout    time.Duration // Timeout for DB batch inserts (default: 500ms)

	// Buffer sizes
	DiscoveryBufferSize   int // Buffer size for file discovery channel (default: 100000)
	CheckpointBufferSize  int // Buffer size for checkpoint channels (default: 100)
	HashProgressLogEvery  int // Log hashing progress every N files (default: 5000)
	DiscoveryLogEvery     int // Log discovery progress every N files (default: 1000)
}

// DefaultScanConfig returns a ScanConfig with sensible defaults
func DefaultScanConfig() ScanConfig {
	return ScanConfig{
		Timeout:          24 * time.Hour,
		ParallelWalkers:  0,
		ProgressInterval: 0,

		CheckpointBatchSize: 50,
		MaxRetries:          3,
		WorkerTimeout:       5 * time.Minute,

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

// WithDefaults returns a copy of the config with zero values replaced by defaults
func (c ScanConfig) WithDefaults() ScanConfig {
	defaults := DefaultScanConfig()

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
