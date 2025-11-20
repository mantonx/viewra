package common

// PaginationParams contains parameters for paginating database queries
type PaginationParams struct {
	Limit  int    // Number of items per page (default: 50, max: 200)
	Offset int    // Number of items to skip (default: 0)
	SortBy string // Sort order: "title_asc" or "title_desc" (default: "title_asc")
}

// DefaultPaginationParams creates pagination params with sensible defaults
func DefaultPaginationParams() *PaginationParams {
	return &PaginationParams{
		Limit:  50,
		Offset: 0,
		SortBy: "title_asc",
	}
}

// NewPaginationParams creates validated pagination parameters
func NewPaginationParams(limit, offset int) *PaginationParams {
	// Validate and apply defaults
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200 // Max page size to prevent abuse
	}
	if offset < 0 {
		offset = 0
	}

	return &PaginationParams{
		Limit:  limit,
		Offset: offset,
		SortBy: "title_asc",
	}
}

// NewPaginationParamsWithSort creates validated pagination parameters with sort order
func NewPaginationParamsWithSort(limit, offset int, sortBy string) *PaginationParams {
	params := NewPaginationParams(limit, offset)
	if sortBy == "" {
		sortBy = "title_asc"
	}
	params.SortBy = sortBy
	return params
}

// PaginationMetadata contains metadata about paginated results
type PaginationMetadata struct {
	Total   int64 `json:"total"`   // Total number of items across all pages
	Limit   int   `json:"limit"`   // Number of items per page
	Offset  int   `json:"offset"`  // Number of items skipped
	HasMore bool  `json:"hasMore"` // Whether there are more items available
}

// NewPaginationMetadata creates pagination metadata from count and params
func NewPaginationMetadata(total int64, params *PaginationParams) *PaginationMetadata {
	if params == nil {
		params = DefaultPaginationParams()
	}

	hasMore := int64(params.Offset+params.Limit) < total

	return &PaginationMetadata{
		Total:   total,
		Limit:   params.Limit,
		Offset:  params.Offset,
		HasMore: hasMore,
	}
}
