package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/adapters"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
	"github.com/sqlc-dev/pqtype"
)

// parseMonitoringConfig parses JSON monitoring config from a nullable string (SQLite).
func parseMonitoringConfig(configJSON sql.NullString) *library.MonitoringConfig {
	if !configJSON.Valid || configJSON.String == "" {
		return nil
	}
	var config library.MonitoringConfig
	if err := json.Unmarshal([]byte(configJSON.String), &config); err != nil {
		return nil
	}
	return &config
}

// parseMonitoringConfigPG parses JSON monitoring config from a PostgreSQL JSONB type.
func parseMonitoringConfigPG(configJSON pqtype.NullRawMessage) *library.MonitoringConfig {
	if !configJSON.Valid || len(configJSON.RawMessage) == 0 {
		return nil
	}
	var config library.MonitoringConfig
	if err := json.Unmarshal(configJSON.RawMessage, &config); err != nil {
		return nil
	}
	return &config
}

// serializeMonitoringConfig converts monitoring config to a nullable JSON string (SQLite).
func serializeMonitoringConfig(config *library.MonitoringConfig) sql.NullString {
	if config == nil {
		return sql.NullString{Valid: false}
	}
	data, err := json.Marshal(config)
	if err != nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: string(data), Valid: true}
}

// serializeMonitoringConfigPG converts monitoring config to PostgreSQL JSONB type.
func serializeMonitoringConfigPG(config *library.MonitoringConfig) pqtype.NullRawMessage {
	if config == nil {
		return pqtype.NullRawMessage{Valid: false}
	}
	data, err := json.Marshal(config)
	if err != nil {
		return pqtype.NullRawMessage{Valid: false}
	}
	return pqtype.NullRawMessage{RawMessage: data, Valid: true}
}

// sqliteLibraryToDomain converts a SQLite library to domain model.
func sqliteLibraryToDomain(sq sqlc_sqlite.Library) *library.Library {
	return &library.Library{
		ID:                sq.ID,
		Name:              sq.Name,
		Path:              sq.Path,
		Type:              library.LibraryType(sq.Type),
		CreatedAt:         common.ParseNullTime(sq.CreatedAt),
		UpdatedAt:         common.ParseNullTime(sq.UpdatedAt),
		MonitoringEnabled: sq.MonitoringEnabled != 0,
		MonitoringConfig:  parseMonitoringConfig(sq.MonitoringConfig),
	}
}

// postgresLibraryToDomain converts a PostgreSQL library to domain model.
func postgresLibraryToDomain(pg sqlc_postgres.Library) *library.Library {
	return &library.Library{
		ID:                int64(pg.ID),
		Name:              pg.Name,
		Path:              pg.Path,
		Type:              library.LibraryType(pg.Type),
		CreatedAt:         common.ParseNullTime(pg.CreatedAt),
		UpdatedAt:         common.ParseNullTime(pg.UpdatedAt),
		MonitoringEnabled: pg.MonitoringEnabled,
		MonitoringConfig:  parseMonitoringConfigPG(pg.MonitoringConfig),
	}
}

// NewRepository creates a new library repository with the appropriate database driver.
// The driver parameter should be "sqlite", "sqlite3", "postgres", or "postgresql".
func NewRepository(db *sql.DB, driver string) *Repository {
	r := &Repository{
		db:      db,
		dbType:  driver,
		adapter: adapters.NewTypeAdapter(),
		router:  common.NewQueryRouter(driver),
	}

	if common.IsPostgres(driver) {
		r.postgres = sqlc_postgres.New(db)
	} else {
		r.sqlite = sqlc_sqlite.New(db)
	}

	return r
}

// Create adds a new library to the database.
func (r *Repository) Create(ctx context.Context, lib *library.Library) error {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.CreateLibrary(ctx, sqlc_postgres.CreateLibraryParams{
				Name: lib.Name,
				Path: lib.Path,
				Type: string(lib.Type),
			})
		},
		func() (any, error) {
			return r.sqlite.CreateLibrary(ctx, sqlc_sqlite.CreateLibraryParams{
				Name: lib.Name,
				Path: lib.Path,
				Type: string(lib.Type),
			})
		},
	)
	if err != nil {
		return err
	}

	// Convert result to domain library
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.Library)
		lib.ID = int64(pgResult.ID)
		lib.CreatedAt = common.ParseNullTime(pgResult.CreatedAt)
		lib.UpdatedAt = common.ParseNullTime(pgResult.UpdatedAt)
	} else {
		sqResult := result.(sqlc_sqlite.Library)
		lib.ID = sqResult.ID
		lib.CreatedAt = common.ParseNullTime(sqResult.CreatedAt)
		lib.UpdatedAt = common.ParseNullTime(sqResult.UpdatedAt)
	}

	return nil
}

