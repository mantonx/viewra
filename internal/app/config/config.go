package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mantonx/viewra/internal/api"
	"github.com/mantonx/viewra/internal/application/transcode"
	"github.com/mantonx/viewra/internal/infrastructure/database"
	"github.com/mantonx/viewra/internal/infrastructure/system"
	pkgLogger "github.com/mantonx/viewra/internal/pkg/logger"
)

const (
	// DefaultDataDir is the default base directory for all ViewRA data.
	DefaultDataDir = "./data"
)

// Config holds all application configuration
type Config struct {
	Environment     string
	DataDir         string // Base directory for all data (default: "./data")
	Database        DatabaseConfig
	Server          ServerConfig
	Media           MediaConfig
	Transcode       TranscodeConfig
	Images          ImagesConfig
	Plugins         PluginsConfig
	Auth            AuthConfig
	SystemProfile   *system.Profile // Detected system profile for auto-tuning
	ShutdownTimeout time.Duration   // Graceful shutdown timeout
	FileConfig      *FileConfig     // Loaded from config.yaml (may be nil)
}

// DataPath returns the full path for a data subdirectory.
// Example: config.DataPath("cache/images") returns "{DataDir}/cache/images"
func (c *Config) DataPath(subpath string) string {
	return filepath.Join(c.DataDir, subpath)
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	JWTSecret          string        // Secret for signing JWTs (generated if empty)
	AccessTokenTTL     time.Duration // Access token lifetime (default: 15m)
	RefreshTokenTTL    time.Duration // Refresh token lifetime (default: 7d)
	MaxSessionsPerUser int           // Maximum concurrent sessions per user (0 = unlimited)
}

// DatabaseConfig holds database connection and migration configuration.
// Supports both SQLite and PostgreSQL databases.
type DatabaseConfig struct {
	Driver     string // "sqlite" or "postgres"
	Host       string // PostgreSQL host
	Port       string // PostgreSQL port
	User       string // PostgreSQL user
	Password   string // PostgreSQL password
	DBName     string // Database name (or SQLite file path)
	SSLMode    string // PostgreSQL SSL mode
	Migrations MigrationConfig
	Pool       PoolConfig // Connection pool settings
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

	// Scan performance
	ScanParallelWalkers  int // Number of concurrent directory walkers (0 = sequential, >0 = parallel)
	ScanProgressInterval int // Log progress every N files discovered (0 = disabled)

	// Scan job cleanup
	ScanJobRetentionMinutes int // How many minutes to keep completed/failed scan jobs before cleanup

	// NOTE: Automatic periodic scanning has been replaced by real-time filesystem monitoring.
	// See ADR-026: Library Filesystem Monitoring. These fields are deprecated and will be removed.
	AutoScanEnabled  bool   // Deprecated: Use filesystem monitoring instead
	AutoScanInterval string // Deprecated: Use filesystem monitoring instead
}

// TranscodeConfig holds transcode cleanup policies and thresholds.
type TranscodeConfig struct {
	CleanupEnabled       bool
	DiskThresholdPercent int
	DiskWarningPercent   int
	MinFreeSpaceGB       int64
	MaxAgeDays           int
	MaxIdleDays          int
	MaxStorageGB         int64
	CleanupBatchSize     int
	KeepFailedHours      int
}

// ImagesConfig holds image cache and processing configuration.
type ImagesConfig struct {
	CacheDir string
}

// PluginsConfig holds plugin system configuration.
type PluginsConfig struct {
	// Dir is the directory containing plugin binaries.
	// Defaults to "data/plugins" relative to the working directory.
	Dir string

	// StorageDir is the base directory for plugin data storage.
	// Each plugin gets its own subdirectory for databases and caches.
	// Defaults to "data/plugins/storage" relative to the working directory.
	StorageDir string

	// Enabled controls whether external plugins are loaded.
	// Built-in enrichers (NFO, local-images) always run regardless of this setting.
	Enabled bool
}

