package handlers

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/media"
	"github.com/mantonx/viewra/internal/infrastructure/streaming"
	"github.com/mantonx/viewra/internal/pkg/logger"
)

// StreamHandler handles media streaming requests
type StreamHandler struct {
	getMedia      media.GetMediaExecutor
	streamService *streaming.Service
	logger        *slog.Logger
}

// NewStreamHandler creates a new stream handler
func NewStreamHandler(getMedia media.GetMediaExecutor, streamService *streaming.Service, log *slog.Logger) *StreamHandler {
	return &StreamHandler{
		getMedia:      getMedia,
		streamService: streamService,
		logger:        logger.DefaultIfNil(log),
	}
}

// Stream handles GET /api/stream/:id
// @Summary Stream media file
// @Description Streams a media file with HTTP range request support for seeking
// @Tags streaming
// @Produce application/octet-stream
// @Param id path int true "Media ID"
// @Param Range header string false "HTTP Range header (e.g., bytes=0-1023)"
// @Success 200 {file} binary "Full file"
// @Success 206 {file} binary "Partial content"
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 416 {object} APIError "Range not satisfiable"
// @Failure 500 {object} APIError
// @Router /api/stream/{id} [get]
func (h *StreamHandler) Stream(c *gin.Context) {
	// Parse media ID
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid media ID",
			Message: err.Error(),
		})
		return
	}

	// Get media metadata from database
	mediaResp, err := h.getMedia.Execute(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	// Prepare stream using streaming service
	stream, err := h.streamService.PrepareStream(streaming.StreamRequest{
		FilePath:    mediaResp.FilePath,
		RangeHeader: c.GetHeader("Range"),
	})
	if err != nil {
		// Check if this is a range error
		if stream == nil {
			c.Header("Content-Range", "bytes */*")
			c.JSON(http.StatusRequestedRangeNotSatisfiable, ErrorResponse{
				Error:   "Invalid range",
				Message: err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to prepare stream",
			Message: err.Error(),
		})
		return
	}
	defer stream.Close()

	// Set common headers
	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Type", stream.ContentType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%q", stream.FileName))
	c.Header("Content-Length", strconv.FormatInt(stream.ContentLength(), 10))

	// Set status and range-specific headers
	if stream.IsPartial {
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d",
			stream.Range.Start, stream.Range.End, stream.FileSize))
		c.Status(http.StatusPartialContent)
	} else {
		c.Status(http.StatusOK)
	}

	// Stream the content
	if _, err := io.Copy(c.Writer, stream.Reader()); err != nil {
		// Log error but don't return since headers already sent
		// Client will see incomplete response
		h.logger.Error("error streaming media after headers sent",
			"media_id", id,
			"file_path", mediaResp.FilePath,
			"error", err)
	}
}
