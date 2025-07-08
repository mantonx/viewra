// Package api provides HTTP API handlers for the playback module.
// The handlers are organized by domain:
//   - handlers_base.go: Base handler struct and common functionality
//   - decision_handlers.go: Playback decision and compatibility checks
//   - session_handlers.go: Session management operations
//   - streaming_handlers.go: Media streaming endpoints
//   - analytics_handlers.go: Analytics and reporting
package api

import (
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core/session"
	"github.com/mantonx/viewra/internal/modules/playbackmodule/core/streaming"
	"github.com/mantonx/viewra/internal/services"
)

// Handler handles HTTP requests for the playback module.
// It coordinates between various services to provide playback functionality.
type Handler struct {
	playbackService services.PlaybackService
	mediaService    services.MediaService
	progressHandler *streaming.ProgressiveHandler
	sessionManager  *session.SessionManager
	logger          hclog.Logger
}

// NewHandler creates a new API handler with the provided dependencies.
// The handler requires:
//   - playbackService: For playback decisions and media analysis
//   - mediaService: For media file information
//   - progressHandler: For progressive download/streaming
//   - sessionManager: For session lifecycle management
//   - logger: For structured logging
func NewHandler(
	playbackService services.PlaybackService,
	mediaService services.MediaService,
	progressHandler *streaming.ProgressiveHandler,
	sessionManager *session.SessionManager,
	logger hclog.Logger,
) *Handler {
	return &Handler{
		playbackService: playbackService,
		mediaService:    mediaService,
		progressHandler: progressHandler,
		sessionManager:  sessionManager,
		logger:          logger,
	}
}