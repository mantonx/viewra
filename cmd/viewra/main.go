package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/infrastructure/database"
)

func main() {
	// Load database configuration
	dbConfig := database.LoadConfigFromEnv()

	// Connect to database
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Printf("Database connection established (driver: %s)", dbConfig.Driver)

	// Set up Gin router
	router := gin.Default()

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "viewra",
			"version": "0.0.1",
			"database": dbConfig.Driver,
		})
	})

	// API routes group
	api := router.Group("/api")
	{
		// Placeholder endpoints
		api.GET("/libraries", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Libraries endpoint - coming soon",
			})
		})

		api.GET("/media", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Media endpoint - coming soon",
			})
		})
	}

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start server
	addr := fmt.Sprintf(":%s", port)
	log.Printf("Starting ViewRA server on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
