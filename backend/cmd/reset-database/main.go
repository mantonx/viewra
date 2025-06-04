package main

import (
	"fmt"
	"log"

	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

func main() {
	fmt.Println("🗑️  Database Reset Utility - WARNING: This will destroy all data!")
	fmt.Println("Press Enter to continue or Ctrl+C to cancel...")
	fmt.Scanln()

	// Initialize database connection
	database.Initialize()
	db := database.GetDB()
	if db == nil {
		log.Fatalf("Failed to initialize database")
	}

	// Drop all existing tables
	fmt.Println("🗑️  Dropping all existing tables...")
	if err := dropAllTables(db); err != nil {
		log.Fatalf("Failed to drop tables: %v", err)
	}

	// Recreate tables with new schema
	fmt.Println("🔧 Creating new schema...")
	if err := createNewSchema(db); err != nil {
		log.Fatalf("Failed to create new schema: %v", err)
	}

	fmt.Println("✅ Database reset complete!")
	fmt.Println("📋 New schema includes:")
	fmt.Println("   - MediaFile (UUID-based)")
	fmt.Println("   - MediaAsset (shared assets)")
	fmt.Println("   - People & Roles (unified cast/crew)")
	fmt.Println("   - Artist, Album, Track (music)")
	fmt.Println("   - Movie (movies)")
	fmt.Println("   - TVShow, Season, Episode (TV)")
	fmt.Println("   - MediaExternalIDs & MediaEnrichment")
}

func dropAllTables(db *gorm.DB) error {
	// Get list of all tables
	var tables []string
	if err := db.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = ?", "public").Scan(&tables).Error; err != nil {
		// Try SQLite syntax
		if err := db.Raw("SELECT name FROM sqlite_master WHERE type='table'").Scan(&tables).Error; err != nil {
			return fmt.Errorf("failed to get table list: %w", err)
		}
	}

	// Drop each table
	for _, table := range tables {
		if table == "sqlite_sequence" { // Skip SQLite system table
			continue
		}
		fmt.Printf("   Dropping table: %s\n", table)
		if err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table)).Error; err != nil {
			log.Printf("Warning: Failed to drop table %s: %v", table, err)
		}
	}

	return nil
}

func createNewSchema(db *gorm.DB) error {
	// Enable UUID extension for PostgreSQL
	db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"")

	// Create all new tables in dependency order
	models := []interface{}{
		// Core system tables
		&database.User{},
		&database.MediaLibrary{},
		&database.ScanJob{},

		// Media content tables
		&database.Artist{},
		&database.Album{},
		&database.Track{},
		&database.Movie{},
		&database.TVShow{},
		&database.Season{},
		&database.Episode{},

		// Shared tables
		&database.MediaFile{},
		&database.MediaAsset{},
		&database.People{},
		&database.Roles{},
		&database.MediaExternalIDs{},
		&database.MediaEnrichment{},

		// Plugin system tables
		&database.Plugin{},
		&database.PluginPermission{},
		&database.PluginEvent{},
		&database.PluginHook{},
		&database.PluginAdminPage{},
		&database.PluginUIComponent{},
		&database.SystemEvent{},
	}

	fmt.Println("   Creating tables...")
	for _, model := range models {
		fmt.Printf("     - %T\n", model)
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to create table for %T: %w", model, err)
		}
	}

	// Create indexes for performance
	fmt.Println("   Creating indexes...")
	if err := createIndexes(db); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

