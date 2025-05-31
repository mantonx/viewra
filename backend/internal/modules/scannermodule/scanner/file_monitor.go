package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/plugins"
	"gorm.io/gorm"
)

// FileMonitor provides real-time file system monitoring for media libraries
type FileMonitor struct {
	db            *gorm.DB
	eventBus      events.EventBus
	pluginManager plugins.Manager
	
	// File system watcher
	watcher *fsnotify.Watcher
	
	// State management
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.RWMutex
	
	// Library monitoring state
	monitoredLibraries map[uint]*MonitoredLibrary // libraryID -> monitor info
	
	// Event processing
	fileProcessor    *FileProcessor
	eventQueue       chan FileEvent
	batchProcessor   *BatchProcessor
	debounceInterval time.Duration
}

// MonitoredLibrary represents a library being monitored
type MonitoredLibrary struct {
	ID             uint
	Path           string
	Type           string
	LastScanJobID  uint
	StartTime      time.Time
	FilesProcessed int64
	Status         string // "monitoring", "processing", "error"
}

// FileEvent represents a file system event to be processed
type FileEvent struct {
	Type      fsnotify.Op
	Path      string
	LibraryID uint
	Timestamp time.Time
}

// FileProcessor handles individual file processing
type FileProcessor struct {
	db            *gorm.DB
	pluginManager plugins.Manager
	eventBus      events.EventBus
}

// NewFileMonitor creates a new file monitor
func NewFileMonitor(db *gorm.DB, eventBus events.EventBus, pluginManager plugins.Manager) (*FileMonitor, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	fm := &FileMonitor{
		db:                 db,
		eventBus:           eventBus,
		pluginManager:      pluginManager,
		watcher:            watcher,
		ctx:                ctx,
		cancel:             cancel,
		monitoredLibraries: make(map[uint]*MonitoredLibrary),
		eventQueue:         make(chan FileEvent, 1000), // Buffer for file events
		debounceInterval:   time.Second * 2,           // Debounce rapid file changes
	}
	
	// Initialize file processor
	fm.fileProcessor = &FileProcessor{
		db:            db,
		pluginManager: pluginManager,
		eventBus:      eventBus,
	}
	
	return fm, nil
}

// Start begins the file monitoring service
func (fm *FileMonitor) Start() error {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	
	logger.Info("Starting file monitor service")
	
	// Start the main event loop
	fm.wg.Add(1)
	go fm.watchEvents()
	
	// Start the file event processor
	fm.wg.Add(1)
	go fm.processFileEvents()
	
	logger.Info("File monitor service started")
	return nil
}

// Stop stops the file monitoring service
func (fm *FileMonitor) Stop() error {
	logger.Info("Stopping file monitor service")
	
	fm.cancel()
	
	if fm.watcher != nil {
		fm.watcher.Close()
	}
	
	fm.wg.Wait()
	
	logger.Info("File monitor service stopped")
	return nil
}

// StartMonitoring begins monitoring a library after scan completion
func (fm *FileMonitor) StartMonitoring(libraryID uint, scanJobID uint) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	
	// Load library info
	var library database.MediaLibrary
	if err := fm.db.First(&library, libraryID).Error; err != nil {
		return fmt.Errorf("failed to load library: %w", err)
	}
	
	// Check if already monitoring
	if _, exists := fm.monitoredLibraries[libraryID]; exists {
		logger.Info("Library already being monitored", "library_id", libraryID)
		return nil
	}
	
	// Add watch for the library path
	if err := fm.watcher.Add(library.Path); err != nil {
		return fmt.Errorf("failed to add watch for %s: %w", library.Path, err)
	}
	
	// Add recursive watching for subdirectories
	if err := fm.addRecursiveWatch(library.Path); err != nil {
		logger.Error("Failed to add recursive watches", "path", library.Path, "error", err)
		// Continue anyway, we'll catch new subdirectories as they're created
	}
	
	// Create monitoring record
	monitored := &MonitoredLibrary{
		ID:            libraryID,
		Path:          library.Path,
		Type:          library.Type,
		LastScanJobID: scanJobID,
		StartTime:     time.Now(),
		Status:        "monitoring",
	}
	
	fm.monitoredLibraries[libraryID] = monitored
	
	// Emit monitoring started event
	if fm.eventBus != nil {
		event := events.NewSystemEvent(
			"library.monitoring.started",
			"Library Monitoring Started",
			fmt.Sprintf("Started monitoring library #%d at %s", libraryID, library.Path),
		)
		event.Data = map[string]interface{}{
			"library_id":     libraryID,
			"library_path":   library.Path,
			"library_type":   library.Type,
			"last_scan_job":  scanJobID,
		}
		fm.eventBus.PublishAsync(event)
	}
	
	logger.Info("Started monitoring library", "library_id", libraryID, "path", library.Path)
	return nil
}