// GetByID retrieves a library by its ID.
func (r *Repository) GetByID(ctx context.Context, id int64) (*library.Library, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetLibraryByID(ctx, int32(id))
		},
		func() (any, error) {
			return r.sqlite.GetLibraryByID(ctx, id)
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, library.ErrLibraryNotFound
		}
		return nil, err
	}

	// Convert to domain library
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.Library)
		return postgresLibraryToDomain(pgResult), nil
	}

	sqResult := result.(sqlc_sqlite.Library)
	return sqliteLibraryToDomain(sqResult), nil
}

// GetByPath retrieves a library by its path.
func (r *Repository) GetByPath(ctx context.Context, path string) (*library.Library, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetLibraryByPath(ctx, path)
		},
		func() (any, error) {
			return r.sqlite.GetLibraryByPath(ctx, path)
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, library.ErrLibraryNotFound
		}
		return nil, err
	}

	// Convert to domain library
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.Library)
		return postgresLibraryToDomain(pgResult), nil
	}

	sqResult := result.(sqlc_sqlite.Library)
	return sqliteLibraryToDomain(sqResult), nil
}

// List retrieves all libraries.
func (r *Repository) List(ctx context.Context) ([]*library.Library, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.ListLibraries(ctx)
		},
		func() (any, error) {
			return r.sqlite.ListLibraries(ctx)
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain libraries
	if r.router.IsPostgresDB() {
		pgResults := result.([]sqlc_postgres.Library)
		libraries := make([]*library.Library, len(pgResults))
		for i, pgResult := range pgResults {
			libraries[i] = postgresLibraryToDomain(pgResult)
		}
		return libraries, nil
	}

	sqResults := result.([]sqlc_sqlite.Library)
	libraries := make([]*library.Library, len(sqResults))
	for i, sqResult := range sqResults {
		libraries[i] = sqliteLibraryToDomain(sqResult)
	}
	return libraries, nil
}

// Update modifies an existing library.
func (r *Repository) Update(ctx context.Context, lib *library.Library) error {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.UpdateLibrary(ctx, sqlc_postgres.UpdateLibraryParams{
				Name: lib.Name,
				Path: lib.Path,
				Type: string(lib.Type),
				ID:   int32(lib.ID),
			})
		},
		func() (any, error) {
			return r.sqlite.UpdateLibrary(ctx, sqlc_sqlite.UpdateLibraryParams{
				Name: lib.Name,
				Path: lib.Path,
				Type: string(lib.Type),
				ID:   lib.ID,
			})
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.ErrLibraryNotFound
		}
		return err
	}

	// Update timestamps
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.Library)
		lib.UpdatedAt = common.ParseNullTime(pgResult.UpdatedAt)
	} else {
		sqResult := result.(sqlc_sqlite.Library)
		lib.UpdatedAt = common.ParseNullTime(sqResult.UpdatedAt)
	}

	return nil
}

// Delete removes a library from the database.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	return r.router.RouteVoid(
		func() error {
			return r.postgres.DeleteLibrary(ctx, int32(id))
		},
		func() error {
			return r.sqlite.DeleteLibrary(ctx, id)
		},
	)
}

// Exists checks if a library with the given path already exists.
func (r *Repository) Exists(ctx context.Context, path string) (bool, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.LibraryExistsByPath(ctx, path)
		},
		func() (any, error) {
			count, err := r.sqlite.LibraryExistsByPath(ctx, path)
			if err != nil {
				return false, err
			}
			return count > 0, nil
		},
	)
	if err != nil {
		return false, err
	}

	return result.(bool), nil
}

// UpdateMonitoring updates the monitoring configuration for a library.
func (r *Repository) UpdateMonitoring(ctx context.Context, id int64, enabled bool, config *library.MonitoringConfig) error {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.UpdateLibraryMonitoring(ctx, sqlc_postgres.UpdateLibraryMonitoringParams{
				MonitoringEnabled: enabled,
				MonitoringConfig:  serializeMonitoringConfigPG(config),
				ID:                int32(id),
			})
		},
		func() (any, error) {
			var enabledInt int64
			if enabled {
				enabledInt = 1
			}
			return r.sqlite.UpdateLibraryMonitoring(ctx, sqlc_sqlite.UpdateLibraryMonitoringParams{
				MonitoringEnabled: enabledInt,
				MonitoringConfig:  serializeMonitoringConfig(config),
				ID:                id,
			})
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.ErrLibraryNotFound
		}
		return err
	}

	_ = result // Result is the updated library, but we don't need to return it
	return nil
}

