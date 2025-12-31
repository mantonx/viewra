package people

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/database/unified"
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
	row, err := r.Q().GetPersonByID(context.Background(), id)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return personToDomain(row), nil
}

// GetPersonByName retrieves a person by their name.
func (r *Repository) GetPersonByName(name string) (*media.Person, error) {
	row, err := r.Q().GetPersonByName(context.Background(), name)
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return personToDomain(row), nil
}

// GetPersonByTMDbID retrieves a person by their TMDb ID.
func (r *Repository) GetPersonByTMDbID(tmdbID int) (*media.Person, error) {
	row, err := r.Q().GetPersonByTMDbID(context.Background(), common.NullInt64(int64(tmdbID)))
	if err != nil {
		return nil, r.ConvertNotFoundError(err)
	}
	return personToDomain(row), nil
}

// CreatePerson creates a new person in the database.
func (r *Repository) CreatePerson(person *media.Person) error {
	result, err := r.Q().CreatePerson(context.Background(), buildCreatePersonParams(person))
	if err != nil {
		return fmt.Errorf("create person: %w", err)
	}
	person.ID = result.ID
	return nil
}

// UpdatePerson updates an existing person in the database.
func (r *Repository) UpdatePerson(person *media.Person) error {
	if err := r.Q().UpdatePerson(context.Background(), buildUpdatePersonParams(person)); err != nil {
		return fmt.Errorf("update person: %w", err)
	}
	return nil
}

// isNotFoundError checks if the error indicates a record was not found.
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
			_ = r.UpdatePerson(person)
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
				_ = r.UpdatePerson(person)
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
		if tmdbID > 0 && person.TMDbID == 0 {
			person.TMDbID = tmdbID
			needsUpdate = true
		}
		if photoURL != "" && person.PhotoURL == "" {
			person.PhotoURL = photoURL
			needsUpdate = true
		}
		if needsUpdate {
			_ = r.UpdatePerson(person)
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
	rows, err := r.Q().GetCreditsForEntity(context.Background(), unified.GetCreditsForEntityParams{
		MediaType: mediaType,
		EntityID:  entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("get credits for entity: %w", err)
	}
	return mapSlice(rows, creditRowToDomain), nil
}

// GetCreditsForPerson retrieves all credits for a person.
func (r *Repository) GetCreditsForPerson(personID int64) ([]*media.Credit, error) {
	rows, err := r.Q().GetCreditsForPerson(context.Background(), personID)
	if err != nil {
		return nil, fmt.Errorf("get credits for person: %w", err)
	}
	return mapSlice(rows, creditsForPersonToDomain), nil
}

// CreateCredit creates a new credit in the database.
func (r *Repository) CreateCredit(credit *media.Credit) error {
	result, err := r.Q().CreateCredit(context.Background(), buildCreateCreditParams(credit))
	if err != nil {
		return fmt.Errorf("create credit: %w", err)
	}
	credit.ID = result.ID
	return nil
}

// DeleteCreditsForEntity deletes all credits for a media entity.
func (r *Repository) DeleteCreditsForEntity(mediaType string, entityID int64) error {
	if err := r.Q().DeleteCreditsForEntity(context.Background(), unified.DeleteCreditsForEntityParams{
		MediaType: mediaType,
		EntityID:  entityID,
	}); err != nil {
		return fmt.Errorf("delete credits for entity: %w", err)
	}
	return nil
}

// ReplaceCreditsForEntity deletes all existing credits and creates new ones.
func (r *Repository) ReplaceCreditsForEntity(mediaType string, entityID int64, credits []*media.Credit) error {
	if err := r.DeleteCreditsForEntity(mediaType, entityID); err != nil {
		return fmt.Errorf("delete existing credits: %w", err)
	}

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
	rows, err := r.Q().GetCastForEntity(context.Background(), unified.GetCastForEntityParams{
		MediaType: mediaType,
		EntityID:  entityID,
		Limit:     int64(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("get cast for entity: %w", err)
	}
	return mapSlice(rows, castRowToDomain), nil
}

// GetDirectorsForEntity retrieves director credits for a media entity.
func (r *Repository) GetDirectorsForEntity(mediaType string, entityID int64) ([]*media.Credit, error) {
	rows, err := r.Q().GetDirectorsForEntity(context.Background(), unified.GetDirectorsForEntityParams{
		MediaType: mediaType,
		EntityID:  entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("get directors for entity: %w", err)
	}
	return mapSlice(rows, directorRowToDomain), nil
}

// GetWritersForEntity retrieves writer credits for a media entity.
func (r *Repository) GetWritersForEntity(mediaType string, entityID int64) ([]*media.Credit, error) {
	rows, err := r.Q().GetWritersForEntity(context.Background(), unified.GetWritersForEntityParams{
		MediaType: mediaType,
		EntityID:  entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("get writers for entity: %w", err)
	}
	return mapSlice(rows, writerRowToDomain), nil
}

// GetCreatorsForEntity retrieves creator credits for a media entity.
func (r *Repository) GetCreatorsForEntity(mediaType string, entityID int64) ([]*media.Credit, error) {
	rows, err := r.Q().GetCreatorsForEntity(context.Background(), unified.GetCreatorsForEntityParams{
		MediaType: mediaType,
		EntityID:  entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("get creators for entity: %w", err)
	}
	return mapSlice(rows, creatorRowToDomain), nil
}

// mapSlice converts a slice of one type to another using the provided mapper function.
func mapSlice[TFrom, TTo any](from []TFrom, mapper func(TFrom) TTo) []TTo {
	result := make([]TTo, len(from))
	for i, v := range from {
		result[i] = mapper(v)
	}
	return result
}
