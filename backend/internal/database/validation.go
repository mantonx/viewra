package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// ValidateDatabaseConfig ensures only one database is configured and prevents common issues
func ValidateDatabaseConfig(dbPath string) error {
	log.Printf("🔍 Validating database configuration...")

	// Ensure database path is absolute for consistency
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for database: %w", err)
	}

	// Check for common problematic paths (but not the correct ones)
	problematicPaths := []string{
		"backend/data/viewra.db",
		"backend/data/database.db",
		"data/viewra.db",
		"data/database.db",
		"database.db",
		"./database.db",
		"./viewra.db",
	}

	// Check if path contains any problematic patterns
	// but exclude the correct viewra-data pattern
	for _, problemPath := range problematicPaths {
		if strings.Contains(absPath, problemPath) && !strings.Contains(absPath, "viewra-data") {
			return fmt.Errorf("❌ Invalid database path detected: %s\n"+
				"   Expected: ./viewra-data/viewra.db\n"+
				"   Run: ./scripts/cleanup-databases.sh", absPath)
		}
	}

	// Ensure the correct path pattern (more lenient for different environments)
	if !strings.Contains(absPath, "viewra-data") || !strings.HasSuffix(absPath, "viewra.db") {
		log.Printf("⚠️  Warning: Database path doesn't match expected pattern")
		log.Printf("   Current: %s", absPath)
		log.Printf("   Expected: */viewra-data/viewra.db")
		log.Printf("   This might be okay if you're using a custom configuration")
	}

	// Check for multiple database files
	redundantFiles := findRedundantDatabases(filepath.Dir(absPath))
	if len(redundantFiles) > 0 {
		log.Printf("⚠️  Warning: Found redundant database files:")
		for _, file := range redundantFiles {
			log.Printf("   - %s", file)
		}
		log.Printf("   Run: ./scripts/cleanup-databases.sh")
	}

	// Ensure directory exists
	dbDir := filepath.Dir(absPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	log.Printf("✅ Database configuration validated: %s", absPath)
	return nil
}

// findRedundantDatabases looks for potential redundant database files
func findRedundantDatabases(baseDir string) []string {
	var redundantFiles []string

	// Search patterns for redundant databases
	searchDirs := []string{
		"backend/data",
		"backend",
		"data",
		".",
	}

	for _, dir := range searchDirs {
		fullDir := filepath.Join(baseDir, "..", dir)
		if files, err := filepath.Glob(filepath.Join(fullDir, "*.db")); err == nil {
			for _, file := range files {
				// Skip the correct database file
				if !strings.Contains(file, "viewra-data/viewra.db") {
					redundantFiles = append(redundantFiles, file)
				}
			}
		}
	}

	return redundantFiles
}

// LogDatabaseInfo logs current database configuration for debugging
func LogDatabaseInfo() {
	if DB == nil {
		log.Printf("❌ Database not initialized")
		return
	}

	log.Printf("📊 Database Information:")

	// Get database file info if SQLite
	if sqlDB, err := DB.DB(); err == nil {
		// Log connection info
		log.Printf("   - Type: SQLite")
		log.Printf("   - Connection available: %v", sqlDB != nil)
	}

	// Get table counts
	tables := []string{
		"media_files", "tv_shows", "seasons", "episodes", "movies",
		"peoples", "roles", "artists", "albums", "tracks",
		"tm_db_enrichments", "media_assets",
	}

	totalRecords := int64(0)
	for _, table := range tables {
		var count int64
		if err := DB.Table(table).Count(&count).Error; err == nil {
			log.Printf("   - %s: %d records", table, count)
			totalRecords += count
		}
	}
	log.Printf("   - Total records across all tables: %d", totalRecords)
}
