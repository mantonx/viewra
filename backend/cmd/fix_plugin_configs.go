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

type PluginSettings struct {
	CorePlugins struct {
		MetadataExtractors []string `json:"metadata_extractors"`
		StructureParsers   []string `json:"structure_parsers"`
		TechnicalAnalyzers []string `json:"technical_analyzers"`
	} `json:"core_plugins"`

	EnrichmentPlugins struct {
		Enabled           bool     `json:"enabled"`
		AutoEnrich        bool     `json:"auto_enrich"`
		AllowedPlugins    []string `json:"allowed_plugins"`
		DisallowedPlugins []string `json:"disallowed_plugins"`
	} `json:"enrichment_plugins"`

	FileTypeRestrictions struct {
		AllowedExtensions    []string `json:"allowed_extensions"`
		DisallowedExtensions []string `json:"disallowed_extensions"`
		MimeTypeFilters      []string `json:"mime_type_filters"`
	} `json:"file_type_restrictions"`

	SharedPlugins struct {
		AllowTechnicalMetadata bool     `json:"allow_technical_metadata"`
		AllowAssetExtraction   bool     `json:"allow_asset_extraction"`
		SharedPluginNames      []string `json:"shared_plugin_names"`
	} `json:"shared_plugins"`
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
			config := createDefaultConfig(library.Type)

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
		var settings PluginSettings
		if err := json.Unmarshal([]byte(config.PluginConfig), &settings); err != nil {
			log.Printf("❌ Invalid config for library %d: %v", config.LibraryID, err)
			continue
		}

		fmt.Printf("✅ Library %d (%s): %d core plugins, %d enrichment plugins\n",
			config.LibraryID,
			config.LibraryType,
			len(settings.CorePlugins.MetadataExtractors)+
				len(settings.CorePlugins.StructureParsers)+
				len(settings.CorePlugins.TechnicalAnalyzers),
			len(settings.EnrichmentPlugins.AllowedPlugins))
	}

	fmt.Println("\n=== Fix Complete ===")
}

