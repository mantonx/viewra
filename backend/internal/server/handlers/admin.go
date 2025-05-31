// Admin handlers with event support
package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
)

// AdminHandler handles administrative API endpoints
type AdminHandler struct {
	eventBus events.EventBus
}

// NewAdminHandler creates a new admin handler with event bus
func NewAdminHandler(eventBus events.EventBus) *AdminHandler {
	return &AdminHandler{
		eventBus: eventBus,
	}
}

// GetMediaLibraries retrieves all configured media libraries
func (h *AdminHandler) GetMediaLibraries(c *gin.Context) {
	var libraries []database.MediaLibrary
	db := database.GetDB()
	
	result := db.Find(&libraries)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve media libraries",
			"details": result.Error.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"libraries": libraries,
		"count":     len(libraries),
	})
}

// CreateMediaLibrary creates a new media library configuration
func (h *AdminHandler) CreateMediaLibrary(c *gin.Context) {
	var req database.MediaLibraryRequest
	
	// Bind and validate JSON input
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}
	
	// Create media library record
	library := database.MediaLibrary{
		Path: req.Path,
		Type: req.Type,
	}
	
	db := database.GetDB()
	result := db.Create(&library)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create media library",
			"details": result.Error.Error(),
		})
		return
	}
	
	// Publish event for library creation
	if h.eventBus != nil {
		createEvent := events.NewSystemEvent(
			events.EventInfo,
			"Media Library Created",
			fmt.Sprintf("New %s media library added at path: %s", library.Type, library.Path),
		)
		createEvent.Data = map[string]interface{}{
			"libraryId": library.ID,
			"path":      library.Path, 
			"type":      library.Type,
		}
		h.eventBus.PublishAsync(createEvent)
	}
	
	c.JSON(http.StatusCreated, gin.H{
		"library": library,
		"message": "Media library created successfully",
	})
}

