package mediamodule

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"gorm.io/gorm"
)

// LibraryManager manages media libraries
type LibraryManager struct {
	db          *gorm.DB
	eventBus    events.EventBus
	initialized bool
	mutex       sync.RWMutex
	
	// Library management
	libraries map[uint]*database.MediaLibrary
	scanners  map[uint]*LibraryScanner
}

// LibraryScanner represents a library scanner
type LibraryScanner struct {
	libraryID uint
	running   bool
	lastScan  time.Time
	mutex     sync.RWMutex
}

// LibraryStats represents library statistics
type LibraryStats struct {
	LibraryID    uint      `json:"library_id"`
	TotalFiles   int       `json:"total_files"`
	TotalSize    int64     `json:"total_size"`
	AudioFiles   int       `json:"audio_files"`
	VideoFiles   int       `json:"video_files"`
	ImageFiles   int       `json:"image_files"`
	LastScan     time.Time `json:"last_scan"`
	ScanDuration time.Duration `json:"scan_duration"`
}

// NewLibraryManager creates a new library manager
func NewLibraryManager(db *gorm.DB, eventBus events.EventBus) *LibraryManager {
	return &LibraryManager{
		db:        db,
		eventBus:  eventBus,
		libraries: make(map[uint]*database.MediaLibrary),
		scanners:  make(map[uint]*LibraryScanner),
	}
}

// Initialize initializes the library manager
func (lm *LibraryManager) Initialize() error {
	log.Println("INFO: Initializing library manager")
	
	// Load existing libraries from database
	if err := lm.loadLibraries(); err != nil {
		return fmt.Errorf("failed to load libraries: %w", err)
	}
	
	// Initialize scanners for existing libraries
	if err := lm.initializeScanners(); err != nil {
		return fmt.Errorf("failed to initialize scanners: %w", err)
	}
	
	lm.initialized = true
	log.Println("INFO: Library manager initialized successfully")
	return nil
}

// loadLibraries loads all libraries from the database
func (lm *LibraryManager) loadLibraries() error {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()
	
	var libraries []database.MediaLibrary
	if err := lm.db.Find(&libraries).Error; err != nil {
		return fmt.Errorf("failed to load libraries: %w", err)
	}
	
	for i := range libraries {
		lm.libraries[libraries[i].ID] = &libraries[i]
		log.Printf("INFO: Loaded library: %s (ID: %d)", libraries[i].Path, libraries[i].ID)
	}
	
	log.Printf("INFO: Loaded %d libraries", len(libraries))
	return nil
}

// initializeScanners initializes scanners for all libraries
func (lm *LibraryManager) initializeScanners() error {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()
	
	for id := range lm.libraries {
		lm.scanners[id] = &LibraryScanner{
			libraryID: id,
			running:   false,
			lastScan:  time.Time{},
		}
	}
	
	return nil
}

// CreateLibrary creates a new media library
func (lm *LibraryManager) CreateLibrary(name, path, libraryType string) (*database.MediaLibrary, error) {
	if !lm.initialized {
		return nil, fmt.Errorf("library manager not initialized")
	}
	
	library := &database.MediaLibrary{
		Path:      path,
		Type:      libraryType,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	if err := lm.db.Create(library).Error; err != nil {
		return nil, fmt.Errorf("failed to create library: %w", err)
	}
	
	lm.mutex.Lock()
	lm.libraries[library.ID] = library
	lm.scanners[library.ID] = &LibraryScanner{
		libraryID: library.ID,
		running:   false,
		lastScan:  time.Time{},
	}
	lm.mutex.Unlock()
	
	// Publish library created event
	if lm.eventBus != nil {
		event := events.NewSystemEvent(
			"library.created",
			"Library Created",
			fmt.Sprintf("Library '%s' created with ID %d", path, library.ID),
		)
		lm.eventBus.PublishAsync(event)
	}
	
	log.Printf("INFO: Created library: %s (ID: %d) at path: %s", path, library.ID, path)
	return library, nil
}

// GetLibrary retrieves a library by ID
func (lm *LibraryManager) GetLibrary(id uint) (*database.MediaLibrary, error) {
	if !lm.initialized {
		return nil, fmt.Errorf("library manager not initialized")
	}
	
	lm.mutex.RLock()
	library, exists := lm.libraries[id]
	lm.mutex.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("library not found")
	}
	
	return library, nil
}

