package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mantonx/viewra/internal/application/trending"
)

// TrendingHandler handles /api/trending requests.
type TrendingHandler struct {
	trendingService *trending.Service
}

// NewTrendingHandler creates a new trending handler.
func NewTrendingHandler(trendingService *trending.Service) *TrendingHandler {
	return &TrendingHandler{
		trendingService: trendingService,
	}
}

// GetTrending handles GET /api/trending
// @Summary Get trending media
// @Description Returns trending movies and TV shows matched to your library
// @Tags trending
// @Produce json
// @Param type query string false "Media type (movie, tv, all)"
// @Param window query string false "Time window (day, week)"
// @Param limit query int false "Maximum results (default 20)"
// @Success 200 {object} trending.Result
// @Failure 503 {object} gin.H "No trending provider available"
// @Router /api/trending [get]
func (h *TrendingHandler) GetTrending(c *gin.Context) {
	if !h.trendingService.HasProvider() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "no trending provider available",
			"message": "Enable a plugin that provides trending data (e.g., TMDb)",
		})
		return
	}

	mediaType := c.DefaultQuery("type", "all")
	window := c.DefaultQuery("window", "week")

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	result, err := h.trendingService.GetTrendingByWindow(c.Request.Context(), mediaType, window, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get trending data"})
		return
	}

	// Set cache headers
	cacheDuration := "3600" // 1 hour
	if window == "day" {
		cacheDuration = "1800" // 30 minutes for daily
	}
	c.Header("Cache-Control", "public, max-age="+cacheDuration)

	c.JSON(http.StatusOK, result)
}

// GetProviders handles GET /api/trending/providers
// @Summary Get trending providers
// @Description Returns available trending data providers
// @Tags trending
// @Produce json
// @Success 200 {array} registry.RegisteredTrendingProvider
// @Router /api/trending/providers [get]
func (h *TrendingHandler) GetProviders(c *gin.Context) {
	providers := h.trendingService.GetProviders()

	// Convert to a simpler response format
	result := make([]map[string]any, 0, len(providers))
	for _, p := range providers {
		result = append(result, map[string]any{
			"plugin_id":   p.PluginID,
			"id":          p.Info.ID,
			"name":        p.Info.Name,
			"description": p.Info.Description,
			"windows":     p.Info.Windows,
			"media_types": p.Info.MediaTypes,
			"update_freq": p.Info.UpdateFreq,
			"enabled":     p.Enabled,
		})
	}

	c.JSON(http.StatusOK, result)
}
