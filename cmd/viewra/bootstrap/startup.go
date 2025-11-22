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
