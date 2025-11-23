package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
)

// RegisterLibraryRoutes registers all library-related routes
func RegisterLibraryRoutes(rg *gin.RouterGroup, handler *handlers.LibraryHandler, scanHandler *handlers.ScanJobHandler) {
	libraries := rg.Group("/libraries")
	libraries.POST("", handler.Create)
	libraries.GET("", handler.List)
	libraries.GET("/:id", handler.Get)
	libraries.PUT("/:id", handler.Update)
	libraries.DELETE("/:id", handler.Delete)

	// Scan operations
	libraries.POST("/:id/scan", handler.Scan)
	libraries.GET("/:id/scan/status", scanHandler.GetStatus)
	libraries.GET("/:id/scan/history", scanHandler.GetHistory)
	libraries.GET("/:id/scan/stream", scanHandler.StreamProgress)
	libraries.GET("/:id/scan/:jobId/errors", scanHandler.GetScanErrors)
	libraries.POST("/:id/scan/:jobId/retry-failed", scanHandler.RetryFailedFiles)
	libraries.POST("/:id/scan/:jobId/pause", scanHandler.PauseScan)
	libraries.POST("/:id/scan/:jobId/resume", scanHandler.ResumeScan)

	// Library-level persistent issues (warnings/errors from scan_state)
	libraries.GET("/:id/issues", scanHandler.GetLibraryIssues)
}
