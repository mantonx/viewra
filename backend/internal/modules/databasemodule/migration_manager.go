package databasemodule

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"gorm.io/gorm"
)

// MigrationFunc represents a database migration function
type MigrationFunc func(*gorm.DB) error

// Migration represents a database migration
type Migration struct {
	ID          string
	Description string
	Up          MigrationFunc
	Down        MigrationFunc
	AppliedAt   *time.Time
}

// MigrationRecord represents a migration record in the database
type MigrationRecord struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	Description string    `json:"description"`
	AppliedAt   time.Time `json:"applied_at"`
	Checksum    string    `json:"checksum"`
}

// MigrationManager handles database migrations
type MigrationManager struct {
	db         *gorm.DB
	migrations map[string]*Migration
	mu         sync.RWMutex
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(db *gorm.DB) (*MigrationManager, error) {
	mm := &MigrationManager{
		db:         db,
		migrations: make(map[string]*Migration),
	}

	// Create migrations table
	if err := mm.createMigrationsTable(); err != nil {
		return nil, fmt.Errorf("failed to create migrations table: %w", err)
	}

	return mm, nil
}

// RegisterMigration registers a new migration
func (mm *MigrationManager) RegisterMigration(migration *Migration) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if migration.ID == "" {
		return fmt.Errorf("migration ID cannot be empty")
	}

	if migration.Up == nil {
		return fmt.Errorf("migration up function cannot be nil")
	}

	if _, exists := mm.migrations[migration.ID]; exists {
		return fmt.Errorf("migration with ID %s already exists", migration.ID)
	}

	mm.migrations[migration.ID] = migration
	return nil
}

// GetMigrations returns all registered migrations
func (mm *MigrationManager) GetMigrations() []*Migration {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	migrations := make([]*Migration, 0, len(mm.migrations))
	for _, migration := range mm.migrations {
		migrations = append(migrations, migration)
	}

	// Sort migrations by ID for consistent ordering
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].ID < migrations[j].ID
	})

	return migrations
}

// GetPendingMigrations returns migrations that haven't been applied
func (mm *MigrationManager) GetPendingMigrations() ([]*Migration, error) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	// Get applied migrations from database
	var records []MigrationRecord
	if err := mm.db.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Create a set of applied migration IDs
	applied := make(map[string]bool)
	for _, record := range records {
		applied[record.ID] = true
	}

	// Find pending migrations
	var pending []*Migration
	for _, migration := range mm.migrations {
		if !applied[migration.ID] {
			pending = append(pending, migration)
		}
	}

	// Sort by ID for consistent ordering
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].ID < pending[j].ID
	})

	return pending, nil
}

// RunMigrations executes all pending migrations
func (mm *MigrationManager) RunMigrations() error {
	pending, err := mm.GetPendingMigrations()
	if err != nil {
		return err
	}

	if len(pending) == 0 {
		return nil
	}

	for _, migration := range pending {
		if err := mm.runMigration(migration); err != nil {
			return fmt.Errorf("failed to run migration %s: %w", migration.ID, err)
		}
	}

	return nil
}

// RunMigration executes a specific migration
func (mm *MigrationManager) RunMigration(migrationID string) error {
	mm.mu.RLock()
	migration, exists := mm.migrations[migrationID]
	mm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("migration %s not found", migrationID)
	}

	// Check if already applied
	var record MigrationRecord
	err := mm.db.Where("id = ?", migrationID).First(&record).Error
	if err == nil {
		return fmt.Errorf("migration %s already applied", migrationID)
	}
	if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check migration status: %w", err)
	}

	return mm.runMigration(migration)
}

// RollbackMigration rolls back a specific migration
func (mm *MigrationManager) RollbackMigration(migrationID string) error {
	mm.mu.RLock()
	migration, exists := mm.migrations[migrationID]
	mm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("migration %s not found", migrationID)
	}

	if migration.Down == nil {
		return fmt.Errorf("migration %s has no rollback function", migrationID)
	}

	// Check if migration is applied
	var record MigrationRecord
	err := mm.db.Where("id = ?", migrationID).First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("migration %s not applied", migrationID)
		}
		return fmt.Errorf("failed to check migration status: %w", err)
	}

	// Run rollback in transaction
	return mm.db.Transaction(func(tx *gorm.DB) error {
		if err := migration.Down(tx); err != nil {
			return fmt.Errorf("rollback function failed: %w", err)
		}

		// Remove migration record
		if err := tx.Delete(&MigrationRecord{}, "id = ?", migrationID).Error; err != nil {
			return fmt.Errorf("failed to remove migration record: %w", err)
		}

		return nil
	})
}

