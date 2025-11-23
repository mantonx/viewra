package scanstate

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements scanner.ScanStateRepository using sqlc.
// Supports both SQLite and PostgreSQL through database-specific queriers.
type Repository struct {
	db       *sql.DB
	dbType   string
	postgres *sqlc_postgres.Queries
	sqlite   *sqlc_sqlite.Queries
	router   *common.QueryRouter
}

// NewRepository creates a new scan state repository with the appropriate database driver.
func NewRepository(db *sql.DB, driver string) *Repository {
	r := &Repository{
		db:     db,
		dbType: driver,
		router: common.NewQueryRouter(driver),
	}

	if common.IsPostgres(driver) {
		r.postgres = sqlc_postgres.New(db)
	} else {
		r.sqlite = sqlc_sqlite.New(db)
	}

	return r
}

// GetLibraryState retrieves all scan state for a library
func (r *Repository) GetLibraryState(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetLibraryScanState(ctx, int32(libraryID))
		},
		func() (any, error) {
			return r.sqlite.GetLibraryScanState(ctx, libraryID)
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*scanner.ScanState{}, nil
		}
		return nil, err
	}

	return r.convertSlice(result), nil
}

// Upsert creates or updates scan state for a file
func (r *Repository) Upsert(ctx context.Context, state *scanner.ScanState) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.UpsertScanState(ctx, sqlc_postgres.UpsertScanStateParams{
				LibraryID:       int32(state.LibraryID),
				FilePath:        state.FilePath,
				FileSize:        state.FileSize,
				FileMtime:       state.FileMTime,
				FileHash:        common.NullString(state.FileHash),
				MediaID:         common.NullInt32Ptr(state.MediaID),
				LastScannedAt:   state.LastScannedAt,
				ScanJobID:       int32(state.ScanJobID),
				HasWarning:      common.NullBool(state.HasWarning),
				WarningMessage:  common.NullString(state.WarningMessage),
				WarningCategory: common.NullString(state.WarningCategory),
				HasError:        common.NullBool(state.HasError),
				ErrorMessage:    common.NullString(state.ErrorMessage),
				ErrorCategory:   common.NullString(state.ErrorCategory),
			})
		},
		func() (any, error) {
			return nil, r.sqlite.UpsertScanState(ctx, sqlc_sqlite.UpsertScanStateParams{
				LibraryID:       state.LibraryID,
				FilePath:        state.FilePath,
				FileSize:        state.FileSize,
				FileMtime:       state.FileMTime,
				FileHash:        common.NullString(state.FileHash),
				MediaID:         common.NullInt64Ptr(state.MediaID),
				LastScannedAt:   state.LastScannedAt,
				ScanJobID:       state.ScanJobID,
				HasWarning:      common.NullBool(state.HasWarning),
				WarningMessage:  common.NullString(state.WarningMessage),
				WarningCategory: common.NullString(state.WarningCategory),
				HasError:        common.NullBool(state.HasError),
				ErrorMessage:    common.NullString(state.ErrorMessage),
				ErrorCategory:   common.NullString(state.ErrorCategory),
			})
		},
	)
	return err
}

