package migration

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// CreateSchema runs migrations to create the schema in the target database.
// It determines the correct migrations path based on the target driver and runs
// all available migrations.
func CreateSchema(db *sql.DB, targetDriver, migrationsPath string) error {
	// Determine the correct migrations subdirectory
	actualPath := migrationsPath
	if targetDriver == "postgres" {
		postgresPath := filepath.Join(migrationsPath, "postgres")
		if _, err := os.Stat(postgresPath); err == nil {
			actualPath = postgresPath
		}
	}

	// Verify migrations exist
	if _, err := os.Stat(actualPath); err != nil {
		return fmt.Errorf("migrations path does not exist: %s", actualPath)
	}

	// Create database driver for golang-migrate
	var driver database.Driver
	var err error

	switch targetDriver {
	case "sqlite", "sqlite3":
		driver, err = sqlite3.WithInstance(db, &sqlite3.Config{})
		if err != nil {
			return fmt.Errorf("failed to create SQLite migration driver: %w", err)
		}

	case "postgres", "postgresql":
		driver, err = postgres.WithInstance(db, &postgres.Config{})
		if err != nil {
			return fmt.Errorf("failed to create PostgreSQL migration driver: %w", err)
		}

	default:
		return fmt.Errorf("unsupported database driver for migration: %s", targetDriver)
	}

	// Create migration instance
	sourceURL := fmt.Sprintf("file://%s", actualPath)
	m, err := migrate.NewWithDatabaseInstance(sourceURL, targetDriver, driver)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	// Check if database already has migrations
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	if dirty {
		// Database is in dirty state - force set version to allow cleanup
		if err := m.Force(int(version)); err != nil {
			return fmt.Errorf("failed to force dirty migration state: %w", err)
		}
	}

	// Check if tables exist (even if migration version is 0)
	// This handles cases where tables exist but schema_migrations was dropped
	hasExistingTables, err := hasAnyTables(db, targetDriver)
	if err != nil {
		return fmt.Errorf("failed to check for existing tables: %w", err)
	}

	// If database has existing migrations OR tables, drop everything and start fresh.
	// This handles retries after failed migration attempts.
	if version > 0 || hasExistingTables {
		// Note: Don't call m.Close() as it closes the db connection we need to reuse.
		// The migrate instance will be garbage collected.

		// Drop all tables (including schema_migrations)
		if err := DropAllTables(db, targetDriver); err != nil {
			return fmt.Errorf("failed to drop existing tables: %w", err)
		}

		// Create a fresh driver since the old one cached schema_migrations
		switch targetDriver {
		case "sqlite", "sqlite3":
			driver, err = sqlite3.WithInstance(db, &sqlite3.Config{})
		case "postgres", "postgresql":
			driver, err = postgres.WithInstance(db, &postgres.Config{})
		}
		if err != nil {
			return fmt.Errorf("failed to recreate migration driver after cleanup: %w", err)
		}

		// Recreate the migrate instance with the new driver
		m, err = migrate.NewWithDatabaseInstance(sourceURL, targetDriver, driver)
		if err != nil {
			return fmt.Errorf("failed to recreate migration instance after cleanup: %w", err)
		}
	}

	// Run all migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Verify migrations ran successfully
	newVersion, dirty, err := m.Version()
	if err != nil {
		return fmt.Errorf("failed to verify migration version: %w", err)
	}

	if dirty {
		return fmt.Errorf("migrations left database in dirty state at version %d", newVersion)
	}

	return nil
}

// GetMigrationsPath returns the default migrations path for the application.
func GetMigrationsPath() string {
	// Check environment variable first
	if path := os.Getenv("MIGRATIONS_PATH"); path != "" {
		return path
	}
	// Default to "migrations" in the current working directory
	return "migrations"
}

// hasAnyTables checks if the database has any user tables.
func hasAnyTables(db *sql.DB, driver string) (bool, error) {
	var count int
	switch driver {
	case "postgres", "postgresql":
		err := db.QueryRow(`
			SELECT COUNT(*) FROM pg_tables WHERE schemaname = 'public'
		`).Scan(&count)
		if err != nil {
			return false, err
		}
	case "sqlite", "sqlite3":
		err := db.QueryRow(`
			SELECT COUNT(*) FROM sqlite_master 
			WHERE type='table' AND name NOT LIKE 'sqlite_%'
		`).Scan(&count)
		if err != nil {
			return false, err
		}
	default:
		return false, fmt.Errorf("unsupported driver: %s", driver)
	}
	return count > 0, nil
}

// DropAllTables drops all tables in the target database.
// This is used when migration fails and we need to clean up.
func DropAllTables(db *sql.DB, driver string) error {
	switch driver {
	case "postgres", "postgresql":
		// Drop all tables in public schema
		_, err := db.Exec(`
			DO $$ DECLARE
				r RECORD;
			BEGIN
				FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
					EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE';
				END LOOP;
			END $$;
		`)
		if err != nil {
			return fmt.Errorf("failed to drop PostgreSQL tables: %w", err)
		}

	case "sqlite", "sqlite3":
		// Get all table names
		rows, err := db.Query(`
			SELECT name FROM sqlite_master 
			WHERE type='table' AND name NOT LIKE 'sqlite_%'
		`)
		if err != nil {
			return fmt.Errorf("failed to query SQLite tables: %w", err)
		}
		defer rows.Close()

		var tables []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return fmt.Errorf("failed to scan table name: %w", err)
			}
			tables = append(tables, name)
		}

		// Disable foreign key checks
		if _, err := db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
			return fmt.Errorf("failed to disable foreign keys: %w", err)
		}

		// Drop each table
		for _, table := range tables {
			if _, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q", table)); err != nil {
				return fmt.Errorf("failed to drop table %s: %w", table, err)
			}
		}

		// Re-enable foreign key checks
		if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
			return fmt.Errorf("failed to re-enable foreign keys: %w", err)
		}

	default:
		return fmt.Errorf("unsupported driver for DropAllTables: %s", driver)
	}

	return nil
}
