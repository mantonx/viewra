package bootstrap

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/mantonx/viewra/internal/api"
	"github.com/mantonx/viewra/internal/infrastructure/database"
	"github.com/mantonx/viewra/internal/pkg/logger"
)

// Config holds all application configuration
type Config struct {
	Environment string
	Database    *database.Config
	Migration   *database.MigrationConfig
	Server      api.ServerConfig
}

// LoadConfig loads all configuration from environment variables
func LoadConfig() *Config {
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development"
	}

	return &Config{
		Environment: env,
		Database:    database.LoadConfigFromEnv(),
		Migration:   database.LoadMigrationConfigFromEnv(),
		Server:      loadServerConfig(),
	}
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

// NewLogger creates a structured logger based on environment
func NewLogger(env string) *slog.Logger {
	return logger.New(env)
}
