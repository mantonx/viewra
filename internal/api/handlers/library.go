package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/library"
)

// LibraryHandler handles HTTP requests for libraries
type LibraryHandler struct {
	service     library.LibraryServiceInterface
	scanLibrary library.ScanLibraryExecutor
}

// NewLibraryHandler creates a new library handler
func NewLibraryHandler(
	service library.LibraryServiceInterface,
	scanLibrary library.ScanLibraryExecutor,
) *LibraryHandler {
	return &LibraryHandler{
		service:     service,
		scanLibrary: scanLibrary,
	}
}

// Create handles POST /api/libraries
// @Summary Create a new library
// @Description Creates a new media library
// @Tags libraries
// @Accept json
// @Produce json
// @Param library body library.CreateLibraryRequest true "Library details"
// @Success 201 {object} library.LibraryResponse
// @Failure 400 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/libraries [post]
func (h *LibraryHandler) Create(c *gin.Context) {
	var req library.CreateLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Invalid request body")
		return
	}

	resp, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// List handles GET /api/libraries
// @Summary List all libraries
// @Description Returns a list of all libraries
// @Tags libraries
// @Produce json
// @Success 200 {object} library.ListLibrariesResponse
// @Failure 500 {object} APIError
// @Router /api/libraries [get]
func (h *LibraryHandler) List(c *gin.Context) {
	resp, err := h.service.List(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Get handles GET /api/libraries/:id
// @Summary Get a library by ID
// @Description Returns details of a specific library
// @Tags libraries
// @Produce json
// @Param id path int true "Library ID"
// @Success 200 {object} library.GetLibraryResponse
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/libraries/{id} [get]
func (h *LibraryHandler) Get(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_LIBRARY_ID", err.Error())
		return
	}

	resp, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, library.GetLibraryResponse{Library: resp})
}

// Update handles PUT /api/libraries/:id
// @Summary Update a library
// @Description Updates an existing library
// @Tags libraries
// @Accept json
// @Produce json
// @Param id path int true "Library ID"
// @Param library body library.UpdateLibraryRequest true "Library details"
// @Success 200 {object} library.LibraryResponse
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/libraries/{id} [put]
func (h *LibraryHandler) Update(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_LIBRARY_ID", err.Error())
		return
	}

	var req library.UpdateLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST_BODY", err.Error())
		return
	}

	resp, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Delete handles DELETE /api/libraries/:id
// @Summary Delete a library
// @Description Deletes a library and all its media entries (does not delete files)
// @Tags libraries
// @Produce json
// @Param id path int true "Library ID"
// @Success 204
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/libraries/{id} [delete]
func (h *LibraryHandler) Delete(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_LIBRARY_ID", err.Error())
		return
	}

	err = h.service.Delete(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// Scan handles POST /api/libraries/:id/scan
// @Summary Scan a library
// @Description Scans a library for media files
// @Tags libraries
// @Produce json
// @Param id path int true "Library ID"
// @Success 202 {object} library.StartScanResponse
// @Failure 400 {object} APIError
// @Failure 404 {object} APIError
// @Failure 409 {object} APIError "Library is already being scanned"
// @Failure 500 {object} APIError
// @Router /api/libraries/{id}/scan [post]
func (h *LibraryHandler) Scan(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_LIBRARY_ID", err.Error())
		return
	}

	resp, err := h.scanLibrary.StartScan(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, resp)
}
