package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the complete application configuration
type Config struct {
	// Server configuration
	Server ServerConfig `yaml:"server" json:"server"`

	// Database configuration
	Database DatabaseFullConfig `yaml:"database" json:"database"`

	// Asset management configuration
	Assets AssetConfig `yaml:"assets" json:"assets"`

	// Scanner configuration
	Scanner ScannerConfig `yaml:"scanner" json:"scanner"`

	// Plugin configuration
	Plugins PluginConfig `yaml:"plugins" json:"plugins"`

	// Library-specific Plugin Configuration
	LibraryPluginRestrictions map[string]LibraryPluginSettings `yaml:"library_plugin_restrictions" json:"library_plugin_restrictions"`

	// Logging configuration
	Logging LoggingConfig `yaml:"logging" json:"logging"`

	// Security configuration
	Security SecurityConfig `yaml:"security" json:"security"`

	// Performance configuration
	Performance PerformanceConfig `yaml:"performance" json:"performance"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Host           string        `yaml:"host" json:"host" env:"VIEWRA_HOST" default:"0.0.0.0"`
	Port           int           `yaml:"port" json:"port" env:"VIEWRA_PORT" default:"8080"`
	ReadTimeout    time.Duration `yaml:"read_timeout" json:"read_timeout" env:"VIEWRA_READ_TIMEOUT" default:"30s"`
	WriteTimeout   time.Duration `yaml:"write_timeout" json:"write_timeout" env:"VIEWRA_WRITE_TIMEOUT" default:"30s"`
	MaxHeaderBytes int           `yaml:"max_header_bytes" json:"max_header_bytes" env:"VIEWRA_MAX_HEADER_BYTES" default:"1048576"`
	EnableCORS     bool          `yaml:"enable_cors" json:"enable_cors" env:"VIEWRA_ENABLE_CORS" default:"true"`
	TrustedProxies []string      `yaml:"trusted_proxies" json:"trusted_proxies" env:"VIEWRA_TRUSTED_PROXIES"`
}

// DatabaseFullConfig extends the basic database config with more options
type DatabaseFullConfig struct {
	Type            string        `yaml:"type" json:"type" env:"DATABASE_TYPE" default:"sqlite"`
	URL             string        `yaml:"url" json:"url" env:"DATABASE_URL"`
	Host            string        `yaml:"host" json:"host" env:"POSTGRES_HOST" default:"localhost"`
	Port            int           `yaml:"port" json:"port" env:"POSTGRES_PORT" default:"5432"`
	Username        string        `yaml:"username" json:"username" env:"POSTGRES_USER" default:"viewra"`
	Password        string        `yaml:"password" json:"password" env:"POSTGRES_PASSWORD"`
	Database        string        `yaml:"database" json:"database" env:"POSTGRES_DB" default:"viewra"`
	DataDir         string        `yaml:"data_dir" json:"data_dir" env:"VIEWRA_DATA_DIR" default:"/app/viewra-data"`
	DatabasePath    string        `yaml:"database_path" json:"database_path" env:"VIEWRA_DATABASE_PATH"`
	MaxOpenConns    int           `yaml:"max_open_conns" json:"max_open_conns" env:"DB_MAX_OPEN_CONNS" default:"100"`
	MaxIdleConns    int           `yaml:"max_idle_conns" json:"max_idle_conns" env:"DB_MAX_IDLE_CONNS" default:"20"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME" default:"2h"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" json:"conn_max_idle_time" env:"DB_CONN_MAX_IDLE_TIME" default:"30m"`
	EnableMetrics   bool          `yaml:"enable_metrics" json:"enable_metrics" env:"DB_ENABLE_METRICS" default:"true"`
	LogQueries      bool          `yaml:"log_queries" json:"log_queries" env:"DB_LOG_QUERIES" default:"false"`
}

// AssetConfig holds asset management configuration
type AssetConfig struct {
	DataDir          string        `yaml:"data_dir" json:"data_dir" env:"VIEWRA_ASSETS_DIR"`
	MaxFileSize      int64         `yaml:"max_file_size" json:"max_file_size" env:"VIEWRA_MAX_ASSET_SIZE" default:"52428800"`
	DefaultQuality   int           `yaml:"default_quality" json:"default_quality" env:"VIEWRA_ASSET_QUALITY" default:"95"`
	EnableWebP       bool          `yaml:"enable_webp" json:"enable_webp" env:"VIEWRA_ENABLE_WEBP" default:"true"`
	EnableThumbnails bool          `yaml:"enable_thumbnails" json:"enable_thumbnails" env:"VIEWRA_ENABLE_THUMBNAILS" default:"true"`
	ThumbnailSizes   []int         `yaml:"thumbnail_sizes" json:"thumbnail_sizes" env:"VIEWRA_THUMBNAIL_SIZES"`
	CacheDuration    time.Duration `yaml:"cache_duration" json:"cache_duration" env:"VIEWRA_ASSET_CACHE_DURATION" default:"24h"`
	CleanupInterval  time.Duration `yaml:"cleanup_interval" json:"cleanup_interval" env:"VIEWRA_ASSET_CLEANUP_INTERVAL" default:"6h"`
}

// ScannerConfig holds scanner configuration
type ScannerConfig struct {
	ParallelScanning  bool          `yaml:"parallel_scanning" json:"parallel_scanning" env:"VIEWRA_PARALLEL_SCANNING" default:"true"`
	WorkerCount       int           `yaml:"worker_count" json:"worker_count" env:"VIEWRA_WORKER_COUNT" default:"0"`
	BatchSize         int           `yaml:"batch_size" json:"batch_size" env:"VIEWRA_BATCH_SIZE" default:"50"`
	ChannelBufferSize int           `yaml:"channel_buffer_size" json:"channel_buffer_size" env:"VIEWRA_CHANNEL_BUFFER_SIZE" default:"100"`
	SmartHashEnabled  bool          `yaml:"smart_hash_enabled" json:"smart_hash_enabled" env:"VIEWRA_SMART_HASH" default:"true"`
	AsyncMetadata     bool          `yaml:"async_metadata" json:"async_metadata" env:"VIEWRA_ASYNC_METADATA" default:"true"`
	MetadataWorkers   int           `yaml:"metadata_workers" json:"metadata_workers" env:"VIEWRA_METADATA_WORKERS" default:"2"`
	ScanInterval      time.Duration `yaml:"scan_interval" json:"scan_interval" env:"VIEWRA_SCAN_INTERVAL" default:"1h"`
	AutoScanEnabled   bool          `yaml:"auto_scan_enabled" json:"auto_scan_enabled" env:"VIEWRA_AUTO_SCAN" default:"false"`
	IgnorePatterns    []string      `yaml:"ignore_patterns" json:"ignore_patterns" env:"VIEWRA_IGNORE_PATTERNS"`
	MaxFileSize       int64         `yaml:"max_file_size" json:"max_file_size" env:"VIEWRA_MAX_SCAN_FILE_SIZE" default:"10737418240"`
}

// PluginConfig holds plugin system configuration
type PluginConfig struct {
	PluginDir            string        `yaml:"plugin_dir" json:"plugin_dir" env:"PLUGIN_DIR" default:"./data/plugins"`
	EnableHotReload      bool          `yaml:"enable_hot_reload" json:"enable_hot_reload" env:"VIEWRA_PLUGIN_HOT_RELOAD" default:"true"`
	DefaultEnabled       bool          `yaml:"default_enabled" json:"default_enabled" env:"VIEWRA_PLUGINS_DEFAULT_ENABLED" default:"false"`
	EnrichmentEnabled    bool          `yaml:"enrichment_enabled" json:"enrichment_enabled" env:"VIEWRA_ENRICHMENT_ENABLED" default:"true"`
	RespectDefaultConfig bool          `yaml:"respect_default_config" json:"respect_default_config" env:"VIEWRA_PLUGIN_RESPECT_DEFAULT" default:"true"`
	MaxExecutionTime     time.Duration `yaml:"max_execution_time" json:"max_execution_time" env:"VIEWRA_PLUGIN_MAX_EXEC_TIME" default:"30s"`
	EnableSandbox        bool          `yaml:"enable_sandbox" json:"enable_sandbox" env:"VIEWRA_PLUGIN_SANDBOX" default:"true"`
	MemoryLimit          int64         `yaml:"memory_limit" json:"memory_limit" env:"VIEWRA_PLUGIN_MEMORY_LIMIT" default:"536870912"`
	AllowNetworkAccess   bool          `yaml:"allow_network_access" json:"allow_network_access" env:"VIEWRA_PLUGIN_NETWORK" default:"true"`
	AllowFileSystemWrite bool          `yaml:"allow_filesystem_write" json:"allow_filesystem_write" env:"VIEWRA_PLUGIN_FS_WRITE" default:"false"`
}

// LibraryPluginSettings defines plugin settings for a specific library type
type LibraryPluginSettings struct {
	CorePlugins          CorePluginSettings       `yaml:"core_plugins" json:"core_plugins"`
	EnrichmentPlugins    EnrichmentPluginSettings `yaml:"enrichment_plugins" json:"enrichment_plugins"`
	FileTypeRestrictions FileTypeRestrictions     `yaml:"file_type_restrictions" json:"file_type_restrictions"`
	SharedPlugins        SharedPluginSettings     `yaml:"shared_plugins" json:"shared_plugins"`
}

// CorePluginSettings defines core plugin configuration
type CorePluginSettings struct {
	MetadataExtractors []string `yaml:"metadata_extractors" json:"metadata_extractors"`
	StructureParsers   []string `yaml:"structure_parsers" json:"structure_parsers"`
	TechnicalAnalyzers []string `yaml:"technical_analyzers" json:"technical_analyzers"`
}

// EnrichmentPluginSettings defines enrichment plugin configuration
type EnrichmentPluginSettings struct {
	Enabled           bool     `yaml:"enabled" json:"enabled"`
	AutoEnrich        bool     `yaml:"auto_enrich" json:"auto_enrich"`
	AllowedPlugins    []string `yaml:"allowed_plugins" json:"allowed_plugins"`
	DisallowedPlugins []string `yaml:"disallowed_plugins" json:"disallowed_plugins"`
}

// FileTypeRestrictions defines file type restrictions for plugins
type FileTypeRestrictions struct {
	AllowedExtensions    []string `yaml:"allowed_extensions" json:"allowed_extensions"`
	DisallowedExtensions []string `yaml:"disallowed_extensions" json:"disallowed_extensions"`
	MimeTypeFilters      []string `yaml:"mime_type_filters" json:"mime_type_filters"`
}

// SharedPluginSettings defines settings for plugins that can run across library types
type SharedPluginSettings struct {
	AllowTechnicalMetadata bool     `yaml:"allow_technical_metadata" json:"allow_technical_metadata"`
	AllowAssetExtraction   bool     `yaml:"allow_asset_extraction" json:"allow_asset_extraction"`
	SharedPluginNames      []string `yaml:"shared_plugin_names" json:"shared_plugin_names"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level            string `yaml:"level" json:"level" env:"VIEWRA_LOG_LEVEL" default:"info"`
	Format           string `yaml:"format" json:"format" env:"VIEWRA_LOG_FORMAT" default:"json"`
	Output           string `yaml:"output" json:"output" env:"VIEWRA_LOG_OUTPUT" default:"stdout"`
	FilePath         string `yaml:"file_path" json:"file_path" env:"VIEWRA_LOG_FILE"`
	MaxFileSize      int    `yaml:"max_file_size" json:"max_file_size" env:"VIEWRA_LOG_MAX_SIZE" default:"100"`
	MaxBackups       int    `yaml:"max_backups" json:"max_backups" env:"VIEWRA_LOG_MAX_BACKUPS" default:"3"`
	MaxAge           int    `yaml:"max_age" json:"max_age" env:"VIEWRA_LOG_MAX_AGE" default:"30"`
	EnableColors     bool   `yaml:"enable_colors" json:"enable_colors" env:"VIEWRA_LOG_COLORS" default:"true"`
	EnableStackTrace bool   `yaml:"enable_stack_trace" json:"enable_stack_trace" env:"VIEWRA_LOG_STACK_TRACE" default:"false"`
}