// StopMonitoring stops monitoring a specific library
func (fm *FileMonitor) StopMonitoring(libraryID uint) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	
	monitored, exists := fm.monitoredLibraries[libraryID]
	if !exists {
		return fmt.Errorf("library %d is not being monitored", libraryID)
	}
	
	// Remove the watch
	if err := fm.watcher.Remove(monitored.Path); err != nil {
		logger.Error("Failed to remove watch", "path", monitored.Path, "error", err)
	}
	
	// Remove from monitoring map
	delete(fm.monitoredLibraries, libraryID)
	
	// Emit monitoring stopped event
	if fm.eventBus != nil {
		event := events.NewSystemEvent(
			"library.monitoring.stopped",
			"Library Monitoring Stopped",
			fmt.Sprintf("Stopped monitoring library #%d", libraryID),
		)
		event.Data = map[string]interface{}{
			"library_id":      libraryID,
			"files_processed": monitored.FilesProcessed,
			"duration":        time.Since(monitored.StartTime).String(),
		}
		fm.eventBus.PublishAsync(event)
	}
	
	logger.Info("Stopped monitoring library", "library_id", libraryID)
	return nil
}

// GetMonitoringStatus returns the monitoring status for all libraries
func (fm *FileMonitor) GetMonitoringStatus() map[uint]*MonitoredLibrary {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	status := make(map[uint]*MonitoredLibrary)
	for id, lib := range fm.monitoredLibraries {
		status[id] = &MonitoredLibrary{
			ID:             lib.ID,
			Path:           lib.Path,
			Type:           lib.Type,
			LastScanJobID:  lib.LastScanJobID,
			StartTime:      lib.StartTime,
			FilesProcessed: lib.FilesProcessed,
			Status:         lib.Status,
		}
	}
	
	return status
}

// addRecursiveWatch adds watches for all subdirectories
func (fm *FileMonitor) addRecursiveWatch(rootPath string) error {
	return filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if info.IsDir() && path != rootPath {
			if err := fm.watcher.Add(path); err != nil {
				logger.Debug("Failed to add watch for subdirectory", "path", path, "error", err)
				// Continue with other directories
			}
		}
		
		return nil
	})
}

// watchEvents is the main event loop that processes file system events
func (fm *FileMonitor) watchEvents() {
	defer fm.wg.Done()
	
	logger.Info("File monitor event loop started")
	
	for {
		select {
		case event, ok := <-fm.watcher.Events:
			if !ok {
				logger.Info("File watcher events channel closed")
				return
			}
			
			fm.handleFileSystemEvent(event)
			
		case err, ok := <-fm.watcher.Errors:
			if !ok {
				logger.Info("File watcher errors channel closed")
				return
			}
			
			logger.Error("File watcher error", "error", err)
			
		case <-fm.ctx.Done():
			logger.Info("File monitor context cancelled")
			return
		}
	}
}

