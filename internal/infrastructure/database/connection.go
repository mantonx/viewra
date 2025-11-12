package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/lib/pq"           // PostgreSQL driver
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// Config holds database configuration
type Config struct {
	Driver   string // "sqlite" or "postgres"
	Host     string // PostgreSQL host
	Port     string // PostgreSQL port
	User     string // PostgreSQL user
	Password string // PostgreSQL password
	DBName   string // Database name (or SQLite file path)
	SSLMode  string // PostgreSQL SSL mode (disable, require, verify-ca, verify-full)
}

// LoadConfigFromEnv loads database configuration from environment variables
func LoadConfigFromEnv() *Config {
	driver := os.Getenv("DB_DRIVER")
	if driver == "" {
		driver = "sqlite" // Default to SQLite
	}

	config := &Config{
		Driver: driver,
	}

	if driver == "postgres" || driver == "postgresql" {
		config.Host = getEnvOrDefault("DB_HOST", "localhost")
		config.Port = getEnvOrDefault("DB_PORT", "5432")
		config.User = getEnvOrDefault("DB_USER", "viewra")
		config.Password = os.Getenv("DB_PASSWORD")
		config.DBName = getEnvOrDefault("DB_NAME", "viewra")
		config.SSLMode = getEnvOrDefault("DB_SSL_MODE", "disable")
	} else {
		// SQLite
		config.DBName = getEnvOrDefault("DB_PATH", "data/viewra.db")
	}

	return config
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate driver
	switch c.Driver {
	case "sqlite", "sqlite3":
		if c.DBName == "" {
			return fmt.Errorf("DB_PATH cannot be empty for SQLite")
		}
		// Check if path is absolute or relative
		if !filepath.IsAbs(c.DBName) && c.DBName[0] != '.' {
			// Ensure it's treated as relative to current directory
			c.DBName = filepath.Clean(c.DBName)
		}

	case "postgres", "postgresql":
		if c.Host == "" {
			return fmt.Errorf("DB_HOST cannot be empty for PostgreSQL")
		}
		if c.Port == "" {
			return fmt.Errorf("DB_PORT cannot be empty for PostgreSQL")
		}
		if c.User == "" {
			return fmt.Errorf("DB_USER cannot be empty for PostgreSQL")
		}
		if c.DBName == "" {
			return fmt.Errorf("DB_NAME cannot be empty for PostgreSQL")
		}
		// Validate SSL mode
		validSSLModes := map[string]bool{
			"disable":     true,
			"require":     true,
			"verify-ca":   true,
			"verify-full": true,
		}
		if !validSSLModes[c.SSLMode] {
			return fmt.Errorf("invalid DB_SSL_MODE: %s (must be disable, require, verify-ca, or verify-full)", c.SSLMode)
		}

	default:
		return fmt.Errorf("unsupported database driver: %s (must be sqlite or postgres)", c.Driver)
	}

	return nil
}

// Connect establishes a database connection based on the configuration
func Connect(config *Config) (*sql.DB, error) {
	var dsn string

	switch config.Driver {
	case "sqlite", "sqlite3":
		// Ensure the data directory exists
		dir := filepath.Dir(config.DBName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}

		// SQLite connection string with recommended settings
		dsn = fmt.Sprintf("%s?_journal_mode=WAL&_timeout=5000&_fk=true", config.DBName)
		log.Printf("Connecting to SQLite database: %s", config.DBName)

	case "postgres", "postgresql":
		// PostgreSQL connection string
		dsn = fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			config.Host,
			config.Port,
			config.User,
			config.Password,
			config.DBName,
			config.SSLMode,
		)
		log.Printf("Connecting to PostgreSQL database: %s@%s:%s/%s",
			config.User, config.Host, config.Port, config.DBName)

	default:
		return nil, fmt.Errorf("unsupported database driver: %s", config.Driver)
	}

	// Open database connection (use "sqlite3" for the sql.Open driver name)
	driverName := config.Driver
	if driverName == "sqlite" {
		driverName = "sqlite3"
	}
	if driverName == "postgresql" {
		driverName = "postgres"
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	log.Printf("Successfully connected to %s database", config.Driver)
	return db, nil
}

// getEnvOrDefault returns environment variable value or default if not set
func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
