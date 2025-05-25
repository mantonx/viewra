package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/viewra/internal/database"
)

// GetMediaLibraries retrieves all configured media libraries
func GetMediaLibraries(c *gin.Context) {
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
func CreateMediaLibrary(c *gin.Context) {
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
	
	c.JSON(http.StatusCreated, gin.H{
		"library": library,
		"message": "Media library created successfully",
	})
}

// DeleteMediaLibrary removes a media library configuration
func DeleteMediaLibrary(c *gin.Context) {
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
	
	// Delete the library
	result = db.Delete(&library)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete media library",
			"details": result.Error.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Media library deleted successfully",
		"id":      id,
	})
}
