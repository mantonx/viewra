package handlers

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/media"
	domainMedia "github.com/mantonx/viewra/internal/domain/media"
	"github.com/mantonx/viewra/internal/infrastructure/subtitles"
)

// SubtitleHandler handles subtitle streaming requests.
type SubtitleHandler struct {
	mediaRepo media.GetMediaExecutor
	trackRepo domainMedia.Repository
	converter *subtitles.Converter
}

// NewSubtitleHandler creates a new subtitle handler.
func NewSubtitleHandler(
	mediaRepo media.GetMediaExecutor,
	trackRepo domainMedia.Repository,
	converter *subtitles.Converter,
) *SubtitleHandler {
	return &SubtitleHandler{
		mediaRepo: mediaRepo,
		trackRepo: trackRepo,
		converter: converter,
	}
}

// GetSubtitle handles GET /api/media/:id/subtitles/:trackId
// @Summary Get subtitle track as WebVTT
// @Description Returns subtitle content converted to WebVTT format for HLS playback
// @Tags subtitles
// @Produce text/vtt
// @Param id path int true "Media ID"
// @Param trackId path int true "Subtitle Track ID"
// @Success 200 {string} string "WebVTT content"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/media/{id}/subtitles/{trackId} [get]
func (h *SubtitleHandler) GetSubtitle(c *gin.Context) {
	mediaID, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid media ID",
			Message: err.Error(),
		})
		return
	}

	trackID, err := parseID(c.Param("trackId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid track ID",
			Message: err.Error(),
		})
		return
	}

	// Get subtitle tracks for this media
	tracks, err := h.trackRepo.GetSubtitleTracksByMediaID(c.Request.Context(), mediaID)
	if err != nil {
		handleError(c, err)
		return
	}

	// Find the requested track
	var track *domainMedia.SubtitleTrack
	for _, t := range tracks {
		if t.ID == trackID {
			track = t
			break
		}
	}

	if track == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Subtitle track not found",
		})
		return
	}

	// Check if this is a bitmap format (cannot be converted)
	if track.IsBitmap {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Bitmap subtitles not supported",
			Message: "PGS/DVD subtitles must be burned into video stream",
		})
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
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Failed to convert subtitle",
				Message: err.Error(),
			})
			return
		}
	} else if track.StreamIndex != nil {
		// Embedded subtitle - extract from media file
		mediaResp, err := h.mediaRepo.Execute(c.Request.Context(), mediaID)
		if err != nil {
			handleError(c, err)
			return
		}

		vttPath, err = h.converter.ExtractAndConvert(c.Request.Context(), mediaResp.FilePath, *track.StreamIndex)
		if err != nil {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Failed to extract subtitle",
				Message: err.Error(),
			})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid subtitle track configuration",
		})
		return
	}

	// Read and serve the WebVTT content
	vttContent, err := subtitles.GetWebVTTContent(vttPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to read subtitle file",
			Message: err.Error(),
		})
		return
	}

	c.Header("Content-Type", "text/vtt; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=86400") // Cache for 24 hours
	c.String(http.StatusOK, vttContent)
}

// GetSubtitleByStreamIndex handles GET /api/media/:id/subtitles/stream/:index
// @Summary Get embedded subtitle by stream index as WebVTT
// @Description Extracts and converts an embedded subtitle stream to WebVTT format
// @Tags subtitles
// @Produce text/vtt
// @Param id path int true "Media ID"
// @Param index path int true "Stream Index"
// @Success 200 {string} string "WebVTT content"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/media/{id}/subtitles/stream/{index} [get]
func (h *SubtitleHandler) GetSubtitleByStreamIndex(c *gin.Context) {
	mediaID, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid media ID",
			Message: err.Error(),
		})
		return
	}

	streamIndex, err := parseID(c.Param("index"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid stream index",
			Message: err.Error(),
		})
		return
	}

	// Get media file path
	mediaResp, err := h.mediaRepo.Execute(c.Request.Context(), mediaID)
	if err != nil {
		handleError(c, err)
		return
	}

	// Extract and convert
	vttPath, err := h.converter.ExtractAndConvert(c.Request.Context(), mediaResp.FilePath, int(streamIndex))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to extract subtitle",
			Message: err.Error(),
		})
		return
	}

	// Read and serve
	vttContent, err := subtitles.GetWebVTTContent(vttPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to read subtitle file",
			Message: err.Error(),
		})
		return
	}

	c.Header("Content-Type", "text/vtt; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=86400")
	c.String(http.StatusOK, vttContent)
}
