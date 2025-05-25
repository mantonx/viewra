// Package handlers contains HTTP request handlers organized by functionality.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/viewra/internal/database"
)

// HandleHealthCheck returns the basic health status of the service
func HandleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "viewra",
	})
}

// HandleHello returns a simple greeting message for connectivity testing
func HandleHello(c *gin.Context) {
	c.String(http.StatusOK, "Hello from Viewra backend!")
}

// HandleDBStatus checks and returns the database connection status
func HandleDBStatus(c *gin.Context) {
	db := database.GetDB()
	sqlDB, err := db.DB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Failed to get database instance: " + err.Error(),
		})
		return
	}
	
	if err := sqlDB.Ping(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  "Database ping failed: " + err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status":   "connected",
		"database": "ready",
	})
}
