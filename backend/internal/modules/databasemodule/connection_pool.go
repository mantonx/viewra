package databasemodule

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/logger"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// ConnectionPool manages database connections
type ConnectionPool struct {
	primary   *gorm.DB
	readOnly  *gorm.DB
	analytics *gorm.DB
	maxConns  int
	mu        sync.RWMutex
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(primary *gorm.DB) *ConnectionPool {
	return &ConnectionPool{
		primary:  primary,
		maxConns: 100, // Default max connections
	}
}

// Initialize sets up the connection pool
func (cp *ConnectionPool) Initialize() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	
	logger.Info("Initializing database connection pool")
	
	// Configure connection pool settings
	if err := cp.configureConnectionPool(); err != nil {
		return fmt.Errorf("failed to configure connection pool: %w", err)
	}
	
	// Setup read-only connection if configured
	if err := cp.setupReadOnlyConnection(); err != nil {
		logger.Warn("Failed to setup read-only connection: %v", err)
		// Not fatal, continue with primary connection
	}
	
	// Setup analytics connection if configured
	if err := cp.setupAnalyticsConnection(); err != nil {
		logger.Warn("Failed to setup analytics connection: %v", err)
		// Not fatal, continue with primary connection
	}
	
	logger.Info("Connection pool initialized successfully")
	return nil
}

// configureConnectionPool sets up connection pool parameters
func (cp *ConnectionPool) configureConnectionPool() error {
	sqlDB, err := cp.primary.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}
	
	// Set connection pool settings
	sqlDB.SetMaxOpenConns(cp.maxConns)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(time.Minute * 30)
	
	return nil
}

// setupReadOnlyConnection creates a read-only database connection
func (cp *ConnectionPool) setupReadOnlyConnection() error {
	readOnlyDSN := os.Getenv("READ_ONLY_DATABASE_URL")
	if readOnlyDSN == "" {
		logger.Info("No read-only database configured, using primary connection")
		cp.readOnly = cp.primary
		return nil
	}
	
	dbType := os.Getenv("DATABASE_TYPE")
	if dbType == "" {
		dbType = "sqlite"
	}
	
	var db *gorm.DB
	var err error
	
	switch dbType {
	case "postgres":
		db, err = gorm.Open(postgres.Open(readOnlyDSN), &gorm.Config{
			Logger: gormLogger.Default.LogMode(gormLogger.Silent),
		})
	case "sqlite":
		db, err = gorm.Open(sqlite.Open(readOnlyDSN), &gorm.Config{
			Logger: gormLogger.Default.LogMode(gormLogger.Silent),
		})
	default:
		return fmt.Errorf("unsupported database type for read-only connection: %s", dbType)
	}
	
	if err != nil {
		return fmt.Errorf("failed to create read-only connection: %w", err)
	}
	
	cp.readOnly = db
	logger.Info("Read-only database connection established")
	return nil
}

// setupAnalyticsConnection creates an analytics database connection
func (cp *ConnectionPool) setupAnalyticsConnection() error {
	analyticsDSN := os.Getenv("ANALYTICS_DATABASE_URL")
	if analyticsDSN == "" {
		logger.Info("No analytics database configured, using primary connection")
		cp.analytics = cp.primary
		return nil
	}
	
	dbType := os.Getenv("ANALYTICS_DATABASE_TYPE")
	if dbType == "" {
		dbType = os.Getenv("DATABASE_TYPE")
		if dbType == "" {
			dbType = "sqlite"
		}
	}
	
	var db *gorm.DB
	var err error
	
	switch dbType {
	case "postgres":
		db, err = gorm.Open(postgres.Open(analyticsDSN), &gorm.Config{
			Logger: gormLogger.Default.LogMode(gormLogger.Silent),
		})
	case "sqlite":
		db, err = gorm.Open(sqlite.Open(analyticsDSN), &gorm.Config{
			Logger: gormLogger.Default.LogMode(gormLogger.Silent),
		})
	default:
		return fmt.Errorf("unsupported database type for analytics connection: %s", dbType)
	}
	
	if err != nil {
		return fmt.Errorf("failed to create analytics connection: %w", err)
	}
	
	cp.analytics = db
	logger.Info("Analytics database connection established")
	return nil
}

// GetPrimary returns the primary database connection
func (cp *ConnectionPool) GetPrimary() *gorm.DB {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.primary
}