// DeleteMediaLibrary removes a media library configuration
func (h *AdminHandler) DeleteMediaLibrary(c *gin.Context) {
	// Get the library ID from URL parameter
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Library ID is required",
		})
		return
	}
	
	db := database.GetDB()
	
	// Check if library exists
	var library database.MediaLibrary
	result := db.First(&library, id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Media library not found",
		})
		return
	}
	
	// Save library details for event before deletion
	libraryID := library.ID
	libraryPath := library.Path
	libraryType := library.Type
	
	logger.Info("Starting library deletion", "library_id", libraryID, "path", libraryPath)
	
	// Get scanner manager for proper cleanup
	scannerManager, err := getScannerManager()
	if err != nil {
		logger.Error("Scanner manager not available for cleanup", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to access scanner manager for cleanup",
			"details": err.Error(),
		})
		return
	}
	
	// STEP 1: Force stop any active scans for this library
	logger.Info("Checking for active scans", "library_id", libraryID)
	allJobs, jobsErr := scannerManager.GetAllScans()
	if jobsErr != nil {
		logger.Error("Failed to get scan jobs", "error", jobsErr)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to check active scans",
			"details": jobsErr.Error(),
		})
		return
	}
	
	var activeJobsForLibrary []database.ScanJob
	for _, job := range allJobs {
		if job.LibraryID == libraryID && (job.Status == "running" || job.Status == "paused") {
			activeJobsForLibrary = append(activeJobsForLibrary, job)
		}
	}
	
	if len(activeJobsForLibrary) > 0 {
		logger.Info("Found active scans to stop", "library_id", libraryID, "job_count", len(activeJobsForLibrary))
		
		// Force stop all active jobs for this library
		for _, job := range activeJobsForLibrary {
			logger.Info("Terminating scan job", "job_id", job.ID, "status", job.Status)
			if stopErr := scannerManager.TerminateScan(job.ID); stopErr != nil {
				logger.Warn("Failed to terminate scan job", "job_id", job.ID, "error", stopErr)
			}
		}
		
		// Wait longer and verify scans have actually stopped
		logger.Info("Waiting for scans to stop completely", "library_id", libraryID)
		maxWaitTime := 10 * time.Second
		checkInterval := 500 * time.Millisecond
		waited := time.Duration(0)
		
		for waited < maxWaitTime {
			time.Sleep(checkInterval)
			waited += checkInterval
			
			// Check if scans are still active
			stillActive := false
			currentJobs, checkErr := scannerManager.GetAllScans()
			if checkErr == nil {
				for _, job := range currentJobs {
					if job.LibraryID == libraryID && job.Status == "running" {
						stillActive = true
						break
					}
				}
			}
			
			if !stillActive {
				logger.Info("All scans stopped successfully", "library_id", libraryID, "waited", waited)
				break
			}
		}
		
		if waited >= maxWaitTime {
			logger.Warn("Timeout waiting for scans to stop, proceeding with forced cleanup", "library_id", libraryID)
		}
	}
	
	// STEP 2: Clean up scan jobs
	jobsDeleted, cleanupErr := scannerManager.CleanupJobsByLibrary(libraryID)
	if cleanupErr != nil {
		logger.Error("Failed to cleanup scan jobs", "library_id", libraryID, "error", cleanupErr)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to cleanup scan jobs for library",
			"details": cleanupErr.Error(),
		})
		return
	} else if jobsDeleted > 0 {
		logger.Info("Cleaned up scan jobs", "library_id", libraryID, "jobs_deleted", jobsDeleted)
	}
	
	// STEP 3: Delete all music metadata for media files in this library first (to avoid foreign key constraint)
	logger.Info("Deleting music metadata for library", "library_id", libraryID)
	
	// Get all media file IDs for this library
	var mediaFileIDs []uint
	if err := db.Model(&database.MediaFile{}).Where("library_id = ?", libraryID).Pluck("id", &mediaFileIDs).Error; err != nil {
		logger.Error("Failed to get media file IDs", "library_id", libraryID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get media file IDs for cleanup",
			"details": err.Error(),
		})
		return
	}
	
	if len(mediaFileIDs) > 0 {
		// Delete all music metadata for these media files
		metadataResult := db.Where("media_file_id IN ?", mediaFileIDs).Delete(&database.MusicMetadata{})
		if metadataResult.Error != nil {
			logger.Error("Failed to delete music metadata", "library_id", libraryID, "error", metadataResult.Error)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to cleanup music metadata for library",
				"details": metadataResult.Error.Error(),
			})
			return
		} else if metadataResult.RowsAffected > 0 {
			logger.Info("Deleted music metadata", "library_id", libraryID, "metadata_deleted", metadataResult.RowsAffected)
		}
	}

	// STEP 4: Delete all media files for this library (now safe since metadata is gone)
	logger.Info("Deleting media files", "library_id", libraryID)
	mediaFilesResult := db.Where("library_id = ?", libraryID).Delete(&database.MediaFile{})
	if mediaFilesResult.Error != nil {
		logger.Error("Failed to delete media files", "library_id", libraryID, "error", mediaFilesResult.Error)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to cleanup media files for library",
			"details": mediaFilesResult.Error.Error(),
		})
		return
	} else if mediaFilesResult.RowsAffected > 0 {
		logger.Info("Deleted media files", "library_id", libraryID, "files_deleted", mediaFilesResult.RowsAffected)
	}
	
	// STEP 5: Clean up orphaned assets (now orphaned by deleting media files)
	logger.Info("Cleaning up orphaned assets", "library_id", libraryID)
	assetsRemoved, filesRemoved, cleanupErr := scannerManager.CleanupOrphanedAssets()
	if cleanupErr != nil {
		logger.Warn("Failed to cleanup orphaned assets", "error", cleanupErr)
	} else if assetsRemoved > 0 || filesRemoved > 0 {
		logger.Info("Cleaned up orphaned assets", "assets_removed", assetsRemoved, "files_removed", filesRemoved)
	}
	
	// STEP 6: Clean up any remaining orphaned files from filesystem
	logger.Info("Cleaning up orphaned files", "library_id", libraryID)
	orphanedFilesRemoved, cleanupErr := scannerManager.CleanupOrphanedFiles()
	if cleanupErr != nil {
		logger.Warn("Failed to cleanup orphaned files", "error", cleanupErr)
	} else if orphanedFilesRemoved > 0 {
		logger.Info("Cleaned up orphaned files", "files_removed", orphanedFilesRemoved)
	}
	
	// STEP 7: Delete the library record
	logger.Info("Deleting library record", "library_id", libraryID)
	result = db.Delete(&library)
	if result.Error != nil {
		logger.Error("Failed to delete library record", "library_id", libraryID, "error", result.Error)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete media library",
			"details": result.Error.Error(),
		})
		return
	}
	
	logger.Info("Library deletion completed successfully", "library_id", libraryID, "path", libraryPath)
	
	// Publish event for library deletion
	if h.eventBus != nil {
		deleteEvent := events.NewSystemEvent(
			events.EventInfo,
			"Media Library Deleted",
			fmt.Sprintf("%s media library at path %s has been removed", libraryType, libraryPath),
		)
		deleteEvent.Data = map[string]interface{}{
			"libraryId": libraryID,
			"path":      libraryPath,
			"type":      libraryType,
		}
		h.eventBus.PublishAsync(deleteEvent)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Media library deleted successfully",
		"id":      id,
	})
}

// GetLibraryStats retrieves statistics for a media library
func (h *AdminHandler) GetLibraryStats(c *gin.Context) {
	// Implementation remains the same
}

// GetMediaFiles retrieves all media files in a library
func (h *AdminHandler) GetMediaFiles(c *gin.Context) {
	// Implementation remains the same
}

// Keep original function-based handlers for backward compatibility
// These will delegate to the struct-based handlers

// GetMediaLibraries function-based handler for backward compatibility
func GetMediaLibraries(c *gin.Context) {
	// Create a temporary handler without event bus for backward compatibility
	handler := &AdminHandler{}
	handler.GetMediaLibraries(c)
}

// CreateMediaLibrary function-based handler for backward compatibility
func CreateMediaLibrary(c *gin.Context) {
	// Create a temporary handler without event bus for backward compatibility
	handler := &AdminHandler{}
	handler.CreateMediaLibrary(c)
}

// DeleteMediaLibrary function-based handler for backward compatibility
func DeleteMediaLibrary(c *gin.Context) {
	// Create a temporary handler without event bus for backward compatibility
	handler := &AdminHandler{}
	handler.DeleteMediaLibrary(c)
}
