package scan

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/domain/scanner/parsers"
	"github.com/mantonx/viewra/internal/infrastructure/ffmpeg"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem/filter"
	"github.com/mantonx/viewra/internal/infrastructure/filesystem/walker"
	"github.com/mantonx/viewra/internal/infrastructure/metadata/music"
	pkgLogger "github.com/mantonx/viewra/internal/pkg/logger"
)

// Coordinator orchestrates the scanning process with worker pool
type Coordinator struct {
	config        Config
	walker        scanner.FileWalker
	filter        scanner.FileFilter
	parser        scanner.FilenameParser
	ffmpegService *ffmpeg.Service
	logger        *slog.Logger

	// Progress tracking (atomic)
	filesFound     atomic.Int64
	filesProcessed atomic.Int64
	bytesProcessed atomic.Int64
	errorCount     atomic.Int64

	// State tracking
	mu        sync.RWMutex
	startTime time.Time
	isRunning bool
}

// NewCoordinator creates a new scanner coordinator
func NewCoordinator(config Config) *Coordinator {
	logger := pkgLogger.DefaultIfNil(config.Logger)

	ffmpegService, err := ffmpeg.NewService(logger)
	if err != nil {
		logger.Warn("FFmpeg not available, technical metadata extraction disabled", "error", err)
	}

	metadataExtractor := music.NewExtractor()

	return &Coordinator{
		config:        config,
		walker:        walker.New(),
		filter:        filter.New(),
		parser:        parsers.NewDefaultParserWithMetadata(metadataExtractor),
		ffmpegService: ffmpegService,
		logger:        logger,
	}
}

// Scan starts a library scan with concurrent file processing
func (c *Coordinator) Scan(ctx context.Context, libraryPath string, resultChan chan<- scanner.ScanResult) error {
	c.mu.Lock()
	if c.isRunning {
		c.mu.Unlock()
		return scanner.ErrAlreadyRunning
	}
	c.isRunning = true
	c.startTime = time.Now()
	c.resetCounters()
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.isRunning = false
		c.mu.Unlock()
	}()

	fileChan := make(chan scanner.FileInfo, c.config.ResultBufferSize)
	discoveryErrChan := make(chan error, 1)

	go c.discoverFiles(ctx, libraryPath, fileChan, discoveryErrChan)

	var wg sync.WaitGroup
	processingCtx, cancelProcessing := context.WithCancel(ctx)
	defer cancelProcessing()

	for i := 0; i < c.config.NumWorkers; i++ {
		wg.Add(1)
		go c.worker(processingCtx, &wg, fileChan, resultChan)
	}

	wg.Wait()

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
	if info.IsDir {
		return false
	}

	osInfo, err := os.Stat(info.Path)
	if err != nil {
		return false
	}

	return c.filter.ShouldProcess(info.Path, osInfo)
}

// isRealError returns true if the error is meaningful (not a cancellation)
func isRealError(err error) bool {
	return err != nil && err != context.Canceled
}

// discoverFiles walks the directory tree and sends files to the channel
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

		c.filesFound.Add(1)

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
				return
			}

			result := c.ProcessFile(ctx, fileInfo)

			select {
			case resultChan <- result:
			case <-ctx.Done():
				return
			}

			c.filesProcessed.Add(1)
			if result.Error != nil {
				c.errorCount.Add(1)
			} else {
				c.bytesProcessed.Add(result.BytesProcessed)
			}
		}
	}
}

