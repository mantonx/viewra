package people

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/media"
)

// GetCreditsForEntityUseCase handles the business logic for getting credits for a media entity.
type GetCreditsForEntityUseCase struct {
	repo media.PeopleRepository
}

// NewGetCreditsForEntityUseCase creates a new instance of GetCreditsForEntityUseCase.
func NewGetCreditsForEntityUseCase(repo media.PeopleRepository) *GetCreditsForEntityUseCase {
	return &GetCreditsForEntityUseCase{
		repo: repo,
	}
}

// Execute retrieves credits for a media entity (movie, tv_show, tv_episode).
func (uc *GetCreditsForEntityUseCase) Execute(ctx context.Context, mediaType string, entityID int64) (*CreditsResponse, error) {
	credits, err := uc.repo.GetCreditsForEntity(mediaType, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credits: %w", err)
	}

	response := ToCreditsResponse(credits)
	return &response, nil
}

// GetCreditsForPersonUseCase handles the business logic for getting a person's filmography.
type GetCreditsForPersonUseCase struct {
	repo media.PeopleRepository
}

// NewGetCreditsForPersonUseCase creates a new instance of GetCreditsForPersonUseCase.
func NewGetCreditsForPersonUseCase(repo media.PeopleRepository) *GetCreditsForPersonUseCase {
	return &GetCreditsForPersonUseCase{
		repo: repo,
	}
}

// Execute retrieves all credits for a person (their filmography).
func (uc *GetCreditsForPersonUseCase) Execute(ctx context.Context, personID int64) (*PersonCreditsResponse, error) {
	person, err := uc.repo.GetPersonByID(personID)
	if err != nil {
		return nil, fmt.Errorf("failed to get person: %w", err)
	}

	credits, err := uc.repo.GetCreditsForPerson(personID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credits for person: %w", err)
	}

	creditResponses := make([]CreditResponse, 0, len(credits))
	for _, c := range credits {
		creditResponses = append(creditResponses, ToCreditResponse(c))
	}

	return &PersonCreditsResponse{
		Person:  ToPersonResponse(person),
		Credits: creditResponses,
	}, nil
}
