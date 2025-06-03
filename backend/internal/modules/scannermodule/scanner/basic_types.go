package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
	"gorm.io/gorm"
)

// Basic types needed by the scanner components

type scanWork struct {
	path      string
	info      os.FileInfo
	libraryID uint
}

type dirWork struct {
	path      string
	libraryID uint
}

type scanResult = ScanResult

type BatchProcessor struct {
	db        *gorm.DB
	batchSize int
	mu        sync.Mutex
}

// Interfaces for adaptive throttling and progress estimation
type ProgressEstimatorInterface interface {
	GetEstimate() (progress float64, eta time.Time, rate float64)
	GetTotalBytes() int64
}

// SystemMetricsStub provides stub system metrics
type SystemMetricsStub struct {
	CPUPercent      float64
	MemoryPercent   float64
	MemoryUsedMB    float64
	IOWaitPercent   float64
	LoadAverage     float64
	NetworkUtilMBps float64
	DiskReadMBps    float64
	DiskWriteMBps   float64
	TimestampUTC    string
}

// ThrottleLimitsStub provides stub throttle limits
type ThrottleLimitsStub struct {
	Enabled          bool
	BatchSize        int
	ProcessingDelay  time.Duration
	NetworkBandwidth int64
	IOThrottle       float64
}

// NetworkStatsStub provides stub network statistics
type NetworkStatsStub struct {
	DNSLatencyMs      float64
	NetworkLatencyMs  float64
	PacketLossPercent float64
	ConnectionErrors  int
	IsHealthy         bool
	LastHealthCheck   time.Time
}

// ThrottleConfigStub provides stub throttle configuration
type ThrottleConfigStub struct {
	TargetCPUPercent        float64
	MaxCPUPercent           float64
	TargetMemoryPercent     float64
	MaxMemoryPercent        float64
	TargetNetworkThroughput float64
	MaxNetworkThroughput    float64
	EmergencyBrakeThreshold float64
}

type AdaptiveThrottlerInterface interface {
	GetCurrentLimits() ThrottleLimitsStub
	GetSystemMetrics() SystemMetricsStub
	GetNetworkStats() NetworkStatsStub
	GetThrottleConfig() ThrottleConfigStub
	ShouldThrottle() (bool, time.Duration)
	DisableThrottling()
	EnableThrottling()
}

// Basic implementations
type ProgressEstimatorStub struct{}

func (p *ProgressEstimatorStub) GetEstimate() (progress float64, eta time.Time, rate float64) {
	return 0, time.Now(), 0
}

func (p *ProgressEstimatorStub) GetTotalBytes() int64 {
	return 0
}

type AdaptiveThrottlerStub struct{}

func (a *AdaptiveThrottlerStub) GetCurrentLimits() ThrottleLimitsStub {
	return ThrottleLimitsStub{
		Enabled:          false,
		BatchSize:        100,
		ProcessingDelay:  0,
		NetworkBandwidth: 0,
		IOThrottle:       0,
	}
}

func (a *AdaptiveThrottlerStub) GetSystemMetrics() SystemMetricsStub {
	return SystemMetricsStub{
		CPUPercent:      0,
		MemoryPercent:   0,
		MemoryUsedMB:    0,
		IOWaitPercent:   0,
		LoadAverage:     0,
		NetworkUtilMBps: 0,
		DiskReadMBps:    0,
		DiskWriteMBps:   0,
		TimestampUTC:    "",
	}
}

func (a *AdaptiveThrottlerStub) GetNetworkStats() NetworkStatsStub {
	return NetworkStatsStub{
		DNSLatencyMs:      0,
		NetworkLatencyMs:  0,
		PacketLossPercent: 0,
		ConnectionErrors:  0,
		IsHealthy:         true,
		LastHealthCheck:   time.Now(),
	}
}

func (a *AdaptiveThrottlerStub) GetThrottleConfig() ThrottleConfigStub {
	return ThrottleConfigStub{
		TargetCPUPercent:        70,
		MaxCPUPercent:           90,
		TargetMemoryPercent:     80,
		MaxMemoryPercent:        95,
		TargetNetworkThroughput: 100,
		MaxNetworkThroughput:    200,
		EmergencyBrakeThreshold: 95,
	}
}

