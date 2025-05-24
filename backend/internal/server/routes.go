package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// setupRoutes configures all API routes
func setupRoutes(r *gin.Engine) {
	// API v1 routes
	api := r.Group("/api")
	{
		// Health check endpoint
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "ok",
				"service": "viewra",
			})
		})
		
		// Hello world endpoint
		api.GET("/hello", func(c *gin.Context) {
			c.String(http.StatusOK, "Hello from Viewra backend!")
		})
	}
	
	// Future routes will be organized here:
	// - /api/media/* - media management endpoints
	// - /api/auth/* - authentication endpoints
	// - /api/users/* - user management endpoints
}