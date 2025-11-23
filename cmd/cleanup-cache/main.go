package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mantonx/viewra/internal/app/config"
	"github.com/mantonx/viewra/internal/application/images"
	"github.com/mantonx/viewra/internal/infrastructure/database"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
	imageRepo "github.com/mantonx/viewra/internal/infrastructure/persistence/image"
	"github.com/mantonx/viewra/internal/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	lgr := logger.New(cfg.Environment)

	// Convert to database.Config for connection
	dbConfig := &database.Config{
		Driver:   cfg.Database.Driver,
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
	}

	// Initialize database connection
	db, err := database.Connect(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	lgr.Info("Starting manual image cache cleanup")

	// Create base repository
	baseRepo := common.NewBaseRepository(db, cfg.Database.Driver)

	// Create image repository
	imageRepository := imageRepo.NewRepository(baseRepo)

	// Create cleanup use case
	cleanup := images.NewCleanupUseCase(imageRepository, cfg.Images.CacheDir, lgr)

	// Run cleanup
	ctx := context.Background()
	stats, err := cleanup.CleanOrphanedImages(ctx)
	if err != nil {
		lgr.Error("Cleanup failed", "error", err)
		os.Exit(1)
	}

	// Print results
	lgr.Info("Cleanup completed successfully",
		"orphaned_files", stats.OrphanedFiles,
		"deleted_files", stats.DeletedFiles,
		"errors", stats.Errors,
		"bytes_freed", stats.BytesFreed,
		"mb_freed", float64(stats.BytesFreed)/1024/1024)

	fmt.Printf("\n")
	fmt.Printf("✅ Cleanup Summary:\n")
	fmt.Printf("   Orphaned files found: %d\n", stats.OrphanedFiles)
	fmt.Printf("   Files deleted: %d\n", stats.DeletedFiles)
	fmt.Printf("   Errors: %d\n", stats.Errors)
	fmt.Printf("   Space freed: %.2f MB\n", float64(stats.BytesFreed)/1024/1024)
}