func (a *AdaptiveThrottlerStub) ShouldThrottle() (bool, time.Duration) { return false, 0 }
func (a *AdaptiveThrottlerStub) DisableThrottling()                    {}
func (a *AdaptiveThrottlerStub) EnableThrottling()                     {}

type LibraryScanner struct {
	db           *gorm.DB
	jobID        uint32
	eventBus     events.EventBus
	pluginModule *pluginmodule.PluginModule
	enrichmentHook ScannerPluginHook

	enhancedPluginRouter interface{}
	libraryPluginManager interface{}
	corePluginsManager   interface{}
	progressEstimator    ProgressEstimatorInterface
	adaptiveThrottler    AdaptiveThrottlerInterface

	// Atomic counters for thread-safe progress tracking
	filesProcessed atomic.Int64
	filesFound     atomic.Int64
	filesSkipped   atomic.Int64
	bytesProcessed atomic.Int64
	bytesFound     atomic.Int64
	errorsCount    atomic.Int64

	// Scanner control
	ctx      context.Context
	cancel   context.CancelFunc
	paused   atomic.Bool
	running  atomic.Bool
	wg       sync.WaitGroup
	
	// Processing
	workers    int
	batchSize  int
	
	// File processing queue
	fileQueue chan string
	
	// Progress tracking
	lastProgressUpdate time.Time
	progressMutex      sync.RWMutex
}

func NewLibraryScanner(db *gorm.DB, jobID uint32, eventBus events.EventBus, pluginModule *pluginmodule.PluginModule, enrichmentHook ScannerPluginHook) *LibraryScanner {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &LibraryScanner{
		db:           db,
		jobID:        jobID,
		eventBus:     eventBus,
		pluginModule: pluginModule,
		enrichmentHook: enrichmentHook,
		ctx:          ctx,
		cancel:       cancel,
		workers:      runtime.NumCPU(),
		batchSize:    100,
		fileQueue:    make(chan string, 1000),
		progressEstimator: &ProgressEstimatorStub{},
		adaptiveThrottler: &AdaptiveThrottlerStub{},
	}
}

func (ls *LibraryScanner) Start(libraryID uint32) error {
	// Prevent multiple concurrent starts
	if ls.running.Load() {
		return fmt.Errorf("scanner already running")
	}
	
	ls.running.Store(true)
	ls.paused.Store(false)
	
	// Create cancellable context
	ls.ctx, ls.cancel = context.WithCancel(context.Background())
	
	// Update scan job status to running
	if err := ls.updateScanJobStatus("running", "Starting filesystem scan..."); err != nil {
		return fmt.Errorf("failed to update scan job status: %w", err)
	}
	
	// Get library information
	var library database.MediaLibrary
	if err := ls.db.First(&library, libraryID).Error; err != nil {
		return fmt.Errorf("failed to get library: %w", err)
	}
	
	logger.Info("Starting scan", "library_id", libraryID, "path", library.Path, "job_id", ls.jobID)
	
	// Start worker goroutines
	for i := 0; i < ls.workers; i++ {
		ls.wg.Add(1)
		go ls.fileWorker(uint(libraryID))
	}
	
	// Start progress updater
	ls.wg.Add(1)
	go ls.progressUpdater()
	
	// Start the main scanning process
	ls.wg.Add(1)
	go func() {
		defer ls.wg.Done()
		if err := ls.scanDirectory(library.Path, uint(libraryID)); err != nil {
			logger.Error("Scan failed", "error", err, "job_id", ls.jobID)
			ls.updateScanJobStatus("failed", fmt.Sprintf("Scan failed: %v", err))
		} else {
			ls.finalizeScan()
		}
		
		// Close file queue to stop workers
		close(ls.fileQueue)
	}()
	
	return nil
}

func (ls *LibraryScanner) Resume(libraryID uint32) error {
	if ls.running.Load() {
		return fmt.Errorf("scanner already running")
	}
	
	logger.Info("Resuming scan", "library_id", libraryID, "job_id", ls.jobID)
	
	// Update status
	if err := ls.updateScanJobStatus("running", "Resuming scan..."); err != nil {
		return fmt.Errorf("failed to update scan job status: %w", err)
	}
	
	// For now, treat resume the same as start
	// In a more sophisticated implementation, we'd track where we left off
	return ls.Start(libraryID)
}

