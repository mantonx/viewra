package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/plugins"
)

// MarketplaceHandler handles marketplace HTTP requests.
type MarketplaceHandler struct {
	marketplace *plugins.MarketplaceService
}

// NewMarketplaceHandler creates a new marketplace handler.
func NewMarketplaceHandler(marketplace *plugins.MarketplaceService) *MarketplaceHandler {
	return &MarketplaceHandler{marketplace: marketplace}
}

// --- Response types for OpenAPI documentation ---

// MarketplaceCatalogResponse is the response for GET /api/plugins/marketplace.
type MarketplaceCatalogResponse = plugins.MarketplaceCatalog

// MarketplacePluginResponse is the response for GET /api/plugins/marketplace/:id.
type MarketplacePluginResponse = plugins.MarketplacePlugin

// InstallPluginRequest is the request body for POST /api/plugins/marketplace/install.
type InstallPluginRequest = plugins.InstallRequest

// UpdatesCheckResponse is the response for GET /api/plugins/updates.
type UpdatesCheckResponse = plugins.UpdatesResponse

// --- Handlers ---

// ListAvailable handles GET /api/plugins/marketplace
// @Summary List available plugins
// @Description Returns all plugins available in the marketplace
// @Tags marketplace
// @Security BearerAuth
// @Produce json
// @Success 200 {object} MarketplaceCatalogResponse
// @Failure 401 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/plugins/marketplace [get]
func (h *MarketplaceHandler) ListAvailable(c *gin.Context) {
	catalog, err := h.marketplace.ListAvailable(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, catalog)
}

// GetDetails handles GET /api/plugins/marketplace/:id
// @Summary Get marketplace plugin details
// @Description Returns details for a specific plugin in the marketplace
// @Tags marketplace
// @Security BearerAuth
// @Produce json
// @Param id path string true "Plugin ID"
// @Success 200 {object} MarketplacePluginResponse
// @Failure 401 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/plugins/marketplace/{id} [get]
func (h *MarketplaceHandler) GetDetails(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "Plugin ID required")
		return
	}

	plugin, err := h.marketplace.GetDetails(c.Request.Context(), id)
	if err != nil {
		handleMarketplaceError(c, err)
		return
	}

	c.JSON(http.StatusOK, plugin)
}

// RefreshCatalog handles POST /api/plugins/marketplace/refresh
// @Summary Refresh marketplace catalog
// @Description Forces a refresh of the cached plugin catalog
// @Tags marketplace
// @Security BearerAuth
// @Produce json
// @Success 200 {object} SuccessResponse
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/plugins/marketplace/refresh [post]
func (h *MarketplaceHandler) RefreshCatalog(c *gin.Context) {
	if err := h.marketplace.RefreshCatalog(c.Request.Context()); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Install handles POST /api/plugins/marketplace/install
// @Summary Install a plugin
// @Description Downloads and installs a plugin from the marketplace. Returns SSE stream with progress updates.
// @Tags marketplace
// @Security BearerAuth
// @Accept json
// @Produce text/event-stream
// @Param body body InstallPluginRequest true "Installation request"
// @Success 200 {object} plugins.InstallProgress
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 409 {object} APIError "Plugin already installed"
// @Failure 500 {object} APIError
// @Router /api/plugins/marketplace/install [post]
func (h *MarketplaceHandler) Install(c *gin.Context) {
	var req plugins.InstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body")
		return
	}

	if req.PluginID == "" {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "plugin_id is required")
		return
	}

	// Set up SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // Disable nginx buffering

	progress := make(chan plugins.InstallProgress, 10)
	done := make(chan error, 1)

	// Run installation in background
	go func() {
		done <- h.marketplace.Install(c.Request.Context(), req, progress)
		close(progress)
	}()

	// Stream progress updates
	c.Stream(func(w io.Writer) bool {
		select {
		case p, ok := <-progress:
			if !ok {
				return false
			}
			data, _ := json.Marshal(p)
			fmt.Fprintf(c.Writer, "event: progress\ndata: %s\n\n", data)
			c.Writer.Flush()
			return true
		case err := <-done:
			if err != nil {
				errResp := plugins.InstallProgress{
					PluginID: req.PluginID,
					Status:   plugins.InstallStatusFailed,
					Error:    err.Error(),
				}
				data, _ := json.Marshal(errResp)
				fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", data)
				c.Writer.Flush()
			}
			return false
		case <-c.Request.Context().Done():
			return false
		case <-time.After(30 * time.Second):
			// Heartbeat to keep connection alive
			fmt.Fprintf(c.Writer, "event: ping\ndata: \n\n")
			c.Writer.Flush()
			return true
		}
	})
}

