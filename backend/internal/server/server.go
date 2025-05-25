package server

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/server/handlers"
)

// SetupRouter configures and returns the main router
func SetupRouter() *gin.Engine {
	// Set Gin to release mode in production
	// gin.SetMode(gin.ReleaseMode)
	
	r := gin.Default()
	
	// CORS middleware for development
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	})
	
	// Setup routes
	setupRoutes(r)
	
	// Initialize scanner manager
	handlers.InitializeScanner()
	
	return r
}