// Load reads configuration from environment variables with sensible defaults.
// Load order: Environment variables → Config file → Defaults
func Load() (*Config, error) {
	env := getEnv("ENVIRONMENT", "development")
	logger := pkgLogger.DefaultIfNil(nil)

	// Load DataDir first as other configs depend on it
	dataDir := loadDataDir(logger)

	// Ensure data directory exists before loading config file
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory %q: %w", dataDir, err)
	}

	// Load config file (if it exists)
	fileConfig, err := LoadConfigFile(dataDir)
	if err != nil {
		logger.Warn("Failed to load config file, using defaults", "error", err)
	}

	// Merge with environment variables (env vars take precedence)
	if fileConfig != nil {
		fileConfig.MergeWithEnv()
	}

	config := &Config{
		Environment:     env,
		DataDir:         dataDir,
		Database:        loadDatabaseConfig(dataDir, fileConfig),
		Server:          loadServerConfig(logger),
		Media:           loadMediaConfig(logger, dataDir),
		Transcode:       loadTranscodeConfig(logger),
		Images:          loadImagesConfig(dataDir),
		Plugins:         loadPluginsConfig(dataDir),
		Auth:            loadAuthConfig(logger),
		ShutdownTimeout: loadShutdownTimeout(fileConfig),
		FileConfig:      fileConfig,
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// loadShutdownTimeout loads the graceful shutdown timeout.
func loadShutdownTimeout(fileConfig *FileConfig) time.Duration {
	// Check environment variable first
	if envTimeout := os.Getenv("SHUTDOWN_TIMEOUT"); envTimeout != "" {
		if d, err := time.ParseDuration(envTimeout); err == nil {
			return d
		}
	}

	// Check file config
	if fileConfig != nil {
		return fileConfig.GetShutdownTimeout()
	}

	// Default
	return 30 * time.Second
}

// loadDataDir loads the base data directory from environment.
// All other data paths derive from this directory.
func loadDataDir(logger *slog.Logger) string {
	dataDir := getEnv("DATA_DIR", DefaultDataDir)

	// Convert to absolute path for consistency
	if !filepath.IsAbs(dataDir) {
		absPath, err := filepath.Abs(dataDir)
		if err != nil {
			logger.Warn("Failed to resolve absolute path for DATA_DIR, using as-is",
				"data_dir", dataDir,
				"error", err)
		} else {
			dataDir = absPath
		}
	}

	logger.Info("Using data directory", "path", dataDir)
	return dataDir
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

	// Auth validation - require JWT_SECRET in production
	if c.IsProduction() && c.Auth.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET environment variable is required in production mode")
	}

	return nil
}

