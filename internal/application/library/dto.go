package library

import (
	"time"

	"github.com/viewra/viewra/internal/domain/library"
)

// CreateLibraryRequest represents the input for creating a new library
type CreateLibraryRequest struct {
	Name string `json:"name" validate:"required,min=1,max=100"`
	Path string `json:"path" validate:"required"`
	Type string `json:"type" validate:"required,oneof=movies tv music"`
}

// CreateLibraryResponse represents the output after creating a library
type CreateLibraryResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpdateLibraryRequest represents the input for updating a library
type UpdateLibraryRequest struct {
	ID   int64  `json:"id" validate:"required"`
	Name string `json:"name" validate:"omitempty,min=1,max=100"`
	Path string `json:"path" validate:"omitempty"`
	Type string `json:"type" validate:"omitempty,oneof=movies tv music"`
}

// UpdateLibraryResponse represents the output after updating a library
type UpdateLibraryResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LibraryResponse represents a single library in responses
type LibraryResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListLibrariesResponse represents a list of libraries
type ListLibrariesResponse struct {
	Libraries []LibraryResponse `json:"libraries"`
	Total     int               `json:"total"`
}

// GetLibraryRequest represents the input for getting a single library
type GetLibraryRequest struct {
	ID int64 `json:"id" validate:"required"`
}

// GetLibraryResponse represents the output for getting a single library
type GetLibraryResponse struct {
	Library LibraryResponse `json:"library"`
}

// DeleteLibraryRequest represents the input for deleting a library
type DeleteLibraryRequest struct {
	ID int64 `json:"id" validate:"required"`
}

// ScanLibraryRequest represents the input for scanning a library
type ScanLibraryRequest struct {
	LibraryID int64 `json:"library_id" validate:"required"`
}

// ScanLibraryResponse represents the output after initiating a library scan
type ScanLibraryResponse struct {
	JobID     int64  `json:"job_id"`
	LibraryID int64  `json:"library_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// ToLibraryResponse converts a domain library entity to a DTO
func ToLibraryResponse(lib *library.Library) LibraryResponse {
	return LibraryResponse{
		ID:        lib.ID,
		Name:      lib.Name,
		Path:      lib.Path,
		Type:      string(lib.Type),
		CreatedAt: lib.CreatedAt,
		UpdatedAt: lib.UpdatedAt,
	}
}

// ToCreateLibraryResponse converts a domain library to create response DTO
func ToCreateLibraryResponse(lib *library.Library) CreateLibraryResponse {
	return CreateLibraryResponse{
		ID:        lib.ID,
		Name:      lib.Name,
		Path:      lib.Path,
		Type:      string(lib.Type),
		CreatedAt: lib.CreatedAt,
		UpdatedAt: lib.UpdatedAt,
	}
}

// ToUpdateLibraryResponse converts a domain library to update response DTO
func ToUpdateLibraryResponse(lib *library.Library) UpdateLibraryResponse {
	return UpdateLibraryResponse{
		ID:        lib.ID,
		Name:      lib.Name,
		Path:      lib.Path,
		Type:      string(lib.Type),
		CreatedAt: lib.CreatedAt,
		UpdatedAt: lib.UpdatedAt,
	}
}

// ToListLibrariesResponse converts a list of domain libraries to list response DTO
func ToListLibrariesResponse(libs []*library.Library) ListLibrariesResponse {
	responses := make([]LibraryResponse, len(libs))
	for i, lib := range libs {
		responses[i] = ToLibraryResponse(lib)
	}

	return ListLibrariesResponse{
		Libraries: responses,
		Total:     len(responses),
	}
}
