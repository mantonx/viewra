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

// TrendingProviderResponse represents a trending provider in API responses.
type TrendingProviderResponse struct {
	PluginID    string   `json:"plugin_id"`
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Windows     []string `json:"windows"`
	MediaTypes  []string `json:"media_types"`
	UpdateFreq  string   `json:"update_freq"`
	Enabled     bool     `json:"enabled"`
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
// @Success 200 {array} TrendingProviderResponse
// @Router /api/trending/providers [get]
func (h *TrendingHandler) GetProviders(c *gin.Context) {
	providers := h.trendingService.GetProviders()

	result := make([]TrendingProviderResponse, 0, len(providers))
	for _, p := range providers {
		result = append(result, TrendingProviderResponse{
			PluginID:    p.PluginID,
			ID:          p.Info.ID,
			Name:        p.Info.Name,
			Description: p.Info.Description,
			Windows:     p.Info.Windows,
			MediaTypes:  p.Info.MediaTypes,
			UpdateFreq:  p.Info.UpdateFreq,
			Enabled:     p.Enabled,
		})
	}

	c.JSON(http.StatusOK, result)
}