// GetAllLibraries retrieves all libraries
func (lm *LibraryManager) GetAllLibraries() ([]*database.MediaLibrary, error) {
	if !lm.initialized {
		return nil, fmt.Errorf("library manager not initialized")
	}
	
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()
	
	libraries := make([]*database.MediaLibrary, 0, len(lm.libraries))
	for _, library := range lm.libraries {
		libraries = append(libraries, library)
	}
	
	return libraries, nil
}

// DeleteLibrary deletes a library and all its files
func (lm *LibraryManager) DeleteLibrary(id uint) error {
	if !lm.initialized {
		return fmt.Errorf("library manager not initialized")
	}
	
	lm.mutex.Lock()
	library, exists := lm.libraries[id]
	if !exists {
		lm.mutex.Unlock()
		return fmt.Errorf("library not found")
	}
	
	// Stop scanner if running
	if scanner, exists := lm.scanners[id]; exists {
		scanner.mutex.Lock()
		scanner.running = false
		scanner.mutex.Unlock()
		delete(lm.scanners, id)
	}
	
	delete(lm.libraries, id)
	lm.mutex.Unlock()
	
	// Delete all files in the library first
	if err := lm.db.Where("library_id = ?", id).Delete(&database.MediaFile{}).Error; err != nil {
		log.Printf("WARNING: Failed to delete media files for library %d: %v", id, err)
	}
	
	// Delete the library
	if err := lm.db.Delete(&database.MediaLibrary{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete library: %w", err)
	}
	
	// Publish library deleted event
	if lm.eventBus != nil {
		event := events.NewSystemEvent(
			"library.deleted",
			"Library Deleted",
			fmt.Sprintf("Library '%s' (ID: %d) deleted", library.Path, id),
		)
		lm.eventBus.PublishAsync(event)
	}
	
	log.Printf("INFO: Deleted library: %s (ID: %d)", library.Path, id)
	return nil
}

// GetLibraryStats retrieves statistics for a library
func (lm *LibraryManager) GetLibraryStats(id uint) (*LibraryStats, error) {
	if !lm.initialized {
		return nil, fmt.Errorf("library manager not initialized")
	}
	
	library, err := lm.GetLibrary(id)
	if err != nil {
		return nil, err
	}
	
	stats := &LibraryStats{
		LibraryID: id,
	}
	
	// Count total files
	var totalFiles int64
	if err := lm.db.Model(&database.MediaFile{}).Where("library_id = ?", id).Count(&totalFiles).Error; err != nil {
		log.Printf("WARNING: Failed to count total files for library %d: %v", id, err)
	}
	stats.TotalFiles = int(totalFiles)
	
	// Calculate total size
	var totalSize sql.NullInt64
	if err := lm.db.Model(&database.MediaFile{}).Where("library_id = ?", id).Select("SUM(size)").Scan(&totalSize).Error; err != nil {
		log.Printf("WARNING: Failed to calculate total size for library %d: %v", id, err)
	}
	if totalSize.Valid {
		stats.TotalSize = totalSize.Int64
	}
	
	// Count by file types
	var audioFiles int64
	if err := lm.db.Model(&database.MediaFile{}).Where("library_id = ? AND file_type LIKE 'audio/%'", id).Count(&audioFiles).Error; err != nil {
		log.Printf("WARNING: Failed to count audio files for library %d: %v", id, err)
	}
	stats.AudioFiles = int(audioFiles)
	
	var videoFiles int64
	if err := lm.db.Model(&database.MediaFile{}).Where("library_id = ? AND file_type LIKE 'video/%'", id).Count(&videoFiles).Error; err != nil {
		log.Printf("WARNING: Failed to count video files for library %d: %v", id, err)
	}
	stats.VideoFiles = int(videoFiles)
	
	var imageFiles int64
	if err := lm.db.Model(&database.MediaFile{}).Where("library_id = ? AND file_type LIKE 'image/%'", id).Count(&imageFiles).Error; err != nil {
		log.Printf("WARNING: Failed to count image files for library %d: %v", id, err)
	}
	stats.ImageFiles = int(imageFiles)
	
	// Get last scan time
	if scanner, exists := lm.scanners[id]; exists {
		scanner.mutex.RLock()
		stats.LastScan = scanner.lastScan
		scanner.mutex.RUnlock()
	}
	
	log.Printf("INFO: Generated stats for library %s: %d files, %d bytes", library.Path, stats.TotalFiles, stats.TotalSize)
	return stats, nil
}

// ScanLibrary scans a library for media files
func (lm *LibraryManager) ScanLibrary(id uint) error {
	if !lm.initialized {
		return fmt.Errorf("library manager not initialized")
	}
	
	library, err := lm.GetLibrary(id)
	if err != nil {
		return err
	}
	
	scanner, exists := lm.scanners[id]
	if !exists {
		return fmt.Errorf("scanner not found for library %d", id)
	}
	
	scanner.mutex.Lock()
	if scanner.running {
		scanner.mutex.Unlock()
		return fmt.Errorf("scan already running for library %d", id)
	}
	scanner.running = true
	scanner.mutex.Unlock()
	
	// Start scan in background
	go func() {
		defer func() {
			scanner.mutex.Lock()
			scanner.running = false
			scanner.lastScan = time.Now()
			scanner.mutex.Unlock()
		}()
		
		log.Printf("INFO: Starting scan for library: %s (ID: %d)", library.Path, id)
		
		// Publish scan started event
		if lm.eventBus != nil {
			event := events.NewSystemEvent(
				"library.scan.started",
				"Library Scan Started",
				fmt.Sprintf("Started scanning library '%s' (ID: %d)", library.Path, id),
			)
			lm.eventBus.PublishAsync(event)
		}
		
		// TODO: Implement actual file scanning logic
		// This would involve:
		// 1. Walking the file system
		// 2. Identifying media files
		// 3. Extracting metadata
		// 4. Updating database
		
		time.Sleep(1 * time.Second) // Simulate scan time
		
		// Publish scan completed event
		if lm.eventBus != nil {
			event := events.NewSystemEvent(
				"library.scan.completed",
				"Library Scan Completed",
				fmt.Sprintf("Completed scanning library '%s' (ID: %d)", library.Path, id),
			)
			lm.eventBus.PublishAsync(event)
		}
		
		log.Printf("INFO: Completed scan for library: %s (ID: %d)", library.Path, id)
	}()
	
	return nil
}

// IsScanning checks if a library is currently being scanned
func (lm *LibraryManager) IsScanning(id uint) bool {
	scanner, exists := lm.scanners[id]
	if !exists {
		return false
	}
	
	scanner.mutex.RLock()
	defer scanner.mutex.RUnlock()
	return scanner.running
}

// Shutdown gracefully shuts down the library manager
func (lm *LibraryManager) Shutdown(ctx context.Context) error {
	log.Println("INFO: Shutting down library manager")
	
	// Stop all running scanners
	lm.mutex.Lock()
	for id, scanner := range lm.scanners {
		scanner.mutex.Lock()
		if scanner.running {
			log.Printf("INFO: Stopping scanner for library %d", id)
			scanner.running = false
		}
		scanner.mutex.Unlock()
	}
	lm.mutex.Unlock()
	
	lm.initialized = false
	log.Println("INFO: Library manager shutdown complete")
	return nil
}
