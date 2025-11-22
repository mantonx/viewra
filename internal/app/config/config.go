package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mantonx/viewra/internal/api"
	"github.com/mantonx/viewra/internal/application/transcode"
	"github.com/mantonx/viewra/internal/infrastructure/database"
)

// Config holds all application configuration
type Config struct {
	Environment string
	Database    DatabaseConfig
	Server      ServerConfig
	Media       MediaConfig
	Transcode   TranscodeConfig
	Images      ImagesConfig
}

// DatabaseConfig holds database connection and migration configuration.
// Supports both SQLite and PostgreSQL databases.
type DatabaseConfig struct {
	Driver   string // "sqlite" or "postgres"
	Host     string // PostgreSQL host
	Port     string // PostgreSQL port
	User     string // PostgreSQL user
	Password string // PostgreSQL password
	DBName   string // Database name (or SQLite file path)
	SSLMode  string // PostgreSQL SSL mode
	Migrations MigrationConfig
}

// MigrationConfig controls database schema migrations.
type MigrationConfig struct {
	Enabled   bool
	SourceDir string
}

// ServerConfig holds HTTP server and path browsing configuration.
type ServerConfig struct {
	Port                 int
	AllowedBasePaths     []string
	DefaultBasePath      string
	CORSAllowedOrigins   []string
	CORSAllowCredentials bool
}

// MediaConfig holds media transcoding configuration including worker pool settings.
type MediaConfig struct {
	// Transcoding
	TranscodeOutputDir    string
	TranscodeWorkers      int
	TranscodePollInterval time.Duration
	TranscodeIdleTimeout  time.Duration

	// Operation timeouts
	ScanTimeout time.Duration // Maximum time for a library scan operation

	// Scan job cleanup
	ScanJobRetentionMinutes int // How many minutes to keep completed/failed scan jobs before cleanup
}

// TranscodeConfig holds transcode cleanup policies and thresholds.
type TranscodeConfig struct {
	CleanupEnabled        bool
	DiskThresholdPercent  int
	DiskWarningPercent    int
	MinFreeSpaceGB        int64
	MaxAgeDays            int
	MaxIdleDays           int
	MaxStorageGB          int64
	CleanupBatchSize      int
	KeepFailedHours       int
}

// ImagesConfig holds image cache and processing configuration.
type ImagesConfig struct {
	CacheDir string
}

