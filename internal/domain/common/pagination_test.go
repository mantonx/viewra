package common

import (
	"testing"
)

func TestDefaultPaginationParams(t *testing.T) {
	params := DefaultPaginationParams()

	if params == nil {
		t.Fatal("DefaultPaginationParams returned nil")
	}
	if params.Limit != 50 {
		t.Errorf("Limit = %d, want 50", params.Limit)
	}
	if params.Offset != 0 {
		t.Errorf("Offset = %d, want 0", params.Offset)
	}
	if params.SortBy != "title_asc" {
		t.Errorf("SortBy = %q, want %q", params.SortBy, "title_asc")
	}
}

func TestNewPaginationParams(t *testing.T) {
	tests := []struct {
		name       string
		limit      int
		offset     int
		wantLimit  int
		wantOffset int
	}{
		{"valid values", 20, 40, 20, 40},
		{"zero limit uses default", 0, 0, 50, 0},
		{"negative limit uses default", -1, 0, 50, 0},
		{"limit exceeds max capped to 200", 500, 0, 200, 0},
		{"negative offset uses zero", 20, -10, 20, 0},
		{"all defaults", 0, -1, 50, 0},
		{"max limit boundary", 200, 0, 200, 0},
		{"just under max limit", 199, 0, 199, 0},
		{"just over max limit", 201, 0, 200, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := NewPaginationParams(tt.limit, tt.offset)

			if params.Limit != tt.wantLimit {
				t.Errorf("Limit = %d, want %d", params.Limit, tt.wantLimit)
			}
			if params.Offset != tt.wantOffset {
				t.Errorf("Offset = %d, want %d", params.Offset, tt.wantOffset)
			}
			if params.SortBy != "title_asc" {
				t.Errorf("SortBy = %q, want %q (default)", params.SortBy, "title_asc")
			}
		})
	}
}

func TestNewPaginationParamsWithSort(t *testing.T) {
	tests := []struct {
		name       string
		limit      int
		offset     int
		sortBy     string
		wantSortBy string
	}{
		{"custom sort", 20, 0, "title_desc", "title_desc"},
		{"empty sort uses default", 20, 0, "", "title_asc"},
		{"date sort", 20, 0, "created_at_desc", "created_at_desc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := NewPaginationParamsWithSort(tt.limit, tt.offset, tt.sortBy)

			if params.SortBy != tt.wantSortBy {
				t.Errorf("SortBy = %q, want %q", params.SortBy, tt.wantSortBy)
			}
		})
	}
}

func TestNewPaginationMetadata(t *testing.T) {
	tests := []struct {
		name        string
		total       int64
		params      *PaginationParams
		wantTotal   int64
		wantHasMore bool
	}{
		{
			name:        "has more items",
			total:       100,
			params:      &PaginationParams{Limit: 20, Offset: 0},
			wantTotal:   100,
			wantHasMore: true, // 0+20 < 100
		},
		{
			name:        "no more items - exact",
			total:       100,
			params:      &PaginationParams{Limit: 20, Offset: 80},
			wantTotal:   100,
			wantHasMore: false, // 80+20 >= 100
		},
		{
			name:        "no more items - past end",
			total:       100,
			params:      &PaginationParams{Limit: 20, Offset: 90},
			wantTotal:   100,
			wantHasMore: false, // 90+20 > 100
		},
		{
			name:        "nil params uses defaults",
			total:       200,
			params:      nil,
			wantTotal:   200,
			wantHasMore: true, // 0+50 < 200
		},
		{
			name:        "empty result set",
			total:       0,
			params:      &PaginationParams{Limit: 20, Offset: 0},
			wantTotal:   0,
			wantHasMore: false, // 0+20 > 0
		},
		{
			name:        "single page exactly fills",
			total:       20,
			params:      &PaginationParams{Limit: 20, Offset: 0},
			wantTotal:   20,
			wantHasMore: false, // 0+20 >= 20
		},
		{
			name:        "single item remaining",
			total:       21,
			params:      &PaginationParams{Limit: 20, Offset: 0},
			wantTotal:   21,
			wantHasMore: true, // 0+20 < 21
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := NewPaginationMetadata(tt.total, tt.params)

			if meta.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", meta.Total, tt.wantTotal)
			}
			if meta.HasMore != tt.wantHasMore {
				t.Errorf("HasMore = %v, want %v", meta.HasMore, tt.wantHasMore)
			}

			// Verify limit/offset match params (or defaults)
			expectedLimit := 50
			expectedOffset := 0
			if tt.params != nil {
				expectedLimit = tt.params.Limit
				expectedOffset = tt.params.Offset
			}
			if meta.Limit != expectedLimit {
				t.Errorf("Limit = %d, want %d", meta.Limit, expectedLimit)
			}
			if meta.Offset != expectedOffset {
				t.Errorf("Offset = %d, want %d", meta.Offset, expectedOffset)
			}
		})
	}
}

func TestPaginationParamsStruct(t *testing.T) {
	params := PaginationParams{
		Limit:  25,
		Offset: 100,
		SortBy: "year_desc",
	}

	if params.Limit != 25 {
		t.Errorf("Limit = %d, want 25", params.Limit)
	}
	if params.Offset != 100 {
		t.Errorf("Offset = %d, want 100", params.Offset)
	}
	if params.SortBy != "year_desc" {
		t.Errorf("SortBy = %q, want %q", params.SortBy, "year_desc")
	}
}

func TestPaginationMetadataStruct(t *testing.T) {
	meta := PaginationMetadata{
		Total:   150,
		Limit:   20,
		Offset:  40,
		HasMore: true,
	}

	if meta.Total != 150 {
		t.Errorf("Total = %d, want 150", meta.Total)
	}
	if meta.Limit != 20 {
		t.Errorf("Limit = %d, want 20", meta.Limit)
	}
	if meta.Offset != 40 {
		t.Errorf("Offset = %d, want 40", meta.Offset)
	}
	if !meta.HasMore {
		t.Error("HasMore should be true")
	}
}
