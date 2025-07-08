package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
)

// GetProviders handles GET /api/v1/transcoding/providers
//
// Returns information about all available transcoding providers.
//
// Response:
//
//	{
//	  "providers": [
//	    {
//	      "id": "string",
//	      "name": "string",
//	      "description": "string",
//	      "version": "string",
//	      "priority": 0
//	    }
//	  ]
//	}
func (h *APIHandler) GetProviders(c *gin.Context) {
	providers := h.service.GetProviders()
	c.JSON(http.StatusOK, gin.H{
		"providers": providers,
	})
}

// GetProvider handles GET /api/v1/transcoding/providers/:id
//
// Returns detailed information about a specific provider.
//
// URL parameters:
//   - id: Provider ID
//
// Response:
//
//	{
//	  "id": "string",
//	  "name": "string",
//	  "description": "string",
//	  "version": "string",
//	  "priority": 0,
//	  "capabilities": {...}
//	}
func (h *APIHandler) GetProvider(c *gin.Context) {
	providerID := c.Param("id")
	if providerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Provider ID is required",
		})
		return
	}

	provider, err := h.service.GetProvider(providerID)
	if err != nil {
		logger.Error("Failed to get provider: %v", err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Provider not found",
		})
		return
	}

	// Get provider info
	info := provider.GetInfo()
	
	c.JSON(http.StatusOK, gin.H{
		"id":           info.ID,
		"name":         info.Name,
		"description":  info.Description,
		"version":      info.Version,
		"priority":     info.Priority,
		"capabilities": map[string]interface{}{
			"hardwareAcceleration": false, // TODO: Get from provider
			"supportedCodecs":      []string{"h264", "h265", "vp9"},
			"supportedContainers":  []string{"mp4", "mkv", "webm"},
		},
	})
}