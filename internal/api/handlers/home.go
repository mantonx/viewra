package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	appHome "github.com/mantonx/viewra/internal/application/home"
	"github.com/mantonx/viewra/internal/domain/home"
)

// HomeHandler handles /api/home requests.
type HomeHandler struct {
	homeService *appHome.Service
}

// NewHomeHandler creates a new home handler.
func NewHomeHandler(homeService *appHome.Service) *HomeHandler {
	return &HomeHandler{
		homeService: homeService,
	}
}

// GetHome handles GET /api/home
// @Summary Get home screen data
// @Description Returns all home screen sections for the authenticated user
// @Tags home
// @Produce json
// @Param inline query bool false "Include section data inline (default: true)"
// @Param sections query string false "Comma-separated section IDs to include"
// @Param X-Client-Type header string false "Client type (web, ios, android, roku, firetv, smarttv)"
// @Param X-Image-Size header string false "Preferred image size (small, medium, large)"
// @Success 200 {object} home.HomeResponse
// @Router /api/home [get]
func (h *HomeHandler) GetHome(c *gin.Context) {
	req := &home.HomeRequest{
		UserID:     getUserIDFromContext(c),
		ClientType: c.GetHeader("X-Client-Type"),
		Inline:     c.DefaultQuery("inline", "true") == "true",
		ImageSize:  c.GetHeader("X-Image-Size"),
	}

	// Parse sections filter
	if sectionsStr := c.Query("sections"); sectionsStr != "" && sectionsStr != "all" {
		req.Sections = strings.Split(sectionsStr, ",")
	}

	resp, err := h.homeService.GetHome(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get home data"})
		return
	}

	// Set cache headers
	c.Header("Cache-Control", "private, max-age=60")
	c.Header("Vary", "Authorization, X-Client-Type, X-Image-Size")

	c.JSON(http.StatusOK, resp)
}

// GetSection handles GET /api/home/sections/:id
// @Summary Get a single home section
// @Description Returns data for a specific home screen section
// @Tags home
// @Produce json
// @Param id path string true "Section ID"
// @Param X-Client-Type header string false "Client type"
// @Success 200 {object} home.Section
// @Failure 404 {object} gin.H "Section not found"
// @Router /api/home/sections/{id} [get]
func (h *HomeHandler) GetSection(c *gin.Context) {
	sectionID := c.Param("id")
	if sectionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "section ID is required"})
		return
	}

	section, err := h.homeService.GetSection(
		c.Request.Context(),
		sectionID,
		getUserIDFromContext(c),
		c.GetHeader("X-Client-Type"),
	)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": "section not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get section"})
		return
	}

	// Set cache headers based on section TTL
	if section.CacheTTLSeconds > 0 {
		c.Header("Cache-Control", "private, max-age="+string(rune(section.CacheTTLSeconds)))
	}

	c.JSON(http.StatusOK, section)
}

// GetPreferences handles GET /api/home/preferences
// @Summary Get user's home preferences
// @Description Returns the user's widget ordering and visibility preferences
// @Tags home
// @Produce json
// @Success 200 {array} home.WidgetPreference
// @Router /api/home/preferences [get]
func (h *HomeHandler) GetPreferences(c *gin.Context) {
	prefs, err := h.homeService.GetPreferences(c.Request.Context(), getUserIDFromContext(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get preferences"})
		return
	}

	c.JSON(http.StatusOK, prefs)
}

// UpdatePreferences handles PUT /api/home/preferences
// @Summary Update user's home preferences
// @Description Updates the user's widget ordering and visibility preferences
// @Tags home
// @Accept json
// @Produce json
// @Param body body home.PreferencesUpdateRequest true "Preferences update"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "Invalid request"
// @Router /api/home/preferences [put]
func (h *HomeHandler) UpdatePreferences(c *gin.Context) {
	var req home.PreferencesUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	err := h.homeService.UpdatePreferences(c.Request.Context(), getUserIDFromContext(c), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update preferences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ResetPreferences handles DELETE /api/home/preferences
// @Summary Reset user's home preferences
// @Description Resets the user's preferences to defaults
// @Tags home
// @Produce json
// @Success 200 {object} gin.H "success"
// @Router /api/home/preferences [delete]
func (h *HomeHandler) ResetPreferences(c *gin.Context) {
	err := h.homeService.ResetPreferences(c.Request.Context(), getUserIDFromContext(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reset preferences"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
