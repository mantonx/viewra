package scanjob

import (
	"database/sql"

	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/viewra/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/viewra/viewra/internal/infrastructure/persistence/adapters"
	"github.com/viewra/viewra/internal/infrastructure/persistence/common"
)

// Repository implements scanner.ScanJobRepository using sqlc.
// It supports both SQLite and PostgreSQL through database-specific queriers.
type Repository struct {
	db       *sql.DB
	dbType   string
	sqlite   *sqlc_sqlite.Queries
	postgres *sqlc_postgres.Queries
	adapter  *adapters.TypeAdapter
	router   *common.QueryRouter
}
