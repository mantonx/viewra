package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// Default database paths - kept for backward compatibility
	DefaultDataDir      = "/app/viewra-data"
	DefaultDatabaseFile = "viewra.db"
	
	// Environment variables - kept for backward compatibility
	EnvDataDir    = "VIEWRA_DATA_DIR"
	EnvDatabasePath = "VIEWRA_DATABASE_PATH"
)

// DatabaseConfig holds database configuration - DEPRECATED: Use config.Get().Database instead
type DatabaseConfig struct {
	DataDir      string
	DatabasePath string
	DatabaseURL  string
}

// GetDatabaseConfig returns the centralized database configuration
// DEPRECATED: Use config.Get().Database instead for new code
func GetDatabaseConfig() *DatabaseConfig {
	// Use the new centralized config system
	cfg := Get()
	
	return &DatabaseConfig{
		DataDir:      cfg.Database.DataDir,
		DatabasePath: cfg.Database.DatabasePath,
		DatabaseURL:  cfg.Database.URL,
	}
}

// GetDataDir returns the data directory path
// DEPRECATED: Use config.Get().Database.DataDir instead
func GetDataDir() string {
	return Get().Database.DataDir
}

// GetDatabasePath returns the database file path
// DEPRECATED: Use config.Get().Database.DatabasePath instead
func GetDatabasePath() string {
	return Get().Database.DatabasePath
}

// GetDatabaseURL returns the database URL for plugins
// DEPRECATED: Use config.Get().Database.URL instead
func GetDatabaseURL() string {
	cfg := Get().Database
	
	// Generate URL if not explicitly set
	if cfg.URL != "" {
		return cfg.URL
	}
	
	switch cfg.Type {
	case "sqlite":
		return "sqlite://" + cfg.DatabasePath
	case "postgres":
		return buildPostgresURL(cfg)
	default:
		return "sqlite://" + cfg.DatabasePath
	}
}

// buildPostgresURL builds a PostgreSQL connection URL from config
func buildPostgresURL(cfg DatabaseFullConfig) string {
	if cfg.Host == "" {
		cfg.Host = "localhost"
	}
	if cfg.Port == 0 {
		cfg.Port = 5432
	}
	if cfg.Username == "" {
		cfg.Username = "viewra"
	}
	if cfg.Database == "" {
		cfg.Database = "viewra"
	}
	
	url := "postgres://"
	if cfg.Username != "" {
		url += cfg.Username
		if cfg.Password != "" {
			url += ":" + cfg.Password
		}
		url += "@"
	}
	url += cfg.Host
	if cfg.Port != 5432 {
		url += fmt.Sprintf(":%d", cfg.Port)
	}
	url += "/" + cfg.Database
	
	return url
}

// LegacyGetDatabaseConfig provides backward compatibility for old database configuration approach
// This function maintains the old behavior while using the new centralized config internally
func LegacyGetDatabaseConfig() *DatabaseConfig {
	// First try to get from centralized config
	cfg := Get().Database
	
	// If centralized config has values, use them
	if cfg.DataDir != "" || cfg.DatabasePath != "" {
		dataDir := cfg.DataDir
		if dataDir == "" {
			dataDir = DefaultDataDir
		}
		
		dbPath := cfg.DatabasePath
		if dbPath == "" {
			dbPath = filepath.Join(dataDir, DefaultDatabaseFile)
		}
		
		// Ensure directory exists
		os.MkdirAll(filepath.Dir(dbPath), 0755)
		
		return &DatabaseConfig{
			DataDir:      dataDir,
			DatabasePath: dbPath,
			DatabaseURL:  GetDatabaseURL(),
		}
	}
	
	// Fallback to legacy environment variable approach
	dataDir := os.Getenv(EnvDataDir)
	if dataDir == "" {
		dataDir = DefaultDataDir
	}
	
	dbPath := os.Getenv(EnvDatabasePath)
	if dbPath == "" {
		dbPath = filepath.Join(dataDir, DefaultDatabaseFile)
	}
	
	// Ensure directory exists
	os.MkdirAll(filepath.Dir(dbPath), 0755)
	
	return &DatabaseConfig{
		DataDir:      dataDir,
		DatabasePath: dbPath,
		DatabaseURL:  "sqlite://" + dbPath,
	}
} 