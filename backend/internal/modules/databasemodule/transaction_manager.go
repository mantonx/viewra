package databasemodule

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mantonx/viewra/internal/logger"
	"gorm.io/gorm"
)

// TransactionManager handles database transactions
type TransactionManager struct {
	db *gorm.DB
	mu sync.RWMutex
}

// TransactionContext wraps a transaction for safe handling
type TransactionContext struct {
	tx      *gorm.DB
	ctx     context.Context
	started time.Time
	id      string
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(db *gorm.DB) *TransactionManager {
	return &TransactionManager{
		db: db,
	}
}

// Initialize sets up the transaction manager
func (tm *TransactionManager) Initialize() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	logger.Info("Initializing transaction manager")

	// Transaction manager doesn't need initialization steps
	// but we keep this for consistency with other components

	logger.Info("Transaction manager initialized successfully")
	return nil
}

// BeginTransaction starts a new database transaction
func (tm *TransactionManager) BeginTransaction(ctx context.Context) (*TransactionContext, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// Start a new transaction
	tx := tm.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Generate transaction ID
	txID := fmt.Sprintf("tx_%d", time.Now().UnixNano())

	txCtx := &TransactionContext{
		tx:      tx,
		ctx:     ctx,
		started: time.Now(),
		id:      txID,
	}

	logger.Debug("Started transaction: %s", txID)
	return txCtx, nil
}

// BeginTransactionWithOptions starts a new transaction with specific options
func (tm *TransactionManager) BeginTransactionWithOptions(ctx context.Context, opts *TransactionOptions) (*TransactionContext, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// Start transaction (GORM doesn't directly support sql.TxOptions in Begin)
	tx := tm.db.Begin()

	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction with options: %w", tx.Error)
	}

	// Set isolation level if specified
	if opts.IsolationLevel != "" {
		if err := tx.Exec(fmt.Sprintf("SET TRANSACTION ISOLATION LEVEL %s", opts.IsolationLevel)).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to set isolation level: %w", err)
		}
	}

	// Set timeout if specified
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)

		// Start a goroutine to automatically rollback on timeout
		go func() {
			<-ctx.Done()
			if ctx.Err() == context.DeadlineExceeded {
				logger.Warn("Transaction timeout, rolling back")
				tx.Rollback()
			}
			cancel()
		}()
	}

	txID := fmt.Sprintf("tx_%d", time.Now().UnixNano())

	txCtx := &TransactionContext{
		tx:      tx,
		ctx:     ctx,
		started: time.Now(),
		id:      txID,
	}

	logger.Debug("Started transaction with options: %s", txID)
	return txCtx, nil
}

// TransactionOptions defines options for database transactions
type TransactionOptions struct {
	IsolationLevel               string
	Timeout                      time.Duration
	PrepareStatements            bool
	DisableForeignKeyConstraints bool
	ReadOnly                     bool
}

// Commit commits the transaction
func (tc *TransactionContext) Commit() error {
	if tc.tx == nil {
		return fmt.Errorf("transaction context is nil")
	}

	if err := tc.tx.Commit().Error; err != nil {
		logger.Error("Failed to commit transaction %s: %v", tc.id, err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	duration := time.Since(tc.started)
	logger.Debug("Committed transaction %s (duration: %v)", tc.id, duration)

	// Clear the transaction to prevent reuse
	tc.tx = nil

	return nil
}

// Rollback rolls back the transaction
func (tc *TransactionContext) Rollback() error {
	if tc.tx == nil {
		return fmt.Errorf("transaction context is nil")
	}

	if err := tc.tx.Rollback().Error; err != nil {
		logger.Error("Failed to rollback transaction %s: %v", tc.id, err)
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}

	duration := time.Since(tc.started)
	logger.Debug("Rolled back transaction %s (duration: %v)", tc.id, duration)

	// Clear the transaction to prevent reuse
	tc.tx = nil

	return nil
}

// DB returns the transaction database instance
func (tc *TransactionContext) DB() *gorm.DB {
	return tc.tx
}

// ID returns the transaction ID
func (tc *TransactionContext) ID() string {
	return tc.id
}

// Started returns when the transaction was started
func (tc *TransactionContext) Started() time.Time {
	return tc.started
}

// Duration returns how long the transaction has been running
func (tc *TransactionContext) Duration() time.Duration {
	return time.Since(tc.started)
}

// Context returns the context associated with the transaction
func (tc *TransactionContext) Context() context.Context {
	return tc.ctx
}

// IsActive checks if the transaction is still active
func (tc *TransactionContext) IsActive() bool {
	return tc.tx != nil
}

// WithTransaction executes a function within a transaction
func (tm *TransactionManager) WithTransaction(ctx context.Context, fn func(*gorm.DB) error) error {
	txCtx, err := tm.BeginTransaction(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if txCtx.IsActive() {
			// If transaction is still active, something went wrong
			txCtx.Rollback()
		}
	}()

	// Execute the function
	if err := fn(txCtx.DB()); err != nil {
		if rollbackErr := txCtx.Rollback(); rollbackErr != nil {
			logger.Error("Failed to rollback transaction after error: %v", rollbackErr)
		}
		return err
	}

	// Commit the transaction
	return txCtx.Commit()
}

// WithTransactionAndOptions executes a function within a transaction with options
func (tm *TransactionManager) WithTransactionAndOptions(ctx context.Context, opts *TransactionOptions, fn func(*gorm.DB) error) error {
	txCtx, err := tm.BeginTransactionWithOptions(ctx, opts)
	if err != nil {
		return err
	}

	defer func() {
		if txCtx.IsActive() {
			// If transaction is still active, something went wrong
			txCtx.Rollback()
		}
	}()

	// Execute the function
	if err := fn(txCtx.DB()); err != nil {
		if rollbackErr := txCtx.Rollback(); rollbackErr != nil {
			logger.Error("Failed to rollback transaction after error: %v", rollbackErr)
		}
		return err
	}

	// Commit the transaction
	return txCtx.Commit()
}

// GetStats returns transaction manager statistics
func (tm *TransactionManager) GetStats() map[string]interface{} {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	stats := make(map[string]interface{})

	// Get database connection stats (includes transaction info)
	if sqlDB, err := tm.db.DB(); err == nil {
		dbStats := sqlDB.Stats()
		stats["connection_stats"] = map[string]interface{}{
			"open_connections": dbStats.OpenConnections,
			"in_use":           dbStats.InUse,
			"idle":             dbStats.Idle,
		}
	}

	stats["transaction_manager"] = "active"
	return stats
}
