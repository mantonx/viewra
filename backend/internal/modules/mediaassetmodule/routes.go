package mediaassetmodule

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// createAsset handles POST /api/assets/
func (m *Module) createAsset(c *gin.Context) {
	var request AssetRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	response, err := m.manager.SaveAsset(&request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to save asset",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response)
}

// getAsset handles GET /api/assets/:id
func (m *Module) getAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid asset ID",
		})
		return
	}

	response, err := m.manager.GetAsset(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Asset not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// getAssetData handles GET /api/assets/:id/data
func (m *Module) getAssetData(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid asset ID",
		})
		return
	}

	// Parse quality parameter (default to 100% for backend)
	qualityStr := c.DefaultQuery("quality", "100")
	quality, err := strconv.Atoi(qualityStr)
	if err != nil || quality < 1 || quality > 100 {
		quality = 100 // Default to 100% quality
	}

	// Get asset metadata first
	asset, err := m.manager.GetAsset(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Asset not found",
			"details": err.Error(),
		})
		return
	}

	// Get asset data with quality adjustment
	data, mimeType, err := m.manager.GetAssetDataWithQuality(uint(id), quality)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve asset data",
			"details": err.Error(),
		})
		return
	}

	// Set appropriate headers
	c.Header("Content-Type", mimeType)
	c.Header("Content-Length", strconv.Itoa(len(data)))
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 24 hours
	
	// Add quality info to headers for debugging
	c.Header("X-Image-Quality", strconv.Itoa(quality))
	c.Header("X-Original-MimeType", asset.MimeType)
	
	// Stream the data
	c.Data(http.StatusOK, mimeType, data)
}

// updateAsset handles PUT /api/assets/:id
func (m *Module) updateAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid asset ID",
		})
		return
	}

	var request AssetRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	response, err := m.manager.UpdateAsset(uint(id), &request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update asset",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// deleteAsset handles DELETE /api/assets/:id
func (m *Module) deleteAsset(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid asset ID",
		})
		return
	}

	err = m.manager.RemoveAsset(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete asset",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Asset deleted successfully",
	})
}

// listAssets handles GET /api/assets/
func (m *Module) listAssets(c *gin.Context) {
	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	category := c.Query("category")
	assetType := c.Query("type")
	mediaFileIDStr := c.Query("media_file_id")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	filter := &AssetFilter{
		Limit:  limit,
		Offset: offset,
	}

	if category != "" {
		filter.Category = AssetCategory(category)
	}

	if assetType != "" {
		filter.Type = AssetType(assetType)
	}

	if mediaFileIDStr != "" {
		mediaFileID, err := strconv.ParseUint(mediaFileIDStr, 10, 32)
		if err == nil {
			filter.MediaFileID = uint(mediaFileID)
		}
	}

	// If category is specified, use category-specific query
	if filter.Category != "" {
		assets, err := m.manager.GetAssetsByCategory(filter.Category, filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve assets",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"assets": assets,
			"count":  len(assets),
			"limit":  limit,
			"offset": offset,
		})
		return
	}

	// For now, return a simple message as we don't have a general list all function
	c.JSON(http.StatusOK, gin.H{
		"message": "Use category parameter to list assets by category",
		"limit":   limit,
		"offset":  offset,
	})
}

// getAssetsByMediaFile handles GET /api/assets/media/:media_id
func (m *Module) getAssetsByMediaFile(c *gin.Context) {
	mediaIDStr := c.Param("media_id")
	mediaID, err := strconv.ParseUint(mediaIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid media file ID",
		})
		return
	}

	assetType := c.Query("type")
	
	var assetTypeFilter AssetType
	if assetType != "" {
		assetTypeFilter = AssetType(assetType)
	}

	assets, err := m.manager.GetAssetsByMediaFile(uint(mediaID), assetTypeFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve assets",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"assets":       assets,
		"count":        len(assets),
		"media_file_id": uint(mediaID),
	})
}

// getAssetsByCategory handles GET /api/assets/category/:category
func (m *Module) getAssetsByCategory(c *gin.Context) {
	categoryStr := c.Param("category")
	category := AssetCategory(categoryStr)

	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	assetType := c.Query("type")
	mediaFileIDStr := c.Query("media_file_id")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 1000 {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	filter := &AssetFilter{
		Limit:  limit,
		Offset: offset,
	}

	if assetType != "" {
		filter.Type = AssetType(assetType)
	}

	if mediaFileIDStr != "" {
		mediaFileID, err := strconv.ParseUint(mediaFileIDStr, 10, 32)
		if err == nil {
			filter.MediaFileID = uint(mediaFileID)
		}
	}

	assets, err := m.manager.GetAssetsByCategory(category, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve assets",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"assets":   assets,
		"count":    len(assets),
		"category": categoryStr,
		"limit":    limit,
		"offset":   offset,
	})
}

// getAssetByHash handles GET /api/assets/hash/:hash
func (m *Module) getAssetByHash(c *gin.Context) {
	hash := c.Param("hash")
	if hash == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Hash parameter is required",
		})
		return
	}

	asset, err := m.manager.GetAssetByHash(hash)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Asset not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, asset)
}

// deleteAssetsByMediaFile handles DELETE /api/assets/media/:media_id
func (m *Module) deleteAssetsByMediaFile(c *gin.Context) {
	mediaIDStr := c.Param("media_id")
	mediaID, err := strconv.ParseUint(mediaIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid media file ID",
		})
		return
	}

	err = m.manager.RemoveAssetsByMediaFile(uint(mediaID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete assets",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Assets deleted successfully",
		"media_file_id": uint(mediaID),
	})
}

// getStats handles GET /api/assets/stats
func (m *Module) getStats(c *gin.Context) {
	stats, err := m.manager.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve asset statistics",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// validateIntegrity handles POST /api/assets/validate
func (m *Module) validateIntegrity(c *gin.Context) {
	err := m.manager.ValidateAssetIntegrity()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Asset integrity validation failed",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Asset integrity validation completed successfully",
	})
}

// getHealth handles GET /api/assets/health
func (m *Module) getHealth(c *gin.Context) {
	health := gin.H{
		"status": "healthy",
		"module": m.name,
		"version": m.version,
		"initialized": m.initialized,
	}

	if m.manager != nil {
		health["manager_initialized"] = m.manager.initialized
		health["root_path"] = m.manager.pathUtil.GetRootPath()
	}

	c.JSON(http.StatusOK, health)
}

// getStatus handles GET /api/assets/status
func (m *Module) getStatus(c *gin.Context) {
	status := gin.H{
		"id":          m.id,
		"name":        m.name,
		"version":     m.version,
		"core":        m.core,
		"initialized": m.initialized,
	}

	if m.manager != nil {
		status["manager_initialized"] = m.manager.initialized
		status["root_path"] = m.manager.pathUtil.GetRootPath()

		// Get basic stats
		if stats, err := m.manager.GetStats(); err == nil {
			status["total_assets"] = stats.TotalAssets
			status["total_size"] = stats.TotalSize
		}
	}

	c.JSON(http.StatusOK, status)
} 