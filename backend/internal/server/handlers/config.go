package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/config"
)

// ConfigResponse represents the configuration response for API
type ConfigResponse struct {
	Message string        `json:"message,omitempty"`
	Config  *config.Config `json:"config,omitempty"`
	Section string        `json:"section,omitempty"`
	Data    interface{}   `json:"data,omitempty"`
}

// GetConfig returns the current application configuration
func GetConfig(c *gin.Context) {
	// Get current configuration
	cfg := config.Get()
	
	// Remove sensitive data before returning
	safeCfg := *cfg
	safeCfg.Security.JWTSecret = "[REDACTED]"
	safeCfg.Database.Password = "[REDACTED]"
	
	c.JSON(http.StatusOK, ConfigResponse{
		Message: "Configuration retrieved successfully",
		Config:  &safeCfg,
	})
}

// GetConfigSection returns a specific section of the configuration
func GetConfigSection(c *gin.Context) {
	section := c.Param("section")
	cfg := config.Get()
	
	var data interface{}
	
	switch section {
	case "server":
		data = cfg.Server
	case "database":
		// Remove sensitive data
		dbCfg := cfg.Database
		dbCfg.Password = "[REDACTED]"
		data = dbCfg
	case "assets":
		data = cfg.Assets
	case "scanner":
		data = cfg.Scanner
	case "plugins":
		data = cfg.Plugins
	case "logging":
		data = cfg.Logging
	case "security":
		// Remove sensitive data
		secCfg := cfg.Security
		secCfg.JWTSecret = "[REDACTED]"
		data = secCfg
	case "performance":
		data = cfg.Performance
	default:
		c.JSON(http.StatusBadRequest, ConfigResponse{
			Message: "Invalid configuration section",
		})
		return
	}
	
	c.JSON(http.StatusOK, ConfigResponse{
		Message: "Configuration section retrieved successfully",
		Section: section,
		Data:    data,
	})
}

// UpdateConfigSection updates a specific section of the configuration
func UpdateConfigSection(c *gin.Context) {
	section := c.Param("section")
	cfg := config.Get()
	
	// Parse the request body based on section
	switch section {
	case "server":
		var update config.ServerConfig
		if err := c.ShouldBindJSON(&update); err != nil {
			c.JSON(http.StatusBadRequest, ConfigResponse{
				Message: "Invalid server configuration: " + err.Error(),
			})
			return
		}
		cfg.Server = update
		
	case "assets":
		var update config.AssetConfig
		if err := c.ShouldBindJSON(&update); err != nil {
			c.JSON(http.StatusBadRequest, ConfigResponse{
				Message: "Invalid asset configuration: " + err.Error(),
			})
			return
		}
		cfg.Assets = update
		
	case "scanner":
		var update config.ScannerConfig
		if err := c.ShouldBindJSON(&update); err != nil {
			c.JSON(http.StatusBadRequest, ConfigResponse{
				Message: "Invalid scanner configuration: " + err.Error(),
			})
			return
		}
		cfg.Scanner = update
		
	case "plugins":
		var update config.PluginConfig
		if err := c.ShouldBindJSON(&update); err != nil {
			c.JSON(http.StatusBadRequest, ConfigResponse{
				Message: "Invalid plugin configuration: " + err.Error(),
			})
			return
		}
		cfg.Plugins = update
		
	case "logging":
		var update config.LoggingConfig
		if err := c.ShouldBindJSON(&update); err != nil {
			c.JSON(http.StatusBadRequest, ConfigResponse{
				Message: "Invalid logging configuration: " + err.Error(),
			})
			return
		}
		cfg.Logging = update
		
	case "performance":
		var update config.PerformanceConfig
		if err := c.ShouldBindJSON(&update); err != nil {
			c.JSON(http.StatusBadRequest, ConfigResponse{
				Message: "Invalid performance configuration: " + err.Error(),
			})
			return
		}
		cfg.Performance = update
		
	default:
		c.JSON(http.StatusBadRequest, ConfigResponse{
			Message: "Invalid or non-updatable configuration section",
		})
		return
	}
	
	// Note: In a real implementation, you would validate and apply the configuration
	// For now, we'll just return success
	c.JSON(http.StatusOK, ConfigResponse{
		Message: "Configuration section updated successfully (restart required for some changes)",
		Section: section,
	})
}

