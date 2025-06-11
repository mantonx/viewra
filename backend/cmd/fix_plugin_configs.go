package main

import (
	"encoding/json"
	"fmt"
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Minimal structs for the fix script
type MediaLibrary struct {
	ID   uint `gorm:"primaryKey"`
	Type string
	Path string
}

type LibraryPluginConfig struct {
	LibraryID    uint   `gorm:"primaryKey"`
	LibraryType  string `gorm:"not null"`
	PluginConfig string `gorm:"type:text"`
	Enabled      bool   `gorm:"default:true"`
}

// Modern simplified plugin configuration
type ModernPluginConfig struct {
	// Core plugins (required for basic functionality)
	CorePlugins []string `json:"core_plugins"`

	// External enrichment plugins
	EnrichmentPlugins struct {
		Enabled        bool     `json:"enabled"`
		AutoEnrich     bool     `json:"auto_enrich"`
		AllowedPlugins []string `json:"allowed_plugins"`
	} `json:"enrichment_plugins"`

	// File filtering
	FileFilters struct {
		AllowedExtensions []string `json:"allowed_extensions"`
		MaxFileSize       int64    `json:"max_file_size"`
	} `json:"file_filters"`
}

func main() {
	fmt.Println("=== Plugin Configuration Fix Utility ===")

	// Open database connection
	db, err := gorm.Open(sqlite.Open("../viewra-data/viewra.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-migrate the configuration table
	if err := db.AutoMigrate(&LibraryPluginConfig{}); err != nil {
		log.Fatalf("Failed to migrate library plugin config table: %v", err)
	}
	fmt.Println("✅ Database migration completed")

	// Get all libraries
	var libraries []MediaLibrary
	if err := db.Find(&libraries).Error; err != nil {
		log.Fatalf("Failed to load libraries: %v", err)
	}
	fmt.Printf("Found %d libraries\n", len(libraries))

	// Check existing configurations
	var existingConfigs []LibraryPluginConfig
	if err := db.Find(&existingConfigs).Error; err != nil {
		log.Fatalf("Failed to load existing configurations: %v", err)
	}
	fmt.Printf("Found %d existing plugin configurations\n", len(existingConfigs))

	// Create configurations for libraries that don't have them
	for _, library := range libraries {
		// Check if configuration already exists
		var existingConfig LibraryPluginConfig
		err := db.Where("library_id = ?", library.ID).First(&existingConfig).Error

		if err == gorm.ErrRecordNotFound {
			// Create default configuration
			config := getModernConfigForLibraryType(library.Type)

			configJSON, err := json.Marshal(config)
			if err != nil {
				log.Printf("ERROR: Failed to serialize config for library %d: %v", library.ID, err)
				continue
			}

			dbConfig := LibraryPluginConfig{
				LibraryID:    library.ID,
				LibraryType:  library.Type,
				PluginConfig: string(configJSON),
				Enabled:      true,
			}

			if err := db.Create(&dbConfig).Error; err != nil {
				log.Printf("ERROR: Failed to create config for library %d: %v", library.ID, err)
				continue
			}

			fmt.Printf("✅ Created default plugin configuration for library %d (%s - %s)\n",
				library.ID, library.Type, library.Type)
		} else if err != nil {
			log.Printf("ERROR: Failed to check config for library %d: %v", library.ID, err)
		} else {
			fmt.Printf("✓ Library %d (%s) already has plugin configuration\n",
				library.ID, library.Type)
		}
	}

	// Validate all configurations
	fmt.Println("\n=== Configuration Validation ===")
	var allConfigs []LibraryPluginConfig
	if err := db.Find(&allConfigs).Error; err != nil {
		log.Fatalf("Failed to load configurations for validation: %v", err)
	}

	for _, config := range allConfigs {
		var settings ModernPluginConfig
		if err := json.Unmarshal([]byte(config.PluginConfig), &settings); err != nil {
			log.Printf("❌ Invalid config for library %d: %v", config.LibraryID, err)
			continue
		}

		fmt.Printf("✅ Library %d (%s): %d core plugins, %d enrichment plugins\n",
			config.LibraryID,
			config.LibraryType,
			len(settings.CorePlugins),
			len(settings.EnrichmentPlugins.AllowedPlugins))
	}

	fmt.Println("\n=== Fix Complete ===")
}

func getModernConfigForLibraryType(libraryType string) ModernPluginConfig {
	config := ModernPluginConfig{}

	// Set core plugins based on library type
	switch libraryType {
	case "music":
		config.CorePlugins = []string{
			"music_metadata_extractor_plugin",
			"ffmpeg_probe_core_plugin",
		}
		config.EnrichmentPlugins.AllowedPlugins = []string{"musicbrainz_enricher"}
	case "tv", "movie":
		config.CorePlugins = []string{
			"ffmpeg_probe_core_plugin",
		}
		if libraryType == "tv" {
			config.CorePlugins = append(config.CorePlugins, "tv_structure_parser_core_plugin")
		} else {
			config.CorePlugins = append(config.CorePlugins, "movie_structure_parser_core_plugin")
		}
		config.EnrichmentPlugins.AllowedPlugins = []string{"tmdb_enricher_v2"}
	}

	// Common settings
	config.EnrichmentPlugins.Enabled = true
	config.EnrichmentPlugins.AutoEnrich = true
	config.FileFilters.MaxFileSize = 10 * 1024 * 1024 * 1024 // 10GB

	return config
}
