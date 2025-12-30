package people

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_postgres"
	"github.com/mantonx/viewra/internal/infrastructure/database/sqlc_sqlite"
	"github.com/mantonx/viewra/internal/infrastructure/persistence/common"
)

// Repository implements media.PeopleRepository using sqlc.
type Repository struct {
	*common.BaseRepository
}

// NewRepository creates a new people repository.
func NewRepository(db *common.BaseRepository) *Repository {
	return &Repository{
		BaseRepository: db,
	}
}

// --- Person Operations ---

// GetPersonByID retrieves a person by their ID.
func (r *Repository) GetPersonByID(id int64) (*media.Person, error) {
	ctx := context.Background()
	return common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.Person, error) {
			return r.Postgres().GetPersonByID(ctx, id)
		},
		func() (sqlc_sqlite.Person, error) {
			return r.SQLite().GetPersonByID(ctx, id)
		},
		postgresPersonToDomain,
		sqlitePersonToDomain,
	)
}

// GetPersonByName retrieves a person by their name.
func (r *Repository) GetPersonByName(name string) (*media.Person, error) {
	ctx := context.Background()
	return common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.Person, error) {
			return r.Postgres().GetPersonByName(ctx, name)
		},
		func() (sqlc_sqlite.Person, error) {
			return r.SQLite().GetPersonByName(ctx, name)
		},
		postgresPersonToDomain,
		sqlitePersonToDomain,
	)
}

// GetPersonByTMDbID retrieves a person by their TMDb ID.
func (r *Repository) GetPersonByTMDbID(tmdbID int) (*media.Person, error) {
	ctx := context.Background()
	return common.QuerySingle(
		r.BaseRepository, ctx,
		func() (sqlc_postgres.Person, error) {
			return r.Postgres().GetPersonByTMDbID(ctx, common.NullInt64(int64(tmdbID)))
		},
		func() (sqlc_sqlite.Person, error) {
			return r.SQLite().GetPersonByTMDbID(ctx, common.NullInt64(int64(tmdbID)))
		},
		postgresPersonToDomain,
		sqlitePersonToDomain,
	)
}

// CreatePerson creates a new person in the database.
func (r *Repository) CreatePerson(person *media.Person) error {
	ctx := context.Background()

	if r.Router().IsPostgresDB() {
		result, err := r.Postgres().CreatePerson(ctx, buildPostgresCreatePersonParams(person))
		if err != nil {
			return fmt.Errorf("create person: %w", err)
		}
		person.ID = result.ID
		return nil
	}

	result, err := r.SQLite().CreatePerson(ctx, buildSQLiteCreatePersonParams(person))
	if err != nil {
		return fmt.Errorf("create person: %w", err)
	}
	person.ID = result.ID
	return nil
}

// UpdatePerson updates an existing person in the database.
func (r *Repository) UpdatePerson(person *media.Person) error {
	ctx := context.Background()
	return common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().UpdatePerson(ctx, buildPostgresUpdatePersonParams(person))
		},
		func() error {
			return r.SQLite().UpdatePerson(ctx, buildSQLiteUpdatePersonParams(person))
		},
	)
}

// isNotFoundError checks if the error indicates a record was not found.
// This handles both sql.ErrNoRows (direct DB access) and media.ErrMediaNotFound (via QuerySingle).
// Uses errors.Is to properly handle wrapped errors.
func isNotFoundError(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, media.ErrMediaNotFound)
}

// FindOrCreatePerson finds a person by TMDb ID or name, creating them if they don't exist.
func (r *Repository) FindOrCreatePerson(name string, tmdbID int) (*media.Person, error) {
	// First try to find by TMDb ID if provided
	if tmdbID > 0 {
		person, err := r.GetPersonByTMDbID(tmdbID)
		if err == nil {
			return person, nil
		}
		if !isNotFoundError(err) {
			return nil, fmt.Errorf("find person by tmdb id: %w", err)
		}
	}

	// Try to find by name
	person, err := r.GetPersonByName(name)
	if err == nil {
		// Update TMDb ID if we have one and they don't
		if tmdbID > 0 && person.TMDbID == 0 {
			person.TMDbID = tmdbID
			if updateErr := r.UpdatePerson(person); updateErr != nil {
				// Non-fatal, just log and continue
				return person, nil
			}
		}
		return person, nil
	}
	if !isNotFoundError(err) {
		return nil, fmt.Errorf("find person by name: %w", err)
	}

	// Create new person
	person = &media.Person{
		Name:   name,
		TMDbID: tmdbID,
	}
	if err := r.CreatePerson(person); err != nil {
		return nil, fmt.Errorf("create person: %w", err)
	}

	return person, nil
}

