// Admin handlers with event support
package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
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
	
	// Delete the library
	result = db.Delete(&library)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete media library",
			"details": result.Error.Error(),
		})
		return
	}
	
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
