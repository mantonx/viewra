package search

import (
	"context"
	"log/slog"

	"github.com/mantonx/viewra/internal/domain/search"
)

// Service provides fallback search when no semantic search plugin is available.
type Service struct {
	repo   search.Repository
	logger *slog.Logger
}

// NewService creates a new search service.
func NewService(repo search.Repository, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

// Search performs a fallback text search across all media.
// This is used when no semantic search plugin is configured.
func (s *Service) Search(ctx context.Context, req *search.Request) (*search.Response, error) {
	results, err := s.repo.Search(ctx, req)
	if err != nil {
		s.logger.Error("fallback search failed", "error", err, "query", req.Query)
		return nil, err
	}

	return &search.Response{
		Results:  results,
		Total:    len(results),
		Fallback: true,
	}, nil
}