func createIndexes(db *gorm.DB) error {
	indexes := []string{
		// MediaFile indexes
		"CREATE INDEX IF NOT EXISTS idx_media_files_media_id ON media_files(media_id)",
		"CREATE INDEX IF NOT EXISTS idx_media_files_media_type ON media_files(media_type)",
		"CREATE INDEX IF NOT EXISTS idx_media_files_library_id ON media_files(library_id)",
		"CREATE INDEX IF NOT EXISTS idx_media_files_path ON media_files(path)",
		"CREATE INDEX IF NOT EXISTS idx_media_files_hash ON media_files(hash)",

		// MediaAsset indexes (updated for clean schema)
		"CREATE INDEX IF NOT EXISTS idx_media_assets_entity_type ON media_assets(entity_type)",
		"CREATE INDEX IF NOT EXISTS idx_media_assets_entity_id ON media_assets(entity_id)",
		"CREATE INDEX IF NOT EXISTS idx_media_assets_type ON media_assets(type)",
		"CREATE INDEX IF NOT EXISTS idx_media_assets_source ON media_assets(source)",
		"CREATE INDEX IF NOT EXISTS idx_media_assets_plugin ON media_assets(plugin_id)",
		"CREATE INDEX IF NOT EXISTS idx_media_assets_preferred ON media_assets(preferred)",
		"CREATE INDEX IF NOT EXISTS idx_media_assets_entity_composite ON media_assets(entity_type, entity_id, type)",

		// People and Roles indexes
		"CREATE INDEX IF NOT EXISTS idx_people_name ON people(name)",
		"CREATE INDEX IF NOT EXISTS idx_roles_person_id ON roles(person_id)",
		"CREATE INDEX IF NOT EXISTS idx_roles_media_id ON roles(media_id)",
		"CREATE INDEX IF NOT EXISTS idx_roles_media_type ON roles(media_type)",
		"CREATE INDEX IF NOT EXISTS idx_roles_role ON roles(role)",

		// Music indexes
		"CREATE INDEX IF NOT EXISTS idx_artists_name ON artists(name)",
		"CREATE INDEX IF NOT EXISTS idx_albums_title ON albums(title)",
		"CREATE INDEX IF NOT EXISTS idx_albums_artist_id ON albums(artist_id)",
		"CREATE INDEX IF NOT EXISTS idx_tracks_title ON tracks(title)",
		"CREATE INDEX IF NOT EXISTS idx_tracks_album_id ON tracks(album_id)",
		"CREATE INDEX IF NOT EXISTS idx_tracks_artist_id ON tracks(artist_id)",

		// Movie indexes
		"CREATE INDEX IF NOT EXISTS idx_movies_title ON movies(title)",
		"CREATE INDEX IF NOT EXISTS idx_movies_tmdb_id ON movies(tmdb_id)",
		"CREATE INDEX IF NOT EXISTS idx_movies_imdb_id ON movies(imdb_id)",

		// TV indexes
		"CREATE INDEX IF NOT EXISTS idx_tv_shows_title ON tv_shows(title)",
		"CREATE INDEX IF NOT EXISTS idx_tv_shows_tmdb_id ON tv_shows(tmdb_id)",
		"CREATE INDEX IF NOT EXISTS idx_seasons_tv_show_id ON seasons(tv_show_id)",
		"CREATE INDEX IF NOT EXISTS idx_seasons_season_number ON seasons(season_number)",
		"CREATE INDEX IF NOT EXISTS idx_episodes_season_id ON episodes(season_id)",
		"CREATE INDEX IF NOT EXISTS idx_episodes_episode_number ON episodes(episode_number)",

		// External IDs and enrichment indexes
		"CREATE INDEX IF NOT EXISTS idx_media_external_ids_media_id ON media_external_ids(media_id)",
		"CREATE INDEX IF NOT EXISTS idx_media_external_ids_source ON media_external_ids(source)",
		"CREATE INDEX IF NOT EXISTS idx_media_enrichment_media_id ON media_enrichment(media_id)",
		"CREATE INDEX IF NOT EXISTS idx_media_enrichment_plugin ON media_enrichment(plugin)",
	}

	for _, indexSQL := range indexes {
		if err := db.Exec(indexSQL).Error; err != nil {
			log.Printf("Warning: Failed to create index: %v", err)
		}
	}

	return nil
}
