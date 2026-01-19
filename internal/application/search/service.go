package search

import (
	"context"
	"log/slog"

	"github.com/mantonx/viewra/internal/domain/search"
)

// Service provides text-based search across media.
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

// Search performs text search across all media.
func (s *Service) Search(ctx context.Context, req *search.Request) (*search.Response, error) {
	results, err := s.repo.Search(ctx, req)
	if err != nil {
		s.logger.Error("search failed", "error", err, "query", req.Query)
		return nil, err
	}

	return &search.Response{
		Results: results,
		Total:   len(results),
	}, nil
}
