package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/server"
)

func main() {
	// Initialize database
	database.Initialize()
	
	// Setup router with plugins
	r := server.SetupRouter()
	
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	
	// Create a context for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		
		log.Println("Shutting down gracefully...")
		
		// Shutdown plugin manager
		if err := server.ShutdownPluginManager(); err != nil {
			log.Printf("Error shutting down plugin manager: %v", err)
		}
		
		cancel()
	}()
	
	log.Printf("🚀 Starting Viewra server on :%s", port)
	r.Run(":" + port)
}