// ListMonitored retrieves all libraries with monitoring enabled.
func (r *Repository) ListMonitored(ctx context.Context) ([]*library.Library, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.ListMonitoredLibraries(ctx)
		},
		func() (any, error) {
			return r.sqlite.ListMonitoredLibraries(ctx)
		},
	)
	if err != nil {
		return nil, err
	}

	// Convert to domain libraries
	if r.router.IsPostgresDB() {
		pgResults := result.([]sqlc_postgres.Library)
		libraries := make([]*library.Library, len(pgResults))
		for i, pgResult := range pgResults {
			libraries[i] = postgresLibraryToDomain(pgResult)
		}
		return libraries, nil
	}

	sqResults := result.([]sqlc_sqlite.Library)
	libraries := make([]*library.Library, len(sqResults))
	for i, sqResult := range sqResults {
		libraries[i] = sqliteLibraryToDomain(sqResult)
	}
	return libraries, nil
}

// CreateWithTx adds a new library to the database within a transaction.
func (r *Repository) CreateWithTx(ctx context.Context, tx *sql.Tx, lib *library.Library) error {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.WithTx(tx).CreateLibrary(ctx, sqlc_postgres.CreateLibraryParams{
				Name: lib.Name,
				Path: lib.Path,
				Type: string(lib.Type),
			})
		},
		func() (any, error) {
			return r.sqlite.WithTx(tx).CreateLibrary(ctx, sqlc_sqlite.CreateLibraryParams{
				Name: lib.Name,
				Path: lib.Path,
				Type: string(lib.Type),
			})
		},
	)
	if err != nil {
		return err
	}

	// Convert result to domain library
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.Library)
		lib.ID = int64(pgResult.ID)
		lib.CreatedAt = common.ParseNullTime(pgResult.CreatedAt)
		lib.UpdatedAt = common.ParseNullTime(pgResult.UpdatedAt)
		lib.MonitoringEnabled = pgResult.MonitoringEnabled
		lib.MonitoringConfig = parseMonitoringConfigPG(pgResult.MonitoringConfig)
	} else {
		sqResult := result.(sqlc_sqlite.Library)
		lib.ID = sqResult.ID
		lib.CreatedAt = common.ParseNullTime(sqResult.CreatedAt)
		lib.UpdatedAt = common.ParseNullTime(sqResult.UpdatedAt)
		lib.MonitoringEnabled = sqResult.MonitoringEnabled != 0
		lib.MonitoringConfig = parseMonitoringConfig(sqResult.MonitoringConfig)
	}

	return nil
}

// GetByIDWithTx retrieves a library by its ID within a transaction.
func (r *Repository) GetByIDWithTx(ctx context.Context, tx *sql.Tx, id int64) (*library.Library, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.WithTx(tx).GetLibraryByID(ctx, int32(id))
		},
		func() (any, error) {
			return r.sqlite.WithTx(tx).GetLibraryByID(ctx, id)
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, library.ErrLibraryNotFound
		}
		return nil, err
	}

	// Convert to domain library
	if r.router.IsPostgresDB() {
		pgResult := result.(sqlc_postgres.Library)
		return postgresLibraryToDomain(pgResult), nil
	}

	sqResult := result.(sqlc_sqlite.Library)
	return sqliteLibraryToDomain(sqResult), nil
}

// DeleteWithTx deletes a library by its ID within a transaction.
func (r *Repository) DeleteWithTx(ctx context.Context, tx *sql.Tx, id int64) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.WithTx(tx).DeleteLibrary(ctx, int32(id))
		},
		func() (any, error) {
			return nil, r.sqlite.WithTx(tx).DeleteLibrary(ctx, id)
		},
	)
	return err
}

// ExistsWithTx checks if a library with the given path exists within a transaction.
func (r *Repository) ExistsWithTx(ctx context.Context, tx *sql.Tx, path string) (bool, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.WithTx(tx).LibraryExistsByPath(ctx, path)
		},
		func() (any, error) {
			count, err := r.sqlite.WithTx(tx).LibraryExistsByPath(ctx, path)
			if err != nil {
				return false, err
			}
			return count > 0, nil
		},
	)
	if err != nil {
		return false, err
	}

	return result.(bool), nil
}
