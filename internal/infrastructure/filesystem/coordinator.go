package filesystem

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/domain/scanner/parsers"
	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg"
	"github.com/mantonx/viewra/internal/infrastructure/metadata/music"
)

// CoordinatorConfig holds configuration for the scanner coordinator
type CoordinatorConfig struct {
	// NumWorkers is the number of concurrent workers for file processing
	NumWorkers int
	// ResultBufferSize is the size of the result channel buffer
	ResultBufferSize int
	// EnableDuplicateDetection enables file hash computation for duplicate detection (deprecated, use HashingStrategy)
	EnableDuplicateDetection bool
	// HashingStrategy determines when to compute file hashes
	HashingStrategy scanner.HashingStrategy
	// ConflictThreshold is size in bytes - below this, always hash (for on_conflict strategy)
	ConflictThreshold int64
	// EnableIncrementalScan enables smart file skipping based on ModTime
	EnableIncrementalScan bool
	// FileCache stores previously scanned file metadata
	FileCache map[string]*scanner.FileCacheEntry
}

// DefaultCoordinatorConfig returns sensible defaults
func DefaultCoordinatorConfig() CoordinatorConfig {
	return CoordinatorConfig{
		NumWorkers:               4,
		ResultBufferSize:         100,
		EnableDuplicateDetection: true,
		HashingStrategy:          scanner.HashingStrategyOnConflict, // Only hash on size conflicts
		ConflictThreshold:        1024 * 1024,                       // 1MB - always hash files smaller than this
		EnableIncrementalScan:    false,                             // Disabled by default for first implementation
		FileCache:                make(map[string]*scanner.FileCacheEntry),
	}
}

// Coordinator orchestrates the scanning process with worker pool
type Coordinator struct {
	config       CoordinatorConfig
	walker       scanner.FileWalker
	filter       scanner.FileFilter
	parser       scanner.FilenameParser
	hasher       *Hasher
	ffmpegClient *ffmpeg.Client

	// Progress tracking (atomic)
	filesFound     atomic.Int64
	filesProcessed atomic.Int64
	bytesProcessed atomic.Int64
	errorCount     atomic.Int64

	// Size tracking for conflict detection
	mu        sync.RWMutex
	startTime time.Time
	isRunning bool
	sizeMap   map[int64]int // tracks count of files per size
}

// NewCoordinator creates a new scanner coordinator
func NewCoordinator(config CoordinatorConfig) *Coordinator {
	ffmpegClient, err := ffmpeg.NewClient()
	if err != nil {
		// Log warning but don't fail - metadata extraction will be skipped
		fmt.Printf("Warning: FFmpeg not available, technical metadata extraction disabled: %v\n", err)
	}

	// Create metadata extractor for music files
	metadataExtractor := music.NewExtractor()

	return &Coordinator{
		config:       config,
		walker:       NewWalker(),
		filter:       NewFilter(),
		parser:       parsers.NewDefaultParserWithMetadata(metadataExtractor),
		hasher:       NewHasher(),
		ffmpegClient: ffmpegClient,
		sizeMap:      make(map[int64]int),
	}
}

// Scan starts a library scan with concurrent file processing
func (c *Coordinator) Scan(ctx context.Context, libraryPath string, resultChan chan<- scanner.ScanResult) error {
	// Mark as running
	c.mu.Lock()
	if c.isRunning {
		c.mu.Unlock()
		return scanner.ErrAlreadyRunning
	}
	c.isRunning = true
	c.startTime = time.Now()
	c.resetCounters()
	c.mu.Unlock()

	// Ensure we mark as not running when done
	defer func() {
		c.mu.Lock()
		c.isRunning = false
		c.mu.Unlock()
	}()

	// Phase 1: Discovery - walk directory tree and find all media files
	// Files are discovered and counted progressively (no pre-counting pass)
	fileChan := make(chan scanner.FileInfo, c.config.ResultBufferSize)
	discoveryErrChan := make(chan error, 1)

	go c.discoverFiles(ctx, libraryPath, fileChan, discoveryErrChan)

	// Phase 2: Processing - worker pool processes discovered files
	var wg sync.WaitGroup
	processingCtx, cancelProcessing := context.WithCancel(ctx)
	defer cancelProcessing()

	// Start worker pool
	for i := 0; i < c.config.NumWorkers; i++ {
		wg.Add(1)
		go c.worker(processingCtx, &wg, fileChan, resultChan)
	}

	// Wait for all workers to finish
	wg.Wait()

	// Check for discovery errors
	select {
	case err := <-discoveryErrChan:
		if err != nil {
			return fmt.Errorf("discovery failed: %w", err)
		}
	default:
	}

	return nil
}

