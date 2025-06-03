package scanner

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/logger"
	"gorm.io/gorm"
)

// Additional safeguard helper types
type TransactionManager struct {
	db                 *gorm.DB
	mu                 sync.RWMutex
	activeTransactions map[string]*gorm.DB
}

type HealthChecker struct {
	db      *gorm.DB
	manager *Manager
	running bool
	mu      sync.RWMutex
	stopCh  chan struct{}
}

type WatchdogService struct {
	db      *gorm.DB
	manager *Manager
	config  *SafeguardConfig
	running bool
	mu      sync.RWMutex
	stopCh  chan struct{}
}

type StateValidator struct {
	db      *gorm.DB
	manager *Manager
	config  *SafeguardConfig
	running bool
	mu      sync.RWMutex
	stopCh  chan struct{}
}

type CleanupScheduler struct {
	db      *gorm.DB
	manager *Manager
	config  *SafeguardConfig
	running bool
	mu      sync.RWMutex
	stopCh  chan struct{}
}

func NewLockManager() *LockManager {
	return &LockManager{
		libraryLocks: make(map[uint32]*sync.Mutex),
		jobLocks:     make(map[uint32]*sync.Mutex),
	}
}

// AcquireJobLock acquires a lock for the specified job
func (lm *LockManager) AcquireJobLock(jobID uint32) error {
	lm.mu.Lock()
	if lm.jobLocks[jobID] == nil {
		lm.jobLocks[jobID] = &sync.Mutex{}
	}
	lock := lm.jobLocks[jobID]
	lm.mu.Unlock()

	// Try to acquire the lock with timeout
	done := make(chan struct{})
	go func() {
		lock.Lock()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout acquiring job lock for job %d", jobID)
	}
}

// ReleaseJobLock releases the lock for the specified job
func (lm *LockManager) ReleaseJobLock(jobID uint32) {
	lm.mu.RLock()
	if lock, exists := lm.jobLocks[jobID]; exists {
		lock.Unlock()
	}
	lm.mu.RUnlock()
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(db *gorm.DB) *TransactionManager {
	return &TransactionManager{
		db:                 db,
		activeTransactions: make(map[string]*gorm.DB),
	}
}

// BeginTransaction starts a new database transaction
func (tm *TransactionManager) BeginTransaction() (*gorm.DB, string, error) {
	tx := tm.db.Begin()
	if tx.Error != nil {
		return nil, "", fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Generate unique transaction ID
	bytes := make([]byte, 8)
	rand.Read(bytes)
	txID := hex.EncodeToString(bytes)

	tm.mu.Lock()
	tm.activeTransactions[txID] = tx
	tm.mu.Unlock()

	return tx, txID, nil
}

// CommitTransaction commits the specified transaction
func (tm *TransactionManager) CommitTransaction(txID string) error {
	tm.mu.Lock()
	tx, exists := tm.activeTransactions[txID]
	if exists {
		delete(tm.activeTransactions, txID)
	}
	tm.mu.Unlock()

	if !exists {
		return fmt.Errorf("transaction %s not found", txID)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// RollbackTransaction rolls back the specified transaction
func (tm *TransactionManager) RollbackTransaction(txID string) error {
	tm.mu.Lock()
	tx, exists := tm.activeTransactions[txID]
	if exists {
		delete(tm.activeTransactions, txID)
	}
	tm.mu.Unlock()

	if !exists {
		return fmt.Errorf("transaction %s not found", txID)
	}

	if err := tx.Rollback().Error; err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}

	return nil
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(db *gorm.DB, manager *Manager) *HealthChecker {
	return &HealthChecker{
		db:      db,
		manager: manager,
		stopCh:  make(chan struct{}),
	}
}

// Start begins health checking
func (hc *HealthChecker) Start() error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hc.running {
		return fmt.Errorf("health checker already running")
	}

	hc.running = true
	go hc.run()
	return nil
}

// Stop stops health checking
func (hc *HealthChecker) Stop() error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if !hc.running {
		return nil
	}

	hc.running = false
	close(hc.stopCh)
	return nil
}

func (hc *HealthChecker) run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.performHealthCheck()
		case <-hc.stopCh:
			return
		}
	}
}

func (hc *HealthChecker) performHealthCheck() {
	// Check database connectivity
	sqlDB, err := hc.db.DB()
	if err != nil {
		logger.Error("Health check: database connection error", "error", err)
		return
	}

	if err := sqlDB.Ping(); err != nil {
		logger.Error("Health check: database ping failed", "error", err)
		return
	}

	// Check for stuck scan jobs
	var stuckJobs []database.ScanJob
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute)

	err = hc.db.Where("status = ? AND updated_at < ?", "running", fiveMinutesAgo).Find(&stuckJobs).Error
	if err != nil {
		logger.Error("Health check: failed to query stuck jobs", "error", err)
		return
	}

	if len(stuckJobs) > 0 {
		logger.Warn("Health check: found stuck scan jobs", "count", len(stuckJobs))
		for _, job := range stuckJobs {
			logger.Warn("Stuck scan job detected", "job_id", job.ID, "library_id", job.LibraryID, "last_update", job.UpdatedAt)
		}
	}
}
