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

	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/server"

	// Force module inclusion by importing directly in main
	"github.com/mantonx/viewra/internal/modules/assetmodule"
	_ "github.com/mantonx/viewra/internal/modules/databasemodule"
	_ "github.com/mantonx/viewra/internal/modules/eventsmodule"
	_ "github.com/mantonx/viewra/internal/modules/mediamodule"
	_ "github.com/mantonx/viewra/internal/modules/playbackmodule"
	_ "github.com/mantonx/viewra/internal/modules/scannermodule"
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

	// Initialize configuration system first
	configPath := os.Getenv("VIEWRA_CONFIG_PATH")
	if configPath == "" {
		// Try default paths
		if _, err := os.Stat("/app/viewra-data/viewra.yaml"); err == nil {
			configPath = "/app/viewra-data/viewra.yaml"
		} else if _, err := os.Stat("./viewra.yaml"); err == nil {
			configPath = "./viewra.yaml"
		}
	}

	if err := config.Load(configPath); err != nil {
		log.Printf("⚠️  Warning: Failed to load configuration from %s: %v", configPath, err)
		log.Printf("Using default configuration")
	} else if configPath != "" {
		log.Printf("✅ Configuration loaded from: %s", configPath)
	} else {
		log.Printf("✅ Using default configuration")
	}

	// Initialize database
	database.Initialize()
	db := database.GetDB()
	if db == nil {
		log.Fatal("Failed to initialize database")
	}

	// Force assetmodule inclusion by calling a function from it
	// This prevents the Go linker from optimizing it away
	_ = assetmodule.GetAssetManager()

	// Setup router with plugins and modules
	r := server.SetupRouter()

	// Get configuration for server setup
	cfg := config.Get()

	// Create a context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create server with graceful shutdown capability
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
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

	// Start the server
	log.Printf("🚀 Starting Viewra server on %s:%d", cfg.Server.Host, cfg.Server.Port)
	err := srv.ListenAndServe()

	// Handle server startup errors
	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}

	<-ctx.Done()
	log.Println("Server shutdown complete")
}
