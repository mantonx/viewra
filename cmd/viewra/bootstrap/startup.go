package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	appconfig "github.com/mantonx/viewra/internal/app/config"
	"github.com/mantonx/viewra/internal/app/repositories"
	"github.com/mantonx/viewra/internal/app/services"
	"github.com/mantonx/viewra/internal/app/usecases"
	"github.com/mantonx/viewra/internal/application/common"
	"github.com/mantonx/viewra/internal/infrastructure/database"
	"github.com/mantonx/viewra/internal/infrastructure/system"
)

// recoverStuckScans automatically resumes scans that were interrupted by server restart or crash
func recoverStuckScans(ctx context.Context, db *sql.DB, driver string, cfg *appconfig.Config, logger *slog.Logger) error {
	// Build dependencies needed for scan resumption
	repos := repositories.BuildRepositories(db, driver)
	svcs, err := services.BuildServices(cfg, repos, logger)
	if err != nil {
		return fmt.Errorf("failed to build services for scan recovery: %w", err)
	}

	// Create transaction manager
	txManager := common.NewTxManager(db)

	// Build use cases
	cases := usecases.BuildUseCases(cfg, repos, svcs, txManager, logger)

	// Resume stuck scans automatically
	return cases.Library.Scan.ResumeStuckScans(ctx)
}

// RunStartupTasksFromConfig executes all startup tasks using the new config structure
func RunStartupTasksFromConfig(ctx context.Context, db *sql.DB, driver string, cfg *appconfig.Config, logger *slog.Logger) error {
	// Profile system resources for optimal performance (fast: <100ms)
	profile, err := system.ProfileSystem(ctx)
	if err != nil {
		logger.Warn("Failed to profile system, using conservative defaults", "error", err)
	} else {
		settings := profile.Calculate()
		logger.Info("System profile detected",
			"cpu_cores", profile.CPU.NumPhysical,
			"cpu_threads", profile.CPU.NumCPU,
			"cpu_model", profile.CPU.Model,
			"memory_gb", float64(profile.Memory.TotalBytes)/(1024*1024*1024),
			"gpu_type", profile.GPU.Type,
			"gpu_available", profile.GPU.Available,
			"gpu_count", profile.GPU.DeviceCount,
			"has_vaapi", profile.GPU.HasVAAPI,
			"has_opencl", profile.GPU.HasOpenCL,
			"has_vulkan", profile.GPU.HasVulkan,
			"hash_workers", settings.HashWorkers,
			"transcode_workers", settings.TranscodeWorkers,
			"transcode_hwaccel", settings.HardwareAccel)

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

	// Resume stuck scans automatically (non-critical)
	if err := recoverStuckScans(ctx, db, driver, cfg, logger); err != nil {
		logger.Error("Failed to resume stuck scans", "error", err)
		// Don't return error - this is not critical
	}

	return nil
}
