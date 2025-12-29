package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/media"
	domainMedia "github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/subtitles"
)

// SubtitleHandler handles subtitle streaming requests.
type SubtitleHandler struct {
	mediaRepo     media.GetMediaExecutor
	tracksUseCase *media.GetTracksUseCase
	converter     *subtitles.Converter
}

// NewSubtitleHandler creates a new subtitle handler.
func NewSubtitleHandler(
	mediaRepo media.GetMediaExecutor,
	tracksUseCase *media.GetTracksUseCase,
	converter *subtitles.Converter,
) *SubtitleHandler {
	return &SubtitleHandler{
		mediaRepo:     mediaRepo,
		tracksUseCase: tracksUseCase,
		converter:     converter,
	}
}

// serveVTTContent reads a VTT file and serves it with appropriate headers.
func serveVTTContent(c *gin.Context, vttPath string) {
	vttContent, err := subtitles.GetWebVTTContent(vttPath)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "FAILED_TO_READ_SUBTITLE_FILE", err.Error())
		return
	}

	c.Header("Content-Type", "text/vtt; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=86400")
	c.String(http.StatusOK, vttContent)
}

// GetSubtitle handles GET /api/media/:id/subtitles/:trackId
// @Summary Get subtitle track as WebVTT
// @Description Returns subtitle content converted to WebVTT format for HLS playback
// @Tags subtitles
// @Produce text/vtt
// @Param id path int true "Media ID"
// @Param trackId path int true "Subtitle Track ID"
// @Success 200 {string} string "WebVTT content"
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/media/{id}/subtitles/{trackId} [get]
func (h *SubtitleHandler) GetSubtitle(c *gin.Context) {
	mediaID, err := parseID(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_MEDIA_ID", err.Error())
		return
	}

	trackID, err := parseID(c.Param("trackId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_TRACK_ID", err.Error())
		return
	}

	// Find the requested track by ID
	track, err := h.tracksUseCase.GetSubtitleTrackByID(c.Request.Context(), mediaID, trackID)
	if err != nil {
		handleError(c, err)
		return
	}
	if track == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "Subtitle track not found")
		return
	}

	// Check if this is a bitmap format (cannot be converted)
	if track.IsBitmap {
		respondError(c, http.StatusBadRequest, "BITMAP_SUBTITLES_NOT_SUPPORTED", "PGS/DVD subtitles must be burned into video stream")
		return
	}

	var vttPath string

	if track.SourceType == domainMedia.SubtitleSourceExternal && track.FilePath != nil {
		// External subtitle file - need library path to resolve
		mediaResp, err := h.mediaRepo.Execute(c.Request.Context(), mediaID)
		if err != nil {
			handleError(c, err)
			return
		}

		// Get library path (extract from media file path)
		// The external subtitle file_path is relative to library root
		mediaDir := filepath.Dir(mediaResp.FilePath)
		subtitlePath := filepath.Join(filepath.Dir(mediaDir), *track.FilePath)

		vttPath, err = h.converter.ConvertExternalSubtitle(c.Request.Context(), subtitlePath)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "FAILED_TO_CONVERT_SUBTITLE", err.Error())
			return
		}
	} else if track.StreamIndex != nil {
		// Embedded subtitle - extract from media file
		mediaResp, err := h.mediaRepo.Execute(c.Request.Context(), mediaID)
		if err != nil {
			handleError(c, err)
			return
		}

		vttPath, err = h.converter.ExtractAndConvert(c.Request.Context(), mediaID, mediaResp.FilePath, *track.StreamIndex)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "FAILED_TO_EXTRACT_SUBTITLE", err.Error())
			return
		}
	} else {
		respondError(c, http.StatusBadRequest, "INVALID_SUBTITLE_TRACK_CONFIGURATION", "Invalid subtitle track configuration")
		return
	}

	serveVTTContent(c, vttPath)
}