// handleFileSystemEvent processes a single file system event
func (fm *FileMonitor) handleFileSystemEvent(event fsnotify.Event) {
	// Find which library this event belongs to
	libraryID := fm.findLibraryForPath(event.Name)
	if libraryID == 0 {
		return // Event not in any monitored library
	}
	
	// Filter out non-media files and system files
	if !fm.isMediaFile(event.Name) {
		return
	}
	
	// Create file event
	fileEvent := FileEvent{
		Type:      event.Op,
		Path:      event.Name,
		LibraryID: libraryID,
		Timestamp: time.Now(),
	}
	
	// Queue for processing (with timeout to avoid blocking)
	select {
	case fm.eventQueue <- fileEvent:
		logger.Debug("Queued file event", "type", event.Op, "path", event.Name)
	case <-time.After(time.Second):
		logger.Warn("File event queue full, dropping event", "path", event.Name)
	}
	
	// Handle directory creation specially (need to add new watch)
	if event.Op&fsnotify.Create == fsnotify.Create {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if err := fm.watcher.Add(event.Name); err != nil {
				logger.Error("Failed to add watch for new directory", "path", event.Name, "error", err)
			} else {
				logger.Debug("Added watch for new directory", "path", event.Name)
			}
		}
	}
}

// processFileEvents processes queued file events with debouncing
func (fm *FileMonitor) processFileEvents() {
	defer fm.wg.Done()
	
	logger.Info("File event processor started")
	
	eventMap := make(map[string]FileEvent) // path -> latest event
	ticker := time.NewTicker(fm.debounceInterval)
	defer ticker.Stop()
	
	for {
		select {
		case event := <-fm.eventQueue:
			// Store/update the latest event for this path
			eventMap[event.Path] = event
			
		case <-ticker.C:
			// Process accumulated events
			if len(eventMap) > 0 {
				fm.processBatchedEvents(eventMap)
				eventMap = make(map[string]FileEvent) // Reset for next batch
			}
			
		case <-fm.ctx.Done():
			// Process any remaining events before shutdown
			if len(eventMap) > 0 {
				fm.processBatchedEvents(eventMap)
			}
			logger.Info("File event processor stopped")
			return
		}
	}
}

// processBatchedEvents processes a batch of debounced file events
func (fm *FileMonitor) processBatchedEvents(eventMap map[string]FileEvent) {
	logger.Debug("Processing file event batch", "count", len(eventMap))
	
	for path, event := range eventMap {
		if err := fm.processFileEvent(event); err != nil {
			logger.Error("Failed to process file event", "path", path, "error", err)
		}
	}
}

// processFileEvent processes a single file event
func (fm *FileMonitor) processFileEvent(event FileEvent) error {
	fm.mu.Lock()
	monitored, exists := fm.monitoredLibraries[event.LibraryID]
	if !exists {
		fm.mu.Unlock()
		return fmt.Errorf("library %d not monitored", event.LibraryID)
	}
	
	// Update status to processing
	monitored.Status = "processing"
	fm.mu.Unlock()
	
	defer func() {
		fm.mu.Lock()
		if monitored, exists := fm.monitoredLibraries[event.LibraryID]; exists {
			monitored.Status = "monitoring"
			monitored.FilesProcessed++
		}
		fm.mu.Unlock()
	}()
	
	// Process based on event type
	switch {
	case event.Type&fsnotify.Create == fsnotify.Create:
		return fm.fileProcessor.ProcessNewFile(event.Path, event.LibraryID)
		
	case event.Type&fsnotify.Write == fsnotify.Write:
		return fm.fileProcessor.ProcessModifiedFile(event.Path, event.LibraryID)
		
	case event.Type&fsnotify.Remove == fsnotify.Remove:
		return fm.fileProcessor.ProcessRemovedFile(event.Path, event.LibraryID)
		
	case event.Type&fsnotify.Rename == fsnotify.Rename:
		return fm.fileProcessor.ProcessRemovedFile(event.Path, event.LibraryID)
		
	default:
		logger.Debug("Ignoring file event", "type", event.Type, "path", event.Path)
		return nil
	}
}

// findLibraryForPath finds which library a file path belongs to
func (fm *FileMonitor) findLibraryForPath(filePath string) uint {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	
	for libraryID, monitored := range fm.monitoredLibraries {
		if strings.HasPrefix(filePath, monitored.Path) {
			return libraryID
		}
	}
	
	return 0 // Not found
}