// FindOrCreatePersonWithPhoto finds or creates a person, also setting/updating their photo URL.
func (r *Repository) FindOrCreatePersonWithPhoto(name string, tmdbID int, photoURL string) (*media.Person, error) {
	// First try to find by TMDb ID if provided
	if tmdbID > 0 {
		person, err := r.GetPersonByTMDbID(tmdbID)
		if err == nil {
			// Update photo URL if we have one and they don't
			if photoURL != "" && person.PhotoURL == "" {
				person.PhotoURL = photoURL
				_ = r.UpdatePerson(person) // Non-fatal if update fails
			}
			return person, nil
		}
		if !isNotFoundError(err) {
			return nil, fmt.Errorf("find person by tmdb id: %w", err)
		}
	}

	// Try to find by name
	person, err := r.GetPersonByName(name)
	if err == nil {
		needsUpdate := false
		// Update TMDb ID if we have one and they don't
		if tmdbID > 0 && person.TMDbID == 0 {
			person.TMDbID = tmdbID
			needsUpdate = true
		}
		// Update photo URL if we have one and they don't
		if photoURL != "" && person.PhotoURL == "" {
			person.PhotoURL = photoURL
			needsUpdate = true
		}
		if needsUpdate {
			_ = r.UpdatePerson(person) // Non-fatal if update fails
		}
		return person, nil
	}
	if !isNotFoundError(err) {
		return nil, fmt.Errorf("find person by name: %w", err)
	}

	// Create new person with all available info
	person = &media.Person{
		Name:     name,
		TMDbID:   tmdbID,
		PhotoURL: photoURL,
	}
	if err := r.CreatePerson(person); err != nil {
		return nil, fmt.Errorf("create person: %w", err)
	}

	return person, nil
}

// --- Credit Operations ---

// GetCreditsForEntity retrieves all credits for a media entity.
func (r *Repository) GetCreditsForEntity(mediaType string, entityID int64) ([]*media.Credit, error) {
	ctx := context.Background()
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.GetCreditsForEntityRow, error) {
			return r.Postgres().GetCreditsForEntity(ctx, sqlc_postgres.GetCreditsForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
		func() ([]sqlc_sqlite.GetCreditsForEntityRow, error) {
			return r.SQLite().GetCreditsForEntity(ctx, sqlc_sqlite.GetCreditsForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
		postgresCreditRowToDomain,
		sqliteCreditRowToDomain,
	)
}

// GetCreditsForPerson retrieves all credits for a person.
func (r *Repository) GetCreditsForPerson(personID int64) ([]*media.Credit, error) {
	ctx := context.Background()
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.GetCreditsForPersonRow, error) {
			return r.Postgres().GetCreditsForPerson(ctx, personID)
		},
		func() ([]sqlc_sqlite.GetCreditsForPersonRow, error) {
			return r.SQLite().GetCreditsForPerson(ctx, personID)
		},
		postgresCreditsForPersonToDomain,
		sqliteCreditsForPersonToDomain,
	)
}

// CreateCredit creates a new credit in the database.
func (r *Repository) CreateCredit(credit *media.Credit) error {
	ctx := context.Background()

	if r.Router().IsPostgresDB() {
		result, err := r.Postgres().CreateCredit(ctx, buildPostgresCreateCreditParams(credit))
		if err != nil {
			return fmt.Errorf("create credit: %w", err)
		}
		credit.ID = result.ID
		return nil
	}

	result, err := r.SQLite().CreateCredit(ctx, buildSQLiteCreateCreditParams(credit))
	if err != nil {
		return fmt.Errorf("create credit: %w", err)
	}
	credit.ID = result.ID
	return nil
}

