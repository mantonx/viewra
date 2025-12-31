package common

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
)

// TransactionContext provides a database-agnostic interface for transactional operations.
// It allows code to work with both SQLite and PostgreSQL transactions uniformly.
type TransactionContext struct {
	tx      *sql.Tx
	dbType  string
	querier *unified.Querier
}

// Q returns the unified Querier bound to this transaction.
// This is the preferred way to access the database within a transaction.
func (tc *TransactionContext) Q() *unified.Querier {
	return tc.querier
}

// IsPostgresDB returns true if this is a PostgreSQL transaction
func (tc *TransactionContext) IsPostgresDB() bool {
	return IsPostgres(tc.dbType)
}

// IsSQLiteDB returns true if this is a SQLite transaction
func (tc *TransactionContext) IsSQLiteDB() bool {
	return !IsPostgres(tc.dbType)
}

// Commit commits the transaction
func (tc *TransactionContext) Commit() error {
	if tc.tx == nil {
		return fmt.Errorf("no active transaction to commit")
	}
	return tc.tx.Commit()
}

// Rollback rolls back the transaction
func (tc *TransactionContext) Rollback() error {
	if tc.tx == nil {
		return fmt.Errorf("no active transaction to rollback")
	}
	return tc.tx.Rollback()
}

// WithTransaction executes a function within a database transaction.
// The transaction is automatically committed if the function returns nil,
// or rolled back if it returns an error or panics.
//
// Example usage:
//
//	err := common.WithTransaction(repo.BaseRepository, ctx, func(tx *common.TransactionContext) error {
//	    // Create artist
//	    artist, err := tx.Q().CreateArtist(ctx, params)
//	    if err != nil {
//	        return err
//	    }
//	    // Create album
//	    album, err := tx.Q().CreateAlbum(ctx, albumParams)
//	    if err != nil {
//	        return err
//	    }
//	    return nil
//	})
func WithTransaction(repo *BaseRepository, ctx context.Context, fn func(tx *TransactionContext) error) error {
	// Begin transaction
	sqlTx, err := repo.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Create transaction context with unified querier
	txCtx := &TransactionContext{
		tx:      sqlTx,
		dbType:  repo.DBType(),
		querier: unified.NewQuerierWithTx(sqlTx, repo.DBType()),
	}

	// Ensure transaction is finalized
	defer func() {
		if p := recover(); p != nil {
			// Panic occurred, rollback and re-panic
			_ = txCtx.Rollback()
			panic(p)
		}
	}()

	// Execute the transaction function
	if err := fn(txCtx); err != nil {
		// Function returned error, rollback
		if rbErr := txCtx.Rollback(); rbErr != nil {
			return errors.Join(fmt.Errorf("transaction failed: %w", err), fmt.Errorf("rollback error: %w", rbErr))
		}
		return err
	}

	// Success, commit transaction
	if err := txCtx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
