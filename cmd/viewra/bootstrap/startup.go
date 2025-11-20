package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	appconfig "github.com/viewra/viewra/internal/app/config"
	"github.com/viewra/viewra/internal/infrastructure/database"
	"github.com/viewra/viewra/internal/infrastructure/persistence/scanjob"
)

// RunStartupTasks executes all startup tasks in order
func RunStartupTasks(ctx context.Context, db *sql.DB, driver string, cfg *Config, logger *slog.Logger) error {
	// Run database migrations
	if err := runMigrations(db, cfg.Database, cfg.Migration, logger); err != nil {
		return err
	}

	// Recover stuck scans (non-critical)
	if err := recoverStuckScans(ctx, db, driver, logger); err != nil {
		logger.Error("Failed to recover stuck scans", "error", err)
		// Don't return error - this is not critical
	}

	return nil
}

// runMigrations runs database migrations if auto-migration is enabled
func runMigrations(db *sql.DB, dbConfig *database.Config, migrationConfig *database.MigrationConfig, logger *slog.Logger) error {
	if err := database.AutoMigrate(db, dbConfig, migrationConfig, logger); err != nil {
		return fmt.Errorf("failed to run database migrations: %w", err)
	}
	return nil
}

// recoverStuckScans checks for scans stuck in "running" status and marks them as failed
func recoverStuckScans(ctx context.Context, db *sql.DB, driver string, logger *slog.Logger) error {
	scanJobRepo := scanjob.NewRepository(db, driver)
	return scanJobRepo.RecoverStuckScans(ctx, logger)
}

// RunStartupTasksFromConfig executes all startup tasks using the new config structure
func RunStartupTasksFromConfig(ctx context.Context, db *sql.DB, driver string, cfg *appconfig.Config, logger *slog.Logger) error {
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

	// Recover stuck scans (non-critical)
	if err := recoverStuckScans(ctx, db, driver, logger); err != nil {
		logger.Error("Failed to recover stuck scans", "error", err)
		// Don't return error - this is not critical
	}

	return nil
}