func createDefaultConfig(libraryType string) *PluginSettings {
	switch libraryType {
	case "music":
		return &PluginSettings{
			CorePlugins: struct {
				MetadataExtractors []string `json:"metadata_extractors"`
				StructureParsers   []string `json:"structure_parsers"`
				TechnicalAnalyzers []string `json:"technical_analyzers"`
			}{
				MetadataExtractors: []string{"music_metadata_extractor_plugin"},
				StructureParsers:   []string{},
				TechnicalAnalyzers: []string{"ffmpeg_probe_core_plugin"},
			},
			EnrichmentPlugins: struct {
				Enabled           bool     `json:"enabled"`
				AutoEnrich        bool     `json:"auto_enrich"`
				AllowedPlugins    []string `json:"allowed_plugins"`
				DisallowedPlugins []string `json:"disallowed_plugins"`
			}{
				Enabled:           true,
				AutoEnrich:        true,
				AllowedPlugins:    []string{"musicbrainz_enricher", "audiodb_enricher"},
				DisallowedPlugins: []string{"tmdb_enricher"},
			},
			FileTypeRestrictions: struct {
				AllowedExtensions    []string `json:"allowed_extensions"`
				DisallowedExtensions []string `json:"disallowed_extensions"`
				MimeTypeFilters      []string `json:"mime_type_filters"`
			}{
				AllowedExtensions:    []string{".mp3", ".flac", ".m4a", ".aac", ".ogg", ".wav", ".wma"},
				DisallowedExtensions: []string{".mp4", ".mkv", ".avi", ".mov"},
				MimeTypeFilters:      []string{"audio/*"},
			},
			SharedPlugins: struct {
				AllowTechnicalMetadata bool     `json:"allow_technical_metadata"`
				AllowAssetExtraction   bool     `json:"allow_asset_extraction"`
				SharedPluginNames      []string `json:"shared_plugin_names"`
			}{
				AllowTechnicalMetadata: true,
				AllowAssetExtraction:   true,
				SharedPluginNames:      []string{"ffmpeg_probe_core_plugin"},
			},
		}

	case "movie":
		return &PluginSettings{
			CorePlugins: struct {
				MetadataExtractors []string `json:"metadata_extractors"`
				StructureParsers   []string `json:"structure_parsers"`
				TechnicalAnalyzers []string `json:"technical_analyzers"`
			}{
				MetadataExtractors: []string{"ffmpeg_probe_core_plugin"},
				StructureParsers:   []string{"movie_structure_parser_core_plugin"},
				TechnicalAnalyzers: []string{"ffmpeg_probe_core_plugin"},
			},
			EnrichmentPlugins: struct {
				Enabled           bool     `json:"enabled"`
				AutoEnrich        bool     `json:"auto_enrich"`
				AllowedPlugins    []string `json:"allowed_plugins"`
				DisallowedPlugins []string `json:"disallowed_plugins"`
			}{
				Enabled:           true,
				AutoEnrich:        true,
				AllowedPlugins:    []string{"tmdb_enricher"},
				DisallowedPlugins: []string{"musicbrainz_enricher", "audiodb_enricher"},
			},
			FileTypeRestrictions: struct {
				AllowedExtensions    []string `json:"allowed_extensions"`
				DisallowedExtensions []string `json:"disallowed_extensions"`
				MimeTypeFilters      []string `json:"mime_type_filters"`
			}{
				AllowedExtensions:    []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v"},
				DisallowedExtensions: []string{".mp3", ".flac", ".m4a", ".aac"},
				MimeTypeFilters:      []string{"video/*"},
			},
			SharedPlugins: struct {
				AllowTechnicalMetadata bool     `json:"allow_technical_metadata"`
				AllowAssetExtraction   bool     `json:"allow_asset_extraction"`
				SharedPluginNames      []string `json:"shared_plugin_names"`
			}{
				AllowTechnicalMetadata: true,
				AllowAssetExtraction:   true,
				SharedPluginNames:      []string{"ffmpeg_probe_core_plugin"},
			},
		}

	case "tv":
		return &PluginSettings{
			CorePlugins: struct {
				MetadataExtractors []string `json:"metadata_extractors"`
				StructureParsers   []string `json:"structure_parsers"`
				TechnicalAnalyzers []string `json:"technical_analyzers"`
			}{
				MetadataExtractors: []string{"ffmpeg_probe_core_plugin"},
				StructureParsers:   []string{"tv_structure_parser_core_plugin"},
				TechnicalAnalyzers: []string{"ffmpeg_probe_core_plugin"},
			},
			EnrichmentPlugins: struct {
				Enabled           bool     `json:"enabled"`
				AutoEnrich        bool     `json:"auto_enrich"`
				AllowedPlugins    []string `json:"allowed_plugins"`
				DisallowedPlugins []string `json:"disallowed_plugins"`
			}{
				Enabled:           true,
				AutoEnrich:        true,
				AllowedPlugins:    []string{"tmdb_enricher"},
				DisallowedPlugins: []string{"musicbrainz_enricher", "audiodb_enricher"},
			},
			FileTypeRestrictions: struct {
				AllowedExtensions    []string `json:"allowed_extensions"`
				DisallowedExtensions []string `json:"disallowed_extensions"`
				MimeTypeFilters      []string `json:"mime_type_filters"`
			}{
				AllowedExtensions:    []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v"},
				DisallowedExtensions: []string{".mp3", ".flac", ".m4a", ".aac"},
				MimeTypeFilters:      []string{"video/*"},
			},
			SharedPlugins: struct {
				AllowTechnicalMetadata bool     `json:"allow_technical_metadata"`
				AllowAssetExtraction   bool     `json:"allow_asset_extraction"`
				SharedPluginNames      []string `json:"shared_plugin_names"`
			}{
				AllowTechnicalMetadata: true,
				AllowAssetExtraction:   true,
				SharedPluginNames:      []string{"ffmpeg_probe_core_plugin"},
			},
		}

	default:
		// Generic configuration for unknown library types
		return &PluginSettings{
			CorePlugins: struct {
				MetadataExtractors []string `json:"metadata_extractors"`
				StructureParsers   []string `json:"structure_parsers"`
				TechnicalAnalyzers []string `json:"technical_analyzers"`
			}{
				MetadataExtractors: []string{"ffmpeg_probe_core_plugin"},
				StructureParsers:   []string{},
				TechnicalAnalyzers: []string{"ffmpeg_probe_core_plugin"},
			},
			EnrichmentPlugins: struct {
				Enabled           bool     `json:"enabled"`
				AutoEnrich        bool     `json:"auto_enrich"`
				AllowedPlugins    []string `json:"allowed_plugins"`
				DisallowedPlugins []string `json:"disallowed_plugins"`
			}{
				Enabled:           true,
				AutoEnrich:        false,
				AllowedPlugins:    []string{},
				DisallowedPlugins: []string{},
			},
			FileTypeRestrictions: struct {
				AllowedExtensions    []string `json:"allowed_extensions"`
				DisallowedExtensions []string `json:"disallowed_extensions"`
				MimeTypeFilters      []string `json:"mime_type_filters"`
			}{
				AllowedExtensions:    []string{},
				DisallowedExtensions: []string{},
				MimeTypeFilters:      []string{},
			},
			SharedPlugins: struct {
				AllowTechnicalMetadata bool     `json:"allow_technical_metadata"`
				AllowAssetExtraction   bool     `json:"allow_asset_extraction"`
				SharedPluginNames      []string `json:"shared_plugin_names"`
			}{
				AllowTechnicalMetadata: true,
				AllowAssetExtraction:   true,
				SharedPluginNames:      []string{"ffmpeg_probe_core_plugin"},
			},
		}
	}
}
