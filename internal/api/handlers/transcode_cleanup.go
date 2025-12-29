package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/transcode"
	"github.com/mantonx/viewra/internal/pkg/format"
)

// GetDiskUsage returns transcode disk usage statistics.
//
// @Summary Get disk usage
// @Description Returns current disk usage statistics for transcode files
// @Tags transcode
// @Produce json
// @Success 200 {object} DiskUsageResponse
// @Failure 500 {object} handlers.APIError
// @Router /api/transcode/disk-usage [get]
func (h *TranscodeHandler) GetDiskUsage(c *gin.Context) {
	usage, err := h.cleanupService.GetDiskUsage(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	c.JSON(http.StatusOK, DiskUsageResponse{
		OutputDir:       usage.OutputDir,
		TotalSizeBytes:  usage.TotalSizeBytes,
		TotalSizeHuman:  format.Bytes(usage.TotalSizeBytes),
		FileCount:       usage.FileCount,
		TotalJobs:       usage.TotalJobs,
		CompletedCount:  usage.CompletedCount,
		FailedCount:     usage.FailedCount,
		QueuedCount:     usage.QueuedCount,
		ProcessingCount: usage.ProcessingCount,
	})
}

// CleanupTranscodes performs cleanup of transcode files.
//
// @Summary Cleanup transcodes
// @Description Cleans up transcode files based on specified criteria
// @Tags transcode
// @Accept json
// @Produce json
// @Param request body CleanupRequest true "Cleanup criteria"
// @Success 200 {object} CleanupResponse
// @Failure 400 {object} handlers.APIError
// @Failure 500 {object} handlers.APIError
// @Router /api/transcode/cleanup [post]
func (h *TranscodeHandler) CleanupTranscodes(c *gin.Context) {
	var req CleanupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	var result *transcode.CleanupResult
	var err error

	if req.Orphans {
		result, err = h.cleanupService.CleanOrphans(c.Request.Context(), req.DryRun)
	} else if req.Failed {
		olderThan := 24 * time.Hour
		if req.OlderThanHours != nil {
			olderThan = time.Duration(*req.OlderThanHours) * time.Hour
		}
		result, err = h.cleanupService.CleanFailed(c.Request.Context(), olderThan, req.DryRun)
	} else if req.MediaID != nil {
		result, err = h.cleanupService.CleanByMediaID(c.Request.Context(), *req.MediaID, req.DryRun)
	} else if req.OlderThanHours != nil {
		olderThan := time.Duration(*req.OlderThanHours) * time.Hour
		result, err = h.cleanupService.CleanOld(c.Request.Context(), olderThan, req.DryRun)
	} else {
		// Custom filter
		filter := transcode.CleanupFilter{
			IncludeCompleted: true,
			IncludeFailed:    req.Failed,
			DryRun:           req.DryRun,
		}
		if req.Quality != nil {
			filter.Quality = req.Quality
		}
		result, err = h.cleanupService.Clean(c.Request.Context(), filter)
	}

	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	// Convert errors to strings
	errorStrings := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		errorStrings[i] = e.Error()
	}

	c.JSON(http.StatusOK, CleanupResponse{
		DeletedCount:     result.DeletedCount,
		DeletedSizeBytes: result.DeletedSizeBytes,
		DeletedSizeHuman: format.Bytes(result.DeletedSizeBytes),
		FailedCount:      result.FailedCount,
		Errors:           errorStrings,
		DryRun:           req.DryRun,
	})
}
