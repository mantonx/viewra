package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/mantonx/viewra/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// configureConnectionPool applies connection pool settings to a database connection
func configureConnectionPool(db *gorm.DB, cfg config.DatabaseFullConfig) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// Set connection pool parameters from configuration
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns) 
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	log.Printf("📊 Database connection pool configured:")
	log.Printf("   - Max Open Connections: %d", cfg.MaxOpenConns)
	log.Printf("   - Max Idle Connections: %d", cfg.MaxIdleConns)
	log.Printf("   - Connection Max Lifetime: %v", cfg.ConnMaxLifetime)
	log.Printf("   - Connection Max Idle Time: %v", cfg.ConnMaxIdleTime)

	return nil
}

// Initialize sets up the database connection based on the DATABASE_TYPE environment variable
func Initialize() {
	log.Printf("🔄 Initializing database...")
	
	// Get database configuration from centralized config
	cfg := config.Get().Database
	
	var err error
	var dbType string
	
	switch cfg.Type {
	case "postgres":
		dbType = "PostgreSQL"
		DB, err = connectPostgres(cfg)
	case "sqlite":
		dbType = "SQLite"
		DB, err = connectSQLite(cfg)
	default:
		log.Printf("⚠️  Unknown database type '%s', defaulting to SQLite", cfg.Type)
		dbType = "SQLite"
		DB, err = connectSQLite(cfg)
	}

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Configure connection pool for optimal performance
	if err := configureConnectionPool(DB, cfg); err != nil {
		log.Printf("⚠️  Warning: Failed to configure connection pool: %v", err)
	}

	// Test the connection pool
	if err := testConnectionPool(DB); err != nil {
		log.Printf("⚠️  Warning: Connection pool test failed: %v", err)
	}
	
	// Auto-migrate the schema
	err = DB.AutoMigrate(
		&User{}, &MediaLibrary{}, &ScanJob{},
		// New comprehensive metadata models
		&MediaFile{}, &MediaAsset{}, &People{}, &Roles{},
		&Artist{}, &Album{}, &Track{},
		&Movie{}, &TVShow{}, &Season{}, &Episode{},
		&MediaExternalIDs{}, &MediaEnrichment{},
		// Plugin system tables
		&Plugin{}, &PluginPermission{}, &PluginEvent{}, &PluginHook{}, &PluginAdminPage{}, &PluginUIComponent{},
		// Event system tables
		&SystemEvent{},
	)
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Printf("✅ Database initialized with %s at %s", dbType, cfg.DatabasePath)
}

// testConnectionPool performs a quick test of the connection pool
func testConnectionPool(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}
	
	// Test basic connectivity
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("connection ping failed: %w", err)
	}
	
	// Get connection statistics
	stats := sqlDB.Stats()
	log.Printf("📊 Initial connection pool stats:")
	log.Printf("   - Open Connections: %d", stats.OpenConnections)
	log.Printf("   - In Use: %d", stats.InUse)
	log.Printf("   - Idle: %d", stats.Idle)
	
	return nil
}

func connectPostgres(cfg config.DatabaseFullConfig) (*gorm.DB, error) {
	// Use configuration values with fallbacks
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}
	
	port := cfg.Port
	if port == 0 {
		port = 5432
	}
	
	user := cfg.Username
	password := cfg.Password
	dbname := cfg.Database
	
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=UTC",
		host, user, password, dbname, port)
	
	// Configure GORM with optimized settings for high-throughput operations
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		
		// Optimize for batch operations (scanner writes many records)
		CreateBatchSize: 1000, // Insert up to 1000 records at once
		
		// Disable automatic time updates for better performance if not needed
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		
		// Skip default transaction for better performance
		SkipDefaultTransaction: true,
		
		// Prepare statements for better performance with repeated queries
		PrepareStmt: true,
	}
	
	return gorm.Open(postgres.Open(dsn), gormConfig)
}

func connectSQLite(cfg config.DatabaseFullConfig) (*gorm.DB, error) {
	dbPath := cfg.DatabasePath
	
	// Add SQLite pragmas for better performance
	dsn := dbPath + "?" +
		"cache=shared&" +          // Enable shared cache
		"mode=rwc&" +              // Read-write-create mode
		"_journal_mode=WAL&" +     // Write-Ahead Logging for better concurrency
		"_synchronous=NORMAL&" +   // Balance between safety and performance
		"_busy_timeout=30000&" +   // 30 second busy timeout
		"_cache_size=-64000&" +    // 64MB cache size (negative = KB)
		"_temp_store=MEMORY&" +    // Store temporary tables in memory
		"_foreign_keys=ON&" +      // Enable foreign key constraints
		"_wal_autocheckpoint=1000&" + // Checkpoint WAL every 1000 pages for performance
		"_journal_size_limit=67108864&" + // 64MB WAL size limit
		"_mmap_size=268435456&" +  // 256MB memory-mapped I/O for better performance
		"_page_size=4096"          // 4KB page size (optimal for most systems)
	
	// Configure GORM with optimized settings for SQLite
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		
		// Optimize for batch operations
		CreateBatchSize: 500, // Smaller batches for SQLite
		
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		
		// SQLite benefits from skipping default transactions in many cases
		SkipDefaultTransaction: true,
		
		// Prepare statements for better performance
		PrepareStmt: true,
	}
	
	return gorm.Open(sqlite.Open(dsn), gormConfig)
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return DB
}

// GetConnectionStats returns current connection pool statistics
func GetConnectionStats() (*sql.DBStats, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	
	sqlDB, err := DB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}
	
	stats := sqlDB.Stats()
	return &stats, nil
}

// LogConnectionStats logs current connection pool statistics
func LogConnectionStats() {
	stats, err := GetConnectionStats()
	if err != nil {
		log.Printf("❌ Failed to get connection stats: %v", err)
		return
	}
	
	log.Printf("📊 Connection Pool Stats:")
	log.Printf("   - Open Connections: %d", stats.OpenConnections)
	log.Printf("   - In Use: %d", stats.InUse)
	log.Printf("   - Idle: %d", stats.Idle)
	log.Printf("   - Wait Count: %d", stats.WaitCount)
	log.Printf("   - Wait Duration: %v", stats.WaitDuration)
	log.Printf("   - Max Idle Closed: %d", stats.MaxIdleClosed)
	log.Printf("   - Max Idle Time Closed: %d", stats.MaxIdleTimeClosed)
	log.Printf("   - Max Lifetime Closed: %d", stats.MaxLifetimeClosed)
}

// HealthCheck performs a comprehensive health check of the database connection
func HealthCheck() error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}
	
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}
	
	// Test basic connectivity
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	
	// Check connection pool health
	stats := sqlDB.Stats()
	if stats.OpenConnections == 0 {
		return fmt.Errorf("no open database connections")
	}
	
	return nil
}
