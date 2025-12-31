package library

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	domaincommon "github.com/mantonx/viewra/internal/domain/common"
	"github.com/mantonx/viewra/internal/domain/library"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// parseMonitoringConfig parses JSON monitoring config from a nullable string.
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

// serializeMonitoringConfig converts monitoring config to a nullable JSON string.
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

// libraryToDomain converts a unified Library to domain model.
func libraryToDomain(lib unified.Library) *library.Library {
	return &library.Library{
		ID:                lib.ID,
		Name:              lib.Name,
		Path:              lib.Path,
		Type:              library.LibraryType(lib.Type),
		CreatedAt:         common.ParseNullTime(lib.CreatedAt),
		UpdatedAt:         common.ParseNullTime(lib.UpdatedAt),
		MonitoringEnabled: lib.MonitoringEnabled != 0,
		MonitoringConfig:  parseMonitoringConfig(lib.MonitoringConfig),
		LastScannedAt:     common.ParseNullTimePtr(lib.LastScannedAt),
	}
}

// NewRepository creates a new library repository with the specified database driver.
func NewRepository(baseRepo *common.BaseRepository) *Repository {
	return &Repository{
		BaseRepository: baseRepo,
	}
}

// Create adds a new library to the database.
func (r *Repository) Create(ctx context.Context, lib *library.Library) error {
	result, err := r.Q().CreateLibrary(ctx, unified.CreateLibraryParams{
		Name: lib.Name,
		Path: lib.Path,
		Type: string(lib.Type),
	})
	if err != nil {
		return err
	}

	lib.ID = result.ID
	lib.CreatedAt = common.ParseNullTime(result.CreatedAt)
	lib.UpdatedAt = common.ParseNullTime(result.UpdatedAt)
	return nil
}

// GetByID retrieves a library by its ID.
func (r *Repository) GetByID(ctx context.Context, id int64) (*library.Library, error) {
	result, err := r.Q().GetLibraryByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, library.ErrLibraryNotFound
		}
		return nil, err
	}
	return libraryToDomain(result), nil
}

// GetByPath retrieves a library by its path.
func (r *Repository) GetByPath(ctx context.Context, path string) (*library.Library, error) {
	result, err := r.Q().GetLibraryByPath(ctx, path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, library.ErrLibraryNotFound
		}
		return nil, err
	}
	return libraryToDomain(result), nil
}

// List retrieves all libraries.
func (r *Repository) List(ctx context.Context) ([]*library.Library, error) {
	results, err := r.Q().ListLibraries(ctx)
	if err != nil {
		return nil, err
	}
	return mapSlice(results, libraryToDomain), nil
}

// Update modifies an existing library.
func (r *Repository) Update(ctx context.Context, lib *library.Library) error {
	result, err := r.Q().UpdateLibrary(ctx, unified.UpdateLibraryParams{
		Name: lib.Name,
		Path: lib.Path,
		Type: string(lib.Type),
		ID:   lib.ID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.ErrLibraryNotFound
		}
		return err
	}

	lib.UpdatedAt = common.ParseNullTime(result.UpdatedAt)
	return nil
}

// Delete removes a library from the database.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	return r.Q().DeleteLibrary(ctx, id)
}

// Exists checks if a library with the given path already exists.
func (r *Repository) Exists(ctx context.Context, path string) (bool, error) {
	count, err := r.Q().LibraryExistsByPath(ctx, path)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpdateMonitoring updates the monitoring configuration for a library.
func (r *Repository) UpdateMonitoring(ctx context.Context, id int64, enabled bool, config *library.MonitoringConfig) error {
	_, err := r.Q().UpdateLibraryMonitoring(ctx, unified.UpdateLibraryMonitoringParams{
		MonitoringEnabled: common.BoolToInt64(enabled),
		MonitoringConfig:  serializeMonitoringConfig(config),
		ID:                id,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return library.ErrLibraryNotFound
		}
		return err
	}
	return nil
}

// ListMonitored retrieves all libraries with monitoring enabled.
func (r *Repository) ListMonitored(ctx context.Context) ([]*library.Library, error) {
	results, err := r.Q().ListMonitoredLibraries(ctx)
	if err != nil {
		return nil, err
	}
	return mapSlice(results, libraryToDomain), nil
}

// CreateWithTx adds a new library to the database within a transaction.
func (r *Repository) CreateWithTx(ctx context.Context, tx domaincommon.Transaction, lib *library.Library) error {
	sqlTx := tx.Unwrap().(*sql.Tx)
	q := r.QWithTx(sqlTx)

	result, err := q.CreateLibrary(ctx, unified.CreateLibraryParams{
		Name: lib.Name,
		Path: lib.Path,
		Type: string(lib.Type),
	})
	if err != nil {
		return err
	}

	lib.ID = result.ID
	lib.CreatedAt = common.ParseNullTime(result.CreatedAt)
	lib.UpdatedAt = common.ParseNullTime(result.UpdatedAt)
	lib.MonitoringEnabled = result.MonitoringEnabled != 0
	lib.MonitoringConfig = parseMonitoringConfig(result.MonitoringConfig)
	return nil
}

// GetByIDWithTx retrieves a library by its ID within a transaction.
func (r *Repository) GetByIDWithTx(ctx context.Context, tx domaincommon.Transaction, id int64) (*library.Library, error) {
	sqlTx := tx.Unwrap().(*sql.Tx)
	q := r.QWithTx(sqlTx)

	result, err := q.GetLibraryByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, library.ErrLibraryNotFound
		}
		return nil, err
	}
	return libraryToDomain(result), nil
}

// DeleteWithTx deletes a library by its ID within a transaction.
func (r *Repository) DeleteWithTx(ctx context.Context, tx domaincommon.Transaction, id int64) error {
	sqlTx := tx.Unwrap().(*sql.Tx)
	return r.QWithTx(sqlTx).DeleteLibrary(ctx, id)
}

// ExistsWithTx checks if a library with the given path exists within a transaction.
func (r *Repository) ExistsWithTx(ctx context.Context, tx domaincommon.Transaction, path string) (bool, error) {
	sqlTx := tx.Unwrap().(*sql.Tx)
	count, err := r.QWithTx(sqlTx).LibraryExistsByPath(ctx, path)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpdateLastScannedAt updates the last_scanned_at timestamp for a library.
// This is called when a scan completes successfully.
func (r *Repository) UpdateLastScannedAt(ctx context.Context, id int64) error {
	return r.Q().UpdateLibraryLastScannedAt(ctx, id)
}

// mapSlice converts a slice of one type to another using the provided mapper function.
func mapSlice[TFrom, TTo any](from []TFrom, mapper func(TFrom) TTo) []TTo {
	result := make([]TTo, len(from))
	for i, v := range from {
		result[i] = mapper(v)
	}
	return result
}