// shouldProcessFile checks if a file should be processed based on filter criteria
func (c *Coordinator) shouldProcessFile(info scanner.FileInfo) bool {
	// Skip directories
	if info.IsDir {
		return false
	}

	// Convert to os.FileInfo for filter
	osInfo, err := os.Stat(info.Path)
	if err != nil {
		// Skip files we can't stat
		return false
	}

	// Check if file should be processed
	return c.filter.ShouldProcess(info.Path, osInfo)
}

// isRealError returns true if the error is meaningful (not a cancellation)
func isRealError(err error) bool {
	return err != nil && err != context.Canceled
}

// discoverFiles walks the directory tree and sends files to the channel
// Files are counted progressively as they are discovered
func (c *Coordinator) discoverFiles(
	ctx context.Context,
	root string,
	fileChan chan<- scanner.FileInfo,
	errChan chan<- error,
) {
	defer close(fileChan)

	err := c.walker.Walk(ctx, root, func(info scanner.FileInfo) error {
		if !c.shouldProcessFile(info) {
			return nil
		}

		// Increment found counter as we discover files
		c.filesFound.Add(1)

		// Send to processing queue
		select {
		case fileChan <- info:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	if isRealError(err) {
		errChan <- err
	}
}

// worker processes files from the queue
func (c *Coordinator) worker(
	ctx context.Context,
	wg *sync.WaitGroup,
	fileChan <-chan scanner.FileInfo,
	resultChan chan<- scanner.ScanResult,
) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case fileInfo, ok := <-fileChan:
			if !ok {
				return // Channel closed, worker done
			}

			// Process the file
			result := c.processFile(ctx, fileInfo)

			// Send result
			select {
			case resultChan <- result:
			case <-ctx.Done():
				return
			}

			// Update counters
			c.filesProcessed.Add(1)
			if result.Error != nil {
				c.errorCount.Add(1)
			} else {
				c.bytesProcessed.Add(result.BytesProcessed)
			}
		}
	}
}

// processFile extracts metadata from a single file
func (c *Coordinator) processFile(ctx context.Context, fileInfo scanner.FileInfo) scanner.ScanResult {
	result := scanner.ScanResult{
		FilePath:       fileInfo.Path,
		MediaType:      scanner.MediaTypeUnknown,
		BytesProcessed: fileInfo.Size,
	}

	// Check for cancellation
	select {
	case <-ctx.Done():
		result.Error = ctx.Err()
		return result
	default:
	}

	// Record file size for conflict detection
	c.recordFileSize(fileInfo.Size)

	// Check if file is in cache and unchanged (incremental scan optimization)
	if c.config.EnableIncrementalScan {
		c.mu.RLock()
		cached, exists := c.config.FileCache[fileInfo.Path]
		c.mu.RUnlock()

		if exists && cached.IsUnchanged(fileInfo.ModTime, fileInfo.Size) {
			// File unchanged, use cached metadata
			result.MediaType = cached.MediaType
			result.Title = cached.Title
			result.Artist = cached.Artist
			result.Album = cached.Album
			result.Year = cached.Year
			result.SeasonNumber = cached.SeasonNumber
			result.EpisodeNumber = cached.EpisodeNumber
			result.TrackNumber = cached.TrackNumber
			result.Hash = cached.Hash
			return result
		}
	}

	// Determine media type
	mediaType := c.filter.GetMediaType(fileInfo.Extension)
	result.MediaType = mediaType

	// Parse based on media type
	switch mediaType {
	case scanner.MediaTypeMovie:
		if movieInfo, err := c.parser.ParseMovie(fileInfo.Path); err == nil && movieInfo != nil {
			result.Title = movieInfo.Title
			result.Year = &movieInfo.Year
		}

	case scanner.MediaTypeEpisode:
		// Try TV episode parser
		if tvInfo, err := c.parser.ParseTVEpisode(fileInfo.Path); err == nil && tvInfo != nil {
			result.MediaType = scanner.MediaTypeEpisode
			result.Title = tvInfo.EpisodeTitle
			result.Year = &tvInfo.Year
			result.SeasonNumber = &tvInfo.Season
			result.EpisodeNumber = &tvInfo.Episode
		} else {
			// Fallback to movie parser if TV parsing fails
			if movieInfo, err := c.parser.ParseMovie(fileInfo.Path); err == nil && movieInfo != nil {
				result.MediaType = scanner.MediaTypeMovie
				result.Title = movieInfo.Title
				result.Year = &movieInfo.Year
			}
		}

	case scanner.MediaTypeTrack:
		if musicInfo, err := c.parser.ParseMusic(fileInfo.Path); err == nil && musicInfo != nil {
			result.Title = musicInfo.Title
			result.Artist = musicInfo.Artist
			result.Album = musicInfo.Album
			result.TrackNumber = &musicInfo.TrackNumber
			if musicInfo.Year > 0 {
				result.Year = &musicInfo.Year
			}
		}
	}

	// Extract technical metadata using FFmpeg (for video/audio files)
	if c.ffmpegClient != nil && (mediaType == scanner.MediaTypeMovie || mediaType == scanner.MediaTypeEpisode) {
		metadata, err := c.ffmpegClient.ExtractMetadata(ctx, fileInfo.Path)
		if err != nil {
			// Log warning but don't fail the entire scan
			// Some files might be corrupted or in unsupported formats
			fmt.Printf("Warning: Failed to extract FFmpeg metadata for %s: %v\n", fileInfo.Path, err)
		} else {
			// Populate result with technical metadata
			result.FileSize = metadata.FileSize
			result.Width = metadata.Width
			result.Height = metadata.Height
			result.VideoCodec = metadata.VideoCodec
			result.AudioCodec = metadata.AudioCodec
			result.Bitrate = metadata.Bitrate
			result.FrameRate = metadata.FrameRate
			result.Duration = int64(metadata.Duration.Seconds())

			// Populate advanced video quality metadata
			result.CodecProfile = metadata.CodecProfile
			result.ScanType = metadata.ScanType
			result.HDRFormat = metadata.HDRFormat
			result.ColorSpace = metadata.ColorSpace
			result.ColorPrimaries = metadata.ColorPrimaries

			// Determine container format from file extension
			result.ContainerFormat = fileInfo.Extension
		}
	}

	// Conditional hashing based on strategy
	if c.shouldHashFile(fileInfo.Size) {
		hash, err := c.hasher.Hash(fileInfo.Path)
		if err != nil {
			result.Error = fmt.Errorf("failed to hash file: %w", err)
			return result
		}
		result.Hash = hash
	}

	// Update file cache if incremental scanning is enabled
	if c.config.EnableIncrementalScan {
		c.updateFileCache(fileInfo, &result)
	}

	return result
}