// IsProduction returns true if running in production mode.
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// loadDatabaseConfig loads database configuration.
// Load order: Environment variables → Config file → Defaults
func loadDatabaseConfig(dataDir string, fileConfig *FileConfig) DatabaseConfig {
	migrationConfig := database.LoadMigrationConfigFromEnv()

	// Start with defaults
	driver := "sqlite"
	host := "localhost"
	port := "5432"
	user := "viewra"
	password := ""
	dbName := filepath.Join(dataDir, "viewra.db")
	sslMode := "disable"
	pool := DefaultPoolConfig()

	// Layer 1: Apply file config if present
	if fileConfig != nil {
		if fileConfig.Database.Driver != "" {
			driver = fileConfig.Database.Driver
		}

		if driver == "sqlite" || driver == "sqlite3" {
			if fileConfig.Database.SQLite.Path != "" {
				path := fileConfig.Database.SQLite.Path
				// Make relative paths relative to dataDir
				if !filepath.IsAbs(path) {
					path = filepath.Join(dataDir, path)
				}
				dbName = path
			}
		} else if driver == "postgres" || driver == "postgresql" {
			pg := fileConfig.Database.Postgres
			if pg.Host != "" {
				host = pg.Host
			}
			if pg.Port > 0 {
				port = strconv.Itoa(pg.Port)
			}
			if pg.User != "" {
				user = pg.User
			}
			if pg.Database != "" {
				dbName = pg.Database
			}
			if pg.SSLMode != "" {
				sslMode = pg.SSLMode
			}
			// Pool settings from file
			pool = fileConfig.GetPoolConfig()
		}

		// Auto-migrate from file config
		if fileConfig.Database.AutoMigrate != nil {
			migrationConfig.AutoMigrate = *fileConfig.Database.AutoMigrate
		}
	}

	// Layer 2: Apply environment variables (highest priority)
	if envDriver := os.Getenv("DB_DRIVER"); envDriver != "" {
		driver = envDriver
	}

	if driver == "sqlite" || driver == "sqlite3" {
		if envPath := os.Getenv("DB_PATH"); envPath != "" {
			dbName = envPath
		}
	} else if driver == "postgres" || driver == "postgresql" {
		if envHost := os.Getenv("DB_HOST"); envHost != "" {
			host = envHost
		}
		if envPort := os.Getenv("DB_PORT"); envPort != "" {
			port = envPort
		}
		if envUser := os.Getenv("DB_USER"); envUser != "" {
			user = envUser
		}
		if envName := os.Getenv("DB_NAME"); envName != "" {
			dbName = envName
		}
		if envSSL := os.Getenv("DB_SSL_MODE"); envSSL != "" {
			sslMode = envSSL
		}
	}

	// Password always from environment (never stored in file)
	password = os.Getenv("DB_PASSWORD")

	// Pool settings from environment
	if envMaxOpen := os.Getenv("DB_MAX_OPEN_CONNS"); envMaxOpen != "" {
		if v := getEnvInt("DB_MAX_OPEN_CONNS", 0); v > 0 {
			pool.MaxOpenConns = v
		}
	}
	if envMaxIdle := os.Getenv("DB_MAX_IDLE_CONNS"); envMaxIdle != "" {
		if v := getEnvInt("DB_MAX_IDLE_CONNS", 0); v > 0 {
			pool.MaxIdleConns = v
		}
	}
	if envLifetime := os.Getenv("DB_CONN_MAX_LIFETIME"); envLifetime != "" {
		if d, err := time.ParseDuration(envLifetime); err == nil {
			pool.ConnMaxLifetime = d
		}
	}
	if envIdleTime := os.Getenv("DB_CONN_MAX_IDLE_TIME"); envIdleTime != "" {
		if d, err := time.ParseDuration(envIdleTime); err == nil {
			pool.ConnMaxIdleTime = d
		}
	}

	return DatabaseConfig{
		Driver:   driver,
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		DBName:   dbName,
		SSLMode:  sslMode,
		Pool:     pool,
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
	corsCredentials := getEnvBool("CORS_ALLOW_CREDENTIALS", true)

	return ServerConfig{
		Port:                 apiConfig.Port,
		AllowedBasePaths:     apiConfig.Browser.AllowedBasePaths,
		DefaultBasePath:      apiConfig.Browser.DefaultBasePath,
		CORSAllowedOrigins:   corsOrigins,
		CORSAllowCredentials: corsCredentials,
	}
}

// loadMediaConfig loads media processing configuration from environment.
// Uses dataDir for default transcode output path.
func loadMediaConfig(logger *slog.Logger, dataDir string) MediaConfig {
	// Default transcode output dir derives from dataDir
	defaultTranscodeDir := filepath.Join(dataDir, "cache", "transcodes")

	return MediaConfig{
		TranscodeOutputDir:      getEnv("TRANSCODE_OUTPUT_DIR", defaultTranscodeDir),
		TranscodeWorkers:        getEnvIntWithLog(logger, "TRANSCODE_WORKERS", 8),
		TranscodePollInterval:   getEnvDurationWithLog(logger, "TRANSCODE_POLL_INTERVAL", 10*time.Second),
		TranscodeIdleTimeout:    getEnvDurationWithLog(logger, "TRANSCODE_IDLE_TIMEOUT", 5*time.Minute),
		ScanTimeout:             getEnvDurationWithLog(logger, "SCAN_TIMEOUT", 24*time.Hour), // Default 24 hours for large libraries
		ScanParallelWalkers:     getEnvIntWithLog(logger, "SCAN_PARALLEL_WALKERS", 10),       // 10 concurrent directory walkers for network storage
		ScanProgressInterval:    getEnvIntWithLog(logger, "SCAN_PROGRESS_INTERVAL", 1000),    // Log every 1000 files discovered
		ScanJobRetentionMinutes: getEnvIntWithLog(logger, "SCAN_JOB_RETENTION_MINUTES", 30),  // Keep scan jobs for 30 minutes by default
		// Deprecated: Filesystem monitoring is now the primary discovery mechanism (ADR-026)
		AutoScanEnabled:  false,
		AutoScanInterval: "",
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

// loadImagesConfig loads image processing configuration from environment.
// Uses dataDir for default cache path.
func loadImagesConfig(dataDir string) ImagesConfig {
	defaultCacheDir := filepath.Join(dataDir, "cache", "images")
	return ImagesConfig{
		CacheDir: getEnv("IMAGE_CACHE_DIR", defaultCacheDir),
	}
}

// loadAuthConfig loads authentication configuration from environment
func loadAuthConfig(logger *slog.Logger) AuthConfig {
	return AuthConfig{
		JWTSecret:          getEnv("JWT_SECRET", ""), // Will be generated if empty
		AccessTokenTTL:     getEnvDurationWithLog(logger, "ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:    getEnvDurationWithLog(logger, "REFRESH_TOKEN_TTL", 7*24*time.Hour),
		MaxSessionsPerUser: getEnvIntWithLog(logger, "MAX_SESSIONS_PER_USER", 10),
	}
}

// loadPluginsConfig loads plugin system configuration from environment.
// Uses dataDir for default plugin paths.
func loadPluginsConfig(dataDir string) PluginsConfig {
	defaultPluginsDir := filepath.Join(dataDir, "plugins")
	defaultStorageDir := filepath.Join(dataDir, "plugins", "storage")
	return PluginsConfig{
		Dir:        getEnv("PLUGINS_DIR", defaultPluginsDir),
		StorageDir: getEnv("PLUGINS_STORAGE_DIR", defaultStorageDir),
		Enabled:    getEnvBool("PLUGINS_ENABLED", true),
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