// SecurityConfig holds security-related configuration
type SecurityConfig struct {
	EnableAuthentication bool          `yaml:"enable_authentication" json:"enable_authentication" env:"VIEWRA_ENABLE_AUTH" default:"false"`
	JWTSecret            string        `yaml:"jwt_secret" json:"-" env:"VIEWRA_JWT_SECRET"`
	JWTExpiration        time.Duration `yaml:"jwt_expiration" json:"jwt_expiration" env:"VIEWRA_JWT_EXPIRATION" default:"24h"`
	SessionTimeout       time.Duration `yaml:"session_timeout" json:"session_timeout" env:"VIEWRA_SESSION_TIMEOUT" default:"30m"`
	RateLimitEnabled     bool          `yaml:"rate_limit_enabled" json:"rate_limit_enabled" env:"VIEWRA_RATE_LIMIT" default:"true"`
	RateLimitRPM         int           `yaml:"rate_limit_rpm" json:"rate_limit_rpm" env:"VIEWRA_RATE_LIMIT_RPM" default:"1000"`
	AllowedOrigins       []string      `yaml:"allowed_origins" json:"allowed_origins" env:"VIEWRA_ALLOWED_ORIGINS"`
	SecureHeaders        bool          `yaml:"secure_headers" json:"secure_headers" env:"VIEWRA_SECURE_HEADERS" default:"true"`
}

