package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
	plugins "github.com/mantonx/viewra/sdk"
)

// APIHandler handles HTTP requests for the transcoding module
type APIHandler struct {
	service TranscodingAPIService
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(service TranscodingAPIService) *APIHandler {
	return &APIHandler{
		service: service,
	}
}

// StartTranscode handles POST /api/v1/transcoding/transcode
// Initiates a new transcoding session
func (h *APIHandler) StartTranscode(c *gin.Context) {
	var req plugins.TranscodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate required fields
	if req.MediaID == "" || req.Container == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mediaId and container are required"})
		return
	}

	// Start transcoding
	handle, err := h.service.StartTranscode(c.Request.Context(), req)
	if err != nil {
		logger.Error("Failed to start transcode", "error", err, "mediaId", req.MediaID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessionId": handle.SessionID,
		"status":    handle.Status,
		"provider":  handle.Provider,
		"startTime": handle.StartTime,
	})
}

// StopTranscode handles DELETE /api/v1/transcoding/transcode/:sessionId
// Stops an active transcoding session
func (h *APIHandler) StopTranscode(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId is required"})
		return
	}

	err := h.service.StopTranscode(sessionID)
	if err != nil {
		logger.Error("Failed to stop transcode", "error", err, "sessionId", sessionID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Transcoding stopped"})
}

// GetProgress handles GET /api/v1/transcoding/progress/:sessionId
// Returns the progress of a transcoding session
func (h *APIHandler) GetProgress(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId is required"})
		return
	}

	progress, err := h.service.GetProgress(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, progress)
}

// ListSessions handles GET /api/v1/transcoding/sessions
// Returns all active transcoding sessions
func (h *APIHandler) ListSessions(c *gin.Context) {
	sessions := h.service.GetAllSessions()

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// GetSession handles GET /api/v1/transcoding/sessions/:sessionId
// Returns details of a specific transcoding session
func (h *APIHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId is required"})
		return
	}

	session, err := h.service.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// ListProviders handles GET /api/v1/transcoding/providers
// Returns all available transcoding providers
func (h *APIHandler) ListProviders(c *gin.Context) {
	providers := h.service.GetProviders()

	c.JSON(http.StatusOK, gin.H{
		"providers": providers,
		"count":     len(providers),
	})
}

// GetProvider handles GET /api/v1/transcoding/providers/:providerId
// Returns details of a specific provider
func (h *APIHandler) GetProvider(c *gin.Context) {
	providerID := c.Param("providerId")
	if providerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "providerId is required"})
		return
	}

	provider, err := h.service.GetProvider(providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
		return
	}

	info := provider.GetInfo()
	c.JSON(http.StatusOK, info)
}

// GetProviderFormats handles GET /api/v1/transcoding/providers/:providerId/formats
// Returns supported formats for a specific provider
func (h *APIHandler) GetProviderFormats(c *gin.Context) {
	providerID := c.Param("providerId")
	if providerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "providerId is required"})
		return
	}

	provider, err := h.service.GetProvider(providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
		return
	}

	formats := provider.GetSupportedFormats()
	c.JSON(http.StatusOK, gin.H{
		"formats": formats,
		"count":   len(formats),
	})
}

// GetPipelineStatus handles GET /api/v1/transcoding/pipeline/status
// Returns the status of the pipeline provider
func (h *APIHandler) GetPipelineStatus(c *gin.Context) {
	status := h.service.GetPipelineStatus()
	c.JSON(http.StatusOK, status)
}

// GetContentHashStats handles GET /api/v1/transcoding/content/stats
// Returns statistics about content hash coverage
func (h *APIHandler) GetContentHashStats(c *gin.Context) {
	migrationService := h.service.GetMigrationService()
	if migrationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Migration service not available"})
		return
	}

	stats, err := migrationService.GetContentHashStats()
	if err != nil {
		logger.Error("Failed to get content hash stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ListSessionsWithoutContentHash handles GET /api/v1/transcoding/content/sessions-without-hash
// Returns sessions that have completed but don't have content hashes
func (h *APIHandler) ListSessionsWithoutContentHash(c *gin.Context) {
	migrationService := h.service.GetMigrationService()
	if migrationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Migration service not available"})
		return
	}

	// Get limit from query parameter
	limit := 50 // Default limit
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := parseIntParam(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	sessions, err := migrationService.ListSessionsWithoutContentHash(limit)
	if err != nil {
		logger.Error("Failed to list sessions without content hash", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"count":    len(sessions),
		"limit":    limit,
	})
}

// MigrateSessionToContentHash handles POST /api/v1/transcoding/content/migrate/:sessionId
// Migrates a session to use content-addressable URLs
func (h *APIHandler) MigrateSessionToContentHash(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sessionId is required"})
		return
	}

	migrationService := h.service.GetMigrationService()
	if migrationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Migration service not available"})
		return
	}

	err := migrationService.MigrateSessionToContentHash(sessionID)
	if err != nil {
		logger.Error("Failed to migrate session", "error", err, "sessionId", sessionID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session migrated successfully"})
}

// CleanupOldSessions handles POST /api/v1/transcoding/content/cleanup
// Cleans up old session directories after content has been migrated
func (h *APIHandler) CleanupOldSessions(c *gin.Context) {
	migrationService := h.service.GetMigrationService()
	if migrationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Migration service not available"})
		return
	}

	// Get olderThanDays from request body or query parameter
	olderThanDays := 30 // Default 30 days
	if daysStr := c.Query("days"); daysStr != "" {
		if parsedDays, err := parseIntParam(daysStr); err == nil && parsedDays > 0 {
			olderThanDays = parsedDays
		}
	}

	err := migrationService.CleanupOldSessions(olderThanDays)
	if err != nil {
		logger.Error("Failed to cleanup old sessions", "error", err, "olderThanDays", olderThanDays)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Cleanup completed successfully",
		"olderThanDays": olderThanDays,
	})
}

// parseIntParam is a utility function to parse integer parameters from strings
func parseIntParam(s string) (int, error) {
	return strconv.Atoi(s)
}
