package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/plugins"
)

// PluginHandler handles plugin management HTTP requests.
type PluginHandler struct {
	service *plugins.Service
}

// NewPluginHandler creates a new plugin handler.
func NewPluginHandler(service *plugins.Service) *PluginHandler {
	return &PluginHandler{service: service}
}

// --- Response types for OpenAPI documentation ---

// PluginListResponse is the response for GET /api/plugins.
type PluginListResponse struct {
	Plugins []plugins.PluginSummary `json:"plugins"`
}

// UpdatePluginSettingsRequest is the request body for PUT /api/plugins/:id/settings.
type UpdatePluginSettingsRequest struct {
	Values json.RawMessage `json:"values" binding:"required"`
}

// PluginLogsResponse is the response for GET /api/plugins/:id/logs.
type PluginLogsResponse struct {
	PluginID string               `json:"plugin_id"`
	Logs     []plugins.LogEntry   `json:"logs"`
}

// --- Handlers ---

// List handles GET /api/plugins
// @Summary List all plugins
// @Description Returns all installed plugins with their status
// @Tags plugins
// @Security BearerAuth
// @Produce json
// @Success 200 {object} PluginListResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/plugins [get]
func (h *PluginHandler) List(c *gin.Context) {
	list, err := h.service.List(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, PluginListResponse{Plugins: list})
}

// Get handles GET /api/plugins/:id
// @Summary Get plugin details
// @Description Returns detailed information about a specific plugin
// @Tags plugins
// @Security BearerAuth
// @Produce json
// @Param id path string true "Plugin ID"
// @Success 200 {object} plugins.PluginDetail
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/plugins/{id} [get]
func (h *PluginHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Plugin ID required"})
		return
	}

	detail, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		handlePluginError(c, err)
		return
	}

	c.JSON(http.StatusOK, detail)
}

// GetSettings handles GET /api/plugins/:id/settings
// @Summary Get plugin settings
// @Description Returns the settings schema and current values for a plugin
// @Tags plugins
// @Security BearerAuth
// @Produce json
// @Param id path string true "Plugin ID"
// @Success 200 {object} plugins.PluginSettings
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/plugins/{id}/settings [get]
func (h *PluginHandler) GetSettings(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Plugin ID required"})
		return
	}

	settings, err := h.service.GetSettings(c.Request.Context(), id)
	if err != nil {
		handlePluginError(c, err)
		return
	}

	c.JSON(http.StatusOK, settings)
}

// UpdateSettings handles PUT /api/plugins/:id/settings
// @Summary Update plugin settings
// @Description Updates the settings for a plugin (admin only)
// @Tags plugins
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Plugin ID"
// @Param body body UpdatePluginSettingsRequest true "Settings values"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/plugins/{id}/settings [put]
func (h *PluginHandler) UpdateSettings(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Plugin ID required"})
		return
	}

	var req UpdatePluginSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}

	if err := h.service.UpdateSettings(c.Request.Context(), id, req.Values); err != nil {
		handlePluginError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Enable handles POST /api/plugins/:id/enable
// @Summary Enable plugin
// @Description Enables a plugin (admin only)
// @Tags plugins
// @Security BearerAuth
// @Produce json
// @Param id path string true "Plugin ID"
// @Success 200 {object} SuccessResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/plugins/{id}/enable [post]
func (h *PluginHandler) Enable(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Plugin ID required"})
		return
	}

	if err := h.service.Enable(c.Request.Context(), id); err != nil {
		handlePluginError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Disable handles POST /api/plugins/:id/disable
// @Summary Disable plugin
// @Description Disables a plugin (admin only). Cannot disable built-in plugins.
// @Tags plugins
// @Security BearerAuth
// @Produce json
// @Param id path string true "Plugin ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse "Cannot disable built-in plugin"
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/plugins/{id}/disable [post]
func (h *PluginHandler) Disable(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Plugin ID required"})
		return
	}

	if err := h.service.Disable(c.Request.Context(), id); err != nil {
		handlePluginError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Restart handles POST /api/plugins/:id/restart
// @Summary Restart plugin
// @Description Restarts a plugin process (admin only)
// @Tags plugins
// @Security BearerAuth
// @Produce json
// @Param id path string true "Plugin ID"
// @Success 200 {object} SuccessResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/plugins/{id}/restart [post]
func (h *PluginHandler) Restart(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Plugin ID required"})
		return
	}

	if err := h.service.Restart(c.Request.Context(), id); err != nil {
		handlePluginError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetHealth handles GET /api/plugins/:id/health
// @Summary Get plugin health
// @Description Returns detailed health information for a plugin
// @Tags plugins
// @Security BearerAuth
// @Produce json
// @Param id path string true "Plugin ID"
// @Success 200 {object} plugins.PluginHealthDetail
// @Failure 401 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/plugins/{id}/health [get]
func (h *PluginHandler) GetHealth(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Plugin ID required"})
		return
	}

	health, err := h.service.GetHealth(c.Request.Context(), id)
	if err != nil {
		handlePluginError(c, err)
		return
	}

	c.JSON(http.StatusOK, health)
}

// GetLogs handles GET /api/plugins/:id/logs
// @Summary Get plugin logs
// @Description Returns recent log entries for a plugin (admin only)
// @Tags plugins
// @Security BearerAuth
// @Produce json
// @Param id path string true "Plugin ID"
// @Param limit query int false "Max entries to return" default(100)
// @Param level query string false "Filter by log level (error, warn, info, debug)"
// @Param since query string false "Only entries after this RFC3339 timestamp"
// @Success 200 {object} PluginLogsResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/plugins/{id}/logs [get]
func (h *PluginHandler) GetLogs(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Plugin ID required"})
		return
	}

	opts := plugins.LogOptions{
		Limit: 100,
		Level: c.Query("level"),
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		var limit int
		if _, err := json.Number(limitStr).Int64(); err == nil {
			limit = int(mustParseInt(limitStr))
			if limit > 0 && limit <= 1000 {
				opts.Limit = limit
			}
		}
	}

	if sinceStr := c.Query("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			opts.Since = t
		}
	}

	logs, err := h.service.GetLogs(c.Request.Context(), id, opts)
	if err != nil {
		handlePluginError(c, err)
		return
	}

	c.JSON(http.StatusOK, PluginLogsResponse{
		PluginID: id,
		Logs:     logs,
	})
}

// handlePluginError converts plugin errors to HTTP responses.
func handlePluginError(c *gin.Context, err error) {
	var notFound plugins.ErrPluginNotFound
	if errors.As(err, &notFound) {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		return
	}

	var builtin plugins.ErrBuiltinPlugin
	if errors.As(err, &builtin) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	handleError(c, err)
}

// mustParseInt parses a string to int64, returning 0 on error.
func mustParseInt(s string) int64 {
	var n int64
	json.Unmarshal([]byte(s), &n)
	return n
}
