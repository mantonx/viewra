package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/logger"
	plugins "github.com/mantonx/viewra/sdk"
)

// GetStats handles GET /api/v1/transcoding/stats
//
// Returns comprehensive transcoding statistics.
//
// Response:
//
//	{
//	  "activeSessions": 5,
//	  "totalSessions": 150,
//	  "completedSessions": 120,
//	  "failedSessions": 25,
//	  "resourceUsage": {...},
//	  "providers": {...}
//	}
func (h *APIHandler) GetStats(c *gin.Context) {
	// Get all sessions for stats
	sessions := h.service.GetAllSessions()

	// Calculate stats
	var active, completed, failed int
	for _, session := range sessions {
		switch session.Status {
		case plugins.TranscodeStatusRunning, plugins.TranscodeStatusStarting:
			active++
		case plugins.TranscodeStatusCompleted:
			completed++
		case plugins.TranscodeStatusFailed:
			failed++
		}
	}

	// Get resource usage
	resourceUsage := h.service.GetResourceUsage()

	c.JSON(http.StatusOK, gin.H{
		"activeSessions":    active,
		"totalSessions":     len(sessions),
		"completedSessions": completed,
		"failedSessions":    failed,
		"resourceUsage":     resourceUsage,
		"providers":         h.service.GetProviders(),
	})
}

// GetPipelineStatus handles GET /api/v1/transcoding/pipeline/status
//
// Returns the status of the file pipeline provider.
//
// Response:
//
//	{
//	  "available": true,
//	  "activeJobs": 2,
//	  "completedJobs": 50,
//	  "failedJobs": 3,
//	  "ffmpegVersion": "4.4.0",
//	  "supportedFormats": ["mp4", "mkv", "webm"]
//	}
func (h *APIHandler) GetPipelineStatus(c *gin.Context) {
	status := h.service.GetPipelineStatus()
	c.JSON(http.StatusOK, status)
}

// GetResourceUsage handles GET /api/v1/transcoding/resources
//
// Returns current resource usage information.
//
// Response:
//
//	{
//	  "activeSessions": 3,
//	  "maxSessions": 5,
//	  "queuedSessions": 2,
//	  "cpuUsage": 65.5,
//	  "memoryUsage": 2048,
//	  "sessionDetails": [...]
//	}
func (h *APIHandler) GetResourceUsage(c *gin.Context) {
	usage := h.service.GetResourceUsage()
	c.JSON(http.StatusOK, usage)
}

// GetContentHashStats handles GET /api/v1/transcoding/content-hash/stats
//
// Returns statistics about content hash migration.
//
// Response:
//
//	{
//	  "totalSessions": 1000,
//	  "sessionsWithHash": 850,
//	  "completedWithoutHash": 150,
//	  "hashCoveragePercent": 85.0
//	}
func (h *APIHandler) GetContentHashStats(c *gin.Context) {
	migrationService := h.service.GetMigrationService()
	if migrationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Migration service not available",
		})
		return
	}

	stats, err := migrationService.GetContentHashStats()
	if err != nil {
		logger.Error("Failed to get content hash stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve statistics",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// MigrateContentHashes handles POST /api/v1/transcoding/content-hash/migrate
//
// Triggers migration of sessions without content hashes.
//
// Query parameters:
//   - limit: Maximum number of sessions to migrate (default: 10)
//
// Response:
//
//	{
//	  "message": "Migration started",
//	  "sessionsToMigrate": 10
//	}
func (h *APIHandler) MigrateContentHashes(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid limit parameter",
		})
		return
	}

	migrationService := h.service.GetMigrationService()
	if migrationService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Migration service not available",
		})
		return
	}

	// Get sessions without content hash
	sessions, err := migrationService.ListSessionsWithoutContentHash(limit)
	if err != nil {
		logger.Error("Failed to list sessions for migration: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve sessions",
		})
		return
	}

	// TODO: Implement actual migration logic
	// For now, just return the count
	c.JSON(http.StatusOK, gin.H{
		"message":           "Migration would process these sessions",
		"sessionsToMigrate": len(sessions),
		"limit":             limit,
	})
}