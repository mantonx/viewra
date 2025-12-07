// Package handlers provides HTTP request handlers for the ViewRA API.
//
// The transcode handlers are split across multiple files for better organization:
//   - transcode.go: Handler struct and constructor (this file)
//   - transcode_types.go: Request/response types and conversion helpers
//   - transcode_jobs.go: Job creation, status, and queue management
//   - transcode_streaming.go: HLS playlist and segment serving
//   - transcode_cleanup.go: Disk usage and cleanup operations
package handlers

import (
	"github.com/mantonx/viewra/internal/application/media"
	"github.com/mantonx/viewra/internal/application/transcode"
	"github.com/mantonx/viewra/internal/infrastructure/subtitles"
	"github.com/mantonx/viewra/internal/infrastructure/transcoding/session"
)

// TranscodeHandler handles transcode-related HTTP requests.
type TranscodeHandler struct {
	createJobUseCase           *transcode.CreateJobUseCase
	getStatusUseCase           *transcode.GetJobStatusUseCase
	serveManifestUseCase       *transcode.ServeManifestUseCase
	serveMasterPlaylistUseCase *transcode.ServeMasterPlaylistUseCase
	getMediaUseCase            *media.GetMediaUseCase
	getTracksUseCase           *media.GetTracksUseCase
	queue                      *transcode.Queue
	cleanupService             *transcode.CleanupService
	sessionManager             *session.Manager
	outputDir                  string
	subtitleConverter          *subtitles.Converter
}

// NewTranscodeHandler creates a new transcode handler.
func NewTranscodeHandler(
	createJobUseCase *transcode.CreateJobUseCase,
	getStatusUseCase *transcode.GetJobStatusUseCase,
	serveManifestUseCase *transcode.ServeManifestUseCase,
	serveMasterPlaylistUseCase *transcode.ServeMasterPlaylistUseCase,
	getMediaUseCase *media.GetMediaUseCase,
	getTracksUseCase *media.GetTracksUseCase,
	queue *transcode.Queue,
	cleanupService *transcode.CleanupService,
	sessionManager *session.Manager,
	outputDir string,
	subtitleConverter *subtitles.Converter,
) *TranscodeHandler {
	return &TranscodeHandler{
		createJobUseCase:           createJobUseCase,
		getStatusUseCase:           getStatusUseCase,
		serveManifestUseCase:       serveManifestUseCase,
		serveMasterPlaylistUseCase: serveMasterPlaylistUseCase,
		getMediaUseCase:            getMediaUseCase,
		getTracksUseCase:           getTracksUseCase,
		queue:                      queue,
		cleanupService:             cleanupService,
		sessionManager:             sessionManager,
		outputDir:                  outputDir,
		subtitleConverter:          subtitleConverter,
	}
}