// UpsertBatch creates or updates scan state for multiple files
func (r *Repository) UpsertBatch(ctx context.Context, states []*scanner.ScanState) error {
	if len(states) == 0 {
		return nil
	}

	// Use a transaction for batch upsert
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if r.router.IsPostgresDB() {
		qtx := r.postgres.WithTx(tx)
		for _, state := range states {
			err := qtx.UpsertScanState(ctx, sqlc_postgres.UpsertScanStateParams{
				LibraryID:       int32(state.LibraryID),
				FilePath:        state.FilePath,
				FileSize:        state.FileSize,
				FileMtime:       state.FileMTime,
				FileHash:        common.NullString(state.FileHash),
				MediaID:         common.NullInt32Ptr(state.MediaID),
				LastScannedAt:   state.LastScannedAt,
				ScanJobID:       int32(state.ScanJobID),
				HasWarning:      common.NullBool(state.HasWarning),
				WarningMessage:  common.NullString(state.WarningMessage),
				WarningCategory: common.NullString(state.WarningCategory),
				HasError:        common.NullBool(state.HasError),
				ErrorMessage:    common.NullString(state.ErrorMessage),
				ErrorCategory:   common.NullString(state.ErrorCategory),
			})
			if err != nil {
				return err
			}
		}
	} else {
		qtx := r.sqlite.WithTx(tx)
		for _, state := range states {
			err := qtx.UpsertScanState(ctx, sqlc_sqlite.UpsertScanStateParams{
				LibraryID:       state.LibraryID,
				FilePath:        state.FilePath,
				FileSize:        state.FileSize,
				FileMtime:       state.FileMTime,
				FileHash:        common.NullString(state.FileHash),
				MediaID:         common.NullInt64Ptr(state.MediaID),
				LastScannedAt:   state.LastScannedAt,
				ScanJobID:       state.ScanJobID,
				HasWarning:      common.NullBool(state.HasWarning),
				WarningMessage:  common.NullString(state.WarningMessage),
				WarningCategory: common.NullString(state.WarningCategory),
				HasError:        common.NullBool(state.HasError),
				ErrorMessage:    common.NullString(state.ErrorMessage),
				ErrorCategory:   common.NullString(state.ErrorCategory),
			})
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// DeleteByPath removes scan state for a specific file
func (r *Repository) DeleteByPath(ctx context.Context, libraryID int64, filePath string) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.DeleteScanStateByPath(ctx, sqlc_postgres.DeleteScanStateByPathParams{
				LibraryID: int32(libraryID),
				FilePath:  filePath,
			})
		},
		func() (any, error) {
			return nil, r.sqlite.DeleteScanStateByPath(ctx, sqlc_sqlite.DeleteScanStateByPathParams{
				LibraryID: libraryID,
				FilePath:  filePath,
			})
		},
	)
	return err
}

// DeleteByPaths removes scan state for multiple files (for deleted files)
func (r *Repository) DeleteByPaths(ctx context.Context, libraryID int64, filePaths []string) error {
	if len(filePaths) == 0 {
		return nil
	}

	// Use a transaction for batch delete
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if r.router.IsPostgresDB() {
		qtx := r.postgres.WithTx(tx)
		for _, filePath := range filePaths {
			err := qtx.DeleteScanStateByPath(ctx, sqlc_postgres.DeleteScanStateByPathParams{
				LibraryID: int32(libraryID),
				FilePath:  filePath,
			})
			if err != nil {
				return err
			}
		}
	} else {
		qtx := r.sqlite.WithTx(tx)
		for _, filePath := range filePaths {
			err := qtx.DeleteScanStateByPath(ctx, sqlc_sqlite.DeleteScanStateByPathParams{
				LibraryID: libraryID,
				FilePath:  filePath,
			})
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// DeleteByLibrary removes all scan state for a library
func (r *Repository) DeleteByLibrary(ctx context.Context, libraryID int64) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.DeleteScanStateByLibrary(ctx, int32(libraryID))
		},
		func() (any, error) {
			return nil, r.sqlite.DeleteScanStateByLibrary(ctx, libraryID)
		},
	)
	return err
}

// GetByPath retrieves scan state for a specific file
func (r *Repository) GetByPath(ctx context.Context, libraryID int64, filePath string) (*scanner.ScanState, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetScanStateByPath(ctx, sqlc_postgres.GetScanStateByPathParams{
				LibraryID: int32(libraryID),
				FilePath:  filePath,
			})
		},
		func() (any, error) {
			return r.sqlite.GetScanStateByPath(ctx, sqlc_sqlite.GetScanStateByPathParams{
				LibraryID: libraryID,
				FilePath:  filePath,
			})
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, scanner.ErrNotFound
		}
		return nil, err
	}

	return r.convertToScanState(result), nil
}

// CountByLibrary returns the number of files tracked for a library
func (r *Repository) CountByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.CountLibraryScanState(ctx, int32(libraryID))
		},
		func() (any, error) {
			return r.sqlite.CountLibraryScanState(ctx, libraryID)
		},
	)
	if err != nil {
		return 0, err
	}

	if r.router.IsPostgresDB() {
		return result.(int64), nil
	}
	return result.(int64), nil
}

