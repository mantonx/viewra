package database

import (
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// MigrationConfig holds configuration for database migrations
type MigrationConfig struct {
	AutoMigrate         bool   // Whether to run migrations automatically on startup
	MigrationsPath      string // Path to migration files
	BackupBeforeMigrate bool   // Whether to backup database before running migrations
	BackupPath          string // Path to store backups
}

// LoadMigrationConfigFromEnv loads migration configuration from environment variables
func LoadMigrationConfigFromEnv() *MigrationConfig {
	autoMigrate := true // Default to true
	if val := os.Getenv("AUTO_MIGRATE"); val == "false" {
		autoMigrate = false
	}

	backupBeforeMigrate := true // Default to true for safety
	if val := os.Getenv("BACKUP_BEFORE_MIGRATE"); val == "false" {
		backupBeforeMigrate = false
	}

	return &MigrationConfig{
		AutoMigrate:         autoMigrate,
		MigrationsPath:      getEnvOrDefault("MIGRATIONS_PATH", "migrations"),
		BackupBeforeMigrate: backupBeforeMigrate,
		BackupPath:          getEnvOrDefault("BACKUP_PATH", "data/backups"),
	}
}

// AutoMigrate runs database migrations if AUTO_MIGRATE is enabled
func AutoMigrate(db *sql.DB, dbConfig *Config, migrationConfig *MigrationConfig, logger *slog.Logger) error {
	if !migrationConfig.AutoMigrate {
		logger.Info("Auto-migration disabled", "AUTO_MIGRATE", "false")
		return nil
	}

	logger.Info("Starting auto-migration", "migrations_path", migrationConfig.MigrationsPath)

	// Adjust migrations path based on database driver
	migrationsPath := migrationConfig.MigrationsPath
	if dbConfig.Driver == "postgres" || dbConfig.Driver == "postgresql" {
		// Use postgres-specific migrations if they exist
		postgresPath := filepath.Join(migrationConfig.MigrationsPath, "postgres")
		if _, err := os.Stat(postgresPath); err == nil {
			migrationsPath = postgresPath
			logger.Info("Using PostgreSQL-specific migrations", "path", postgresPath)
		}
	}

	// Create migration instance
	m, err := createMigration(db, dbConfig, migrationsPath)
	if err != nil {
		return fmt.Errorf("failed to create migration: %w", err)
	}
	// Note: We don't close the migrate instance because we used WithInstance,
	// which means the migrate library doesn't own the database connection.
	// Closing it would close our database connection that we need to keep alive.

	// Get current version
	currentVersion, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	if dirty {
		return fmt.Errorf("database is in dirty state at version %d - please fix manually", currentVersion)
	}

	// Check if we need to migrate
	// Create a second instance to check if there are pending migrations
	targetVersion, err := getTargetVersion(migrationsPath)
	if err != nil {
		return fmt.Errorf("failed to determine target version: %w", err)
	}

	if err == migrate.ErrNilVersion {
		logger.Info("Database is uninitialized, will run migrations")
	} else if currentVersion >= targetVersion {
		logger.Info("Database is up to date", "version", currentVersion)
		return nil
	} else {
		logger.Info("Database needs migration", "current", currentVersion, "target", targetVersion)
	}

	// Backup database before migration (SQLite only)
	if migrationConfig.BackupBeforeMigrate && (dbConfig.Driver == "sqlite" || dbConfig.Driver == "sqlite3") {
		if err := backupDatabase(dbConfig.DBName, migrationConfig.BackupPath, logger); err != nil {
			logger.Warn("Failed to backup database before migration", "error", err)
			// Continue with migration despite backup failure
		}
	}

	// Run migrations
	logger.Info("Running database migrations...")
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Get new version
	newVersion, _, err := m.Version()
	if err != nil {
		return fmt.Errorf("failed to get new version: %w", err)
	}

	logger.Info("Database migration completed successfully", "version", newVersion)

	// Run SQLite-specific migrations if applicable
	if dbConfig.Driver == "sqlite" || dbConfig.Driver == "sqlite3" {
		sqlitePath := filepath.Join(migrationConfig.MigrationsPath, "sqlite")
		if _, err := os.Stat(sqlitePath); err == nil {
			if err := runSQLiteSpecificMigrations(db, dbConfig, sqlitePath, logger); err != nil {
				return fmt.Errorf("failed to run SQLite-specific migrations: %w", err)
			}
		}
	}

	return nil
}

// runSQLiteSpecificMigrations runs SQLite-specific migrations from the sqlite/ subdirectory
// These migrations use a separate schema_migrations_sqlite table to track their state
func runSQLiteSpecificMigrations(db *sql.DB, dbConfig *Config, migrationsPath string, logger *slog.Logger) error {
	logger.Info("Checking for SQLite-specific migrations", "path", migrationsPath)

	// Create a separate migration instance for SQLite-specific migrations
	// This uses a different migrations table to avoid conflicts
	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{
		MigrationsTable: "schema_migrations_sqlite",
	})
	if err != nil {
		return fmt.Errorf("failed to create SQLite driver for specific migrations: %w", err)
	}

	sourceURL := fmt.Sprintf("file://%s", migrationsPath)
	m, err := migrate.NewWithDatabaseInstance(sourceURL, dbConfig.Driver, driver)
	if err != nil {
		return fmt.Errorf("failed to create SQLite-specific migration instance: %w", err)
	}

	// Get current version
	currentVersion, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get SQLite-specific migration version: %w", err)
	}

	if dirty {
		return fmt.Errorf("SQLite-specific migrations are in dirty state at version %d - please fix manually", currentVersion)
	}

	// Check target version
	targetVersion, err := getTargetVersion(migrationsPath)
	if err != nil {
		return fmt.Errorf("failed to determine SQLite-specific target version: %w", err)
	}

	if currentVersion >= targetVersion {
		logger.Info("SQLite-specific migrations are up to date", "version", currentVersion)
		return nil
	}

	logger.Info("Running SQLite-specific migrations", "current", currentVersion, "target", targetVersion)

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run SQLite-specific migrations: %w", err)
	}

	newVersion, _, err := m.Version()
	if err != nil {
		return fmt.Errorf("failed to get new SQLite-specific version: %w", err)
	}

	logger.Info("SQLite-specific migrations completed", "version", newVersion)
	return nil
}