// PerformanceConfig holds performance-related configuration
type PerformanceConfig struct {
	EnablePprof              bool    `yaml:"enable_pprof" json:"enable_pprof" env:"VIEWRA_ENABLE_PPROF" default:"false"`
	EnableMetrics            bool    `yaml:"enable_metrics" json:"enable_metrics" env:"VIEWRA_ENABLE_METRICS" default:"true"`
	MaxConcurrentScans       int     `yaml:"max_concurrent_scans" json:"max_concurrent_scans" env:"VIEWRA_MAX_CONCURRENT_SCANS" default:"2"`
	GCPercent                int     `yaml:"gc_percent" json:"gc_percent" env:"GOGC" default:"100"`
	MaxProcs                 int     `yaml:"max_procs" json:"max_procs" env:"GOMAXPROCS" default:"0"`
	MemoryThreshold          float64 `yaml:"memory_threshold" json:"memory_threshold" env:"VIEWRA_MEMORY_THRESHOLD" default:"85.0"`
	CPUThreshold             float64 `yaml:"cpu_threshold" json:"cpu_threshold" env:"VIEWRA_CPU_THRESHOLD" default:"80.0"`
	EnableAdaptiveThrottling bool    `yaml:"enable_adaptive_throttling" json:"enable_adaptive_throttling" env:"VIEWRA_ADAPTIVE_THROTTLING" default:"true"`
}

