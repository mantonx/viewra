package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigFileName is the name of the configuration file.
const ConfigFileName = "config.yaml"

// FileConfig represents the structure of the YAML configuration file.
// This is the on-disk format, which is then merged with environment variables.
type FileConfig struct {
	Database DatabaseFileConfig `yaml:"database"`
	Server   ServerFileConfig   `yaml:"server"`
}

// DatabaseFileConfig represents database settings in the config file.
type DatabaseFileConfig struct {
	Driver      string             `yaml:"driver"`       // "sqlite" or "postgres"
	AutoMigrate *bool              `yaml:"auto_migrate"` // Whether to run migrations on startup
	SQLite      SQLiteFileConfig   `yaml:"sqlite"`
	Postgres    PostgresFileConfig `yaml:"postgres"`
}

// SQLiteFileConfig represents SQLite-specific settings.
type SQLiteFileConfig struct {
	Path string `yaml:"path"` // Database file path (relative to data dir or absolute)
}

// PostgresFileConfig represents PostgreSQL-specific settings.
type PostgresFileConfig struct {
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	User            string `yaml:"user"`
	Database        string `yaml:"database"`
	SSLMode         string `yaml:"ssl_mode"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime"`  // Duration string, e.g. "5m"
	ConnMaxIdleTime string `yaml:"conn_max_idle_time"` // Duration string, e.g. "1m"
}

// ServerFileConfig represents server settings in the config file.
type ServerFileConfig struct {
	ShutdownTimeout string `yaml:"shutdown_timeout"` // Duration string, e.g. "30s"
}

// PoolConfig holds database connection pool settings.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DefaultPoolConfig returns sensible default pool settings.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}
}

// LoadConfigFile loads configuration from a YAML file.
// Returns nil if the file doesn't exist (not an error, just means use defaults).
func LoadConfigFile(dataDir string) (*FileConfig, error) {
	configPath := filepath.Join(dataDir, ConfigFileName)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Config file doesn't exist - that's fine, return nil
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config FileConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// SaveConfigFile saves configuration to a YAML file.
func SaveConfigFile(dataDir string, config *FileConfig) error {
	configPath := filepath.Join(dataDir, ConfigFileName)

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write with a header comment
	header := []byte("# Viewra Configuration\n# Environment variables take precedence over this file.\n# Password must be set via DB_PASSWORD environment variable.\n\n")
	fullData := append(header, data...)

	if err := os.WriteFile(configPath, fullData, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ConfigFileExists checks if a config file exists in the data directory.
func ConfigFileExists(dataDir string) bool {
	configPath := filepath.Join(dataDir, ConfigFileName)
	_, err := os.Stat(configPath)
	return err == nil
}

// GetConfigFilePath returns the full path to the config file.
func GetConfigFilePath(dataDir string) string {
	return filepath.Join(dataDir, ConfigFileName)
}

// ParseDuration parses a duration string with a default value.
func ParseDuration(s string, defaultVal time.Duration) time.Duration {
	if s == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultVal
	}
	return d
}

// MergeWithEnv merges file config with environment variables.
// Environment variables take precedence.
func (fc *FileConfig) MergeWithEnv() {
	// Database driver
	if envDriver := os.Getenv("DB_DRIVER"); envDriver != "" {
		fc.Database.Driver = envDriver
	}

	// SQLite settings
	if envPath := os.Getenv("DB_PATH"); envPath != "" {
		fc.Database.SQLite.Path = envPath
	}

	// PostgreSQL settings
	if envHost := os.Getenv("DB_HOST"); envHost != "" {
		fc.Database.Postgres.Host = envHost
	}
	if envPort := os.Getenv("DB_PORT"); envPort != "" {
		if port := parseIntOrZero(envPort); port > 0 {
			fc.Database.Postgres.Port = port
		}
	}
	if envUser := os.Getenv("DB_USER"); envUser != "" {
		fc.Database.Postgres.User = envUser
	}
	if envName := os.Getenv("DB_NAME"); envName != "" {
		fc.Database.Postgres.Database = envName
	}
	if envSSL := os.Getenv("DB_SSL_MODE"); envSSL != "" {
		fc.Database.Postgres.SSLMode = envSSL
	}

	// Connection pool settings from env
	if envMaxOpen := os.Getenv("DB_MAX_OPEN_CONNS"); envMaxOpen != "" {
		if v := parseIntOrZero(envMaxOpen); v > 0 {
			fc.Database.Postgres.MaxOpenConns = v
		}
	}
	if envMaxIdle := os.Getenv("DB_MAX_IDLE_CONNS"); envMaxIdle != "" {
		if v := parseIntOrZero(envMaxIdle); v > 0 {
			fc.Database.Postgres.MaxIdleConns = v
		}
	}
	if envLifetime := os.Getenv("DB_CONN_MAX_LIFETIME"); envLifetime != "" {
		fc.Database.Postgres.ConnMaxLifetime = envLifetime
	}
	if envIdleTime := os.Getenv("DB_CONN_MAX_IDLE_TIME"); envIdleTime != "" {
		fc.Database.Postgres.ConnMaxIdleTime = envIdleTime
	}

	// Auto-migrate setting
	if envAutoMigrate := os.Getenv("AUTO_MIGRATE"); envAutoMigrate != "" {
		val := envAutoMigrate == "true" || envAutoMigrate == "1"
		fc.Database.AutoMigrate = &val
	}

	// Server settings
	if envShutdown := os.Getenv("SHUTDOWN_TIMEOUT"); envShutdown != "" {
		fc.Server.ShutdownTimeout = envShutdown
	}
}

// GetPoolConfig returns the connection pool configuration with defaults applied.
func (fc *FileConfig) GetPoolConfig() PoolConfig {
	defaults := DefaultPoolConfig()

	pool := PoolConfig{
		MaxOpenConns:    fc.Database.Postgres.MaxOpenConns,
		MaxIdleConns:    fc.Database.Postgres.MaxIdleConns,
		ConnMaxLifetime: ParseDuration(fc.Database.Postgres.ConnMaxLifetime, defaults.ConnMaxLifetime),
		ConnMaxIdleTime: ParseDuration(fc.Database.Postgres.ConnMaxIdleTime, defaults.ConnMaxIdleTime),
	}

	// Apply defaults for zero values
	if pool.MaxOpenConns <= 0 {
		pool.MaxOpenConns = defaults.MaxOpenConns
	}
	if pool.MaxIdleConns <= 0 {
		pool.MaxIdleConns = defaults.MaxIdleConns
	}

	return pool
}

// GetShutdownTimeout returns the server shutdown timeout with a default.
func (fc *FileConfig) GetShutdownTimeout() time.Duration {
	return ParseDuration(fc.Server.ShutdownTimeout, 30*time.Second)
}

// ToMinimalYAML generates a minimal YAML config for the current database settings.
// This is used when saving after a migration.
func GenerateConfigForDatabase(driver string, sqlitePath string, pgHost string, pgPort int, pgUser, pgDatabase, pgSSLMode string) *FileConfig {
	config := &FileConfig{
		Database: DatabaseFileConfig{
			Driver: driver,
		},
	}

	switch driver {
	case "sqlite":
		config.Database.SQLite = SQLiteFileConfig{
			Path: sqlitePath,
		}
	case "postgres":
		config.Database.Postgres = PostgresFileConfig{
			Host:     pgHost,
			Port:     pgPort,
			User:     pgUser,
			Database: pgDatabase,
			SSLMode:  pgSSLMode,
		}
	}

	return config
}

func parseIntOrZero(s string) int {
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}