// Load reads configuration from environment variables with sensible defaults
func Load() (*Config, error) {
	env := getEnv("ENVIRONMENT", "development")
	logger := slog.Default()

	config := &Config{
		Environment: env,
		Database:    loadDatabaseConfig(),
		Server:      loadServerConfig(logger),
		Media:       loadMediaConfig(logger),
		Transcode:   loadTranscodeConfig(logger),
		Images:      loadImagesConfig(),
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// Validate ensures all required configuration values are present and valid
func (c *Config) Validate() error {
	// Database validation
	if c.Database.Driver == "" {
		return fmt.Errorf("database driver is required")
	}
	if c.Database.Driver != "sqlite" && c.Database.Driver != "postgres" {
		return fmt.Errorf("database driver must be 'sqlite' or 'postgres', got %q", c.Database.Driver)
	}
	if c.Database.DBName == "" {
		return fmt.Errorf("database name is required")
	}

	// Server validation
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535, got %d", c.Server.Port)
	}

	// Media validation
	if c.Media.TranscodeWorkers < 1 {
		return fmt.Errorf("transcode workers must be at least 1, got %d", c.Media.TranscodeWorkers)
	}
	if c.Media.TranscodeWorkers > 20 {
		return fmt.Errorf("transcode workers should not exceed 20, got %d", c.Media.TranscodeWorkers)
	}
	if c.Media.TranscodePollInterval < 1*time.Second {
		return fmt.Errorf("transcode poll interval must be at least 1 second, got %v", c.Media.TranscodePollInterval)
	}
	if c.Media.TranscodeIdleTimeout < 1*time.Minute {
		return fmt.Errorf("transcode idle timeout must be at least 1 minute, got %v", c.Media.TranscodeIdleTimeout)
	}
	if c.Media.TranscodeOutputDir == "" {
		return fmt.Errorf("transcode output directory is required")
	}
	if c.Media.ScanTimeout < 1*time.Minute {
		return fmt.Errorf("scan timeout must be at least 1 minute, got %v", c.Media.ScanTimeout)
	}

	// Transcode cleanup validation
	if c.Transcode.DiskThresholdPercent < 0 || c.Transcode.DiskThresholdPercent > 100 {
		return fmt.Errorf("disk threshold percent must be between 0 and 100, got %d", c.Transcode.DiskThresholdPercent)
	}
	if c.Transcode.DiskWarningPercent < 0 || c.Transcode.DiskWarningPercent > 100 {
		return fmt.Errorf("disk warning percent must be between 0 and 100, got %d", c.Transcode.DiskWarningPercent)
	}
	if c.Transcode.DiskWarningPercent >= c.Transcode.DiskThresholdPercent {
		return fmt.Errorf("disk warning percent (%d) must be less than threshold percent (%d)",
			c.Transcode.DiskWarningPercent, c.Transcode.DiskThresholdPercent)
	}
	if c.Transcode.MinFreeSpaceGB < 0 {
		return fmt.Errorf("min free space must be non-negative, got %d GB", c.Transcode.MinFreeSpaceGB)
	}
	if c.Transcode.MaxAgeDays < 0 {
		return fmt.Errorf("max age days must be non-negative, got %d", c.Transcode.MaxAgeDays)
	}
	if c.Transcode.MaxIdleDays < 0 {
		return fmt.Errorf("max idle days must be non-negative, got %d", c.Transcode.MaxIdleDays)
	}
	if c.Transcode.MaxStorageGB < 0 {
		return fmt.Errorf("max storage GB must be non-negative, got %d", c.Transcode.MaxStorageGB)
	}
	if c.Transcode.CleanupBatchSize < 1 {
		return fmt.Errorf("cleanup batch size must be at least 1, got %d", c.Transcode.CleanupBatchSize)
	}
	if c.Transcode.KeepFailedHours < 0 {
		return fmt.Errorf("keep failed hours must be non-negative, got %d", c.Transcode.KeepFailedHours)
	}

	// Images validation
	if c.Images.CacheDir == "" {
		return fmt.Errorf("image cache directory is required")
	}

	return nil
}

// loadDatabaseConfig loads database configuration from environment
func loadDatabaseConfig() DatabaseConfig {
	// Delegate to existing database package
	dbConfig := database.LoadConfigFromEnv()
	migrationConfig := database.LoadMigrationConfigFromEnv()

	return DatabaseConfig{
		Driver:   dbConfig.Driver,
		Host:     dbConfig.Host,
		Port:     dbConfig.Port,
		User:     dbConfig.User,
		Password: dbConfig.Password,
		DBName:   dbConfig.DBName,
		SSLMode:  dbConfig.SSLMode,
		Migrations: MigrationConfig{
			Enabled:   migrationConfig.AutoMigrate,
			SourceDir: migrationConfig.MigrationsPath,
		},
	}
}

// loadServerConfig loads server configuration from environment
func loadServerConfig(logger *slog.Logger) ServerConfig {
	// Delegate to existing API package for server config
	apiConfig := api.DefaultServerConfig()

	if portStr := os.Getenv("PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			apiConfig.Port = port
		} else {
			logger.Warn("Invalid PORT environment variable, using default",
				"value", portStr,
				"error", err,
				"default", apiConfig.Port)
		}
	}

	// CORS configuration
	corsOrigins := getEnvStringSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:5173", "http://localhost:8080"})
	corsCredentials := getEnvBool("CORS_ALLOW_CREDENTIALS", false)

	return ServerConfig{
		Port:                 apiConfig.Port,
		AllowedBasePaths:     apiConfig.Browser.AllowedBasePaths,
		DefaultBasePath:      apiConfig.Browser.DefaultBasePath,
		CORSAllowedOrigins:   corsOrigins,
		CORSAllowCredentials: corsCredentials,
	}
}

// loadMediaConfig loads media processing configuration from environment
func loadMediaConfig(logger *slog.Logger) MediaConfig {
	return MediaConfig{
		TranscodeOutputDir:      getEnv("TRANSCODE_OUTPUT_DIR", "./data/cache/transcodes"),
		TranscodeWorkers:        getEnvIntWithLog(logger, "TRANSCODE_WORKERS", 8),
		TranscodePollInterval:   getEnvDurationWithLog(logger, "TRANSCODE_POLL_INTERVAL", 10*time.Second),
		TranscodeIdleTimeout:    getEnvDurationWithLog(logger, "TRANSCODE_IDLE_TIMEOUT", 5*time.Minute),
		ScanTimeout:             getEnvDurationWithLog(logger, "SCAN_TIMEOUT", 24*time.Hour), // Default 24 hours for large libraries
		ScanJobRetentionMinutes: getEnvIntWithLog(logger, "SCAN_JOB_RETENTION_MINUTES", 30), // Keep scan jobs for 30 minutes by default
	}
}

