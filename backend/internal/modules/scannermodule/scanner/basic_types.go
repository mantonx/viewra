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

// execCommand is a variable to allow mocking of exec.Command in tests.
var execCommand = exec.Command

// filepathWalkDir is a variable to allow mocking of filepath.WalkDir in tests.
var filepathWalkDir = filepath.WalkDir

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
	return filepathWalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
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
		
		// CRITICAL: Skip artwork and metadata files that should NOT be processed as media
		fileName := strings.ToLower(filepath.Base(path))
		
		// Skip poster and banner files (these should be assets, not media files)
		artworkPatterns := []string{
			"poster.", "banner.", "thumb.", "thumbnail.", "cover.", "artwork.", 
			"fanart.", "background.", "backdrop.", "clearlogo.", "clearart.", 
			"landscape.", "disc.", "folder.", "albumart.", "-poster.", "-banner.", 
			"-thumb.", "-thumbnail.", "-cover.", "-artwork.", "-fanart.", 
			"-background.", "-backdrop.", "-clearlogo.", "-clearart.", 
			"-landscape.", "-disc.", "season01-poster.", "season01-banner.", 
			"season02-poster.", "season02-banner.", "season03-poster.", 
			"season03-banner.", "season04-poster.", "season04-banner.", 
			"season05-poster.", "season05-banner.", "specials-poster.", 
			"specials-banner.", "season-specials-poster.", "season-specials-banner.",
		}
		
		shouldSkip := false
		for _, pattern := range artworkPatterns {
			if strings.Contains(fileName, pattern) {
				shouldSkip = true
				logger.Debug("Skipping artwork file", "path", path, "pattern", pattern)
				break
			}
		}
		
		// Skip subtitle and metadata files
		metadataExtensions := []string{".nfo", ".xml", ".srt", ".vtt", ".ass", ".ssa", ".sub", ".idx"}
		ext := strings.ToLower(filepath.Ext(path))
		for _, metaExt := range metadataExtensions {
			if ext == metaExt {
				shouldSkip = true
				logger.Debug("Skipping metadata file", "path", path, "extension", ext)
				break
			}
		}
		
		// Skip system and temporary files
		systemPatterns := []string{".ds_store", "thumbs.db", ".tmp", ".temp", ".bak", ".backup", ".old", ".orig"}
		for _, sysPattern := range systemPatterns {
			if strings.Contains(fileName, sysPattern) {
				shouldSkip = true
				logger.Debug("Skipping system file", "path", path, "pattern", sysPattern)
				break
			}
		}
		
		if shouldSkip {
			ls.filesSkipped.Add(1)
			return nil
		}
		
		// Check if it's a media file (after filtering out artwork/metadata)
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
	
	// Get library information to determine media type
	var library database.MediaLibrary
	if err := ls.db.First(&library, libraryID).Error; err != nil {
		return fmt.Errorf("failed to get library: %w", err)
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
	
	// IMPORTANT: Set media_type based on library type and file extension
	mediaFile.MediaType = ls.determineMediaType(library.Type, ext)
	
	// Extract technical metadata using FFprobe BEFORE saving to database
	if err := ls.extractTechnicalMetadata(mediaFile); err != nil {
		logger.Warn("Failed to extract technical metadata", "path", filePath, "error", err)
		// Continue even if technical metadata extraction fails
	}
	
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
		
		// DEBUG: Log before reload
		logger.Debug("Before reload", "media_file_id", mediaFile.ID, "original_media_id", mediaFile.MediaID, "original_media_type", mediaFile.MediaType)
		
		if err := ls.db.Where("id = ?", mediaFile.ID).First(&updatedMediaFile).Error; err == nil {
			mediaFile = &updatedMediaFile
			logger.Debug("Reloaded media file after metadata extraction", "media_file_id", mediaFile.ID, "media_id", mediaFile.MediaID, "media_type", mediaFile.MediaType)
		} else {
			logger.Warn("Failed to reload media file after extraction", "media_file_id", mediaFile.ID, "error", err)
		}
		
		// DEBUG: Also try a direct SQL query to see what's in the database
		var dbCheckResult struct {
			ID        string                `gorm:"column:id"`
			MediaID   string                `gorm:"column:media_id"`
			MediaType database.MediaType    `gorm:"column:media_type"`
		}
		if err := ls.db.Table("media_files").Select("id, media_id, media_type").Where("id = ?", mediaFile.ID).First(&dbCheckResult).Error; err == nil {
			logger.Debug("Direct DB check", "id", dbCheckResult.ID, "media_id", dbCheckResult.MediaID, "media_type", dbCheckResult.MediaType)
		} else {
			logger.Debug("Direct DB check failed", "error", err)
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
	
	var processedBy []string
	var lastError error
	
	// Run ALL matching handlers, not just the first one
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
				lastError = err
				continue // Try next handler
			}
			
			logger.Debug("Successfully processed with handler", "handler", handler.GetName(), "file", mediaFile.Path)
			processedBy = append(processedBy, handler.GetName())
		}
	}
	
	if len(processedBy) > 0 {
		logger.Debug("File processed by multiple handlers", "file", mediaFile.Path, "handlers", processedBy)
		return nil // Success if at least one handler succeeded
	}
	
	// If no handlers processed the file and we had errors, return the last error
	if lastError != nil {
		return lastError
	}
	
	// No handlers matched this file - not an error
	logger.Debug("No handlers matched file", "file", mediaFile.Path)
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
		
		// IMPORTANT: Images are NOT media files - they should be treated as assets
		// Removing image extensions from media file detection to prevent
		// banner/poster files from being processed as TV episodes or movies
		// ".jpg": false, ".jpeg": false, ".png": false, ".gif": false, ".bmp": false,
		// ".tiff": false, ".tif": false, ".webp": false, ".svg": false, ".raw": false,
		// ".cr2": false, ".nef": false, ".arw": false, ".dng": false,
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
	
	// IMPORTANT: If no metadata was found from database records, try to extract metadata directly from the file
	// This handles the case where enrichment plugins need metadata but database records don't exist yet
	shouldExtractDirect := false
	
	// For audio files (tracks), check if we're missing essential audio metadata
	if mediaFile.MediaType == "track" && (metadata["title"] == nil || metadata["artist"] == nil || metadata["album"] == nil) {
		shouldExtractDirect = true
	}
	
	// For video files (movies, episodes), check if we're missing essential video metadata  
	if (mediaFile.MediaType == "movie" || mediaFile.MediaType == "episode") && metadata["title"] == nil {
		shouldExtractDirect = true
	}
	
	// For any other media type, extract metadata if we don't have a title
	if mediaFile.MediaType != "track" && mediaFile.MediaType != "movie" && mediaFile.MediaType != "episode" && metadata["title"] == nil {
		shouldExtractDirect = true
	}
	
	if shouldExtractDirect {
		if directMetadata := ls.extractDirectMetadata(mediaFile.Path); directMetadata != nil {
			logger.Debug("Extracted direct metadata from file", "media_file_id", mediaFile.ID, "media_type", mediaFile.MediaType, "title", directMetadata["title"], "extracted_fields", getMapKeys(directMetadata))
			
			// Add direct metadata, but don't overwrite existing database metadata
			for key, value := range directMetadata {
				if metadata[key] == nil && value != nil && value != "" {
					metadata[key] = value
				}
			}
		} else {
			logger.Debug("No direct metadata extracted", "media_file_id", mediaFile.ID, "media_type", mediaFile.MediaType, "path", mediaFile.Path)
		}
	} else {
		logger.Debug("Skipping direct metadata extraction", "media_file_id", mediaFile.ID, "media_type", mediaFile.MediaType, "has_title", metadata["title"] != nil, "has_artist", metadata["artist"] != nil, "has_album", metadata["album"] != nil)
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

// extractDirectMetadata extracts metadata directly from media files using FFprobe
func (ls *LibraryScanner) extractDirectMetadata(filePath string) map[string]interface{} {
	// Check if this is a supported media file (audio or video)
	ext := strings.ToLower(filepath.Ext(filePath))
	
	// Audio file extensions
	audioExts := map[string]bool{
		".mp3": true, ".flac": true, ".wav": true, ".m4a": true, ".aac": true,
		".ogg": true, ".wma": true, ".opus": true, ".aiff": true, ".ape": true, ".wv": true,
	}
	
	// Video file extensions  
	videoExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".wmv": true,
		".flv": true, ".webm": true, ".m4v": true, ".3gp": true, ".ts": true,
		".mpg": true, ".mpeg": true, ".rm": true, ".rmvb": true, ".asf": true, ".divx": true,
	}
	
	isAudioFile := audioExts[ext]
	isVideoFile := videoExts[ext]
	
	if !isAudioFile && !isVideoFile {
		return nil // Not a supported media file
	}
	
	// Use FFprobe to extract metadata
	cmd := execCommand("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath)
	
	output, err := cmd.Output()
	if err != nil {
		logger.Debug("Failed to run ffprobe for direct metadata", "file", filePath, "error", err)
		return nil
	}
	
	// Parse JSON output
	var probeOutput struct {
		Format struct {
			Tags     map[string]string `json:"tags"`
			Duration string            `json:"duration"`
			Bitrate  string            `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			CodecType string            `json:"codec_type"`
			Tags      map[string]string `json:"tags"`
		} `json:"streams"`
	}
	
	if err := json.Unmarshal(output, &probeOutput); err != nil {
		logger.Debug("Failed to parse ffprobe output for direct metadata", "file", filePath, "error", err)
		return nil
	}
	
	metadata := make(map[string]interface{})
	
	// Extract metadata from format tags
	formatTags := probeOutput.Format.Tags
	if formatTags != nil {
		// Common metadata fields for both audio and video
		if title := getTagValue(formatTags, "TITLE", "Title", "title"); title != "" {
			metadata["title"] = title
		}
		
		if description := getTagValue(formatTags, "DESCRIPTION", "Description", "description", "COMMENT", "Comment", "comment"); description != "" {
			metadata["description"] = description
		}
		
		if date := getTagValue(formatTags, "DATE", "YEAR", "Year", "year", "CREATION_TIME", "creation_time"); date != "" {
			// Extract just the year part from dates like "1997-01-01" or "2023-01-01T12:00:00.000000Z"
			if len(date) >= 4 {
				if yearInt, err := strconv.Atoi(date[:4]); err == nil {
					metadata["year"] = yearInt
				}
			}
		}
		
		if genre := getTagValue(formatTags, "GENRE", "Genre", "genre"); genre != "" {
			metadata["genre"] = genre
		}
		
		// Audio-specific metadata
		if isAudioFile {
			if artist := getTagValue(formatTags, "ARTIST", "Artist", "artist"); artist != "" {
				metadata["artist"] = artist
			}
			
			if album := getTagValue(formatTags, "ALBUM", "Album", "album"); album != "" {
				metadata["album"] = album
			}
			
			if albumArtist := getTagValue(formatTags, "ALBUMARTIST", "AlbumArtist", "album_artist"); albumArtist != "" {
				metadata["album_artist"] = albumArtist
			}
			
			if track := getTagValue(formatTags, "TRACK", "track", "TRACKNUMBER"); track != "" {
				if trackNum, err := strconv.Atoi(strings.Split(track, "/")[0]); err == nil {
					metadata["track_number"] = trackNum
				}
			}
		}
		
		// Video-specific metadata
		if isVideoFile {
			// For movies and TV shows, we might have director, studio, etc.
			if director := getTagValue(formatTags, "DIRECTOR", "Director", "director"); director != "" {
				metadata["director"] = director
			}
			
			if studio := getTagValue(formatTags, "STUDIO", "Studio", "studio", "PUBLISHER", "Publisher"); studio != "" {
				metadata["studio"] = studio
			}
			
			if show := getTagValue(formatTags, "SHOW", "Show", "show", "SERIES", "Series", "series"); show != "" {
				metadata["show"] = show
			}
			
			if season := getTagValue(formatTags, "SEASON", "Season", "season"); season != "" {
				if seasonNum, err := strconv.Atoi(season); err == nil {
					metadata["season_number"] = seasonNum
				}
			}
			
			if episode := getTagValue(formatTags, "EPISODE", "Episode", "episode"); episode != "" {
				if episodeNum, err := strconv.Atoi(episode); err == nil {
					metadata["episode_number"] = episodeNum
				}
			}
		}
	}
	
	// Also check stream tags for additional metadata
	for _, stream := range probeOutput.Streams {
		if stream.Tags != nil {
			// If we didn't get title from format, try from streams
			if metadata["title"] == nil {
				if title := getTagValue(stream.Tags, "TITLE", "Title", "title"); title != "" {
					metadata["title"] = title
				}
			}
			
			// Language information
			if language := getTagValue(stream.Tags, "LANGUAGE", "Language", "language"); language != "" {
				metadata["language"] = language
			}
		}
	}
	
	// Extract duration and bitrate from format
	if probeOutput.Format.Duration != "" {
		if durationFloat, err := strconv.ParseFloat(probeOutput.Format.Duration, 64); err == nil {
			metadata["duration"] = int(durationFloat)
		}
	}
	
	if probeOutput.Format.Bitrate != "" {
		if bitrateInt, err := strconv.Atoi(probeOutput.Format.Bitrate); err == nil {
			metadata["bitrate"] = bitrateInt / 1000 // Convert to kbps
		}
	}
	
	logger.Debug("Extracted direct metadata", "file", filePath, "extracted_fields", getMapKeys(metadata), "is_audio", isAudioFile, "is_video", isVideoFile)
	
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

func (ls *LibraryScanner) determineMediaType(libraryType string, ext string) database.MediaType {
	// Audio file extensions
	audioExts := map[string]bool{
		".mp3": true, ".flac": true, ".wav": true, ".m4a": true, ".aac": true,
		".ogg": true, ".wma": true, ".opus": true, ".aiff": true, ".ape": true, ".wv": true,
	}
	
	// Video file extensions  
	videoExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".wmv": true,
		".flv": true, ".webm": true, ".m4v": true, ".3gp": true, ".ts": true,
		".mpg": true, ".mpeg": true, ".rm": true, ".rmvb": true, ".asf": true, ".divx": true,
	}
	
	// Image file extensions
	imageExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true,
		".tiff": true, ".tif": true, ".webp": true, ".svg": true, ".raw": true,
		".cr2": true, ".nef": true, ".arw": true, ".dng": true,
	}
	
	// IMPORTANT: First check for specific media types regardless of library type
	// Images should always be images, even in music libraries (album art, folder art, etc.)
	if imageExts[ext] {
		return database.MediaTypeImage
	}
	
	// Determine media_type based on library type and file extension
	switch libraryType {
	case "music":
		// For music libraries, audio files should be tracks
		if audioExts[ext] {
			return database.MediaTypeTrack
		}
		// Video files in music libraries (music videos) should be movies
		if videoExts[ext] {
			logger.Info("Video file found in music library - treating as movie", "ext", ext, "library_type", libraryType)
			return database.MediaTypeMovie
		}
		// Other files in music libraries - log and skip processing
		logger.Warn("Unsupported file type in music library", "ext", ext, "library_type", libraryType)
		return database.MediaTypeTrack // Fallback for backwards compatibility
		
	case "movie":
		// For movie libraries, files should be movies (video files)
		if videoExts[ext] {
			return database.MediaTypeMovie
		}
		// Audio files in movie libraries (soundtracks) should be tracks
		if audioExts[ext] {
			logger.Info("Audio file found in movie library - treating as track", "ext", ext, "library_type", libraryType)
			return database.MediaTypeTrack
		}
		// Other files - log and use movie as fallback
		logger.Warn("Unsupported file type in movie library", "ext", ext, "library_type", libraryType)
		return database.MediaTypeMovie // Default to movie for movie libraries
		
	case "tv":
		// For TV libraries, files should be episodes (video files)
		if videoExts[ext] {
			return database.MediaTypeEpisode
		}
		// Audio files in TV libraries should be tracks
		if audioExts[ext] {
			logger.Info("Audio file found in TV library - treating as track", "ext", ext, "library_type", libraryType)
			return database.MediaTypeTrack
		}
		// Other files - log and use episode as fallback
		logger.Warn("Unsupported file type in TV library", "ext", ext, "library_type", libraryType)
		return database.MediaTypeEpisode // Default to episode for TV libraries
		
	default:
		// Unknown library type - try to guess based on file extension
		logger.Warn("Unknown library type, guessing media_type from extension", "library_type", libraryType, "ext", ext)
		if audioExts[ext] {
			return database.MediaTypeTrack
		} else if videoExts[ext] {
			return database.MediaTypeMovie // Default video to movie
		}
		// If we can't determine, default to track (safest fallback)
		return database.MediaTypeTrack
	}
}

// extractTechnicalMetadata extracts technical metadata directly from media files using FFprobe
func (ls *LibraryScanner) extractTechnicalMetadata(mediaFile *database.MediaFile) error {
	// Use FFprobe to extract technical metadata
	cmd := execCommand("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		mediaFile.Path)
	
	output, err := cmd.Output()
	if err != nil {
		logger.Debug("Failed to run ffprobe for technical metadata", "file", mediaFile.Path, "error", err)
		return nil
	}
	
	// Parse JSON output
	var probeOutput struct {
		Format struct {
			Tags     map[string]string `json:"tags"`
			Duration string            `json:"duration"`
			Bitrate  string            `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			Index     int               `json:"index"`
			CodecType string            `json:"codec_type"`
			CodecName string            `json:"codec_name"`
			Width     int               `json:"width,omitempty"`
			Height    int               `json:"height,omitempty"`
			SampleRate string           `json:"sample_rate,omitempty"`
			Channels   int               `json:"channels,omitempty"`
			FrameRate  string            `json:"avg_frame_rate,omitempty"`
			BitRate    string            `json:"bit_rate,omitempty"`
			Profile    string            `json:"profile,omitempty"`
			Level      int               `json:"level,omitempty"`
			ChannelLayout string         `json:"channel_layout,omitempty"`
			Tags       map[string]string `json:"tags"`
		} `json:"streams"`
	}
	
	if err := json.Unmarshal(output, &probeOutput); err != nil {
		logger.Debug("Failed to parse ffprobe output for technical metadata", "file", mediaFile.Path, "error", err)
		return nil
	}
	
	// Extract duration and bitrate from format
	if probeOutput.Format.Duration != "" {
		if durationFloat, err := strconv.ParseFloat(probeOutput.Format.Duration, 64); err == nil {
			mediaFile.Duration = int(durationFloat)
		}
	}
	
	if probeOutput.Format.Bitrate != "" {
		if bitrateInt, err := strconv.Atoi(probeOutput.Format.Bitrate); err == nil {
			mediaFile.BitrateKbps = bitrateInt / 1000 // Convert to kbps
		}
	}
	
	// Extract technical information from streams
	var videoStreamFound, audioStreamFound bool
	extractedFields := []string{}
	
	for _, stream := range probeOutput.Streams {
		if stream.CodecType == "video" && !videoStreamFound {
			// Video stream information
			mediaFile.VideoCodec = stream.CodecName
			if stream.Width > 0 && stream.Height > 0 {
				mediaFile.VideoWidth = stream.Width
				mediaFile.VideoHeight = stream.Height
				mediaFile.Resolution = fmt.Sprintf("%dx%d", stream.Width, stream.Height)
			}
			if stream.FrameRate != "" && stream.FrameRate != "0/0" {
				mediaFile.VideoFramerate = stream.FrameRate
			}
			if stream.Profile != "" {
				mediaFile.VideoProfile = stream.Profile
			}
			if stream.Level > 0 {
				mediaFile.VideoLevel = stream.Level
			}
			videoStreamFound = true
			extractedFields = append(extractedFields, "video_codec", "resolution", "video_width", "video_height")
			
		} else if stream.CodecType == "audio" && !audioStreamFound {
			// Audio stream information
			mediaFile.AudioCodec = stream.CodecName
			if stream.SampleRate != "" {
				if sampleRateInt, err := strconv.Atoi(stream.SampleRate); err == nil {
					mediaFile.SampleRate = sampleRateInt
					mediaFile.AudioSampleRate = sampleRateInt
				}
			}
			if stream.Channels > 0 {
				mediaFile.Channels = fmt.Sprintf("%d", stream.Channels)
				mediaFile.AudioChannels = stream.Channels
			}
			if stream.ChannelLayout != "" {
				mediaFile.AudioLayout = stream.ChannelLayout
			}
			if stream.Profile != "" {
				mediaFile.AudioProfile = stream.Profile
			}
			audioStreamFound = true
			extractedFields = append(extractedFields, "audio_codec", "sample_rate", "channels", "audio_channels")
		}
	}
	
	logger.Debug("Extracted technical metadata", "file", mediaFile.Path, "duration", mediaFile.Duration, "video_codec", mediaFile.VideoCodec, "audio_codec", mediaFile.AudioCodec, "resolution", mediaFile.Resolution, "sample_rate", mediaFile.SampleRate, "extracted_fields", extractedFields)
	
	return nil
}