func (ls *LibraryScanner) Pause() {
	logger.Info("Pausing scan", "job_id", ls.jobID)
	
	ls.paused.Store(true)
	ls.updateScanJobStatus("paused", "Scan paused by user")
	
	if ls.cancel != nil {
		ls.cancel()
	}
}

func (ls *LibraryScanner) scanDirectory(dirPath string, libraryID uint) error {
	logger.Info("Scanning directory", "path", dirPath, "job_id", ls.jobID)
	
	// Walk the directory tree
	return filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		// Check for cancellation
		select {
		case <-ls.ctx.Done():
			return fmt.Errorf("scan cancelled")
		default:
		}
		
		// Check for pause
		if ls.paused.Load() {
			return fmt.Errorf("scan paused")
		}
		
		if err != nil {
			logger.Warn("Error accessing path", "path", path, "error", err)
			ls.errorsCount.Add(1)
			return nil // Continue walking
		}
		
		// Skip directories
		if d.IsDir() {
			return nil
		}
		
		// Check if it's a media file
		if ls.isMediaFile(path) {
			ls.filesFound.Add(1)
			
			// Add file info for byte counting
			if info, err := d.Info(); err == nil {
				ls.bytesFound.Add(info.Size())
			}
			
			// Queue file for processing
			select {
			case ls.fileQueue <- path:
				// File queued successfully
			case <-ls.ctx.Done():
				return fmt.Errorf("scan cancelled while queueing file")
			case <-time.After(5 * time.Second):
				logger.Warn("File queue full, skipping file", "path", path)
				ls.filesSkipped.Add(1)
			}
		} else {
			// Not a media file, skip
			ls.filesSkipped.Add(1)
		}
		
		return nil
	})
}

func (ls *LibraryScanner) fileWorker(libraryID uint) {
	defer ls.wg.Done()
	
	for {
		select {
		case filePath, ok := <-ls.fileQueue:
			if !ok {
				return // Channel closed, worker done
			}
			
			if err := ls.processFile(filePath, libraryID); err != nil {
				logger.Error("Failed to process file", "path", filePath, "error", err)
				ls.errorsCount.Add(1)
			} else {
				ls.filesProcessed.Add(1)
			}
			
		case <-ls.ctx.Done():
			return // Context cancelled
		}
	}
}

