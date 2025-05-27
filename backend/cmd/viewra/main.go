package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/logger"
	"github.com/mantonx/viewra/internal/server"
)

func main() {
	// Super early file log
	f, err_f := os.OpenFile("/app/startup.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err_f == nil {
		defer f.Close()
		f.WriteString(fmt.Sprintf("Viewra Main Started at: %s\n", time.Now().Format(time.RFC3339)))
	} else {
		// If we can't write to file, try stdout for this specific early log
		fmt.Println("Viewra Main Started (file log failed)")
	}

	// Print startup banner
	fmt.Println("=======================================")
	fmt.Println("  Viewra Media Server - Module System  ")
	fmt.Println("=======================================")
	
	// Initialize database
	database.Initialize()
	db := database.GetDB()
	if db == nil {
		log.Fatal("Failed to initialize database")
	}
	
	// Setup router with plugins and modules
	r := server.SetupRouter()
	
	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	
	// Create a context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Create server with graceful shutdown capability
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}
	
	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		
		log.Println("\nShutting down gracefully...")
		
		// Create a deadline for shutdown
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		
		// Shutdown HTTP server
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
		
		// Shutdown plugin manager
		if err := server.ShutdownPluginManager(); err != nil {
			log.Printf("Plugin manager shutdown error: %v", err)
		}
		
		// Shutdown event bus
		if err := server.ShutdownEventBus(); err != nil {
			log.Printf("Event bus shutdown error: %v", err)
		}
		
		cancel()
	}()
	
	// Try to start the server on the requested port
	log.Printf("🚀 Starting Viewra server on :%s", port)
	err := srv.ListenAndServe()
	
	// If the port is in use, try a fallback port
	if err != nil && err != http.ErrServerClosed {
		log.Printf("Failed to start server on port %s: %v", port, err)
		fallbackPort := "8081"
		log.Printf("Trying fallback port %s", fallbackPort)
		
		// Create a new server with the fallback port
		srv = &http.Server{
			Addr:    ":" + fallbackPort,
			Handler: r,
		}
		
		// Log the new port
		logger.Info("🚀 Starting Viewra server on fallback port :%s", fallbackPort)
		
		// Start the server on the fallback port
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server on fallback port %s: %v", fallbackPort, err)
		}
	}
	
	<-ctx.Done()
	log.Println("Server shutdown complete")
}
