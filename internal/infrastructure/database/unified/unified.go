// Package unified provides a database-agnostic interface for sqlc-generated code.
//
// The Querier type wraps both SQLite and PostgreSQL backends and routes calls
// to the appropriate implementation. Type aliases in types.go allow repositories
// to use a single set of types regardless of the underlying database.
package unified

import (
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
)

// IsPostgres returns true if the driver is PostgreSQL.
func IsPostgres(driver string) bool {
	return driver == "postgres" || driver == "postgresql" || driver == "pgx"
}

// NewQuerierWithTx creates a new unified Querier that uses the provided transaction.
func NewQuerierWithTx(tx DBTX, driver string) *Querier {
	if IsPostgres(driver) {
		return &Querier{
			postgres:   sqlc_postgres.New(tx),
			isPostgres: true,
		}
	}
	return &Querier{
		sqlite: sqlc_sqlite.New(tx),
	}
}
