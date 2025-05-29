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
	}
}

// Initialize initializes the library manager
func (lm *LibraryManager) Initialize() error {
	log.Println("INFO: Initializing library manager")
	
	// Load existing libraries from database
	if err := lm.loadLibraries(); err != nil {
		return fmt.Errorf("failed to load libraries: %w", err)
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
	// If cache is empty, try to reload from database
	if len(lm.libraries) == 0 {
		lm.mutex.RUnlock()
		
		// Reload libraries from database
		log.Println("INFO: Library cache is empty, reloading from database")
		if err := lm.loadLibraries(); err != nil {
			return nil, fmt.Errorf("failed to reload libraries: %w", err)
		}
		
		lm.mutex.RLock()
	}
	
	libraries := make([]*database.MediaLibrary, 0, len(lm.libraries))
	for _, library := range lm.libraries {
		libraries = append(libraries, library)
	}
	lm.mutex.RUnlock()
	
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
	
	// Note: LastScan and ScanDuration are now handled by the scanner module
	// This information should be retrieved from scan jobs if needed
	
	log.Printf("INFO: Generated stats for library %s: %d files, %d bytes", library.Path, stats.TotalFiles, stats.TotalSize)
	return stats, nil
}

// Shutdown gracefully shuts down the library manager
func (lm *LibraryManager) Shutdown(ctx context.Context) error {
	log.Println("INFO: Shutting down library manager")
	
	lm.initialized = false
	log.Println("INFO: Library manager shutdown complete")
	return nil
}