// convertSlice converts a slice of sqlc scan states to domain scan states
func (r *Repository) convertSlice(result any) []*scanner.ScanState {
	if r.router.IsPostgresDB() {
		pgSlice := result.([]sqlc_postgres.ScanState)
		states := make([]*scanner.ScanState, len(pgSlice))
		for i, pg := range pgSlice {
			states[i] = r.convertToScanState(pg)
		}
		return states
	}

	sqSlice := result.([]sqlc_sqlite.ScanState)
	states := make([]*scanner.ScanState, len(sqSlice))
	for i, sq := range sqSlice {
		states[i] = r.convertToScanState(sq)
	}
	return states
}

// convertToScanState converts sqlc result to domain ScanState
func (r *Repository) convertToScanState(result any) *scanner.ScanState {
	if r.router.IsPostgresDB() {
		pg := result.(sqlc_postgres.ScanState)
		return &scanner.ScanState{
			ID:              int64(pg.ID),
			LibraryID:       int64(pg.LibraryID),
			FilePath:        pg.FilePath,
			FileSize:        pg.FileSize,
			FileMTime:       pg.FileMtime,
			FileHash:        common.ParseNullString(pg.FileHash),
			MediaID:         common.ParseNullInt32Ptr(pg.MediaID),
			LastScannedAt:   pg.LastScannedAt,
			ScanJobID:       int64(pg.ScanJobID),
			CreatedAt:       common.ParseNullTime(pg.CreatedAt),
			HasWarning:      common.ParseNullBool(pg.HasWarning),
			WarningMessage:  common.ParseNullString(pg.WarningMessage),
			WarningCategory: common.ParseNullString(pg.WarningCategory),
			HasError:        common.ParseNullBool(pg.HasError),
			ErrorMessage:    common.ParseNullString(pg.ErrorMessage),
			ErrorCategory:   common.ParseNullString(pg.ErrorCategory),
		}
	}

	sq := result.(sqlc_sqlite.ScanState)
	return &scanner.ScanState{
		ID:              sq.ID,
		LibraryID:       sq.LibraryID,
		FilePath:        sq.FilePath,
		FileSize:        sq.FileSize,
		FileMTime:       sq.FileMtime,
		FileHash:        common.ParseNullString(sq.FileHash),
		MediaID:         common.ParseNullInt64Ptr(sq.MediaID),
		LastScannedAt:   sq.LastScannedAt,
		ScanJobID:       sq.ScanJobID,
		CreatedAt:       common.ParseNullTime(sq.CreatedAt),
		HasWarning:      common.ParseNullBool(sq.HasWarning),
		WarningMessage:  common.ParseNullString(sq.WarningMessage),
		WarningCategory: common.ParseNullString(sq.WarningCategory),
		HasError:        common.ParseNullBool(sq.HasError),
		ErrorMessage:    common.ParseNullString(sq.ErrorMessage),
		ErrorCategory:   common.ParseNullString(sq.ErrorCategory),
	}
}

// GetLibraryWarnings retrieves all files with warnings for a library
func (r *Repository) GetLibraryWarnings(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetLibraryWarnings(ctx, int32(libraryID))
		},
		func() (any, error) {
			return r.sqlite.GetLibraryWarnings(ctx, libraryID)
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*scanner.ScanState{}, nil
		}
		return nil, err
	}
	return r.convertSlice(result), nil
}

// GetLibraryErrors retrieves all files with errors for a library
func (r *Repository) GetLibraryErrors(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetLibraryErrors(ctx, int32(libraryID))
		},
		func() (any, error) {
			return r.sqlite.GetLibraryErrors(ctx, libraryID)
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*scanner.ScanState{}, nil
		}
		return nil, err
	}
	return r.convertSlice(result), nil
}

