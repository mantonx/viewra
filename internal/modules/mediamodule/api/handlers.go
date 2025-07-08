// Package api provides HTTP API handlers for the media module.
// The handlers are organized by domain:
//   - handlers.go: Base handler struct and common functionality
//   - media_handlers.go: Media file operations
//   - tv_handlers.go: TV show related endpoints
//   - movie_handlers.go: Movie related endpoints  
//   - music_handlers.go: Music and audio related endpoints
//   - library_handlers.go: Media library management
package api

import (
	"github.com/mantonx/viewra/internal/modules/mediamodule/core/library"
	"github.com/mantonx/viewra/internal/services"
)

// Handler provides HTTP handlers for media operations.
// It coordinates between the media service for file operations
// and the library manager for library organization.
type Handler struct {
	mediaService   services.MediaService
	libraryManager *library.Manager
}

// NewHandler creates a new API handler with the provided dependencies.
// The handler requires:
//   - mediaService: For media file operations and queries
//   - libraryManager: For library management operations
func NewHandler(mediaService services.MediaService, libraryManager *library.Manager) *Handler {
	return &Handler{
		mediaService:   mediaService,
		libraryManager: libraryManager,
	}
}