// ConfigManager manages application configuration with hot-reload support
type ConfigManager struct {
	config     *Config
	configPath string
	watchers   []ConfigWatcher
	mu         sync.RWMutex
}

// ConfigWatcher is called when configuration changes
type ConfigWatcher func(oldConfig, newConfig *Config)

var (
	globalConfigManager *ConfigManager
	configOnce          sync.Once
)

// GetConfigManager returns the global configuration manager instance
func GetConfigManager() *ConfigManager {
	configOnce.Do(func() {
		globalConfigManager = NewConfigManager()
	})
	return globalConfigManager
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		config:   DefaultConfig(),
		watchers: make([]ConfigWatcher, 0),
	}
}

// DefaultConfig returns the default application configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:           "0.0.0.0",
			Port:           8080,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			MaxHeaderBytes: 1 << 20, // 1MB
			EnableCORS:     true,
			TrustedProxies: []string{},
		},
		Database: DatabaseFullConfig{
			Type:            "sqlite",
			DataDir:         "/app/viewra-data",
			MaxOpenConns:    100,
			MaxIdleConns:    20,
			ConnMaxLifetime: 2 * time.Hour,
			ConnMaxIdleTime: 30 * time.Minute,
			EnableMetrics:   true,
			LogQueries:      false,
		},
		Assets: AssetConfig{
			MaxFileSize:      50 * 1024 * 1024, // 50MB
			DefaultQuality:   95,
			EnableWebP:       true,
			EnableThumbnails: true,
			ThumbnailSizes:   []int{150, 300, 600},
			CacheDuration:    24 * time.Hour,
			CleanupInterval:  6 * time.Hour,
		},
		Scanner: ScannerConfig{
			ParallelScanning:  true,
			WorkerCount:       0, // Auto-detect
			BatchSize:         50,
			ChannelBufferSize: 100,
			SmartHashEnabled:  true,
			AsyncMetadata:     true,
			MetadataWorkers:   2,
			ScanInterval:      1 * time.Hour,
			AutoScanEnabled:   false,
			IgnorePatterns:    []string{".*", "Thumbs.db", ".DS_Store"},
			MaxFileSize:       10 * 1024 * 1024 * 1024, // 10GB
		},
		Plugins: PluginConfig{
			PluginDir:            "./data/plugins",
			EnableHotReload:      true,
			DefaultEnabled:       false,
			EnrichmentEnabled:    true,
			RespectDefaultConfig: true,
			MaxExecutionTime:     30 * time.Second,
			EnableSandbox:        true,
			MemoryLimit:          512 * 1024 * 1024, // 512MB
			AllowNetworkAccess:   true,
			AllowFileSystemWrite: false,
		},
		LibraryPluginRestrictions: map[string]LibraryPluginSettings{
			"music": {
				CorePlugins: CorePluginSettings{
					MetadataExtractors: []string{"music_metadata_extractor_plugin"},
					StructureParsers:   []string{},
					TechnicalAnalyzers: []string{"ffmpeg_probe_core_plugin"},
				},
				EnrichmentPlugins: EnrichmentPluginSettings{
					Enabled:           true,
					AutoEnrich:        true,
					AllowedPlugins:    []string{"musicbrainz_enricher", "audiodb_enricher"},
					DisallowedPlugins: []string{"tmdb_enricher"},
				},
				FileTypeRestrictions: FileTypeRestrictions{
					AllowedExtensions:    []string{".mp3", ".flac", ".m4a", ".aac", ".ogg", ".wav", ".wma"},
					DisallowedExtensions: []string{".mp4", ".mkv", ".avi", ".mov"},
					MimeTypeFilters:      []string{"audio/*"},
				},
				SharedPlugins: SharedPluginSettings{
					AllowTechnicalMetadata: true,
					AllowAssetExtraction:   true,
					SharedPluginNames:      []string{"ffmpeg_probe_core_plugin"},
				},
			},
			"tv": {
				CorePlugins: CorePluginSettings{
					MetadataExtractors: []string{"ffmpeg_probe_core_plugin"},
					StructureParsers:   []string{"tv_structure_parser_core_plugin"},
					TechnicalAnalyzers: []string{"ffmpeg_probe_core_plugin"},
				},
				EnrichmentPlugins: EnrichmentPluginSettings{
					Enabled:           true,
					AutoEnrich:        true,
					AllowedPlugins:    []string{"tmdb_enricher"},
					DisallowedPlugins: []string{"musicbrainz_enricher", "audiodb_enricher"},
				},
				FileTypeRestrictions: FileTypeRestrictions{
					AllowedExtensions:    []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v"},
					DisallowedExtensions: []string{".mp3", ".flac", ".m4a", ".aac"},
					MimeTypeFilters:      []string{"video/*"},
				},
				SharedPlugins: SharedPluginSettings{
					AllowTechnicalMetadata: true,
					AllowAssetExtraction:   true,
					SharedPluginNames:      []string{"ffmpeg_probe_core_plugin"},
				},
			},
			"movie": {
				CorePlugins: CorePluginSettings{
					MetadataExtractors: []string{"ffmpeg_probe_core_plugin"},
					StructureParsers:   []string{"movie_structure_parser_core_plugin"},
					TechnicalAnalyzers: []string{"ffmpeg_probe_core_plugin"},
				},
				EnrichmentPlugins: EnrichmentPluginSettings{
					Enabled:           true,
					AutoEnrich:        true,
					AllowedPlugins:    []string{"tmdb_enricher"},
					DisallowedPlugins: []string{"musicbrainz_enricher", "audiodb_enricher"},
				},
				FileTypeRestrictions: FileTypeRestrictions{
					AllowedExtensions:    []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v"},
					DisallowedExtensions: []string{".mp3", ".flac", ".m4a", ".aac"},
					MimeTypeFilters:      []string{"video/*"},
				},
				SharedPlugins: SharedPluginSettings{
					AllowTechnicalMetadata: true,
					AllowAssetExtraction:   true,
					SharedPluginNames:      []string{"ffmpeg_probe_core_plugin"},
				},
			},
		},
		Logging: LoggingConfig{
			Level:            "info",
			Format:           "json",
			Output:           "stdout",
			MaxFileSize:      100,
			MaxBackups:       3,
			MaxAge:           30,
			EnableColors:     true,
			EnableStackTrace: false,
		},
		Security: SecurityConfig{
			EnableAuthentication: false,
			JWTExpiration:        24 * time.Hour,
			SessionTimeout:       30 * time.Minute,
			RateLimitEnabled:     true,
			RateLimitRPM:         1000,
			AllowedOrigins:       []string{"*"},
			SecureHeaders:        true,
		},
		Performance: PerformanceConfig{
			EnablePprof:              false,
			EnableMetrics:            true,
			MaxConcurrentScans:       2,
			GCPercent:                100,
			MaxProcs:                 0, // Auto-detect
			MemoryThreshold:          85.0,
			CPUThreshold:             80.0,
			EnableAdaptiveThrottling: true,
		},
	}
}

