package bootstrap

import (
	"database/sql"
	"fmt"
	"log/slog"

	appconfig "github.com/mantonx/viewra/internal/app/config"
	"github.com/mantonx/viewra/internal/infrastructure/database"
)

// DatabaseConnection holds the database connection and metadata
type DatabaseConnection struct {
	DB     *sql.DB
	Driver string
}

// InitializeDatabase connects to the database and validates configuration
func InitializeDatabase(cfg *database.Config, logger *slog.Logger) (*DatabaseConnection, error) {
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid database configuration: %w", err)
	}

	// Connect to database
	db, err := database.Connect(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	logger.Info("Database connection established", "driver", cfg.Driver)

	return &DatabaseConnection{
		DB:     db,
		Driver: cfg.Driver,
	}, nil
}

// Close safely closes the database connection
func (dc *DatabaseConnection) Close(logger *slog.Logger) {
	if err := dc.DB.Close(); err != nil {
		logger.Error("Error closing database", "error", err)
	}
}

// InitializeDatabaseFromConfig connects to the database using the new config structure
func InitializeDatabaseFromConfig(cfg *appconfig.DatabaseConfig, logger *slog.Logger) (*DatabaseConnection, error) {
	// Convert to database.Config for connection
	dbConfig := &database.Config{
		Driver:          cfg.Driver,
		Host:            cfg.Host,
		Port:            cfg.Port,
		User:            cfg.User,
		Password:        cfg.Password,
		DBName:          cfg.DBName,
		SSLMode:         cfg.SSLMode,
		MaxOpenConns:    cfg.Pool.MaxOpenConns,
		MaxIdleConns:    cfg.Pool.MaxIdleConns,
		ConnMaxLifetime: cfg.Pool.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Pool.ConnMaxIdleTime,
	}

	// Validate configuration
	if err := dbConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid database configuration: %w", err)
	}

	// Connect to database
	db, err := database.Connect(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	logger.Info("Database connection established", "driver", cfg.Driver)

	return &DatabaseConnection{
		DB:     db,
		Driver: cfg.Driver,
	}, nil
}
