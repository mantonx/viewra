package common

import "github.com/viewra/viewra/internal/domain/common"

// ListIDsResponse represents a list of IDs for prefetching
// This is shared across movies, TV shows, and music to avoid duplication
type ListIDsResponse struct {
	IDs        []int64                    `json:"ids"`
	Total      int                        `json:"total"`
	Pagination *common.PaginationMetadata `json:"pagination,omitempty"`
}