// LoadConfig loads configuration from file and environment variables
func (cm *ConfigManager) LoadConfig(configPath string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	oldConfig := *cm.config
	cm.configPath = configPath

	// Start with default configuration
	newConfig := DefaultConfig()

	// Load from file if it exists
	if configPath != "" && fileExists(configPath) {
		if err := cm.loadFromFile(configPath, newConfig); err != nil {
			return fmt.Errorf("failed to load config from file: %w", err)
		}
		log.Printf("✅ Configuration loaded from file: %s", configPath)
	}

	// Override with environment variables
	if err := cm.loadFromEnv(newConfig); err != nil {
		return fmt.Errorf("failed to load config from environment: %w", err)
	}

	// Validate configuration
	if err := cm.validateConfig(newConfig); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Apply derived configurations
	cm.applyDerivedConfig(newConfig)

	cm.config = newConfig

	// Notify watchers of config change
	for _, watcher := range cm.watchers {
		go watcher(&oldConfig, newConfig)
	}

	log.Printf("✅ Configuration loaded successfully")
	return nil
}

// GetConfig returns the current configuration (thread-safe)
func (cm *ConfigManager) GetConfig() *Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Return a copy to prevent external modifications
	configCopy := *cm.config
	return &configCopy
}

// AddWatcher adds a configuration change watcher
func (cm *ConfigManager) AddWatcher(watcher ConfigWatcher) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.watchers = append(cm.watchers, watcher)
}