// DeleteCreditsForEntity deletes all credits for a media entity.
func (r *Repository) DeleteCreditsForEntity(mediaType string, entityID int64) error {
	ctx := context.Background()
	return common.ExecuteCommand(
		r.BaseRepository, ctx,
		func() error {
			return r.Postgres().DeleteCreditsForEntity(ctx, sqlc_postgres.DeleteCreditsForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
		func() error {
			return r.SQLite().DeleteCreditsForEntity(ctx, sqlc_sqlite.DeleteCreditsForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
	)
}

// ReplaceCreditsForEntity deletes all existing credits and creates new ones.
func (r *Repository) ReplaceCreditsForEntity(mediaType string, entityID int64, credits []*media.Credit) error {
	// Delete existing credits
	if err := r.DeleteCreditsForEntity(mediaType, entityID); err != nil {
		return fmt.Errorf("delete existing credits: %w", err)
	}

	// Create new credits
	for _, credit := range credits {
		credit.MediaType = mediaType
		credit.EntityID = entityID
		if err := r.CreateCredit(credit); err != nil {
			return fmt.Errorf("create credit for %s: %w", credit.Person.Name, err)
		}
	}

	return nil
}

// --- Convenience Methods ---

// GetCastForEntity retrieves cast credits for a media entity.
func (r *Repository) GetCastForEntity(mediaType string, entityID int64, limit int) ([]*media.Credit, error) {
	ctx := context.Background()
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.GetCastForEntityRow, error) {
			return r.Postgres().GetCastForEntity(ctx, sqlc_postgres.GetCastForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
				Limit:     int32(limit),
			})
		},
		func() ([]sqlc_sqlite.GetCastForEntityRow, error) {
			return r.SQLite().GetCastForEntity(ctx, sqlc_sqlite.GetCastForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
				Limit:     int64(limit),
			})
		},
		postgresCastRowToDomain,
		sqliteCastRowToDomain,
	)
}

// GetDirectorsForEntity retrieves director credits for a media entity.
func (r *Repository) GetDirectorsForEntity(mediaType string, entityID int64) ([]*media.Credit, error) {
	ctx := context.Background()
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.GetDirectorsForEntityRow, error) {
			return r.Postgres().GetDirectorsForEntity(ctx, sqlc_postgres.GetDirectorsForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
		func() ([]sqlc_sqlite.GetDirectorsForEntityRow, error) {
			return r.SQLite().GetDirectorsForEntity(ctx, sqlc_sqlite.GetDirectorsForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
		postgresDirectorRowToDomain,
		sqliteDirectorRowToDomain,
	)
}

// GetWritersForEntity retrieves writer credits for a media entity.
func (r *Repository) GetWritersForEntity(mediaType string, entityID int64) ([]*media.Credit, error) {
	ctx := context.Background()
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.GetWritersForEntityRow, error) {
			return r.Postgres().GetWritersForEntity(ctx, sqlc_postgres.GetWritersForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
		func() ([]sqlc_sqlite.GetWritersForEntityRow, error) {
			return r.SQLite().GetWritersForEntity(ctx, sqlc_sqlite.GetWritersForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
		postgresWriterRowToDomain,
		sqliteWriterRowToDomain,
	)
}

// GetCreatorsForEntity retrieves creator credits for a media entity.
func (r *Repository) GetCreatorsForEntity(mediaType string, entityID int64) ([]*media.Credit, error) {
	ctx := context.Background()
	return common.QueryMany(
		r.BaseRepository, ctx,
		func() ([]sqlc_postgres.GetCreatorsForEntityRow, error) {
			return r.Postgres().GetCreatorsForEntity(ctx, sqlc_postgres.GetCreatorsForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
		func() ([]sqlc_sqlite.GetCreatorsForEntityRow, error) {
			return r.SQLite().GetCreatorsForEntity(ctx, sqlc_sqlite.GetCreatorsForEntityParams{
				MediaType: mediaType,
				EntityID:  entityID,
			})
		},
		postgresCreatorRowToDomain,
		sqliteCreatorRowToDomain,
	)
}