func (ls *LibraryScanner) processFile(filePath string, libraryID uint) error {
	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	
	// Check if file already exists in database
	var existingFile database.MediaFile
	err = ls.db.Where("path = ? AND library_id = ?", filePath, libraryID).First(&existingFile).Error
	if err == nil {
		// File already exists, update last_seen
		ls.db.Model(&existingFile).Update("last_seen", time.Now())
		ls.bytesProcessed.Add(fileInfo.Size())
		return nil
	}
	
	// Create new media file record
	mediaFile := &database.MediaFile{
		ID:        uuid.New().String(),
		LibraryID: uint32(libraryID),
		Path:      filePath,
		SizeBytes: fileInfo.Size(),
		ScanJobID: &ls.jobID,
		LastSeen:  time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Detect file type from extension
	ext := strings.ToLower(filepath.Ext(filePath))
	mediaFile.Container = ls.getContainerFromExtension(ext)
	
	// Save to database FIRST before calling plugins
	if err := ls.db.Create(mediaFile).Error; err != nil {
		return fmt.Errorf("failed to save media file: %w", err)
	}
	
	// Extract metadata using plugins if available (AFTER saving to database)
	if ls.pluginModule != nil {
		if err := ls.extractMetadata(mediaFile); err != nil {
			logger.Warn("Failed to extract metadata", "path", filePath, "error", err)
			// Continue even if metadata extraction fails
		}
		
		// IMPORTANT: Reload media file from database after metadata extraction
		// to ensure we have the updated media_id and media_type that was set by
		// the enrichment plugins (linkMediaFileToTrack)
		var updatedMediaFile database.MediaFile
		if err := ls.db.Where("id = ?", mediaFile.ID).First(&updatedMediaFile).Error; err == nil {
			mediaFile = &updatedMediaFile
			logger.Debug("Reloaded media file after metadata extraction", "media_file_id", mediaFile.ID, "media_id", mediaFile.MediaID, "media_type", mediaFile.MediaType)
		} else {
			logger.Warn("Failed to reload media file after extraction", "media_file_id", mediaFile.ID, "error", err)
		}
	}
	
	// Call enrichment hook after processing the file
	logger.Debug("About to call enrichment hook", "path", filePath, "hook_available", ls.enrichmentHook != nil, "media_file_id", mediaFile.ID)
	if ls.enrichmentHook != nil {
		// Retrieve actual metadata from database after extraction
		metadata := ls.getMetadataForEnrichment(mediaFile)
		
		logger.Debug("Calling enrichment hook with metadata", "path", filePath, "metadata_size", len(metadata), "media_file_id", mediaFile.ID)
		if err := ls.enrichmentHook.OnMediaFileScanned(mediaFile, metadata); err != nil {
			logger.Warn("Enrichment hook failed", "path", filePath, "error", err)
			// Continue even if enrichment hook fails
		} else {
			logger.Debug("Successfully called enrichment hook", "path", filePath)
		}
	} else {
		logger.Warn("No enrichment hook available", "path", filePath, "media_file_id", mediaFile.ID)
	}
	
	ls.bytesProcessed.Add(fileInfo.Size())
	
	logger.Debug("Processed file", "path", filePath, "size", fileInfo.Size())
	return nil
}

func (ls *LibraryScanner) extractMetadata(mediaFile *database.MediaFile) error {
	// Get enabled file handlers from plugin system
	handlers := ls.pluginModule.GetEnabledFileHandlers()
	
	// Get file info for plugin matching
	fileInfo, err := os.Stat(mediaFile.Path)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	
	for _, handler := range handlers {
		// Check if this handler supports the file
		if handler.Match(mediaFile.Path, fileInfo) {
			ctx := &pluginmodule.MetadataContext{
				DB:        ls.db,
				MediaFile: mediaFile,
				EventBus:  ls.eventBus,
				PluginID:  handler.GetName(),
			}
			
			if err := handler.HandleFile(mediaFile.Path, ctx); err != nil {
				logger.Warn("Handler failed", "handler", handler.GetName(), "file", mediaFile.Path, "error", err)
				continue // Try next handler
			}
			
			logger.Debug("Successfully processed with handler", "handler", handler.GetName(), "file", mediaFile.Path)
			break // Successfully processed, no need to try other handlers
		}
	}
	
	return nil
}

func (ls *LibraryScanner) progressUpdater() {
	defer ls.wg.Done()
	
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			ls.updateProgress()
		case <-ls.ctx.Done():
			return
		}
	}
}

func (ls *LibraryScanner) updateProgress() {
	ls.progressMutex.Lock()
	defer ls.progressMutex.Unlock()
	
	filesFound := ls.filesFound.Load()
	filesProcessed := ls.filesProcessed.Load()
	bytesProcessed := ls.bytesProcessed.Load()
	errorsCount := ls.errorsCount.Load()
	
	// Calculate progress percentage
	var progress float64
	if filesFound > 0 {
		progress = float64(filesProcessed) / float64(filesFound) * 100.0
		if progress > 100 {
			progress = 100
		}
	}
	
	// Update scan job in database
	updates := map[string]interface{}{
		"progress":         progress,
		"files_found":      filesFound,
		"files_processed":  filesProcessed,
		"bytes_processed":  bytesProcessed,
		"status_message":   fmt.Sprintf("Processed %d of %d files (%d errors)", filesProcessed, filesFound, errorsCount),
		"updated_at":       time.Now(),
	}
	
	if err := ls.db.Model(&database.ScanJob{}).Where("id = ?", ls.jobID).Updates(updates).Error; err != nil {
		logger.Error("Failed to update scan progress", "job_id", ls.jobID, "error", err)
	}
	
	// Publish progress event
	if ls.eventBus != nil {
		event := events.NewSystemEvent(
			events.EventScanProgress,
			"Scan Progress Update",
			fmt.Sprintf("Scan job #%d: %.1f%% complete", ls.jobID, progress),
		)
		event.Data = map[string]interface{}{
			"scanJobId":       ls.jobID,
			"progress":        progress,
			"filesFound":      filesFound,
			"filesProcessed":  filesProcessed,
			"bytesProcessed":  bytesProcessed,
			"errorsCount":     errorsCount,
		}
		ls.eventBus.PublishAsync(event)
	}
	
	ls.lastProgressUpdate = time.Now()
}

