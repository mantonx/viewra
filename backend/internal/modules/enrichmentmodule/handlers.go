package enrichmentmodule

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// =============================================================================
// HTTP API HANDLERS
// =============================================================================
// These handlers provide HTTP REST API access to enrichment management
// functionality for web clients and internal management tools.

// RegisterRoutes registers HTTP routes for enrichment management
func (m *Module) RegisterRoutes(r *gin.Engine) {
	if !m.enabled {
		return
	}
	
	api := r.Group("/api")
	enrichment := api.Group("/enrichment")
	{
		enrichment.GET("/status/:mediaFileId", m.GetEnrichmentStatusHandler)
		enrichment.POST("/apply/:mediaFileId/:fieldName/:sourceName", m.ForceApplyEnrichmentHandler)
		enrichment.GET("/sources", m.GetEnrichmentSourcesHandler)
		enrichment.PUT("/sources/:sourceName", m.UpdateEnrichmentSourceHandler)
		enrichment.GET("/jobs", m.GetEnrichmentJobsHandler)
		enrichment.POST("/jobs/:mediaFileId", m.TriggerEnrichmentJobHandler)
	}
	
	log.Printf("✅ Registered enrichment module HTTP routes")
}

// GetEnrichmentStatusHandler returns enrichment status for a media file
func (m *Module) GetEnrichmentStatusHandler(c *gin.Context) {
	mediaFileID := c.Param("mediaFileId")
	if mediaFileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Media file ID is required",
		})
		return
	}

	status, err := m.GetEnrichmentStatus(mediaFileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get enrichment status",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": status,
	})
}

// ForceApplyEnrichmentHandler manually applies enrichment for a specific field
func (m *Module) ForceApplyEnrichmentHandler(c *gin.Context) {
	mediaFileID := c.Param("mediaFileId")
	fieldName := c.Param("fieldName")
	sourceName := c.Param("sourceName")

	if mediaFileID == "" || fieldName == "" || sourceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Media file ID, field name, and source name are required",
		})
		return
	}

	if err := m.ForceApplyEnrichment(mediaFileID, fieldName, sourceName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to apply enrichment",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Enrichment applied successfully",
		"media_file_id": mediaFileID,
		"field_name": fieldName,
		"source_name": sourceName,
	})
}

// GetEnrichmentSourcesHandler returns all enrichment sources
func (m *Module) GetEnrichmentSourcesHandler(c *gin.Context) {
	var sources []EnrichmentSource
	if err := m.db.Find(&sources).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch enrichment sources",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sources": sources,
	})
}

// UpdateEnrichmentSourceHandler updates an enrichment source configuration
func (m *Module) UpdateEnrichmentSourceHandler(c *gin.Context) {
	sourceName := c.Param("sourceName")
	if sourceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Source name is required",
		})
		return
	}

	var updateData struct {
		Priority int  `json:"priority"`
		Enabled  bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Find and update the source
	var source EnrichmentSource
	if err := m.db.Where("name = ?", sourceName).First(&source).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Enrichment source not found",
		})
		return
	}

	source.Priority = updateData.Priority
	source.Enabled = updateData.Enabled

	if err := m.db.Save(&source).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update enrichment source",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Enrichment source updated successfully",
		"source":  source,
	})
}

// GetEnrichmentJobsHandler returns enrichment jobs with optional filtering
func (m *Module) GetEnrichmentJobsHandler(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	status := c.Query("status")
	mediaFileID := c.Query("media_file_id")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	query := m.db.Model(&EnrichmentJob{}).Order("created_at DESC")

	if status != "" {
		query = query.Where("status = ?", status)
	}

	if mediaFileID != "" {
		query = query.Where("media_file_id = ?", mediaFileID)
	}

	var jobs []EnrichmentJob
	if err := query.Limit(limit).Offset(offset).Find(&jobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch enrichment jobs",
			"details": err.Error(),
		})
		return
	}

	var total int64
	query.Count(&total)

	c.JSON(http.StatusOK, gin.H{
		"jobs":   jobs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// TriggerEnrichmentJobHandler manually triggers an enrichment job for a media file
func (m *Module) TriggerEnrichmentJobHandler(c *gin.Context) {
	mediaFileID := c.Param("mediaFileId")
	if mediaFileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Media file ID is required",
		})
		return
	}

	// Create a new enrichment job
	job := EnrichmentJob{
		MediaFileID: mediaFileID,
		JobType:     "apply_enrichment",
		Status:      "pending",
	}

	if err := m.db.Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create enrichment job",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Enrichment job created successfully",
		"job":     job,
	})
} 