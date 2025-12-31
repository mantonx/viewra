package scanstate

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mantonx/viewra/internal/domain/scanner"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements scanner.ScanStateRepository using sqlc.
// It embeds BaseRepository for dual-database support.
type Repository struct {
	*common.BaseRepository
}

// NewRepository creates a new scan state repository with the appropriate database driver.
func NewRepository(db *common.BaseRepository) *Repository {
	return &Repository{
		BaseRepository: db,
	}
}

// GetLibraryState retrieves all scan state for a library
func (r *Repository) GetLibraryState(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	rows, err := r.Q().GetLibraryScanState(ctx, libraryID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*scanner.ScanState{}, nil
		}
		return nil, err
	}
	return mapSlice(rows, scanStateToDomain), nil
}

// Upsert creates or updates scan state for a file
func (r *Repository) Upsert(ctx context.Context, state *scanner.ScanState) error {
	return r.Q().UpsertScanState(ctx, buildUpsertParams(state))
}

// UpsertBatch creates or updates scan state for multiple files
func (r *Repository) UpsertBatch(ctx context.Context, states []*scanner.ScanState) error {
	if len(states) == 0 {
		return nil
	}

	tx, err := r.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := r.QWithTx(tx)
	for _, state := range states {
		if err := qtx.UpsertScanState(ctx, buildUpsertParams(state)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteByPath removes scan state for a specific file
func (r *Repository) DeleteByPath(ctx context.Context, libraryID int64, filePath string) error {
	return r.Q().DeleteScanStateByPath(ctx, unified.DeleteScanStateByPathParams{
		LibraryID: libraryID,
		FilePath:  filePath,
	})
}

// DeleteByPaths removes scan state for multiple files (for deleted files)
func (r *Repository) DeleteByPaths(ctx context.Context, libraryID int64, filePaths []string) error {
	if len(filePaths) == 0 {
		return nil
	}

	tx, err := r.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := r.QWithTx(tx)
	for _, filePath := range filePaths {
		if err := qtx.DeleteScanStateByPath(ctx, unified.DeleteScanStateByPathParams{
			LibraryID: libraryID,
			FilePath:  filePath,
		}); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteByLibrary removes all scan state for a library
func (r *Repository) DeleteByLibrary(ctx context.Context, libraryID int64) error {
	return r.Q().DeleteScanStateByLibrary(ctx, libraryID)
}

// GetByPath retrieves scan state for a specific file
func (r *Repository) GetByPath(ctx context.Context, libraryID int64, filePath string) (*scanner.ScanState, error) {
	row, err := r.Q().GetScanStateByPath(ctx, unified.GetScanStateByPathParams{
		LibraryID: libraryID,
		FilePath:  filePath,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, scanner.ErrNotFound
		}
		return nil, err
	}
	return scanStateToDomain(row), nil
}

// CountByLibrary returns the number of files tracked for a library
func (r *Repository) CountByLibrary(ctx context.Context, libraryID int64) (int64, error) {
	return r.Q().CountLibraryScanState(ctx, libraryID)
}

// GetLibraryWarnings retrieves all files with warnings for a library
func (r *Repository) GetLibraryWarnings(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	rows, err := r.Q().GetLibraryWarnings(ctx, libraryID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*scanner.ScanState{}, nil
		}
		return nil, err
	}
	return mapSlice(rows, scanStateToDomain), nil
}

// GetLibraryErrors retrieves all files with errors for a library
func (r *Repository) GetLibraryErrors(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	rows, err := r.Q().GetLibraryErrors(ctx, libraryID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*scanner.ScanState{}, nil
		}
		return nil, err
	}
	return mapSlice(rows, scanStateToDomain), nil
}

// GetLibraryIssues retrieves all files with either warnings or errors for a library
func (r *Repository) GetLibraryIssues(ctx context.Context, libraryID int64) ([]*scanner.ScanState, error) {
	rows, err := r.Q().GetLibraryIssues(ctx, libraryID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*scanner.ScanState{}, nil
		}
		return nil, err
	}
	return mapSlice(rows, scanStateToDomain), nil
}

// CountLibraryIssues returns counts of errors and warnings for a library
func (r *Repository) CountLibraryIssues(ctx context.Context, libraryID int64) (*scanner.LibraryIssueCounts, error) {
	row, err := r.Q().CountLibraryIssues(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	return &scanner.LibraryIssueCounts{
		ErrorCount:   row.ErrorCount,
		WarningCount: row.WarningCount,
	}, nil
}

// SetWarning sets a warning for a file
func (r *Repository) SetWarning(ctx context.Context, libraryID int64, filePath, message, category string) error {
	return r.Q().SetScanStateWarning(ctx, unified.SetScanStateWarningParams{
		HasWarning:      common.NullInt64FromBool(true),
		WarningMessage:  common.NullString(message),
		WarningCategory: common.NullString(category),
		LibraryID:       libraryID,
		FilePath:        filePath,
	})
}

// SetError sets an error for a file
func (r *Repository) SetError(ctx context.Context, libraryID int64, filePath, message, category string) error {
	return r.Q().SetScanStateError(ctx, unified.SetScanStateErrorParams{
		HasError:      common.NullInt64FromBool(true),
		ErrorMessage:  common.NullString(message),
		ErrorCategory: common.NullString(category),
		LibraryID:     libraryID,
		FilePath:      filePath,
	})
}

// ClearWarning clears the warning for a file
func (r *Repository) ClearWarning(ctx context.Context, libraryID int64, filePath string) error {
	return r.Q().ClearScanStateWarning(ctx, unified.ClearScanStateWarningParams{
		LibraryID: libraryID,
		FilePath:  filePath,
	})
}

// ClearError clears the error for a file
func (r *Repository) ClearError(ctx context.Context, libraryID int64, filePath string) error {
	return r.Q().ClearScanStateError(ctx, unified.ClearScanStateErrorParams{
		LibraryID: libraryID,
		FilePath:  filePath,
	})
}

// buildUpsertParams creates upsert params from domain scan state
func buildUpsertParams(state *scanner.ScanState) unified.UpsertScanStateParams {
	return unified.UpsertScanStateParams{
		LibraryID:       state.LibraryID,
		FilePath:        state.FilePath,
		FileSize:        state.FileSize,
		FileMtime:       state.FileMTime,
		FileHash:        common.NullString(state.FileHash),
		MediaID:         common.NullInt64Ptr(state.MediaID),
		LastScannedAt:   state.LastScannedAt,
		ScanJobID:       state.ScanJobID,
		HasWarning:      common.NullInt64FromBool(state.HasWarning),
		WarningMessage:  common.NullString(state.WarningMessage),
		WarningCategory: common.NullString(state.WarningCategory),
		HasError:        common.NullInt64FromBool(state.HasError),
		ErrorMessage:    common.NullString(state.ErrorMessage),
		ErrorCategory:   common.NullString(state.ErrorCategory),
	}
}

// scanStateToDomain converts sqlc ScanState to domain ScanState
func scanStateToDomain(row unified.ScanState) *scanner.ScanState {
	return &scanner.ScanState{
		ID:              row.ID,
		LibraryID:       row.LibraryID,
		FilePath:        row.FilePath,
		FileSize:        row.FileSize,
		FileMTime:       row.FileMtime,
		FileHash:        common.ParseNullString(row.FileHash),
		MediaID:         common.ParseNullInt64Ptr(row.MediaID),
		LastScannedAt:   row.LastScannedAt,
		ScanJobID:       row.ScanJobID,
		CreatedAt:       common.ParseNullTime(row.CreatedAt),
		HasWarning:      common.NullInt64ToBool(row.HasWarning),
		WarningMessage:  common.ParseNullString(row.WarningMessage),
		WarningCategory: common.ParseNullString(row.WarningCategory),
		HasError:        common.NullInt64ToBool(row.HasError),
		ErrorMessage:    common.ParseNullString(row.ErrorMessage),
		ErrorCategory:   common.ParseNullString(row.ErrorCategory),
	}
}

// mapSlice converts a slice of one type to another using the provided mapper function.
func mapSlice[TFrom, TTo any](from []TFrom, mapper func(TFrom) TTo) []TTo {
	result := make([]TTo, len(from))
	for i, v := range from {
		result[i] = mapper(v)
	}
	return result
}
