package config

import (
	"os"
	"path/filepath"
)

const (
	// Default database paths
	DefaultDataDir      = "/app/viewra-data"
	DefaultDatabaseFile = "database.db"
	
	// Environment variables
	EnvDataDir    = "VIEWRA_DATA_DIR"
	EnvDatabasePath = "VIEWRA_DATABASE_PATH"
)

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	DataDir      string
	DatabasePath string
	DatabaseURL  string
}

// GetDatabaseConfig returns the centralized database configuration
func GetDatabaseConfig() *DatabaseConfig {
	// Determine data directory
	dataDir := os.Getenv(EnvDataDir)
	if dataDir == "" {
		dataDir = DefaultDataDir
	}
	
	// Determine database path
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

// GetDataDir returns the data directory path
func GetDataDir() string {
	return GetDatabaseConfig().DataDir
}

// GetDatabasePath returns the database file path
func GetDatabasePath() string {
	return GetDatabaseConfig().DatabasePath
}

// GetDatabaseURL returns the database URL for plugins
func GetDatabaseURL() string {
	return GetDatabaseConfig().DatabaseURL
} 