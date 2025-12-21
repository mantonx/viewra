package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	appconfig "github.com/mantonx/viewra/internal/app/config"
	"github.com/mantonx/viewra/internal/application/library"
	"github.com/mantonx/viewra/internal/infrastructure/database"
	"github.com/mantonx/viewra/internal/infrastructure/system"
)

// RunPreContainerTasks executes startup tasks that must run before the container is created.
// This includes system profiling and database migrations.
func RunPreContainerTasks(ctx context.Context, db *sql.DB, cfg *appconfig.Config, logger *slog.Logger) error {
	// Profile system resources for optimal performance (fast: <100ms)
	profile, err := system.ProfileSystem(ctx)
	if err != nil {
		logger.Warn("Failed to profile system, using conservative defaults", "error", err)
	} else {
		settings := profile.Calculate()

		// Log system profile as a formatted table
		profile.PrintTable(os.Stderr, "System Profile", &settings)

		// Store profile in config for use throughout application
		cfg.SystemProfile = profile

		// Update config with profile-based defaults if not explicitly set via environment
		// Only override if the current value matches the hardcoded default (8)
		if cfg.Media.TranscodeWorkers == 8 {
			cfg.Media.TranscodeWorkers = settings.TranscodeWorkers
			logger.Info("Updated transcode workers based on system profile",
				"workers", cfg.Media.TranscodeWorkers)
		}
	}

	// Run database migrations
	if cfg.Database.Migrations.Enabled {
		dbConfig := &database.Config{
			Driver:   cfg.Database.Driver,
			Host:     cfg.Database.Host,
			Port:     cfg.Database.Port,
			User:     cfg.Database.User,
			Password: cfg.Database.Password,
			DBName:   cfg.Database.DBName,
			SSLMode:  cfg.Database.SSLMode,
		}
		migrationConfig := &database.MigrationConfig{
			AutoMigrate:    cfg.Database.Migrations.Enabled,
			MigrationsPath: cfg.Database.Migrations.SourceDir,
		}
		if err := database.AutoMigrate(db, dbConfig, migrationConfig, logger); err != nil {
			return fmt.Errorf("failed to run database migrations: %w", err)
		}
	}

	return nil
}

// RunPostContainerTasks executes startup tasks that require the container's services.
// This reuses the already-built services instead of rebuilding them.
func RunPostContainerTasks(ctx context.Context, scanUseCase *library.ScanLibraryUseCase, logger *slog.Logger) {
	// Resume stuck scans automatically (non-critical)
	if err := scanUseCase.ResumeStuckScans(ctx); err != nil {
		logger.Error("Failed to resume stuck scans", "error", err)
		// Don't propagate error - this is not critical
	}
}