// loadTranscodeConfig loads transcode cleanup configuration from environment
func loadTranscodeConfig(logger *slog.Logger) TranscodeConfig {
	defaults := transcode.DefaultCleanupSchedulerConfig()

	return TranscodeConfig{
		CleanupEnabled:       getEnvBool("TRANSCODE_CLEANUP_ENABLED", defaults.Enabled),
		DiskThresholdPercent: getEnvIntWithLog(logger, "TRANSCODE_CLEANUP_DISK_THRESHOLD", defaults.DiskThresholdPercent),
		DiskWarningPercent:   getEnvIntWithLog(logger, "TRANSCODE_CLEANUP_DISK_WARNING", defaults.DiskWarningPercent),
		MinFreeSpaceGB:       getEnvInt64WithLog(logger, "TRANSCODE_MIN_FREE_SPACE_GB", defaults.MinFreeSpaceGB),
		MaxAgeDays:           getEnvIntWithLog(logger, "TRANSCODE_MAX_AGE_DAYS", defaults.MaxAgeHours/24),
		MaxIdleDays:          getEnvIntWithLog(logger, "TRANSCODE_MAX_IDLE_DAYS", defaults.MaxIdleHours/24),
		MaxStorageGB:         getEnvInt64WithLog(logger, "TRANSCODE_MAX_STORAGE_GB", defaults.MaxStorageGB),
		CleanupBatchSize:     getEnvIntWithLog(logger, "TRANSCODE_CLEANUP_BATCH_SIZE", defaults.CleanupBatchSize),
		KeepFailedHours:      getEnvIntWithLog(logger, "TRANSCODE_KEEP_FAILED_HOURS", defaults.KeepFailedHours),
	}
}

// loadImagesConfig loads image processing configuration from environment
func loadImagesConfig() ImagesConfig {
	return ImagesConfig{
		CacheDir: getEnv("IMAGE_CACHE_DIR", "./data/cache/images"),
	}
}

// ToCleanupSchedulerConfig converts TranscodeConfig to the internal CleanupSchedulerConfig format.
func (c *TranscodeConfig) ToCleanupSchedulerConfig() *transcode.CleanupSchedulerConfig {
	return &transcode.CleanupSchedulerConfig{
		Enabled:              c.CleanupEnabled,
		DiskThresholdPercent: c.DiskThresholdPercent,
		DiskWarningPercent:   c.DiskWarningPercent,
		MinFreeSpaceGB:       c.MinFreeSpaceGB,
		MaxAgeHours:          c.MaxAgeDays * 24,
		MaxIdleHours:         c.MaxIdleDays * 24,
		MaxStorageGB:         c.MaxStorageGB,
		CleanupBatchSize:     c.CleanupBatchSize,
		KeepFailedHours:      c.KeepFailedHours,
	}
}

// ToAPIServerConfig converts ServerConfig to the internal api.ServerConfig format.
func (c *ServerConfig) ToAPIServerConfig() api.ServerConfig {
	return api.ServerConfig{
		Port: c.Port,
		Browser: api.BrowserConfig{
			AllowedBasePaths: c.AllowedBasePaths,
			DefaultBasePath:  c.DefaultBasePath,
		},
		CORSAllowedOrigins:   c.CORSAllowedOrigins,
		CORSAllowCredentials: c.CORSAllowCredentials,
	}
}

// Helper functions for reading environment variables

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvIntWithLog(logger *slog.Logger, key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		} else {
			logger.Warn("Invalid integer environment variable, using default",
				"key", key,
				"value", value,
				"error", err,
				"default", defaultValue)
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvInt64WithLog(logger *slog.Logger, key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal
		} else {
			logger.Warn("Invalid int64 environment variable, using default",
				"key", key,
				"value", value,
				"error", err,
				"default", defaultValue)
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

func getEnvDurationWithLog(logger *slog.Logger, key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		} else {
			logger.Warn("Invalid duration environment variable, using default",
				"key", key,
				"value", value,
				"error", err,
				"default", defaultValue)
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Split by comma and trim whitespace
		var result []string
		for _, v := range strings.Split(value, ",") {
			trimmed := strings.TrimSpace(v)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}