// ProcessFile extracts metadata from a single file (exported for checkpoint resumption)
func (c *Coordinator) ProcessFile(ctx context.Context, fileInfo scanner.FileInfo) scanner.ScanResult {
	result := scanner.ScanResult{
		FilePath:       fileInfo.Path,
		MediaType:      scanner.MediaTypeUnknown,
		BytesProcessed: fileInfo.Size,
	}

	select {
	case <-ctx.Done():
		result.Error = ctx.Err()
		return result
	default:
	}

	// Check cache for incremental scan
	if c.config.EnableIncrementalScan {
		c.mu.RLock()
		cached, exists := c.config.FileCache[fileInfo.Path]
		c.mu.RUnlock()

		if exists && cached.IsUnchanged(fileInfo.ModTime, fileInfo.Size) {
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

	mediaType := c.filter.GetMediaType(fileInfo.Extension)
	result.MediaType = mediaType

	// Parse based on media type
	// Note: Episode detection happens at the application layer based on library type,
	// not here. The filter only returns MediaTypeMovie (video) or MediaTypeTrack (audio).
	switch mediaType {
	case scanner.MediaTypeMovie:
		if movieInfo, err := c.parser.ParseMovie(fileInfo.Path); err == nil && movieInfo != nil {
			result.Title = movieInfo.Title
			result.Year = &movieInfo.Year
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

	// Extract technical metadata using FFmpeg
	if c.ffmpegService != nil && (mediaType == scanner.MediaTypeMovie || mediaType == scanner.MediaTypeTrack) {
		metadata, err := c.ffmpegService.ExtractMetadata(ctx, fileInfo.Path)
		if err != nil {
			c.logger.Warn("Failed to extract FFmpeg metadata",
				"file_path", fileInfo.Path,
				"media_type", mediaType,
				"error", err)

			result.Warning = fmt.Errorf("failed to extract metadata: %w", err)
			result.WarningCategory = "ffmpeg"
		} else {
			result.FileSize = metadata.FileSize
			result.Width = metadata.Width
			result.Height = metadata.Height
			result.VideoCodec = metadata.VideoCodec
			result.AudioCodec = metadata.AudioCodec
			result.Bitrate = metadata.Bitrate
			result.FrameRate = metadata.FrameRate
			result.Duration = int64(metadata.Duration.Seconds())

			result.CodecProfile = metadata.CodecProfile
			result.ScanType = metadata.ScanType
			result.HDRFormat = metadata.HDRFormat
			result.ColorSpace = metadata.ColorSpace
			result.ColorPrimaries = metadata.ColorPrimaries

			result.ContainerFormat = strings.TrimPrefix(fileInfo.Extension, ".")

			if mediaType == scanner.MediaTypeMovie {
				tracks, err := c.ffmpegService.ExtractTracks(ctx, fileInfo.Path)
				if err != nil {
					c.logger.Warn("Failed to extract audio/subtitle tracks",
						"file_path", fileInfo.Path,
						"error", err)
				} else {
					result.AudioTracks = make([]scanner.AudioTrackInfo, len(tracks.AudioTracks))
					for i, t := range tracks.AudioTracks {
						result.AudioTracks[i] = scanner.AudioTrackInfo{
							StreamIndex:   t.StreamIndex,
							Codec:         t.Codec,
							CodecProfile:  t.CodecProfile,
							Channels:      t.Channels,
							ChannelLayout: t.ChannelLayout,
							SampleRate:    t.SampleRate,
							BitRate:       t.BitRate,
							Language:      t.Language,
							Title:         t.Title,
							IsDefault:     t.IsDefault,
							IsCommentary:  t.IsCommentary,
							IsDescriptive: t.IsDescriptive,
						}
					}
					result.SubtitleTracks = make([]scanner.SubtitleTrackInfo, len(tracks.SubtitleTracks))
					for i, t := range tracks.SubtitleTracks {
						result.SubtitleTracks[i] = scanner.SubtitleTrackInfo{
							StreamIndex:  t.StreamIndex,
							Codec:        t.Codec,
							Language:     t.Language,
							Title:        t.Title,
							IsDefault:    t.IsDefault,
							IsForced:     t.IsForced,
							IsSDH:        t.IsSDH,
							IsCommentary: t.IsCommentary,
							IsBitmap:     t.IsBitmap,
						}
					}
				}
			}
		}
	}

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
}
