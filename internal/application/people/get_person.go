package people

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/media"
)

// GetPersonUseCase handles the business logic for getting a single person.
type GetPersonUseCase struct {
	repo media.PeopleRepository
}

// NewGetPersonUseCase creates a new instance of GetPersonUseCase.
func NewGetPersonUseCase(repo media.PeopleRepository) *GetPersonUseCase {
	return &GetPersonUseCase{
		repo: repo,
	}
}

// Execute retrieves a person by their ID.
func (uc *GetPersonUseCase) Execute(ctx context.Context, id int64) (*PersonResponse, error) {
	person, err := uc.repo.GetPersonByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get person: %w", err)
	}

	response := ToPersonResponse(person)
	return &response, nil
}
