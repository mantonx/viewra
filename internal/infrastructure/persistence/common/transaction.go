package common

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
)

// TransactionContext provides a database-agnostic interface for transactional operations.
// It allows code to work with both SQLite and PostgreSQL transactions uniformly.
type TransactionContext struct {
	sqliteTx   *sql.Tx
	postgresTx *sql.Tx
	dbType     string
}

// SQLite returns a sqlc Queries instance bound to the SQLite transaction
func (tc *TransactionContext) SQLite() *sqlc_sqlite.Queries {
	if tc.sqliteTx == nil {
		return nil
	}
	return sqlc_sqlite.New(tc.sqliteTx)
}

// Postgres returns a sqlc Queries instance bound to the PostgreSQL transaction
func (tc *TransactionContext) Postgres() *sqlc_postgres.Queries {
	if tc.postgresTx == nil {
		return nil
	}
	return sqlc_postgres.New(tc.postgresTx)
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
	if tc.postgresTx != nil {
		return tc.postgresTx.Commit()
	}
	if tc.sqliteTx != nil {
		return tc.sqliteTx.Commit()
	}
	return fmt.Errorf("no active transaction to commit")
}

// Rollback rolls back the transaction
func (tc *TransactionContext) Rollback() error {
	if tc.postgresTx != nil {
		return tc.postgresTx.Rollback()
	}
	if tc.sqliteTx != nil {
		return tc.sqliteTx.Rollback()
	}
	return fmt.Errorf("no active transaction to rollback")
}

// WithTransaction executes a function within a database transaction.
// The transaction is automatically committed if the function returns nil,
// or rolled back if it returns an error or panics.
//
// Example usage:
//
//	err := common.WithTransaction(repo.BaseRepository, ctx, func(tx *common.TransactionContext) error {
//	    // Create artist
//	    artist, err := tx.Postgres().CreateArtist(ctx, params)
//	    if err != nil {
//	        return err
//	    }
//	    // Create album
//	    album, err := tx.Postgres().CreateAlbum(ctx, albumParams)
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

	// Create transaction context
	txCtx := &TransactionContext{
		dbType: repo.DBType(),
	}

	if repo.Router().IsPostgresDB() {
		txCtx.postgresTx = sqlTx
	} else {
		txCtx.sqliteTx = sqlTx
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