// SaveConfig saves the current configuration to file
func (cm *ConfigManager) SaveConfig() error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.configPath == "" {
		return fmt.Errorf("no config path set")
	}

	return cm.saveToFile(cm.configPath, cm.config)
}

// Helper methods

func (cm *ConfigManager) loadFromFile(path string, config *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return yaml.Unmarshal(data, config)
	case ".json":
		return json.Unmarshal(data, config)
	default:
		return fmt.Errorf("unsupported config file format: %s", ext)
	}
}

func (cm *ConfigManager) saveToFile(path string, config *Config) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(path))
	var data []byte
	var err error

	switch ext {
	case ".yaml", ".yml":
		data, err = yaml.Marshal(config)
	case ".json":
		data, err = json.MarshalIndent(config, "", "  ")
	default:
		return fmt.Errorf("unsupported config file format: %s", ext)
	}

	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (cm *ConfigManager) loadFromEnv(config *Config) error {
	return loadStructFromEnv(reflect.ValueOf(config).Elem())
}

func loadStructFromEnv(v reflect.Value) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		if !field.CanSet() {
			continue
		}

		// Handle nested structs recursively
		if field.Kind() == reflect.Struct {
			if err := loadStructFromEnv(field); err != nil {
				return err
			}
			continue
		}

		// Get environment variable name
		envTag := fieldType.Tag.Get("env")
		if envTag == "" {
			continue
		}

		// Get default value
		defaultTag := fieldType.Tag.Get("default")

		// Get environment value
		envValue := os.Getenv(envTag)
		if envValue == "" && defaultTag != "" {
			envValue = defaultTag
		}

		if envValue == "" {
			continue
		}

		// Set field value based on type
		if err := setFieldValue(field, envValue); err != nil {
			return fmt.Errorf("failed to set field %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

func setFieldValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			duration, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			field.SetInt(int64(duration))
		} else {
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			field.SetInt(intVal)
		}
	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		field.SetFloat(floatVal)
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(boolVal)
	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.String {
			values := strings.Split(value, ",")
			for i, v := range values {
				values[i] = strings.TrimSpace(v)
			}
			field.Set(reflect.ValueOf(values))
		} else if field.Type().Elem().Kind() == reflect.Int {
			stringValues := strings.Split(value, ",")
			intValues := make([]int, len(stringValues))
			for i, v := range stringValues {
				intVal, err := strconv.Atoi(strings.TrimSpace(v))
				if err != nil {
					return err
				}
				intValues[i] = intVal
			}
			field.Set(reflect.ValueOf(intValues))
		}
	default:
		return fmt.Errorf("unsupported field type: %v", field.Kind())
	}

	return nil
}