// GetSubtitleByStreamIndex handles GET /api/media/:id/subtitles/stream/:index
// @Summary Get embedded subtitle by stream index as WebVTT
// @Description Extracts and converts an embedded subtitle stream to WebVTT format
// @Tags subtitles
// @Produce text/vtt
// @Param id path int true "Media ID"
// @Param index path int true "Stream Index"
// @Success 200 {string} string "WebVTT content"
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/media/{id}/subtitles/stream/{index} [get]
func (h *SubtitleHandler) GetSubtitleByStreamIndex(c *gin.Context) {
	mediaID, err := parseID(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_MEDIA_ID", err.Error())
		return
	}

	streamIndex, err := parseID(c.Param("index"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_STREAM_INDEX", err.Error())
		return
	}

	// Get media file path
	mediaResp, err := h.mediaRepo.Execute(c.Request.Context(), mediaID)
	if err != nil {
		handleError(c, err)
		return
	}

	// Extract and convert
	vttPath, err := h.converter.ExtractAndConvert(c.Request.Context(), mediaID, mediaResp.FilePath, int(streamIndex))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "FAILED_TO_EXTRACT_SUBTITLE", err.Error())
		return
	}

	// Read and serve
	vttContent, err := subtitles.GetWebVTTContent(vttPath)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "FAILED_TO_READ_SUBTITLE_FILE", err.Error())
		return
	}

	c.Header("Content-Type", "text/vtt; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=86400")
	c.String(http.StatusOK, vttContent)
}

// StreamTextSubtitle handles GET /api/media/:id/subtitles/text/:index/stream
// @Summary Stream embedded text subtitle as WebVTT
// @Description Streams the embedded subtitle stream, converting to WebVTT. Supports time-windowed extraction for faster initial load.
// @Tags subtitles
// @Produce text/vtt
// @Param id path int true "Media ID"
// @Param index path int true "Stream Index (relative, 0-based among subtitle streams)"
// @Param start query int false "Start time in milliseconds (default: 0, extracts full file)"
// @Param end query int false "End time in milliseconds (default: 0, extracts full file)"
// @Success 200 {string} string "WebVTT content"
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/media/{id}/subtitles/text/{index}/stream [get]
func (h *SubtitleHandler) StreamTextSubtitle(c *gin.Context) {
	mediaID, err := parseID(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_MEDIA_ID", err.Error())
		return
	}

	relativeIndex, err := parseID(c.Param("index"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_STREAM_INDEX", err.Error())
		return
	}

	// Parse optional time window parameters
	startMS := int64(0)
	if startParam := c.Query("start"); startParam != "" {
		if parsed, err := parseID(startParam); err == nil {
			startMS = parsed
		}
	}

	endMS := int64(0)
	if endParam := c.Query("end"); endParam != "" {
		if parsed, err := parseID(endParam); err == nil {
			endMS = parsed
		}
	}

	// Find text track at this relative index
	targetTrack, err := h.tracksUseCase.GetSubtitleTrackByRelativeIndex(c.Request.Context(), mediaID, int(relativeIndex), false)
	if err != nil {
		handleError(c, err)
		return
	}
	if targetTrack == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "Subtitle track not found")
		return
	}

	// Get media file path
	mediaResp, err := h.mediaRepo.Execute(c.Request.Context(), mediaID)
	if err != nil {
		handleError(c, err)
		return
	}

	// Use windowed extraction (or full extraction if no time bounds)
	vttContent, err := h.converter.StreamTextSubtitleWindow(c.Request.Context(), mediaID, mediaResp.FilePath, *targetTrack.StreamIndex, startMS, endMS)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "FAILED_TO_EXTRACT_SUBTITLE", err.Error())
		return
	}

	c.Header("Content-Type", "text/vtt; charset=utf-8")
	// Only cache full extractions (no time bounds) - windowed extractions are dynamic
	if startMS == 0 && endMS == 0 {
		c.Header("Cache-Control", "public, max-age=86400")
	} else {
		c.Header("Cache-Control", "public, max-age=3600") // Cache windows for 1 hour
	}
	c.String(http.StatusOK, string(vttContent))
}