// GetMigrationStatus returns the status of all migrations
func (mm *MigrationManager) GetMigrationStatus() ([]MigrationStatus, error) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	// Get applied migrations from database
	var records []MigrationRecord
	if err := mm.db.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Create a map of applied migrations
	applied := make(map[string]*MigrationRecord)
	for i := range records {
		applied[records[i].ID] = &records[i]
	}

	// Build status for all migrations
	var statuses []MigrationStatus
	for _, migration := range mm.migrations {
		status := MigrationStatus{
			ID:          migration.ID,
			Description: migration.Description,
			Applied:     false,
		}

		if record, exists := applied[migration.ID]; exists {
			status.Applied = true
			status.AppliedAt = &record.AppliedAt
		}

		statuses = append(statuses, status)
	}

	// Sort by ID
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].ID < statuses[j].ID
	})

	return statuses, nil
}

// MigrationStatus represents the status of a migration
type MigrationStatus struct {
	ID          string     `json:"id"`
	Description string     `json:"description"`
	Applied     bool       `json:"applied"`
	AppliedAt   *time.Time `json:"applied_at,omitempty"`
}

// createMigrationsTable creates the migrations tracking table
func (mm *MigrationManager) createMigrationsTable() error {
	return mm.db.AutoMigrate(&MigrationRecord{})
}

// runMigration executes a migration in a transaction
func (mm *MigrationManager) runMigration(migration *Migration) error {
	return mm.db.Transaction(func(tx *gorm.DB) error {
		// Run the migration
		if err := migration.Up(tx); err != nil {
			return fmt.Errorf("migration function failed: %w", err)
		}

		// Record the migration
		record := MigrationRecord{
			ID:          migration.ID,
			Description: migration.Description,
			AppliedAt:   time.Now(),
			Checksum:    "", // TODO: Implement checksum for migration content validation
		}

		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("failed to record migration: %w", err)
		}

		return nil
	})
}

// GetStats returns migration statistics
func (mm *MigrationManager) GetStats() (MigrationStats, error) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	var stats MigrationStats
	stats.TotalMigrations = len(mm.migrations)

	// Count applied migrations
	var appliedCount int64
	if err := mm.db.Model(&MigrationRecord{}).Count(&appliedCount).Error; err != nil {
		return stats, fmt.Errorf("failed to count applied migrations: %w", err)
	}

	stats.AppliedMigrations = int(appliedCount)
	stats.PendingMigrations = stats.TotalMigrations - stats.AppliedMigrations

	// Get last migration time
	var lastRecord MigrationRecord
	err := mm.db.Order("applied_at DESC").First(&lastRecord).Error
	if err == nil {
		stats.LastMigrationAt = &lastRecord.AppliedAt
	} else if err != gorm.ErrRecordNotFound {
		return stats, fmt.Errorf("failed to get last migration time: %w", err)
	}

	return stats, nil
}

// MigrationStats represents migration statistics
type MigrationStats struct {
	TotalMigrations   int        `json:"total_migrations"`
	AppliedMigrations int        `json:"applied_migrations"`
	PendingMigrations int        `json:"pending_migrations"`
	LastMigrationAt   *time.Time `json:"last_migration_at,omitempty"`
}

// Initialize sets up the migration manager
func (mm *MigrationManager) Initialize() error {
	mm.mu.Lock()
	defer mm.mu.Unlock()
	
	// Migration manager is already initialized when created
	// This method is kept for consistency with other components
	return nil
}

// ExecutePendingMigrations is an alias for RunMigrations for API compatibility
func (mm *MigrationManager) ExecutePendingMigrations() error {
	return mm.RunMigrations()
}