// updateFileCache stores the processed file metadata in cache
func (c *Coordinator) updateFileCache(fileInfo scanner.FileInfo, result *scanner.ScanResult) {
	entry := &scanner.FileCacheEntry{
		Path:          fileInfo.Path,
		Size:          fileInfo.Size,
		ModTime:       fileInfo.ModTime,
		Hash:          result.Hash,
		MediaType:     result.MediaType,
		Title:         result.Title,
		Artist:        result.Artist,
		Album:         result.Album,
		Year:          result.Year,
		SeasonNumber:  result.SeasonNumber,
		EpisodeNumber: result.EpisodeNumber,
		TrackNumber:   result.TrackNumber,
	}

	c.mu.Lock()
	c.config.FileCache[fileInfo.Path] = entry
	c.mu.Unlock()
}

// GetProgress returns current scan progress
func (c *Coordinator) GetProgress() scanner.Progress {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return scanner.Progress{
		FilesFound:     c.filesFound.Load(),
		FilesProcessed: c.filesProcessed.Load(),
		BytesProcessed: c.bytesProcessed.Load(),
		ErrorCount:     c.errorCount.Load(),
		StartTime:      c.startTime,
		LastUpdate:     time.Now(),
	}
}

// IsRunning returns true if a scan is currently in progress
func (c *Coordinator) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isRunning
}

// resetCounters resets all progress counters
func (c *Coordinator) resetCounters() {
	c.filesFound.Store(0)
	c.filesProcessed.Store(0)
	c.bytesProcessed.Store(0)
	c.errorCount.Store(0)
	c.sizeMap = make(map[int64]int)
}

// shouldHashFile determines if a file should be hashed based on strategy
func (c *Coordinator) shouldHashFile(size int64) bool {
	// Support legacy flag
	if c.config.EnableDuplicateDetection && c.config.HashingStrategy == "" {
		return true
	}

	switch c.config.HashingStrategy {
	case scanner.HashingStrategyAlways:
		return true

	case scanner.HashingStrategyDisabled:
		return false

	case scanner.HashingStrategyOnConflict:
		// Always hash small files (below threshold)
		if size < c.config.ConflictThreshold {
			return true
		}

		// Check if we've seen this size before (potential duplicate)
		c.mu.RLock()
		count := c.sizeMap[size]
		c.mu.RUnlock()

		return count > 0 // Hash if we've seen this size before

	default:
		// Unknown strategy, default to always hash
		return true
	}
}

// recordFileSize tracks file sizes for conflict detection
func (c *Coordinator) recordFileSize(size int64) {
	c.mu.Lock()
	c.sizeMap[size]++
	c.mu.Unlock()
}