func (ls *LibraryScanner) finalizeScan() {
	// Wait for all workers to finish
	ls.wg.Wait()
	
	filesFound := ls.filesFound.Load()
	filesProcessed := ls.filesProcessed.Load()
	bytesProcessed := ls.bytesProcessed.Load()
	errorsCount := ls.errorsCount.Load()
	
	// Final status update
	status := "completed"
	message := fmt.Sprintf("Scan completed: %d files processed, %d errors", filesProcessed, errorsCount)
	
	now := time.Now()
	updates := map[string]interface{}{
		"status":           status,
		"status_message":   message,
		"progress":         100.0,
		"files_found":      filesFound,
		"files_processed":  filesProcessed,
		"bytes_processed":  bytesProcessed,
		"completed_at":     now,
		"updated_at":       now,
	}
	
	if err := ls.db.Model(&database.ScanJob{}).Where("id = ?", ls.jobID).Updates(updates).Error; err != nil {
		logger.Error("Failed to finalize scan job", "job_id", ls.jobID, "error", err)
	}
	
	ls.running.Store(false)
	
	logger.Info("Scan completed", 
		"job_id", ls.jobID, 
		"files_found", filesFound, 
		"files_processed", filesProcessed, 
		"bytes_processed", bytesProcessed,
		"errors", errorsCount)
}

func (ls *LibraryScanner) updateScanJobStatus(status, message string) error {
	updates := map[string]interface{}{
		"status":         status,
		"status_message": message,
		"updated_at":     time.Now(),
	}
	
	if status == "running" {
		updates["started_at"] = time.Now()
	}
	
	return ls.db.Model(&database.ScanJob{}).Where("id = ?", ls.jobID).Updates(updates).Error
}

func (ls *LibraryScanner) isMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	
	mediaExts := map[string]bool{
		// Audio
		".mp3": true, ".flac": true, ".wav": true, ".m4a": true, ".aac": true,
		".ogg": true, ".wma": true, ".aiff": true, ".ape": true, ".opus": true,
		".m4p": true, ".mp4": true, // Some audio files use .mp4
		
		// Video  
		".mkv": true, ".avi": true, ".mov": true, ".wmv": true, ".flv": true,
		".webm": true, ".m4v": true, ".3gp": true, ".ts": true, ".mpg": true,
		".mpeg": true, ".rm": true, ".rmvb": true, ".asf": true, ".divx": true,
		
		// Images
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true,
		".tiff": true, ".tif": true, ".webp": true, ".svg": true, ".raw": true,
		".cr2": true, ".nef": true, ".arw": true, ".dng": true,
	}
	
	return mediaExts[ext]
}

func (ls *LibraryScanner) getContainerFromExtension(ext string) string {
	// Map file extensions to container formats
	containerMap := map[string]string{
		// Audio containers
		".mp3":  "mp3",
		".flac": "flac", 
		".wav":  "wav",
		".m4a":  "m4a",
		".aac":  "aac",
		".ogg":  "ogg",
		".wma":  "wma",
		".opus": "opus",
		".aiff": "aiff",
		".ape":  "ape",
		
		// Video containers
		".mp4":  "mp4",
		".mkv":  "mkv",
		".avi":  "avi", 
		".mov":  "mov",
		".wmv":  "wmv",
		".flv":  "flv",
		".webm": "webm",
		".m4v":  "m4v",
		".3gp":  "3gp",
		".ts":   "ts",
		".mpg":  "mpg",
		".mpeg": "mpeg",
		
		// Image formats
		".jpg":  "jpeg",
		".jpeg": "jpeg",
		".png":  "png", 
		".gif":  "gif",
		".bmp":  "bmp",
		".tiff": "tiff",
		".webp": "webp",
		".svg":  "svg",
	}
	
	if container, exists := containerMap[ext]; exists {
		return container
	}
	
	// Remove the dot and return the extension as container
	if len(ext) > 1 && ext[0] == '.' {
		return ext[1:]
	}
	
	return "unknown"
}

