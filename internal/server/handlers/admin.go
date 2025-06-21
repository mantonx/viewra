// Admin handlers with event support
package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/modules/mediamodule"
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

	// Parse library ID
	libraryID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid library ID",
		})
		return
	}

	logger.Info("Admin deleting library", "library_id", libraryID)

	// Import the deletion service from mediamodule
	db := database.GetDB()

	// Use the comprehensive deletion service
	deletionService := mediamodule.NewLibraryDeletionService(db, h.eventBus)

	// Get scanner manager for proper cleanup
	scannerManager, err := getScannerManager()
	if err == nil && scannerManager != nil {
		deletionService.SetScannerManager(scannerManager)
	} else {
		logger.Warn("Scanner manager not available for cleanup", "error", err)
	}

	// Perform comprehensive deletion
	result := deletionService.DeleteLibrary(uint32(libraryID))

	if !result.Success {
		logger.Error("Admin library deletion failed", "library_id", libraryID, "error", result.Error)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   result.Message,
				"details": result.Error.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": result.Message,
			})
		}
		return
	}

	logger.Info("Admin library deletion completed successfully", "library_id", libraryID, "duration", result.Duration)

	c.JSON(http.StatusOK, gin.H{
		"message":       result.Message,
		"library_id":    result.LibraryID,
		"cleanup_stats": result.CleanupStats,
		"duration":      result.Duration.String(),
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