// createMigration creates a migrate instance for the given database
func createMigration(db *sql.DB, dbConfig *Config, migrationsPath string) (*migrate.Migrate, error) {
	// Create database driver
	var driver database.Driver
	var err error

	switch dbConfig.Driver {
	case "sqlite", "sqlite3":
		driver, err = sqlite3.WithInstance(db, &sqlite3.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to create SQLite driver: %w", err)
		}

	case "postgres", "postgresql":
		driver, err = postgres.WithInstance(db, &postgres.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to create PostgreSQL driver: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported database driver: %s", dbConfig.Driver)
	}

	// Create migration instance with file source
	sourceURL := fmt.Sprintf("file://%s", migrationsPath)
	m, err := migrate.NewWithDatabaseInstance(sourceURL, dbConfig.Driver, driver)
	if err != nil {
		return nil, fmt.Errorf("failed to create migration instance: %w", err)
	}

	return m, nil
}

// getTargetVersion reads the migrations directory to find the highest version
func getTargetVersion(migrationsPath string) (uint, error) {
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var maxVersion uint
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Parse version from filename (e.g., "000001_init.up.sql" -> 1)
		var version uint
		_, err := fmt.Sscanf(entry.Name(), "%d_", &version)
		if err != nil {
			continue // Skip files that don't match pattern
		}

		if version > maxVersion {
			maxVersion = version
		}
	}

	return maxVersion, nil
}

// backupDatabase creates a backup of the SQLite database
func backupDatabase(dbPath, backupPath string, logger *slog.Logger) error {
	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(backupPath, 0o755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupFilename := fmt.Sprintf("viewra-backup-%s.db", timestamp)
	backupFullPath := filepath.Join(backupPath, backupFilename)

	// Open source database
	source, err := os.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer source.Close()

	// Create destination backup file
	dest, err := os.Create(backupFullPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dest.Close()

	// Copy database file
	_, err = io.Copy(dest, source)
	if err != nil {
		return fmt.Errorf("failed to copy database: %w", err)
	}

	logger.Info("Database backed up successfully", "backup", backupFullPath)

	// Clean old backups (keep last 7 days)
	if err := cleanOldBackups(backupPath, 7, logger); err != nil {
		logger.Warn("Failed to clean old backups", "error", err)
	}

	return nil
}

// cleanOldBackups removes backups older than the specified number of days
func cleanOldBackups(backupPath string, keepDays int, logger *slog.Logger) error {
	entries, err := os.ReadDir(backupPath)
	if err != nil {
		return err
	}

	cutoffTime := time.Now().AddDate(0, 0, -keepDays)
	var removed int

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only clean backup files
		matched, err := filepath.Match("viewra-backup-*.db", entry.Name())
		if err != nil || !matched {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoffTime) {
			fullPath := filepath.Join(backupPath, entry.Name())
			if err := os.Remove(fullPath); err != nil {
				logger.Warn("Failed to remove old backup", "file", fullPath, "error", err)
			} else {
				removed++
			}
		}
	}

	if removed > 0 {
		logger.Info("Cleaned old backups", "removed", removed, "older_than_days", keepDays)
	}

	return nil
}