// isMediaFile checks if a file is a media file worth processing
func (fm *FileMonitor) isMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	
	// Common media file extensions
	mediaExts := map[string]bool{
		// Audio
		".mp3": true, ".flac": true, ".wav": true, ".m4a": true, ".aac": true,
		".ogg": true, ".wma": true, ".aiff": true, ".ape": true, ".opus": true,
		
		// Video
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".wmv": true,
		".flv": true, ".webm": true, ".m4v": true, ".3gp": true, ".ts": true,
		
		// Image
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true,
		".tiff": true, ".webp": true, ".svg": true, ".raw": true,
	}
	
	return mediaExts[ext]
}

// ProcessNewFile handles new file creation
func (fp *FileProcessor) ProcessNewFile(filePath string, libraryID uint) error {
	logger.Debug("Processing new file", "path", filePath, "library_id", libraryID)
	
	// Check if file already exists in database
	var existingFile database.MediaFile
	err := fp.db.Where("path = ? AND library_id = ?", filePath, libraryID).First(&existingFile).Error
	if err == nil {
		logger.Debug("File already exists in database", "path", filePath)
		return nil // Already processed
	}
	
	// Process the file similar to how the scanner does it
	return fp.scanAndSaveFile(filePath, libraryID)
}

// ProcessModifiedFile handles file modifications
func (fp *FileProcessor) ProcessModifiedFile(filePath string, libraryID uint) error {
	logger.Debug("Processing modified file", "path", filePath, "library_id", libraryID)
	
	// Remove existing entry and re-process
	fp.db.Where("path = ? AND library_id = ?", filePath, libraryID).Delete(&database.MediaFile{})
	
	return fp.scanAndSaveFile(filePath, libraryID)
}

// ProcessRemovedFile handles file deletion
func (fp *FileProcessor) ProcessRemovedFile(filePath string, libraryID uint) error {
	logger.Debug("Processing removed file", "path", filePath, "library_id", libraryID)
	
	// Remove from database
	result := fp.db.Where("path = ? AND library_id = ?", filePath, libraryID).Delete(&database.MediaFile{})
	if result.Error != nil {
		return fmt.Errorf("failed to remove file from database: %w", result.Error)
	}
	
	if result.RowsAffected > 0 {
		logger.Info("Removed file from database", "path", filePath, "library_id", libraryID)
		
		// Emit file removed event
		if fp.eventBus != nil {
			event := events.NewSystemEvent(
				"media.file.removed",
				"Media File Removed",
				fmt.Sprintf("File removed: %s", filepath.Base(filePath)),
			)
			event.Data = map[string]interface{}{
				"file_path":  filePath,
				"library_id": libraryID,
			}
			fp.eventBus.PublishAsync(event)
		}
	}
	
	return nil
}

// scanAndSaveFile scans and saves a single file (similar to scanner logic)
func (fp *FileProcessor) scanAndSaveFile(filePath string, libraryID uint) error {
	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	
	// Create basic media file record
	mediaFile := &database.MediaFile{
		LibraryID: uint32(libraryID),
		Path:      filePath,
		SizeBytes: fileInfo.Size(),
		ScanJobID: nil, // Files discovered by monitoring don't belong to a specific scan job
		LastSeen:  time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Try to extract metadata using plugins if available
	if fp.pluginManager != nil {
		// This would use the same metadata extraction logic as the scanner
		// For now, we'll just save the basic file info
	}
	
	// Save to database
	if err := fp.db.Create(mediaFile).Error; err != nil {
		return fmt.Errorf("failed to save media file: %w", err)
	}
	
	logger.Info("Added new media file", "path", filePath, "library_id", libraryID, "size", fileInfo.Size())
	
	// Emit file added event
	if fp.eventBus != nil {
		event := events.NewSystemEvent(
			"media.file.added",
			"Media File Added",
			fmt.Sprintf("New file detected: %s", filepath.Base(filePath)),
		)
		event.Data = map[string]interface{}{
			"file_path":  filePath,
			"library_id": libraryID,
			"file_size":  fileInfo.Size(),
		}
		fp.eventBus.PublishAsync(event)
	}
	
	return nil
} 