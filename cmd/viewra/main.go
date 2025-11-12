package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/viewra/viewra/docs/swagger" // Import generated docs
	"github.com/viewra/viewra/internal/api"
	"github.com/viewra/viewra/internal/app"
	"github.com/viewra/viewra/internal/infrastructure/database"
	"github.com/viewra/viewra/internal/pkg/logger"
)

// @title           ViewRA Media Server API
// @version         0.0.1
// @description     Self-hosted media server for movies, TV shows, and music
// @termsOfService  http://swagger.io/terms/

// @contact.name   ViewRA Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /

// @schemes http https

func main() {
	// Initialize structured logger
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development"
	}
	logger := logger.New(env)

	logger.Info("Starting ViewRA Media Server", "version", "0.0.1", "environment", env)

	// Load and validate database configuration
	dbConfig := database.LoadConfigFromEnv()
	if err := dbConfig.Validate(); err != nil {
		logger.Error("Invalid database configuration", "error", err)
		os.Exit(1)
	}

	// Connect to database
	db, err := database.Connect(dbConfig)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer closeDatabase(db, logger)

	logger.Info("Database connection established", "driver", dbConfig.Driver)

	// Load server configuration
	serverConfig := loadServerConfig()

	// Initialize application container with all dependencies
	container := app.NewContainer(db, dbConfig.Driver, serverConfig, logger)

	// Add Swagger documentation endpoint
	container.Server.Router().GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Start server and handle graceful shutdown
	runServer(container.Server, serverConfig.Port, logger)
}

// loadServerConfig loads server configuration from environment variables
func loadServerConfig() api.ServerConfig {
	config := api.DefaultServerConfig()

	if portStr := os.Getenv("PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.Port = port
		}
	}

	return config
}

// runServer starts the HTTP server and handles graceful shutdown
func runServer(server *api.Server, port int, logger *slog.Logger) {
	// Start server in goroutine
	go func() {
		logger.Info("HTTP server starting",
			"port", port,
			"swagger", fmt.Sprintf("http://localhost:%d/swagger/index.html", port))
		if err := server.Start(); err != nil {
			logger.Error("Server error", "error", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info("Shutdown signal received", "signal", sig.String())

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("Server stopped gracefully")
}

// closeDatabase safely closes the database connection
func closeDatabase(db interface{ Close() error }, logger *slog.Logger) {
	if err := db.Close(); err != nil {
		logger.Error("Error closing database", "error", err)
	}
}
