package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/system"
)

// MaintenanceResponse is returned when the server is in maintenance mode.
type MaintenanceResponse struct {
	Error        string `json:"error"`
	Code         string `json:"code"`
	Reason       string `json:"reason,omitempty"`
	EstimatedEnd string `json:"estimatedEnd,omitempty"`
}

// MaintenanceMode returns middleware that blocks requests when maintenance mode is enabled.
// Some endpoints are always allowed (health checks, auth, maintenance status).
func MaintenanceMode(manager *system.MaintenanceManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Always allow these paths
		if isMaintenanceExempt(c.Request.URL.Path) {
			c.Next()
			return
		}

		// Check if maintenance mode is enabled
		state := manager.GetState()
		if !state.Enabled {
			c.Next()
			return
		}

		// Build response
		response := MaintenanceResponse{
			Error:  "Service is temporarily unavailable due to scheduled maintenance",
			Code:   "MAINTENANCE_MODE",
			Reason: state.Reason,
		}

		if state.EstimatedEnd != nil {
			response.EstimatedEnd = state.EstimatedEnd.Format("2006-01-02T15:04:05Z")
		}

		c.AbortWithStatusJSON(http.StatusServiceUnavailable, response)
	}
}

// isMaintenanceExempt returns true if the path should be allowed during maintenance.
func isMaintenanceExempt(path string) bool {
	exemptPaths := []string{
		"/health",
		"/health/live",
		"/health/ready",
		"/api/auth/login",
		"/api/auth/refresh",
		"/api/admin/system/maintenance",
	}

	for _, exempt := range exemptPaths {
		if path == exempt {
			return true
		}
	}

	// Allow all paths under /api/admin/system/ for admin operations
	if len(path) > 18 && path[:18] == "/api/admin/system/" {
		return true
	}

	return false
}
