package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/mantonx/viewra/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// ConnectionPoolConfig holds configuration for database connection pooling
type ConnectionPoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// GetOptimalPoolConfig returns optimized connection pool settings based on database type and environment
func GetOptimalPoolConfig(dbType string) *ConnectionPoolConfig {
	// Check for custom environment variables
	maxOpenConns := getEnvInt("DB_MAX_OPEN_CONNS", 0)
	maxIdleConns := getEnvInt("DB_MAX_IDLE_CONNS", 0)
	connMaxLifetime := getEnvDuration("DB_CONN_MAX_LIFETIME", 0)
	connMaxIdleTime := getEnvDuration("DB_CONN_MAX_IDLE_TIME", 0)
	
	// Set defaults based on database type and use case
	switch dbType {
	case "postgres":
		// PostgreSQL can handle more concurrent connections efficiently
		if maxOpenConns == 0 {
			maxOpenConns = 100 // High for media scanner workloads
		}
		if maxIdleConns == 0 {
			maxIdleConns = 20 // Keep more idle connections for PostgreSQL
		}
		if connMaxLifetime == 0 {
			connMaxLifetime = 2 * time.Hour // Longer for PostgreSQL
		}
		if connMaxIdleTime == 0 {
			connMaxIdleTime = 30 * time.Minute
		}
		
	case "sqlite":
		// SQLite has more limitations but we can still optimize
		if maxOpenConns == 0 {
			maxOpenConns = 25 // Conservative for SQLite but higher than default
		}
		if maxIdleConns == 0 {
			maxIdleConns = 5 // Lower for SQLite
		}
		if connMaxLifetime == 0 {
			connMaxLifetime = 1 * time.Hour
		}
		if connMaxIdleTime == 0 {
			connMaxIdleTime = 15 * time.Minute
		}
		
	default:
		// Default conservative settings
		if maxOpenConns == 0 {
			maxOpenConns = 10
		}
		if maxIdleConns == 0 {
			maxIdleConns = 2
		}
		if connMaxLifetime == 0 {
			connMaxLifetime = 1 * time.Hour
		}
		if connMaxIdleTime == 0 {
			connMaxIdleTime = 10 * time.Minute
		}
	}
	
	return &ConnectionPoolConfig{
		MaxOpenConns:    maxOpenConns,
		MaxIdleConns:    maxIdleConns,
		ConnMaxLifetime: connMaxLifetime,
		ConnMaxIdleTime: connMaxIdleTime,
	}
}

// getEnvInt gets an integer environment variable with fallback
func getEnvInt(key string, defaultVal int) int {
	if str := os.Getenv(key); str != "" {
		if val, err := strconv.Atoi(str); err == nil {
			return val
		}
	}
	return defaultVal
}

// getEnvDuration gets a duration environment variable with fallback
func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if str := os.Getenv(key); str != "" {
		if val, err := time.ParseDuration(str); err == nil {
			return val
		}
	}
	return defaultVal
}

// configureConnectionPool applies connection pool settings to a database connection
func configureConnectionPool(db *gorm.DB, dbType string) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	
	config := GetOptimalPoolConfig(dbType)
	
	// Apply connection pool settings
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	
	// Log the applied configuration
	log.Printf("✅ Connection pool configured for %s:", dbType)
	log.Printf("   - Max Open Connections: %d", config.MaxOpenConns)
	log.Printf("   - Max Idle Connections: %d", config.MaxIdleConns)
	log.Printf("   - Connection Max Lifetime: %v", config.ConnMaxLifetime)
	log.Printf("   - Connection Max Idle Time: %v", config.ConnMaxIdleTime)
	
	return nil
}

// Initialize sets up the database connection based on the DATABASE_TYPE environment variable
func Initialize() {
	var err error
	
	dbType := os.Getenv("DATABASE_TYPE")
	if dbType == "" {
		dbType = "sqlite" // default to SQLite
	}
	
	switch dbType {
	case "postgres":
		DB, err = connectPostgres()
	case "sqlite":
		DB, err = connectSQLite()
	default:
		log.Fatalf("Unsupported database type: %s", dbType)
	}

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Configure connection pool for optimal performance
	if err := configureConnectionPool(DB, dbType); err != nil {
		log.Printf("⚠️  Warning: Failed to configure connection pool: %v", err)
	}

	// Test the connection pool
	if err := testConnectionPool(DB); err != nil {
		log.Printf("⚠️  Warning: Connection pool test failed: %v", err)
	}
	
	// Auto-migrate the schema
	err = DB.AutoMigrate(
		&Media{}, &User{}, &MediaLibrary{}, &MediaFile{}, &MusicMetadata{}, &ScanJob{},
		// Plugin system tables
		&Plugin{}, &PluginPermission{}, &PluginEvent{}, &PluginHook{}, &PluginAdminPage{}, &PluginUIComponent{},
		// Event system tables
		&SystemEvent{},
	)
	if err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	log.Printf("✅ Database initialized with %s at %s", dbType, config.GetDatabasePath())
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

func connectPostgres() (*gorm.DB, error) {
	host := os.Getenv("POSTGRES_HOST")
	port := os.Getenv("POSTGRES_PORT")
	user := os.Getenv("POSTGRES_USER")
	password := os.Getenv("POSTGRES_PASSWORD")
	dbname := os.Getenv("POSTGRES_DB")
	
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}
	
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
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

func connectSQLite() (*gorm.DB, error) {
	dbPath := config.GetDatabasePath()
	
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
