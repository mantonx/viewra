package migration

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// OpenTargetDB opens a connection to the target database.
func OpenTargetDB(config TargetConfig) (*sql.DB, error) {
	switch config.Driver {
	case "postgres":
		if config.Postgres == nil {
			return nil, fmt.Errorf("postgres config required")
		}
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			config.Postgres.Host,
			config.Postgres.Port,
			config.Postgres.User,
			config.Postgres.Password,
			config.Postgres.Database,
			config.Postgres.SSLMode,
		)
		return sql.Open("postgres", dsn)
	case "sqlite":
		if config.SQLite == nil {
			return nil, fmt.Errorf("sqlite config required")
		}
		return sql.Open("sqlite3", config.SQLite.Path)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", config.Driver)
	}
}

// GetConnectionDetails gets details about a database connection.
func GetConnectionDetails(ctx context.Context, db *sql.DB, driver string) (*ConnectionTestDetails, error) {
	details := &ConnectionTestDetails{
		ServerTime: time.Now().UTC(),
	}

	// Get version
	var version string
	switch driver {
	case "postgres":
		err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
		if err != nil {
			return nil, err
		}
		details.Version = version
	case "sqlite":
		err := db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version)
		if err != nil {
			return nil, err
		}
		details.Version = version
	}

	// Count existing tables
	var tableCount int
	switch driver {
	case "postgres":
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tableCount)
		if err != nil {
			return nil, err
		}
	case "sqlite":
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tableCount)
		if err != nil {
			return nil, err
		}
	}

	details.ExistingTables = tableCount
	details.IsEmpty = tableCount == 0

	return details, nil
}

// TestConnection tests a connection to the target database.
func TestConnection(ctx context.Context, config TargetConfig) ConnectionTestResult {
	db, err := OpenTargetDB(config)
	if err != nil {
		return ConnectionTestResult{
			Success: false,
			Message: "Connection failed",
			Error:   err.Error(),
		}
	}
	defer db.Close()

	// Test the connection
	if err := db.PingContext(ctx); err != nil {
		return ConnectionTestResult{
			Success: false,
			Message: "Connection failed",
			Error:   err.Error(),
		}
	}

	// Get database version and info
	details, err := GetConnectionDetails(ctx, db, config.Driver)
	if err != nil {
		return ConnectionTestResult{
			Success: true,
			Message: "Connected successfully (could not retrieve details)",
			Details: nil,
		}
	}

	driverName := "PostgreSQL"
	if config.Driver == "sqlite" {
		driverName = "SQLite"
	}

	return ConnectionTestResult{
		Success: true,
		Message: fmt.Sprintf("Successfully connected to %s %s", driverName, details.Version),
		Details: details,
	}
}