func (cm *ConfigManager) validateConfig(config *Config) error {
	// Basic validation
	if config.Server.Port < 1 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", config.Server.Port)
	}

	if config.Database.Type != "sqlite" && config.Database.Type != "postgres" {
		return fmt.Errorf("unsupported database type: %s", config.Database.Type)
	}

	if config.Scanner.WorkerCount < 0 {
		return fmt.Errorf("invalid worker count: %d", config.Scanner.WorkerCount)
	}

	if config.Assets.MaxFileSize <= 0 {
		return fmt.Errorf("invalid max file size: %d", config.Assets.MaxFileSize)
	}

	return nil
}

func (cm *ConfigManager) applyDerivedConfig(config *Config) {
	// Set derived database path if not explicitly set
	if config.Database.DatabasePath == "" && config.Database.Type == "sqlite" {
		config.Database.DatabasePath = filepath.Join(config.Database.DataDir, "viewra.db")
	}

	// Set derived asset data dir if not explicitly set
	if config.Assets.DataDir == "" {
		config.Assets.DataDir = filepath.Join(config.Database.DataDir, "assets")
	}

	// Auto-detect worker count if not set
	if config.Scanner.WorkerCount == 0 {
		// Use number of CPU cores, with reasonable limits
		config.Scanner.WorkerCount = min(max(1, getCPUCount()), 16)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func getCPUCount() int {
	return runtime.NumCPU()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Global convenience functions

// Get returns the current global configuration
func Get() *Config {
	return GetConfigManager().GetConfig()
}

// Load loads configuration from the specified path
func Load(configPath string) error {
	return GetConfigManager().LoadConfig(configPath)
}

// AddWatcher adds a global configuration watcher
func AddWatcher(watcher ConfigWatcher) {
	GetConfigManager().AddWatcher(watcher)
}

// Save saves the current configuration
func Save() error {
	return GetConfigManager().SaveConfig()
}