// GetReadOnly returns the read-only database connection
func (cp *ConnectionPool) GetReadOnly() *gorm.DB {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	if cp.readOnly != nil {
		return cp.readOnly
	}
	return cp.primary // Fallback to primary
}

// GetAnalytics returns the analytics database connection
func (cp *ConnectionPool) GetAnalytics() *gorm.DB {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	if cp.analytics != nil {
		return cp.analytics
	}
	return cp.primary // Fallback to primary
}

// Health checks the health of all connections in the pool
func (cp *ConnectionPool) Health() error {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	
	// Check primary connection
	if err := cp.checkConnection(cp.primary, "primary"); err != nil {
		return err
	}
	
	// Check read-only connection if different from primary
	if cp.readOnly != nil && cp.readOnly != cp.primary {
		if err := cp.checkConnection(cp.readOnly, "read-only"); err != nil {
			logger.Warn("Read-only connection health check failed: %v", err)
			// Not fatal, continue
		}
	}
	
	// Check analytics connection if different from primary
	if cp.analytics != nil && cp.analytics != cp.primary {
		if err := cp.checkConnection(cp.analytics, "analytics"); err != nil {
			logger.Warn("Analytics connection health check failed: %v", err)
			// Not fatal, continue
		}
	}
	
	return nil
}

// checkConnection performs a health check on a specific connection
func (cp *ConnectionPool) checkConnection(db *gorm.DB, name string) error {
	if db == nil {
		return fmt.Errorf("%s connection is nil", name)
	}
	
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get %s sql.DB: %w", name, err)
	}
	
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("%s connection ping failed: %w", name, err)
	}
	
	return nil
}

// GetStats returns connection pool statistics
func (cp *ConnectionPool) GetStats() map[string]interface{} {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	
	stats := make(map[string]interface{})
	
	// Get primary connection stats
	if sqlDB, err := cp.primary.DB(); err == nil {
		dbStats := sqlDB.Stats()
		stats["primary"] = map[string]interface{}{
			"open_connections":     dbStats.OpenConnections,
			"in_use":              dbStats.InUse,
			"idle":                dbStats.Idle,
			"wait_count":          dbStats.WaitCount,
			"wait_duration":       dbStats.WaitDuration.String(),
			"max_idle_closed":     dbStats.MaxIdleClosed,
			"max_idle_time_closed": dbStats.MaxIdleTimeClosed,
			"max_lifetime_closed": dbStats.MaxLifetimeClosed,
		}
	}
	
	// Add read-only stats if available
	if cp.readOnly != nil && cp.readOnly != cp.primary {
		if sqlDB, err := cp.readOnly.DB(); err == nil {
			dbStats := sqlDB.Stats()
			stats["read_only"] = map[string]interface{}{
				"open_connections": dbStats.OpenConnections,
				"in_use":          dbStats.InUse,
				"idle":            dbStats.Idle,
			}
		}
	}
	
	// Add analytics stats if available
	if cp.analytics != nil && cp.analytics != cp.primary {
		if sqlDB, err := cp.analytics.DB(); err == nil {
			dbStats := sqlDB.Stats()
			stats["analytics"] = map[string]interface{}{
				"open_connections": dbStats.OpenConnections,
				"in_use":          dbStats.InUse,
				"idle":            dbStats.Idle,
			}
		}
	}
	
	return stats
}

// Close closes all connections in the pool
func (cp *ConnectionPool) Close() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	
	var errors []error
	
	// Close primary connection
	if cp.primary != nil {
		if sqlDB, err := cp.primary.DB(); err == nil {
			if err := sqlDB.Close(); err != nil {
				errors = append(errors, fmt.Errorf("failed to close primary connection: %w", err))
			}
		}
	}
	
	// Close read-only connection if different from primary
	if cp.readOnly != nil && cp.readOnly != cp.primary {
		if sqlDB, err := cp.readOnly.DB(); err == nil {
			if err := sqlDB.Close(); err != nil {
				errors = append(errors, fmt.Errorf("failed to close read-only connection: %w", err))
			}
		}
	}
	
	// Close analytics connection if different from primary
	if cp.analytics != nil && cp.analytics != cp.primary {
		if sqlDB, err := cp.analytics.DB(); err == nil {
			if err := sqlDB.Close(); err != nil {
				errors = append(errors, fmt.Errorf("failed to close analytics connection: %w", err))
			}
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("connection pool close errors: %v", errors)
	}
	
	logger.Info("Connection pool closed successfully")
	return nil
}