// Update handles POST /api/plugins/:id/update
// @Summary Update a plugin
// @Description Updates an installed plugin to the latest version. Returns SSE stream with progress updates.
// @Tags marketplace
// @Security BearerAuth
// @Produce text/event-stream
// @Param id path string true "Plugin ID"
// @Success 200 {object} plugins.InstallProgress
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/plugins/{id}/update [post]
func (h *MarketplaceHandler) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "Plugin ID required")
		return
	}

	// Set up SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	progress := make(chan plugins.InstallProgress, 10)
	done := make(chan error, 1)

	// Run update in background
	go func() {
		done <- h.marketplace.Update(c.Request.Context(), id, progress)
		close(progress)
	}()

	// Stream progress updates
	c.Stream(func(w io.Writer) bool {
		select {
		case p, ok := <-progress:
			if !ok {
				return false
			}
			data, _ := json.Marshal(p)
			fmt.Fprintf(c.Writer, "event: progress\ndata: %s\n\n", data)
			c.Writer.Flush()
			return true
		case err := <-done:
			if err != nil {
				errResp := plugins.InstallProgress{
					PluginID: id,
					Status:   plugins.InstallStatusFailed,
					Error:    err.Error(),
				}
				data, _ := json.Marshal(errResp)
				fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n", data)
				c.Writer.Flush()
			}
			return false
		case <-c.Request.Context().Done():
			return false
		case <-time.After(30 * time.Second):
			fmt.Fprintf(c.Writer, "event: ping\ndata: \n\n")
			c.Writer.Flush()
			return true
		}
	})
}

// Uninstall handles DELETE /api/plugins/:id
// @Summary Uninstall a plugin
// @Description Removes a plugin. Built-in plugins cannot be uninstalled.
// @Tags marketplace
// @Security BearerAuth
// @Produce json
// @Param id path string true "Plugin ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} APIError "Cannot uninstall built-in plugin"
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/plugins/{id} [delete]
func (h *MarketplaceHandler) Uninstall(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		respondError(c, http.StatusBadRequest, "BAD_REQUEST", "Plugin ID required")
		return
	}

	if err := h.marketplace.Uninstall(c.Request.Context(), id); err != nil {
		handleMarketplaceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// CheckUpdates handles GET /api/plugins/updates
// @Summary Check for plugin updates
// @Description Returns a list of installed plugins that have updates available
// @Tags marketplace
// @Security BearerAuth
// @Produce json
// @Success 200 {object} UpdatesCheckResponse
// @Failure 401 {object} APIError
// @Failure 500 {object} APIError
// @Router /api/plugins/updates [get]
func (h *MarketplaceHandler) CheckUpdates(c *gin.Context) {
	updates, err := h.marketplace.CheckUpdates(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, updates)
}

// handleMarketplaceError converts marketplace errors to HTTP responses.
func handleMarketplaceError(c *gin.Context, err error) {
	requestID := getRequestID(c)
	timestamp := time.Now()

	var notFound plugins.ErrPluginNotFound
	if errors.As(err, &notFound) {
		c.JSON(http.StatusNotFound, APIError{
			Code:      "PLUGIN_NOT_FOUND",
			Message:   "Plugin not found: " + notFound.PluginID,
			RequestID: requestID,
			Timestamp: timestamp,
		})
		return
	}

	if errors.Is(err, plugins.ErrPluginNotInMarketplace) {
		c.JSON(http.StatusNotFound, APIError{
			Code:      "NOT_IN_MARKETPLACE",
			Message:   "Plugin not found in marketplace",
			RequestID: requestID,
			Timestamp: timestamp,
		})
		return
	}

	if errors.Is(err, plugins.ErrPluginAlreadyInstalled) {
		c.JSON(http.StatusConflict, APIError{
			Code:      "ALREADY_INSTALLED",
			Message:   "Plugin is already installed",
			RequestID: requestID,
			Timestamp: timestamp,
		})
		return
	}

	if errors.Is(err, plugins.ErrCannotUninstallBuiltin) {
		c.JSON(http.StatusBadRequest, APIError{
			Code:      "BUILTIN_PLUGIN",
			Message:   "Cannot uninstall built-in plugins",
			RequestID: requestID,
			Timestamp: timestamp,
		})
		return
	}

	if errors.Is(err, plugins.ErrNoCompatibleAsset) {
		c.JSON(http.StatusBadRequest, APIError{
			Code:      "NO_COMPATIBLE_ASSET",
			Message:   "No compatible binary for this platform",
			RequestID: requestID,
			Timestamp: timestamp,
		})
		return
	}

	if errors.Is(err, plugins.ErrChecksumMismatch) {
		c.JSON(http.StatusBadRequest, APIError{
			Code:      "CHECKSUM_MISMATCH",
			Message:   "Download verification failed - checksum mismatch",
			RequestID: requestID,
			Timestamp: timestamp,
		})
		return
	}

	handleError(c, err)
}