// GetLibraryIssues retrieves all files with either warnings or errors for a library
func (r *Repository) GetLibraryIssues(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.GetLibraryIssues(ctx, int32(libraryID))
		},
		func() (any, error) {
			return r.sqlite.GetLibraryIssues(ctx, libraryID)
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*scanner.ScanState{}, nil
		}
		return nil, err
	}
	return r.convertSlice(result), nil
}

// CountLibraryIssues returns counts of errors and warnings for a library
func (r *Repository) CountLibraryIssues(ctx context.Context, libraryID int64) (*scanner.LibraryIssueCounts, error) {
	result, err := r.router.Route(
		func() (any, error) {
			return r.postgres.CountLibraryIssues(ctx, int32(libraryID))
		},
		func() (any, error) {
			return r.sqlite.CountLibraryIssues(ctx, libraryID)
		},
	)
	if err != nil {
		return nil, err
	}

	if r.router.IsPostgresDB() {
		row := result.(sqlc_postgres.CountLibraryIssuesRow)
		return &scanner.LibraryIssueCounts{
			ErrorCount:   row.ErrorCount,
			WarningCount: row.WarningCount,
		}, nil
	}

	row := result.(sqlc_sqlite.CountLibraryIssuesRow)
	return &scanner.LibraryIssueCounts{
		ErrorCount:   row.ErrorCount,
		WarningCount: row.WarningCount,
	}, nil
}

// SetWarning sets a warning for a file
func (r *Repository) SetWarning(ctx context.Context, libraryID int64, filePath, message, category string) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.SetScanStateWarning(ctx, sqlc_postgres.SetScanStateWarningParams{
				HasWarning:      common.NullBool(true),
				WarningMessage:  common.NullString(message),
				WarningCategory: common.NullString(category),
				LibraryID:       int32(libraryID),
				FilePath:        filePath,
			})
		},
		func() (any, error) {
			return nil, r.sqlite.SetScanStateWarning(ctx, sqlc_sqlite.SetScanStateWarningParams{
				HasWarning:      common.NullBool(true),
				WarningMessage:  common.NullString(message),
				WarningCategory: common.NullString(category),
				LibraryID:       libraryID,
				FilePath:        filePath,
			})
		},
	)
	return err
}

// SetError sets an error for a file
func (r *Repository) SetError(ctx context.Context, libraryID int64, filePath, message, category string) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.SetScanStateError(ctx, sqlc_postgres.SetScanStateErrorParams{
				HasError:      common.NullBool(true),
				ErrorMessage:  common.NullString(message),
				ErrorCategory: common.NullString(category),
				LibraryID:     int32(libraryID),
				FilePath:      filePath,
			})
		},
		func() (any, error) {
			return nil, r.sqlite.SetScanStateError(ctx, sqlc_sqlite.SetScanStateErrorParams{
				HasError:      common.NullBool(true),
				ErrorMessage:  common.NullString(message),
				ErrorCategory: common.NullString(category),
				LibraryID:     libraryID,
				FilePath:      filePath,
			})
		},
	)
	return err
}

// ClearWarning clears the warning for a file
func (r *Repository) ClearWarning(ctx context.Context, libraryID int64, filePath string) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.ClearScanStateWarning(ctx, sqlc_postgres.ClearScanStateWarningParams{
				LibraryID: int32(libraryID),
				FilePath:  filePath,
			})
		},
		func() (any, error) {
			return nil, r.sqlite.ClearScanStateWarning(ctx, sqlc_sqlite.ClearScanStateWarningParams{
				LibraryID: libraryID,
				FilePath:  filePath,
			})
		},
	)
	return err
}

// ClearError clears the error for a file
func (r *Repository) ClearError(ctx context.Context, libraryID int64, filePath string) error {
	_, err := r.router.Route(
		func() (any, error) {
			return nil, r.postgres.ClearScanStateError(ctx, sqlc_postgres.ClearScanStateErrorParams{
				LibraryID: int32(libraryID),
				FilePath:  filePath,
			})
		},
		func() (any, error) {
			return nil, r.sqlite.ClearScanStateError(ctx, sqlc_sqlite.ClearScanStateErrorParams{
				LibraryID: libraryID,
				FilePath:  filePath,
			})
		},
	)
	return err
}