func (ls *LibraryScanner) GetWorkerStats() (active, min, max, queueLen int) {
	return ls.workers, 1, ls.workers * 2, len(ls.fileQueue)
}

func NewEnhancedPluginRouter(pm *pluginmodule.PluginModule, lpm, cpm interface{}) interface{} {
	return nil
}

func (ls *LibraryScanner) getMetadataForEnrichment(mediaFile *database.MediaFile) map[string]interface{} {
	metadata := make(map[string]interface{})
	
	// Debug logging
	logger.Debug("Getting metadata for enrichment", "media_file_id", mediaFile.ID, "media_id", mediaFile.MediaID, "media_type", mediaFile.MediaType, "path", mediaFile.Path)
	
	// For music files, retrieve track metadata
	if mediaFile.MediaType == "track" && mediaFile.MediaID != "" {
		var track struct {
			ID          string `gorm:"column:id"`
			Title       string `gorm:"column:title"`
			TrackNumber *int   `gorm:"column:track_number"`
			Duration    *int   `gorm:"column:duration"`
			Lyrics      string `gorm:"column:lyrics"`
			ArtistID    string `gorm:"column:artist_id"`
			AlbumID     string `gorm:"column:album_id"`
		}
		
		var artist struct {
			ID   string `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		
		var album struct {
			ID     string `gorm:"column:id"`
			Title  string `gorm:"column:title"`
			Year   *int   `gorm:"column:year"`
			Genre  string `gorm:"column:genre"`
		}
		
		// Get track information
		if err := ls.db.Table("tracks").Where("id = ?", mediaFile.MediaID).First(&track).Error; err == nil {
			metadata["title"] = track.Title
			if track.TrackNumber != nil {
				metadata["track_number"] = *track.TrackNumber
			}
			if track.Duration != nil {
				metadata["duration"] = *track.Duration
			}
			if track.Lyrics != "" {
				metadata["lyrics"] = track.Lyrics
			}
			
			// Get artist information
			if track.ArtistID != "" {
				if err := ls.db.Table("artists").Where("id = ?", track.ArtistID).First(&artist).Error; err == nil {
					metadata["artist"] = artist.Name
					metadata["artist_id"] = artist.ID
				} else {
					logger.Debug("Failed to get artist", "artist_id", track.ArtistID, "error", err)
				}
			}
			
			// Get album information
			if track.AlbumID != "" {
				if err := ls.db.Table("albums").Where("id = ?", track.AlbumID).First(&album).Error; err == nil {
					metadata["album"] = album.Title
					metadata["album_id"] = album.ID
					if album.Year != nil {
						metadata["year"] = *album.Year
					}
					if album.Genre != "" {
						metadata["genre"] = album.Genre
					}
				} else {
					logger.Debug("Failed to get album", "album_id", track.AlbumID, "error", err)
				}
			}
			
			logger.Debug("Found track metadata", "media_file_id", mediaFile.ID, "title", track.Title, "artist", metadata["artist"], "album", metadata["album"])
		} else {
			logger.Debug("Failed to get track", "media_id", mediaFile.MediaID, "error", err)
		}
	} else {
		logger.Debug("Skipping metadata lookup", "media_type", mediaFile.MediaType, "media_id", mediaFile.MediaID, "reason", "not a track or empty media_id")
	}
	
	// IMPORTANT: If no metadata was found from database records, try to extract ID3 tags directly
	// This handles the case where enrichment plugins need metadata but database records don't exist yet
	if metadata["title"] == nil || metadata["artist"] == nil || metadata["album"] == nil {
		if directMetadata := ls.extractDirectMetadata(mediaFile.Path); directMetadata != nil {
			logger.Debug("Extracted direct metadata from file", "media_file_id", mediaFile.ID, "title", directMetadata["title"], "artist", directMetadata["artist"], "album", directMetadata["album"])
			
			// Add direct metadata, but don't overwrite existing database metadata
			for key, value := range directMetadata {
				if metadata[key] == nil && value != nil && value != "" {
					metadata[key] = value
				}
			}
		}
	}
	
	// Add basic file information
	metadata["file_path"] = mediaFile.Path
	metadata["media_type"] = mediaFile.MediaType
	metadata["container"] = mediaFile.Container
	if mediaFile.Duration > 0 {
		metadata["file_duration"] = mediaFile.Duration
	}
	if mediaFile.BitrateKbps > 0 {
		metadata["bitrate"] = mediaFile.BitrateKbps
	}
	metadata["size_bytes"] = mediaFile.SizeBytes
	
	logger.Debug("Enrichment metadata prepared", "media_file_id", mediaFile.ID, "metadata_keys", getMapKeys(metadata))
	
	return metadata
}

// extractDirectMetadata extracts ID3 tags directly from audio files using FFprobe
func (ls *LibraryScanner) extractDirectMetadata(filePath string) map[string]interface{} {
	// Check if this is an audio file
	ext := strings.ToLower(filepath.Ext(filePath))
	audioExts := map[string]bool{
		".mp3": true, ".flac": true, ".wav": true, ".m4a": true, ".aac": true,
		".ogg": true, ".wma": true, ".opus": true, ".aiff": true, ".ape": true, ".wv": true,
	}
	
	if !audioExts[ext] {
		return nil // Not an audio file
	}
	
	// Use FFprobe to extract metadata tags
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		filePath)
	
	output, err := cmd.Output()
	if err != nil {
		logger.Debug("Failed to run ffprobe for direct metadata", "file", filePath, "error", err)
		return nil
	}
	
	// Parse JSON output
	var probeOutput struct {
		Format struct {
			Tags map[string]string `json:"tags"`
		} `json:"format"`
	}
	
	if err := json.Unmarshal(output, &probeOutput); err != nil {
		logger.Debug("Failed to parse ffprobe output for direct metadata", "file", filePath, "error", err)
		return nil
	}
	
	if probeOutput.Format.Tags == nil {
		return nil
	}
	
	tags := probeOutput.Format.Tags
	metadata := make(map[string]interface{})
	
	// Extract common ID3 tags with fallbacks
	if title := getTagValue(tags, "TITLE", "Title", "title"); title != "" {
		metadata["title"] = title
	}
	
	if artist := getTagValue(tags, "ARTIST", "Artist", "artist"); artist != "" {
		metadata["artist"] = artist
	}
	
	if album := getTagValue(tags, "ALBUM", "Album", "album"); album != "" {
		metadata["album"] = album
	}
	
	if albumArtist := getTagValue(tags, "ALBUMARTIST", "AlbumArtist", "album_artist"); albumArtist != "" {
		metadata["album_artist"] = albumArtist
	}
	
	if genre := getTagValue(tags, "GENRE", "Genre", "genre"); genre != "" {
		metadata["genre"] = genre
	}
	
	if year := getTagValue(tags, "DATE", "YEAR", "Year", "year"); year != "" {
		// Extract just the year part from dates like "1997-01-01"
		if len(year) >= 4 {
			if yearInt, err := strconv.Atoi(year[:4]); err == nil {
				metadata["year"] = yearInt
			}
		}
	}
	
	if track := getTagValue(tags, "TRACK", "track", "TRACKNUMBER"); track != "" {
		if trackNum, err := strconv.Atoi(strings.Split(track, "/")[0]); err == nil {
			metadata["track_number"] = trackNum
		}
	}
	
	logger.Debug("Extracted direct metadata", "file", filePath, "extracted_fields", getMapKeys(metadata))
	
	return metadata
}

// Helper function to get map keys for debugging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// getTagValue retrieves a tag value with case-insensitive fallbacks
func getTagValue(tags map[string]string, keys ...string) string {
	for _, key := range keys {
		if value, exists := tags[key]; exists && value != "" {
			return value
		}
	}
	return ""
}