// ReloadConfig reloads the configuration from file
func ReloadConfig(c *gin.Context) {
	configPath := c.Query("path")
	if configPath == "" {
		configPath = "/app/viewra-data/viewra.yaml" // Default path
	}
	
	configManager := config.GetConfigManager()
	if err := configManager.LoadConfig(configPath); err != nil {
		c.JSON(http.StatusInternalServerError, ConfigResponse{
			Message: "Failed to reload configuration: " + err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, ConfigResponse{
		Message: "Configuration reloaded successfully from " + configPath,
	})
}

// SaveConfig saves the current configuration to file
func SaveConfig(c *gin.Context) {
	configManager := config.GetConfigManager()
	if err := configManager.SaveConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, ConfigResponse{
			Message: "Failed to save configuration: " + err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, ConfigResponse{
		Message: "Configuration saved successfully",
	})
}

// ValidateConfig validates the current configuration
func ValidateConfig(c *gin.Context) {
	cfg := config.Get()
	
	issues := []string{}
	
	// Validate server configuration
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		issues = append(issues, "Invalid server port: "+strconv.Itoa(cfg.Server.Port))
	}
	
	// Validate database configuration
	if cfg.Database.Type != "sqlite" && cfg.Database.Type != "postgres" {
		issues = append(issues, "Unsupported database type: "+cfg.Database.Type)
	}
	
	// Validate scanner configuration
	if cfg.Scanner.WorkerCount < 0 {
		issues = append(issues, "Invalid worker count: "+strconv.Itoa(cfg.Scanner.WorkerCount))
	}
	
	if cfg.Scanner.BatchSize <= 0 {
		issues = append(issues, "Invalid batch size: "+strconv.Itoa(cfg.Scanner.BatchSize))
	}
	
	// Validate asset configuration
	if cfg.Assets.MaxFileSize <= 0 {
		issues = append(issues, "Invalid max file size: "+strconv.FormatInt(cfg.Assets.MaxFileSize, 10))
	}
	
	if cfg.Assets.DefaultQuality < 1 || cfg.Assets.DefaultQuality > 100 {
		issues = append(issues, "Invalid default quality: "+strconv.Itoa(cfg.Assets.DefaultQuality))
	}
	
	// Validate performance configuration
	if cfg.Performance.MaxConcurrentScans <= 0 {
		issues = append(issues, "Invalid max concurrent scans: "+strconv.Itoa(cfg.Performance.MaxConcurrentScans))
	}
	
	if cfg.Performance.MemoryThreshold <= 0 || cfg.Performance.MemoryThreshold > 100 {
		issues = append(issues, "Invalid memory threshold: "+strconv.FormatFloat(cfg.Performance.MemoryThreshold, 'f', 1, 64))
	}
	
	// Validate plugin configuration
	if cfg.Plugins.MemoryLimit <= 0 {
		issues = append(issues, "Invalid plugin memory limit: "+strconv.FormatInt(cfg.Plugins.MemoryLimit, 10))
	}
	
	if len(issues) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Configuration validation failed",
			"issues":  issues,
			"valid":   false,
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Configuration is valid",
		"valid":   true,
	})
}

// GetConfigDefaults returns the default configuration
func GetConfigDefaults(c *gin.Context) {
	defaultCfg := config.DefaultConfig()
	
	// Remove sensitive defaults
	defaultCfg.Security.JWTSecret = ""
	defaultCfg.Database.Password = ""
	
	c.JSON(http.StatusOK, ConfigResponse{
		Message: "Default configuration retrieved successfully",
		Config:  defaultCfg,
	})
}

// GetConfigInfo returns information about the configuration system
func GetConfigInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Configuration system information",
		"info": gin.H{
			"version":                "1.0.0",
			"supports_hot_reload":    true,
			"supports_env_override":  true,
			"supported_formats":      []string{"yaml", "json"},
			"config_sections": []string{
				"server",
				"database", 
				"assets",
				"scanner",
				"plugins",
				"logging",
				"security",
				"performance",
			},
			"environment_variables": gin.H{
				"server":      []string{"VIEWRA_HOST", "VIEWRA_PORT", "VIEWRA_ENABLE_CORS"},
				"database":    []string{"DATABASE_TYPE", "POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_USER", "POSTGRES_PASSWORD", "POSTGRES_DB", "VIEWRA_DATA_DIR"},
				"scanner":     []string{"VIEWRA_PARALLEL_SCANNING", "VIEWRA_WORKER_COUNT", "VIEWRA_BATCH_SIZE"},
				"assets":      []string{"VIEWRA_ASSETS_DIR", "VIEWRA_MAX_ASSET_SIZE", "VIEWRA_ASSET_QUALITY"},
				"plugins":     []string{"PLUGIN_DIR", "VIEWRA_PLUGIN_HOT_RELOAD"},
				"logging":     []string{"VIEWRA_LOG_LEVEL", "VIEWRA_LOG_FORMAT", "VIEWRA_LOG_OUTPUT"},
				"security":    []string{"VIEWRA_ENABLE_AUTH", "VIEWRA_JWT_SECRET", "VIEWRA_RATE_LIMIT"},
				"performance": []string{"VIEWRA_ENABLE_METRICS", "VIEWRA_MAX_CONCURRENT_SCANS", "GOGC", "GOMAXPROCS"},
			},
		},
	})
} 