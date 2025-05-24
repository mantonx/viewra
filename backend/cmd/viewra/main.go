package main

import (
	"log"
	"os"

	"github.com/yourusername/viewra/internal/database"
	"github.com/yourusername/viewra/internal/server"
)

func main() {
	// Initialize database
	database.Initialize()
	
	r := server.SetupRouter()
	
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	
	log.Printf("🚀 Starting Viewra server on :%s", port)
	r.Run(":" + port)
}