// StreamPGSSubtitle handles GET /api/media/:id/subtitles/pgs/:index/stream
// @Summary Stream PGS subtitle frames as WebP images
// @Description Extracts PGS subtitle frames and returns them as JSON lines with base64-encoded WebP images. Use start/end params to request frames in time windows for efficient playback.
// @Tags subtitles
// @Produce application/x-ndjson
// @Param id path int true "Media ID"
// @Param index path int true "Stream Index (relative, 0-based among bitmap subtitle streams)"
// @Param start query int false "Start time in milliseconds (default: 0)"
// @Param end query int false "End time in milliseconds (default: 5 minutes from start). Use 0 for unlimited (not recommended for large files)."
// @Success 200 {object} subtitles.PGSFrame "JSON lines of PGS frames"
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/media/{id}/subtitles/pgs/{index}/stream [get]
func (h *SubtitleHandler) StreamPGSSubtitle(c *gin.Context) {
	mediaID, err := parseID(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_MEDIA_ID", err.Error())
		return
	}

	relativeIndex, err := parseID(c.Param("index"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_STREAM_INDEX", err.Error())
		return
	}

	// Parse time window parameters
	startMS := int64(0)
	if startParam := c.Query("start"); startParam != "" {
		if parsed, err := parseID(startParam); err == nil {
			startMS = parsed
		}
	}

	// Default to 5 minutes from start if no end specified
	const defaultWindowMS = 5 * 60 * 1000 // 5 minutes
	endMS := startMS + defaultWindowMS
	if endParam := c.Query("end"); endParam != "" {
		if parsed, err := parseID(endParam); err == nil {
			endMS = parsed
			// Allow explicit 0 to mean "unlimited" but warn in docs
		}
	}

	// Find the bitmap track at the requested relative index
	targetTrack, err := h.tracksUseCase.GetSubtitleTrackByRelativeIndex(c.Request.Context(), mediaID, int(relativeIndex), true)
	if err != nil {
		handleError(c, err)
		return
	}
	if targetTrack == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "PGS subtitle track not found")
		return
	}

	// Get media file path
	mediaResp, err := h.mediaRepo.Execute(c.Request.Context(), mediaID)
	if err != nil {
		handleError(c, err)
		return
	}

	// Stream PGS frames for the requested time window
	frames, errs := h.converter.StreamPGSFrames(c.Request.Context(), mediaID, mediaResp.FilePath, *targetTrack.StreamIndex, startMS, endMS)

	// Set headers for streaming JSON lines
	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 24 hours
	c.Header("Transfer-Encoding", "chunked")

	// Stream frames as JSON lines
	c.Stream(func(w io.Writer) bool {
		select {
		case frame, ok := <-frames:
			if !ok {
				return false // Channel closed, stop streaming
			}
			// Write JSON line
			data, err := json.Marshal(frame)
			if err != nil {
				return false
			}
			w.Write(data)
			w.Write([]byte("\n"))
			return true
		case err := <-errs:
			if err != nil {
				// Log error but don't break the stream
				// Client will see incomplete data
			}
			return false
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// GetAllPGSFrames handles GET /api/media/:id/subtitles/pgs/:index
// @Summary Get all PGS subtitle frames as JSON array
// @Description Extracts all PGS subtitle frames and returns them as a JSON array with base64-encoded WebP images
// @Tags subtitles
// @Produce application/json
// @Param id path int true "Media ID"
// @Param index path int true "Stream Index (relative, 0-based among bitmap subtitle streams)"
// @Success 200 {array} subtitles.PGSFrame "Array of PGS frames"
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/media/{id}/subtitles/pgs/{index} [get]
func (h *SubtitleHandler) GetAllPGSFrames(c *gin.Context) {
	mediaID, err := parseID(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_MEDIA_ID", err.Error())
		return
	}

	relativeIndex, err := parseID(c.Param("index"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_STREAM_INDEX", err.Error())
		return
	}

	// Find the bitmap track at the requested relative index
	targetTrack, err := h.tracksUseCase.GetSubtitleTrackByRelativeIndex(c.Request.Context(), mediaID, int(relativeIndex), true)
	if err != nil {
		handleError(c, err)
		return
	}
	if targetTrack == nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "PGS subtitle track not found")
		return
	}

	// Get media file path
	mediaResp, err := h.mediaRepo.Execute(c.Request.Context(), mediaID)
	if err != nil {
		handleError(c, err)
		return
	}

	// Get all PGS frames
	frames, err := h.converter.GetAllPGSFrames(c.Request.Context(), mediaID, mediaResp.FilePath, *targetTrack.StreamIndex)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "FAILED_TO_EXTRACT_PGS_SUBTITLE", err.Error())
		return
	}

	c.Header("Cache-Control", "public, max-age=86400")
	c.JSON(http.StatusOK, frames)
}
