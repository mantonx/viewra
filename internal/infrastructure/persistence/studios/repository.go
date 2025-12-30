package studios

import (
	"context"
	"database/sql"
	"errors"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements media.StudioRepository.
type Repository struct {
	*common.BaseRepository
}

// NewRepository creates a new studios repository.
func NewRepository(base *common.BaseRepository) *Repository {
	return &Repository{BaseRepository: base}
}

// GetStudioByID retrieves a studio by its ID.
func (r *Repository) GetStudioByID(id int64) (*media.Studio, error) {
	ctx := context.Background()
	return common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.Studio, error) {
			return r.Postgres().GetStudioByID(ctx, id)
		},
		func() (sqlc_sqlite.Studio, error) {
			return r.SQLite().GetStudioByID(ctx, id)
		},
		postgresStudioToDomain,
		sqliteStudioToDomain,
	)
}

// GetStudioByName retrieves a studio by its name.
func (r *Repository) GetStudioByName(name string) (*media.Studio, error) {
	ctx := context.Background()
	return common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.Studio, error) {
			return r.Postgres().GetStudioByName(ctx, name)
		},
		func() (sqlc_sqlite.Studio, error) {
			return r.SQLite().GetStudioByName(ctx, name)
		},
		postgresStudioToDomain,
		sqliteStudioToDomain,
	)
}

// GetStudioByTMDbID retrieves a studio by its TMDb ID.
func (r *Repository) GetStudioByTMDbID(tmdbID int) (*media.Studio, error) {
	ctx := context.Background()
	return common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.Studio, error) {
			return r.Postgres().GetStudioByTMDbID(ctx, sql.NullInt64{Int64: int64(tmdbID), Valid: true})
		},
		func() (sqlc_sqlite.Studio, error) {
			return r.SQLite().GetStudioByTMDbID(ctx, sql.NullInt64{Int64: int64(tmdbID), Valid: true})
		},
		postgresStudioToDomain,
		sqliteStudioToDomain,
	)
}

// CreateStudio creates a new studio.
func (r *Repository) CreateStudio(studio *media.Studio) error {
	ctx := context.Background()

	if r.Router().IsPostgresDB() {
		result, err := r.Postgres().CreateStudio(ctx, buildPostgresCreateStudioParams(studio))
		if err != nil {
			return err
		}
		studio.ID = result.ID
		return nil
	}

	result, err := r.SQLite().CreateStudio(ctx, buildSQLiteCreateStudioParams(studio))
	if err != nil {
		return err
	}
	studio.ID = result.ID
	return nil
}

// UpdateStudio updates an existing studio.
func (r *Repository) UpdateStudio(studio *media.Studio) error {
	ctx := context.Background()
	return common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().UpdateStudio(ctx, buildPostgresUpdateStudioParams(studio))
		},
		func() error {
			return r.SQLite().UpdateStudio(ctx, buildSQLiteUpdateStudioParams(studio))
		},
	)
}

// isNotFoundError checks if the error indicates a record was not found.
// This handles both sql.ErrNoRows (direct DB access) and media.ErrMediaNotFound (via QuerySingle).
// Uses errors.Is to properly handle wrapped errors.
func isNotFoundError(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, media.ErrMediaNotFound)
}

// FindOrCreateStudio finds a studio by name or TMDb ID, or creates it if not found.
func (r *Repository) FindOrCreateStudio(name string, tmdbID int) (*media.Studio, error) {
	// Try to find by TMDb ID first
	if tmdbID > 0 {
		studio, err := r.GetStudioByTMDbID(tmdbID)
		if err == nil {
			return studio, nil
		}
		if !isNotFoundError(err) {
			return nil, err
		}
	}

	// Try to find by name
	studio, err := r.GetStudioByName(name)
	if err == nil {
		// Update TMDb ID if we have one and it wasn't set
		if tmdbID > 0 && studio.TMDbID == 0 {
			studio.TMDbID = tmdbID
			_ = r.UpdateStudio(studio) // Ignore error, just try to update
		}
		return studio, nil
	}
	if !isNotFoundError(err) {
		return nil, err
	}

	// Create new studio
	newStudio := &media.Studio{
		Name:   name,
		TMDbID: tmdbID,
	}
	if err := r.CreateStudio(newStudio); err != nil {
		return nil, err
	}
	return newStudio, nil
}

// GetStudiosForEntity retrieves all studios for a media entity.
func (r *Repository) GetStudiosForEntity(mediaType string, entityID int64) ([]*media.Studio, error) {
	ctx := context.Background()
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.Studio, error) {
			return r.Postgres().GetStudiosForEntity(ctx, sqlc_postgres.GetStudiosForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
		func() ([]sqlc_sqlite.Studio, error) {
			return r.SQLite().GetStudiosForEntity(ctx, sqlc_sqlite.GetStudiosForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
		postgresStudioToDomain,
		sqliteStudioToDomain,
	)
}

// AddStudioToEntity adds a studio association to a media entity.
func (r *Repository) AddStudioToEntity(mediaType string, entityID int64, studioID int64) error {
	ctx := context.Background()
	return common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().AddMediaStudio(ctx, sqlc_postgres.AddMediaStudioParams{
				MediaType: mediaType,
				EntityID:  entityID,
				StudioID:  studioID,
			})
		},
		func() error {
			return r.SQLite().AddMediaStudio(ctx, sqlc_sqlite.AddMediaStudioParams{
				MediaType: mediaType,
				EntityID:  entityID,
				StudioID:  studioID,
			})
		},
	)
}

// RemoveStudioFromEntity removes a studio association from a media entity.
func (r *Repository) RemoveStudioFromEntity(mediaType string, entityID int64, studioID int64) error {
	ctx := context.Background()
	return common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().RemoveMediaStudio(ctx, sqlc_postgres.RemoveMediaStudioParams{
				MediaType: mediaType,
				EntityID:  entityID,
				StudioID:  studioID,
			})
		},
		func() error {
			return r.SQLite().RemoveMediaStudio(ctx, sqlc_sqlite.RemoveMediaStudioParams{
				MediaType: mediaType,
				EntityID:  entityID,
				StudioID:  studioID,
			})
		},
	)
}

// ClearStudiosForEntity removes all studio associations for a media entity.
func (r *Repository) ClearStudiosForEntity(mediaType string, entityID int64) error {
	ctx := context.Background()
	return common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().ClearStudiosForEntity(ctx, sqlc_postgres.ClearStudiosForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
		func() error {
			return r.SQLite().ClearStudiosForEntity(ctx, sqlc_sqlite.ClearStudiosForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
	)
}

// ReplaceStudiosForEntity clears existing studios and adds new ones.
func (r *Repository) ReplaceStudiosForEntity(mediaType string, entityID int64, studios []*media.Studio) error {
	ctx := context.Background()

	// Clear existing studios
	err := common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().ClearStudiosForEntity(ctx, sqlc_postgres.ClearStudiosForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
		func() error {
			return r.SQLite().ClearStudiosForEntity(ctx, sqlc_sqlite.ClearStudiosForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
	)
	if err != nil {
		return err
	}

	// Add new studios
	for _, studio := range studios {
		if err := r.AddStudioToEntity(mediaType, entityID, studio.ID); err != nil {
			return err
		}
	}

	return nil
}
