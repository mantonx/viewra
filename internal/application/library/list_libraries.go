package library

import (
	"context"
	"fmt"

	"github.com/mantonx/viewra/internal/domain/library"
)

// ListLibrariesUseCase handles the business logic for listing libraries
type ListLibrariesUseCase struct {
	repo library.Repository
}

// NewListLibrariesUseCase creates a new instance of ListLibrariesUseCase
func NewListLibrariesUseCase(repo library.Repository) *ListLibrariesUseCase {
	return &ListLibrariesUseCase{
		repo: repo,
	}
}

// Execute retrieves all libraries
func (uc *ListLibrariesUseCase) Execute(ctx context.Context) (ListLibrariesResponse, error) {
	libs, err := uc.repo.List(ctx)
	if err != nil {
		return ListLibrariesResponse{}, fmt.Errorf("failed to list libraries: %w", err)
	}

	return ToListLibrariesResponse(libs), nil
}
