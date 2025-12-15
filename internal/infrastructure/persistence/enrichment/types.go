package enrichment

import (
	"database/sql"

	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/adapters"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// QueueRepository implements enrichment.QueueRepository using sqlc.
// It supports both SQLite and PostgreSQL through database-specific queriers.
type QueueRepository struct {
	db       *sql.DB
	dbType   string
	sqlite   *sqlc_sqlite.Queries
	postgres *sqlc_postgres.Queries
	adapter  *adapters.TypeAdapter
	router   *common.QueryRouter
}

// StatusRepository implements enrichment.StatusRepository using sqlc.
// It supports both SQLite and PostgreSQL through database-specific queriers.
type StatusRepository struct {
	db       *sql.DB
	dbType   string
	sqlite   *sqlc_sqlite.Queries
	postgres *sqlc_postgres.Queries
	adapter  *adapters.TypeAdapter
	router   *common.QueryRouter
}

// PipelineRepository implements enrichment.PipelineRepository using sqlc.
// It supports both SQLite and PostgreSQL through database-specific queriers.
type PipelineRepository struct {
	db       *sql.DB
	dbType   string
	sqlite   *sqlc_sqlite.Queries
	postgres *sqlc_postgres.Queries
	adapter  *adapters.TypeAdapter
	router   *common.QueryRouter
}

// ExternalIDRepository implements enrichment.ExternalIDRepository using sqlc.
// It supports both SQLite and PostgreSQL through database-specific queriers.
type ExternalIDRepository struct {
	db       *sql.DB
	dbType   string
	sqlite   *sqlc_sqlite.Queries
	postgres *sqlc_postgres.Queries
	adapter  *adapters.TypeAdapter
	router   *common.QueryRouter
}
