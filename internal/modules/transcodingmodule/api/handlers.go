// Package api provides HTTP handlers and routes for the transcoding module.
// It implements the REST API endpoints for transcoding operations, session management,
// and content delivery.
package api

import (
	"github.com/gin-gonic/gin"
)

// APIHandler handles HTTP requests for the transcoding module.
// It provides RESTful endpoints for managing transcoding operations,
// including starting sessions, checking progress, and accessing content.
type APIHandler struct {
	service TranscodingAPIService
}

// NewAPIHandler creates a new API handler.
//
// Parameters:
//   - service: The transcoding service that handles business logic
//
// The handler translates HTTP requests into service calls and
// formats responses for API consumers.
func NewAPIHandler(service TranscodingAPIService) *APIHandler {
	return &APIHandler{
		service: service,
	}
}

// HealthCheck handles GET /api/v1/transcoding/health
//
// Returns the health status of the transcoding service.
//
// Response:
//
//	{
//	  "status": "healthy",
//	  "providers": 3,
//	  "activeSessions": 2
//	}
func (h *APIHandler) HealthCheck(c *gin.Context) {
	sessions := h.service.GetAllSessions()
	providers := h.service.GetProviders()
	
	// Count active sessions
	active := 0
	for _, session := range sessions {
		if session.Status == "running" || session.Status == "starting" {
			active++
		}
	}

	c.JSON(200, gin.H{
		"status":         "healthy",
		"providers":      len(providers),
		"activeSessions": active,
	})
}