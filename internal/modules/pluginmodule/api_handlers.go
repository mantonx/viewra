package pluginmodule

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"
	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// APIResponse provides a standardized response format for all plugin API endpoints
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Message   string      `json:"message,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	RequestID string      `json:"request_id,omitempty"`
}

// PaginatedResponse extends APIResponse with pagination metadata
type PaginatedResponse struct {
	APIResponse
	Pagination *PaginationMeta `json:"pagination,omitempty"`
}

// PaginationMeta provides pagination information
type PaginationMeta struct {
	Page        int   `json:"page"`
	Limit       int   `json:"limit"`
	Total       int64 `json:"total"`
	TotalPages  int   `json:"total_pages"`
	HasNext     bool  `json:"has_next"`
	HasPrevious bool  `json:"has_previous"`
}

// PluginAPIHandlers provides comprehensive plugin API endpoints
type PluginAPIHandlers struct {
	pluginModule *PluginModule
	db           *gorm.DB
	logger       hclog.Logger
}

// NewPluginAPIHandlers creates a new plugin API handlers instance
func NewPluginAPIHandlers(pm *PluginModule, db *gorm.DB, logger hclog.Logger) *PluginAPIHandlers {
	return &PluginAPIHandlers{
		pluginModule: pm,
		db:           db,
		logger:       logger.Named("plugin-api"),
	}
}

// RegisterRoutes registers all plugin API routes with consistent structure
func (h *PluginAPIHandlers) RegisterRoutes(router *gin.Engine) {
	h.logger.Info("registering comprehensive plugin API routes")

	// Main plugin management API group
	pluginAPI := router.Group("/api/v1/plugins")
	{
		// Plugin Discovery & Listing
		pluginAPI.GET("/", h.handleListAllPlugins)
		pluginAPI.GET("/search", h.handleSearchPlugins)
		pluginAPI.GET("/categories", h.handleGetPluginCategories)
		pluginAPI.GET("/capabilities", h.handleGetSystemCapabilities)

		// Individual Plugin Management
		pluginAPI.GET("/:id", h.handleGetPlugin)
		pluginAPI.PUT("/:id", h.handleUpdatePlugin)
		pluginAPI.DELETE("/:id", h.handleUninstallPlugin)

		// Plugin Lifecycle
		pluginAPI.POST("/:id/enable", h.handleEnablePlugin)
		pluginAPI.POST("/:id/disable", h.handleDisablePlugin)
		pluginAPI.POST("/:id/restart", h.handleRestartPlugin)
		pluginAPI.POST("/:id/reload", h.handleReloadPlugin)

		// Plugin Health & Monitoring
		pluginAPI.GET("/:id/health", h.handleGetPluginHealth)
		pluginAPI.GET("/:id/metrics", h.handleGetPluginMetrics)
		pluginAPI.GET("/:id/logs", h.handleGetPluginLogs)
		pluginAPI.POST("/:id/health/reset", h.handleResetPluginHealth)

		// Plugin Configuration
		pluginAPI.GET("/:id/config", h.handleGetPluginConfig)
		pluginAPI.PUT("/:id/config", h.handleUpdatePluginConfig)
		pluginAPI.GET("/:id/config/schema", h.handleGetPluginConfigSchema)
		pluginAPI.POST("/:id/config/validate", h.handleValidatePluginConfig)
		pluginAPI.POST("/:id/config/reset", h.handleResetPluginConfig)

		// Plugin Events & History
		pluginAPI.GET("/:id/events", h.handleGetPluginEvents)
		pluginAPI.GET("/:id/history", h.handleGetPluginHistory)
		pluginAPI.DELETE("/:id/events", h.handleClearPluginEvents)

		// Plugin Admin Pages & UI
		pluginAPI.GET("/:id/admin-pages", h.handleGetPluginAdminPages)
		pluginAPI.GET("/:id/admin/:pageId/status", h.handleGetAdminPageStatus)
		pluginAPI.POST("/:id/admin/:pageId/actions/:actionId", h.handleExecuteAdminPageAction)
		pluginAPI.GET("/:id/ui-components", h.handleGetPluginUIComponents)
		pluginAPI.GET("/:id/assets", h.handleGetPluginAssets)

		// Plugin Dependencies & Requirements
		pluginAPI.GET("/:id/dependencies", h.handleGetPluginDependencies)
		pluginAPI.GET("/:id/dependents", h.handleGetPluginDependents)
		pluginAPI.POST("/:id/validate-dependencies", h.handleValidateDependencies)

		// Plugin Testing & Validation
		pluginAPI.POST("/:id/test", h.handleTestPlugin)
		pluginAPI.GET("/:id/test-results", h.handleGetTestResults)
		pluginAPI.POST("/:id/validate", h.handleValidatePlugin)
	}

	// Core Plugins API group
	coreAPI := router.Group("/api/v1/plugins/core")
	{
		coreAPI.GET("/", h.handleListCorePlugins)
		coreAPI.GET("/:name", h.handleGetCorePlugin)
		coreAPI.POST("/:name/enable", h.handleEnableCorePlugin)
		coreAPI.POST("/:name/disable", h.handleDisableCorePlugin)
		coreAPI.GET("/:name/config", h.handleGetCorePluginConfig)
		coreAPI.PUT("/:name/config", h.handleUpdateCorePluginConfig)
	}

	// External Plugins API group
	externalAPI := router.Group("/api/v1/plugins/external")
	{
		externalAPI.GET("/", h.handleListExternalPlugins)
		externalAPI.POST("/", h.handleInstallPlugin)
		externalAPI.POST("/refresh", h.handleRefreshExternalPlugins)
		externalAPI.GET("/:id", h.handleGetExternalPlugin)
		externalAPI.POST("/:id/load", h.handleLoadExternalPlugin)
		externalAPI.POST("/:id/unload", h.handleUnloadExternalPlugin)
		externalAPI.GET("/:id/manifest", h.handleGetPluginManifest)
	}

	// Plugin System Management
	systemAPI := router.Group("/api/v1/plugins/system")
	{
		systemAPI.GET("/status", h.handleGetSystemStatus)
		systemAPI.GET("/stats", h.handleGetSystemStats)
		systemAPI.POST("/refresh", h.handleRefreshAllPlugins)
		systemAPI.POST("/cleanup", h.handleCleanupSystem)

		// Hot Reload Management
		systemAPI.GET("/hot-reload", h.handleGetHotReloadStatus)
		systemAPI.POST("/hot-reload/enable", h.handleEnableHotReload)
		systemAPI.POST("/hot-reload/disable", h.handleDisableHotReload)
		systemAPI.POST("/hot-reload/trigger/:id", h.handleTriggerHotReload)

		// Bulk Operations
		systemAPI.POST("/bulk/enable", h.handleBulkEnable)
		systemAPI.POST("/bulk/disable", h.handleBulkDisable)
		systemAPI.POST("/bulk/update", h.handleBulkUpdate)
	}

	// Transcoding Session Management
	transcodingAPI := router.Group("/api/v1/transcoding")
	{
		transcodingAPI.POST("/sessions/:sessionId/:action", h.handleSessionAction)
		transcodingAPI.POST("/engine/:action", h.handleEngineAction)
	}

	// Admin Panel Integration
	adminAPI := router.Group("/api/v1/plugins/admin")
	{
		adminAPI.GET("/pages", h.handleGetAllAdminPages)
		adminAPI.GET("/navigation", h.handleGetAdminNavigation)
		adminAPI.GET("/permissions", h.handleGetPluginPermissions)
		adminAPI.PUT("/permissions", h.handleUpdatePluginPermissions)
		adminAPI.GET("/settings", h.handleGetGlobalPluginSettings)
		adminAPI.PUT("/settings", h.handleUpdateGlobalPluginSettings)
	}

	h.logger.Info("plugin API routes registered successfully")
}

// Helper methods for consistent responses

func (h *PluginAPIHandlers) successResponse(c *gin.Context, data interface{}, message string) {
	c.JSON(http.StatusOK, APIResponse{
		Success:   true,
		Data:      data,
		Message:   message,
		Timestamp: time.Now(),
		RequestID: h.getRequestID(c),
	})
}

func (h *PluginAPIHandlers) errorResponse(c *gin.Context, statusCode int, err error, message string) {
	response := APIResponse{
		Success:   false,
		Error:     err.Error(),
		Message:   message,
		Timestamp: time.Now(),
		RequestID: h.getRequestID(c),
	}

	h.logger.Error("API error", "status", statusCode, "error", err, "message", message)
	c.JSON(statusCode, response)
}

func (h *PluginAPIHandlers) paginatedResponse(c *gin.Context, data interface{}, pagination *PaginationMeta, message string) {
	c.JSON(http.StatusOK, PaginatedResponse{
		APIResponse: APIResponse{
			Success:   true,
			Data:      data,
			Message:   message,
			Timestamp: time.Now(),
			RequestID: h.getRequestID(c),
		},
		Pagination: pagination,
	})
}

func (h *PluginAPIHandlers) getRequestID(c *gin.Context) string {
	if id := c.GetHeader("X-Request-ID"); id != "" {
		return id
	}
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

func (h *PluginAPIHandlers) parsePagination(c *gin.Context) (page int, limit int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	return page, limit
}

func (h *PluginAPIHandlers) createPaginationMeta(page, limit int, total int64) *PaginationMeta {
	totalPages := int((total + int64(limit) - 1) / int64(limit))
	return &PaginationMeta{
		Page:        page,
		Limit:       limit,
		Total:       total,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
	}
}

// Plugin Discovery & Listing Handlers

func (h *PluginAPIHandlers) handleListAllPlugins(c *gin.Context) {
	page, limit := h.parsePagination(c)
	category := c.Query("category")
	status := c.Query("status")
	pluginType := c.Query("type")

	if h.pluginModule == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("plugin module not initialized"), "Plugin module unavailable")
		return
	}

	// Get all plugins
	allPlugins := h.pluginModule.ListAllPlugins()

	// Apply filters
	filteredPlugins := h.filterPlugins(allPlugins, category, status, pluginType)

	// Apply pagination
	total := int64(len(filteredPlugins))
	start := (page - 1) * limit
	end := start + limit

	if start > len(filteredPlugins) {
		start = len(filteredPlugins)
	}
	if end > len(filteredPlugins) {
		end = len(filteredPlugins)
	}

	paginatedPlugins := filteredPlugins[start:end]

	// Enhance plugins with additional info
	enhancedPlugins := make([]EnhancedPluginInfo, len(paginatedPlugins))
	for i, plugin := range paginatedPlugins {
		enhancedPlugins[i] = h.enhancePluginInfo(plugin)
	}

	pagination := h.createPaginationMeta(page, limit, total)
	h.paginatedResponse(c, enhancedPlugins, pagination, "Plugins retrieved successfully")
}

func (h *PluginAPIHandlers) handleSearchPlugins(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		h.errorResponse(c, http.StatusBadRequest,
			fmt.Errorf("search query required"), "Search query parameter 'q' is required")
		return
	}

	page, limit := h.parsePagination(c)

	if h.pluginModule == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("plugin module not initialized"), "Plugin module unavailable")
		return
	}

	// Search plugins
	allPlugins := h.pluginModule.ListAllPlugins()
	searchResults := h.searchPlugins(allPlugins, query)

	// Apply pagination
	total := int64(len(searchResults))
	start := (page - 1) * limit
	end := start + limit

	if start > len(searchResults) {
		start = len(searchResults)
	}
	if end > len(searchResults) {
		end = len(searchResults)
	}

	paginatedResults := searchResults[start:end]

	// Enhance search results
	enhancedResults := make([]EnhancedPluginInfo, len(paginatedResults))
	for i, plugin := range paginatedResults {
		enhancedResults[i] = h.enhancePluginInfo(plugin)
	}

	pagination := h.createPaginationMeta(page, limit, total)
	h.paginatedResponse(c, enhancedResults, pagination,
		fmt.Sprintf("Found %d plugins matching '%s'", total, query))
}

func (h *PluginAPIHandlers) handleGetPluginCategories(c *gin.Context) {
	if h.pluginModule == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("plugin module not initialized"), "Plugin module unavailable")
		return
	}

	categories := h.getPluginCategories()
	h.successResponse(c, categories, "Plugin categories retrieved successfully")
}

func (h *PluginAPIHandlers) handleGetSystemCapabilities(c *gin.Context) {
	capabilities := h.getSystemCapabilities()
	h.successResponse(c, capabilities, "System capabilities retrieved successfully")
}

// Individual Plugin Management Handlers

func (h *PluginAPIHandlers) handleGetPlugin(c *gin.Context) {
	pluginID := c.Param("id")
	if pluginID == "" {
		h.errorResponse(c, http.StatusBadRequest,
			fmt.Errorf("plugin ID required"), "Plugin ID parameter is required")
		return
	}

	if h.pluginModule == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("plugin module not initialized"), "Plugin module unavailable")
		return
	}

	// Try to find the plugin
	pluginInfo := h.findPluginByID(pluginID)
	if pluginInfo == nil {
		h.errorResponse(c, http.StatusNotFound,
			fmt.Errorf("plugin not found"), fmt.Sprintf("Plugin '%s' not found", pluginID))
		return
	}

	enhancedInfo := h.enhancePluginInfo(*pluginInfo)
	h.successResponse(c, enhancedInfo, "Plugin information retrieved successfully")
}

func (h *PluginAPIHandlers) handleUpdatePlugin(c *gin.Context) {
	pluginID := c.Param("id")
	if pluginID == "" {
		h.errorResponse(c, http.StatusBadRequest,
			fmt.Errorf("plugin ID required"), "Plugin ID parameter is required")
		return
	}

	var updateReq PluginUpdateRequest
	if err := c.ShouldBindJSON(&updateReq); err != nil {
		h.errorResponse(c, http.StatusBadRequest, err, "Invalid update request format")
		return
	}

	// TODO: Implement plugin update logic
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("plugin updates not yet implemented"), "Plugin update functionality coming soon")
}

func (h *PluginAPIHandlers) handleUninstallPlugin(c *gin.Context) {
	pluginID := c.Param("id")
	if pluginID == "" {
		h.errorResponse(c, http.StatusBadRequest,
			fmt.Errorf("plugin ID required"), "Plugin ID parameter is required")
		return
	}

	// TODO: Implement plugin uninstallation logic
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("plugin uninstallation not yet implemented"), "Plugin uninstallation functionality coming soon")
}

// Plugin Lifecycle Handlers

func (h *PluginAPIHandlers) handleEnablePlugin(c *gin.Context) {
	pluginID := c.Param("id")
	if pluginID == "" {
		h.errorResponse(c, http.StatusBadRequest,
			fmt.Errorf("plugin ID required"), "Plugin ID parameter is required")
		return
	}

	if h.pluginModule == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("plugin module not initialized"), "Plugin module unavailable")
		return
	}

	// Try enabling as core plugin first
	if err := h.pluginModule.EnableCorePlugin(pluginID); err == nil {
		h.successResponse(c, gin.H{"plugin_id": pluginID, "type": "core"},
			"Core plugin enabled successfully")
		return
	}

	// Try loading as external plugin
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := h.pluginModule.LoadExternalPlugin(ctx, pluginID); err != nil {
		h.errorResponse(c, http.StatusInternalServerError, err,
			"Failed to enable plugin")
		return
	}

	h.successResponse(c, gin.H{"plugin_id": pluginID, "type": "external"},
		"External plugin enabled successfully")
}

func (h *PluginAPIHandlers) handleDisablePlugin(c *gin.Context) {
	pluginID := c.Param("id")
	if pluginID == "" {
		h.errorResponse(c, http.StatusBadRequest,
			fmt.Errorf("plugin ID required"), "Plugin ID parameter is required")
		return
	}

	if h.pluginModule == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("plugin module not initialized"), "Plugin module unavailable")
		return
	}

	// Try disabling as core plugin first
	if err := h.pluginModule.DisableCorePlugin(pluginID); err == nil {
		h.successResponse(c, gin.H{"plugin_id": pluginID, "type": "core"},
			"Core plugin disabled successfully")
		return
	}

	// Try unloading as external plugin
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := h.pluginModule.UnloadExternalPlugin(ctx, pluginID); err != nil {
		h.errorResponse(c, http.StatusInternalServerError, err,
			"Failed to disable plugin")
		return
	}

	h.successResponse(c, gin.H{"plugin_id": pluginID, "type": "external"},
		"External plugin disabled successfully")
}

func (h *PluginAPIHandlers) handleRestartPlugin(c *gin.Context) {
	pluginID := c.Param("id")
	if pluginID == "" {
		h.errorResponse(c, http.StatusBadRequest,
			fmt.Errorf("plugin ID required"), "Plugin ID parameter is required")
		return
	}

	if h.pluginModule == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("plugin module not initialized"), "Plugin module unavailable")
		return
	}

	// For external plugins, restart = unload + load
	if _, exists := h.pluginModule.GetExternalPlugin(pluginID); exists {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Unload first
		if err := h.pluginModule.UnloadExternalPlugin(ctx, pluginID); err != nil {
			h.errorResponse(c, http.StatusInternalServerError, err,
				"Failed to unload plugin during restart")
			return
		}

		// Wait a moment for cleanup
		time.Sleep(time.Second)

		// Load again
		if err := h.pluginModule.LoadExternalPlugin(ctx, pluginID); err != nil {
			h.errorResponse(c, http.StatusInternalServerError, err,
				"Failed to load plugin during restart")
			return
		}

		h.successResponse(c, gin.H{"plugin_id": pluginID},
			"External plugin restarted successfully")
		return
	}

	// For core plugins, restart = disable + enable
	if _, exists := h.pluginModule.GetCorePlugin(pluginID); exists {
		if err := h.pluginModule.DisableCorePlugin(pluginID); err != nil {
			h.errorResponse(c, http.StatusInternalServerError, err,
				"Failed to disable core plugin during restart")
			return
		}

		time.Sleep(time.Millisecond * 500)

		if err := h.pluginModule.EnableCorePlugin(pluginID); err != nil {
			h.errorResponse(c, http.StatusInternalServerError, err,
				"Failed to enable core plugin during restart")
			return
		}

		h.successResponse(c, gin.H{"plugin_id": pluginID},
			"Core plugin restarted successfully")
		return
	}

	h.errorResponse(c, http.StatusNotFound,
		fmt.Errorf("plugin not found"), fmt.Sprintf("Plugin '%s' not found", pluginID))
}

func (h *PluginAPIHandlers) handleReloadPlugin(c *gin.Context) {
	pluginID := c.Param("id")
	if pluginID == "" {
		h.errorResponse(c, http.StatusBadRequest,
			fmt.Errorf("plugin ID required"), "Plugin ID parameter is required")
		return
	}

	if h.pluginModule == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("plugin module not initialized"), "Plugin module unavailable")
		return
	}

	// Trigger hot reload if available
	if err := h.pluginModule.TriggerPluginReload(pluginID); err != nil {
		h.errorResponse(c, http.StatusInternalServerError, err,
			"Failed to trigger plugin reload")
		return
	}

	h.successResponse(c, gin.H{"plugin_id": pluginID},
		"Plugin reload triggered successfully")
}

// Placeholder handlers - to be implemented
func (h *PluginAPIHandlers) handleGetPluginHealth(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin health endpoint coming soon")
}

func (h *PluginAPIHandlers) handleGetPluginMetrics(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin metrics endpoint coming soon")
}

func (h *PluginAPIHandlers) handleGetPluginLogs(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin logs endpoint coming soon")
}

func (h *PluginAPIHandlers) handleResetPluginHealth(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin health reset endpoint coming soon")
}

func (h *PluginAPIHandlers) handleGetPluginConfig(c *gin.Context) {
	pluginID := c.Param("id")
	if pluginID == "" {
		h.errorResponse(c, http.StatusBadRequest,
			fmt.Errorf("plugin ID required"), "Plugin ID parameter is required")
		return
	}

	if h.pluginModule == nil || h.pluginModule.configManager == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("configuration manager not available"), "Configuration manager unavailable")
		return
	}

	config, err := h.pluginModule.configManager.GetPluginConfiguration(pluginID)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, err,
			"Failed to retrieve plugin configuration")
		return
	}

	h.successResponse(c, config, "Plugin configuration retrieved successfully")
}

func (h *PluginAPIHandlers) handleUpdatePluginConfig(c *gin.Context) {
	pluginID := c.Param("id")
	if pluginID == "" {
		h.errorResponse(c, http.StatusBadRequest,
			fmt.Errorf("plugin ID required"), "Plugin ID parameter is required")
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		h.errorResponse(c, http.StatusBadRequest, err, "Invalid configuration update format")
		return
	}

	if h.pluginModule == nil || h.pluginModule.configManager == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("configuration manager not available"), "Configuration manager unavailable")
		return
	}

	// Get user information for audit trail (would be from auth middleware)
	modifiedBy := "api-user" // TODO: Extract from authentication context

	config, err := h.pluginModule.configManager.UpdatePluginConfiguration(pluginID, updates, modifiedBy)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, err,
			"Failed to update plugin configuration")
		return
	}

	h.successResponse(c, config, "Plugin configuration updated successfully")
}

func (h *PluginAPIHandlers) handleGetPluginConfigSchema(c *gin.Context) {
	pluginID := c.Param("id")
	if pluginID == "" {
		h.errorResponse(c, http.StatusBadRequest,
			fmt.Errorf("plugin ID required"), "Plugin ID parameter is required")
		return
	}

	if h.pluginModule == nil || h.pluginModule.configManager == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("configuration manager not available"), "Configuration manager unavailable")
		return
	}

	schema, err := h.pluginModule.configManager.GetConfigurationSchema(pluginID)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, err,
			"Failed to retrieve plugin configuration schema")
		return
	}

	h.successResponse(c, schema, "Plugin configuration schema retrieved successfully")
}

func (h *PluginAPIHandlers) handleValidatePluginConfig(c *gin.Context) {
	pluginID := c.Param("id")
	if pluginID == "" {
		h.errorResponse(c, http.StatusBadRequest,
			fmt.Errorf("plugin ID required"), "Plugin ID parameter is required")
		return
	}

	var settings map[string]interface{}
	if err := c.ShouldBindJSON(&settings); err != nil {
		h.errorResponse(c, http.StatusBadRequest, err, "Invalid configuration format")
		return
	}

	if h.pluginModule == nil || h.pluginModule.configManager == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("configuration manager not available"), "Configuration manager unavailable")
		return
	}

	result, err := h.pluginModule.configManager.ValidateConfiguration(pluginID, settings)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, err,
			"Failed to validate plugin configuration")
		return
	}

	h.successResponse(c, result, "Plugin configuration validation completed")
}

func (h *PluginAPIHandlers) handleResetPluginConfig(c *gin.Context) {
	pluginID := c.Param("id")
	if pluginID == "" {
		h.errorResponse(c, http.StatusBadRequest,
			fmt.Errorf("plugin ID required"), "Plugin ID parameter is required")
		return
	}

	if h.pluginModule == nil || h.pluginModule.configManager == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("configuration manager not available"), "Configuration manager unavailable")
		return
	}

	// Get user information for audit trail (would be from auth middleware)
	modifiedBy := "api-user" // TODO: Extract from authentication context

	config, err := h.pluginModule.configManager.ResetConfiguration(pluginID, modifiedBy)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, err,
			"Failed to reset plugin configuration")
		return
	}

	h.successResponse(c, config, "Plugin configuration reset to defaults successfully")
}

func (h *PluginAPIHandlers) handleGetPluginEvents(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin events endpoint coming soon")
}

func (h *PluginAPIHandlers) handleGetPluginHistory(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin history endpoint coming soon")
}

func (h *PluginAPIHandlers) handleClearPluginEvents(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin events clearing endpoint coming soon")
}

func (h *PluginAPIHandlers) handleGetPluginAdminPages(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin admin pages endpoint coming soon")
}

func (h *PluginAPIHandlers) handleGetPluginUIComponents(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin UI components endpoint coming soon")
}

func (h *PluginAPIHandlers) handleGetPluginAssets(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin assets endpoint coming soon")
}

func (h *PluginAPIHandlers) handleGetPluginDependencies(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin dependencies endpoint coming soon")
}

func (h *PluginAPIHandlers) handleGetPluginDependents(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin dependents endpoint coming soon")
}

func (h *PluginAPIHandlers) handleValidateDependencies(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin dependency validation endpoint coming soon")
}

func (h *PluginAPIHandlers) handleTestPlugin(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin testing endpoint coming soon")
}

func (h *PluginAPIHandlers) handleGetTestResults(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin test results endpoint coming soon")
}

func (h *PluginAPIHandlers) handleValidatePlugin(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin validation endpoint coming soon")
}

// Core Plugin handlers
func (h *PluginAPIHandlers) handleListCorePlugins(c *gin.Context) {
	if h.pluginModule == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("plugin module not initialized"), "Plugin module unavailable")
		return
	}

	plugins := h.pluginModule.GetCoreManager().ListCorePluginInfo()
	h.successResponse(c, plugins, "Core plugins retrieved successfully")
}

func (h *PluginAPIHandlers) handleGetCorePlugin(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Core plugin details endpoint coming soon")
}

func (h *PluginAPIHandlers) handleEnableCorePlugin(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Core plugin enable endpoint coming soon")
}

func (h *PluginAPIHandlers) handleDisableCorePlugin(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Core plugin disable endpoint coming soon")
}

func (h *PluginAPIHandlers) handleGetCorePluginConfig(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Core plugin configuration endpoint coming soon")
}

func (h *PluginAPIHandlers) handleUpdateCorePluginConfig(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Core plugin configuration update endpoint coming soon")
}

// External Plugin handlers
func (h *PluginAPIHandlers) handleListExternalPlugins(c *gin.Context) {
	if h.pluginModule == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("plugin module not initialized"), "Plugin module unavailable")
		return
	}

	plugins := h.pluginModule.GetExternalManager().ListPlugins()
	h.successResponse(c, plugins, "External plugins retrieved successfully")
}

func (h *PluginAPIHandlers) handleInstallPlugin(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin installation endpoint coming soon")
}

func (h *PluginAPIHandlers) handleRefreshExternalPlugins(c *gin.Context) {
	if h.pluginModule == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("plugin module not initialized"), "Plugin module unavailable")
		return
	}

	if err := h.pluginModule.RefreshExternalPlugins(); err != nil {
		h.errorResponse(c, http.StatusInternalServerError, err,
			"Failed to refresh external plugins")
		return
	}

	h.successResponse(c, nil, "External plugins refreshed successfully")
}

func (h *PluginAPIHandlers) handleGetExternalPlugin(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "External plugin details endpoint coming soon")
}

func (h *PluginAPIHandlers) handleLoadExternalPlugin(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "External plugin load endpoint coming soon")
}

func (h *PluginAPIHandlers) handleUnloadExternalPlugin(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "External plugin unload endpoint coming soon")
}

func (h *PluginAPIHandlers) handleGetPluginManifest(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin manifest endpoint coming soon")
}

// System handlers - placeholder implementations
func (h *PluginAPIHandlers) handleGetSystemStatus(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "System status endpoint coming soon")
}

func (h *PluginAPIHandlers) handleGetSystemStats(c *gin.Context) {
	if h.pluginModule == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("plugin module not initialized"), "Plugin module unavailable")
		return
	}

	// Get all plugins
	allPlugins := h.pluginModule.ListAllPlugins()

	totalPlugins := len(allPlugins)
	enabledPlugins := 0
	disabledPlugins := 0
	healthyPlugins := 0
	unhealthyPlugins := 0

	for _, plugin := range allPlugins {
		if plugin.Enabled {
			enabledPlugins++
			healthyPlugins++ // Assume enabled plugins are healthy for now
		} else {
			disabledPlugins++
			unhealthyPlugins++
		}
	}

	// Count admin pages
	var adminPageCount int64
	h.db.Model(&database.PluginAdminPage{}).Where("enabled = ?", true).Count(&adminPageCount)

	// Get hot reload status
	hotReloadStatus := h.pluginModule.GetHotReloadStatus()
	hotReloadEnabled := false
	if enabled, ok := hotReloadStatus["enabled"].(bool); ok {
		hotReloadEnabled = enabled
	}

	stats := map[string]interface{}{
		"total_plugins":      totalPlugins,
		"enabled_plugins":    enabledPlugins,
		"disabled_plugins":   disabledPlugins,
		"healthy_plugins":    healthyPlugins,
		"unhealthy_plugins":  unhealthyPlugins,
		"total_admin_pages":  adminPageCount,
		"hot_reload_enabled": hotReloadEnabled,
		"memory_usage":       0.0, // TODO: Implement memory monitoring
		"cpu_usage":          0.0, // TODO: Implement CPU monitoring
		"uptime":             0,   // TODO: Implement uptime tracking
	}

	h.successResponse(c, stats, "System stats retrieved successfully")
}

func (h *PluginAPIHandlers) handleRefreshAllPlugins(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Refresh all plugins endpoint coming soon")
}

func (h *PluginAPIHandlers) handleCleanupSystem(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "System cleanup endpoint coming soon")
}

func (h *PluginAPIHandlers) handleGetHotReloadStatus(c *gin.Context) {
	if h.pluginModule == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("plugin module not initialized"), "Plugin module unavailable")
		return
	}

	status := h.pluginModule.GetHotReloadStatus()
	h.successResponse(c, status, "Hot reload status retrieved successfully")
}

func (h *PluginAPIHandlers) handleEnableHotReload(c *gin.Context) {
	if h.pluginModule == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("plugin module not initialized"), "Plugin module unavailable")
		return
	}

	if err := h.pluginModule.SetHotReloadEnabled(true); err != nil {
		h.errorResponse(c, http.StatusInternalServerError, err,
			"Failed to enable hot reload")
		return
	}

	h.successResponse(c, nil, "Hot reload enabled successfully")
}

func (h *PluginAPIHandlers) handleDisableHotReload(c *gin.Context) {
	if h.pluginModule == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("plugin module not initialized"), "Plugin module unavailable")
		return
	}

	if err := h.pluginModule.SetHotReloadEnabled(false); err != nil {
		h.errorResponse(c, http.StatusInternalServerError, err,
			"Failed to disable hot reload")
		return
	}

	h.successResponse(c, nil, "Hot reload disabled successfully")
}

func (h *PluginAPIHandlers) handleTriggerHotReload(c *gin.Context) {
	pluginID := c.Param("id")
	if pluginID == "" {
		h.errorResponse(c, http.StatusBadRequest,
			fmt.Errorf("plugin ID required"), "Plugin ID parameter is required")
		return
	}

	if h.pluginModule == nil {
		h.errorResponse(c, http.StatusServiceUnavailable,
			fmt.Errorf("plugin module not initialized"), "Plugin module unavailable")
		return
	}

	if err := h.pluginModule.TriggerPluginReload(pluginID); err != nil {
		h.errorResponse(c, http.StatusInternalServerError, err,
			"Failed to trigger hot reload")
		return
	}

	h.successResponse(c, gin.H{"plugin_id": pluginID},
		"Hot reload triggered successfully")
}

func (h *PluginAPIHandlers) handleBulkEnable(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Bulk enable endpoint coming soon")
}

func (h *PluginAPIHandlers) handleBulkDisable(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Bulk disable endpoint coming soon")
}

func (h *PluginAPIHandlers) handleBulkUpdate(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Bulk update endpoint coming soon")
}

// Admin Panel handlers
func (h *PluginAPIHandlers) handleGetAllAdminPages(c *gin.Context) {
	var pages []AdminPageInfo

	// Get admin pages directly from plugins instead of database
	if h.pluginModule != nil {
		// Get FFmpeg plugin admin pages directly
		if ffmpegPlugin := h.findPluginByID("ffmpeg_transcoder"); ffmpegPlugin != nil {
			// Direct call to get FFmpeg admin pages - temporary fix for type field issue
			ffmpegPages := []AdminPageInfo{
				{
					ID:       "ffmpeg_config",
					Title:    "Transcoding Settings",
					Path:     "/admin/plugins/ffmpeg_transcoder/config",
					Icon:     "settings",
					Category: "transcoding",
					URL:      "/admin/plugins/ffmpeg_transcoder/config",
					Type:     "configuration",
				},
				{
					ID:       "ffmpeg_monitoring",
					Title:    "Active Jobs",
					Path:     "/admin/plugins/ffmpeg_transcoder/monitor",
					Icon:     "activity",
					Category: "transcoding",
					URL:      "/admin/plugins/ffmpeg_transcoder/monitor",
					Type:     "dashboard",
				},
				{
					ID:       "ffmpeg_sessions",
					Title:    "Transcode Sessions",
					Path:     "/admin/plugins/ffmpeg_transcoder/sessions",
					Icon:     "clock",
					Category: "transcoding",
					URL:      "/admin/plugins/ffmpeg_transcoder/sessions",
					Type:     "status",
				},
				{
					ID:       "ffmpeg_health",
					Title:    "System Health",
					Path:     "/admin/plugins/ffmpeg_transcoder/health",
					Icon:     "heart",
					Category: "transcoding",
					URL:      "/admin/plugins/ffmpeg_transcoder/health",
					Type:     "status",
				},
			}
			pages = append(pages, ffmpegPages...)
		}
	}

	// Fallback to database if direct plugin access doesn't work
	if len(pages) == 0 {
		var adminPages []database.PluginAdminPage
		if err := h.db.Where("enabled = ?", true).Find(&adminPages).Error; err != nil {
			h.errorResponse(c, http.StatusInternalServerError, err,
				"Failed to retrieve admin pages")
			return
		}

		// Convert to API format
		pages = make([]AdminPageInfo, len(adminPages))
		for i, page := range adminPages {
			pages[i] = AdminPageInfo{
				ID:       page.PageID,
				Title:    page.Title,
				Path:     page.Path,
				Icon:     page.Icon,
				Category: page.Category,
				URL:      page.URL,
				Type:     page.Type,
			}
		}
	}

	h.successResponse(c, pages, "Admin pages retrieved successfully")
}

func (h *PluginAPIHandlers) handleGetAdminNavigation(c *gin.Context) {
	// Query database for all plugin admin pages to build navigation
	var adminPages []database.PluginAdminPage
	if err := h.db.Where("enabled = ?", true).Order("category, sort_order, title").Find(&adminPages).Error; err != nil {
		h.errorResponse(c, http.StatusInternalServerError, err,
			"Failed to retrieve admin navigation")
		return
	}

	// Group pages by category for navigation structure
	navigation := make(map[string][]AdminPageInfo)
	for _, page := range adminPages {
		category := page.Category
		if category == "" {
			category = "General"
		}

		if navigation[category] == nil {
			navigation[category] = []AdminPageInfo{}
		}

		navigation[category] = append(navigation[category], AdminPageInfo{
			ID:       page.PageID,
			Title:    page.Title,
			Path:     page.Path,
			Icon:     page.Icon,
			Category: page.Category,
			URL:      page.URL,
			Type:     page.Type,
		})
	}

	h.successResponse(c, navigation, "Admin navigation retrieved successfully")
}

func (h *PluginAPIHandlers) handleGetPluginPermissions(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin permissions endpoint coming soon")
}

func (h *PluginAPIHandlers) handleUpdatePluginPermissions(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Plugin permissions update endpoint coming soon")
}

func (h *PluginAPIHandlers) handleGetGlobalPluginSettings(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Global plugin settings endpoint coming soon")
}

func (h *PluginAPIHandlers) handleUpdateGlobalPluginSettings(c *gin.Context) {
	h.errorResponse(c, http.StatusNotImplemented,
		fmt.Errorf("not implemented"), "Global plugin settings update endpoint coming soon")
}

// Helper methods - to be implemented
func (h *PluginAPIHandlers) filterPlugins(plugins []PluginInfo, category, status, pluginType string) []PluginInfo {
	// TODO: Implement filtering logic
	return plugins
}

func (h *PluginAPIHandlers) searchPlugins(plugins []PluginInfo, query string) []PluginInfo {
	// TODO: Implement search logic
	var results []PluginInfo
	queryLower := strings.ToLower(query)

	for _, plugin := range plugins {
		if strings.Contains(strings.ToLower(plugin.Name), queryLower) ||
			strings.Contains(strings.ToLower(plugin.Description), queryLower) ||
			strings.Contains(strings.ToLower(plugin.Type), queryLower) {
			results = append(results, plugin)
		}
	}

	return results
}

func (h *PluginAPIHandlers) getPluginCategories() []string {
	// TODO: Implement category discovery
	return []string{"enrichment", "transcoding", "scanner", "metadata", "ui"}
}

func (h *PluginAPIHandlers) getSystemCapabilities() map[string]interface{} {
	// TODO: Implement capability discovery
	return map[string]interface{}{
		"hot_reload":       true,
		"external_plugins": true,
		"core_plugins":     true,
		"admin_pages":      true,
		"ui_components":    true,
	}
}

func (h *PluginAPIHandlers) findPluginByID(pluginID string) *PluginInfo {
	if h.pluginModule == nil {
		return nil
	}

	allPlugins := h.pluginModule.ListAllPlugins()
	for _, plugin := range allPlugins {
		if plugin.ID == pluginID || plugin.Name == pluginID {
			return &plugin
		}
	}

	return nil
}

func (h *PluginAPIHandlers) enhancePluginInfo(plugin PluginInfo) EnhancedPluginInfo {
	// TODO: Enhance plugin info with additional data
	return EnhancedPluginInfo{
		PluginInfo: plugin,
		// Additional fields will be populated as we implement them
	}
}

// Data structures for enhanced plugin information

// EnhancedPluginInfo provides comprehensive plugin information for admin panel
type EnhancedPluginInfo struct {
	PluginInfo
	Health            *PluginHealthInfo      `json:"health,omitempty"`
	Configuration     map[string]interface{} `json:"configuration,omitempty"`
	Dependencies      []string               `json:"dependencies,omitempty"`
	Dependents        []string               `json:"dependents,omitempty"`
	AdminPages        []AdminPageInfo        `json:"admin_pages,omitempty"`
	Permissions       []string               `json:"permissions,omitempty"`
	LastActivity      *time.Time             `json:"last_activity,omitempty"`
	InstallationDate  *time.Time             `json:"installation_date,omitempty"`
	UpdateAvailable   bool                   `json:"update_available,omitempty"`
	LatestVersion     string                 `json:"latest_version,omitempty"`
	ConfigurationHash string                 `json:"configuration_hash,omitempty"`
}

// PluginHealthInfo provides detailed health information
type PluginHealthInfo struct {
	Status              string        `json:"status"`
	Running             bool          `json:"running"`
	Healthy             bool          `json:"healthy"`
	ErrorRate           float64       `json:"error_rate"`
	TotalRequests       int64         `json:"total_requests"`
	SuccessfulRequests  int64         `json:"successful_requests"`
	FailedRequests      int64         `json:"failed_requests"`
	ConsecutiveFailures int           `json:"consecutive_failures"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	Uptime              time.Duration `json:"uptime"`
	LastError           string        `json:"last_error,omitempty"`
	LastCheckTime       time.Time     `json:"last_check_time"`
	StartTime           time.Time     `json:"start_time"`
}

// AdminPageInfo provides admin page information
type AdminPageInfo struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Path        string   `json:"path"`
	Icon        string   `json:"icon,omitempty"`
	Category    string   `json:"category,omitempty"`
	URL         string   `json:"url"`
	Type        string   `json:"type"`
	Permissions []string `json:"permissions,omitempty"`
}

// PluginUpdateRequest represents a plugin update request
type PluginUpdateRequest struct {
	Version       string                 `json:"version,omitempty"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
	Enabled       *bool                  `json:"enabled,omitempty"`
}

// handleGetAdminPageStatus gets real-time status for a specific admin page
func (h *PluginAPIHandlers) handleGetAdminPageStatus(c *gin.Context) {
	pluginID := c.Param("id")
	pageID := c.Param("pageId")

	h.logger.Debug("getting admin page status", "plugin_id", pluginID, "page_id", pageID)

	// Find the plugin
	plugin := h.findPluginByID(pluginID)
	if plugin == nil {
		h.errorResponse(c, http.StatusNotFound, fmt.Errorf("plugin not found"), "Plugin not found")
		return
	}

	// For now, provide mock data for FFmpeg transcoder - this should be replaced with real plugin communication
	if pluginID == "ffmpeg_transcoder" {
		status := h.getMockFFmpegStatus(pageID)
		h.successResponse(c, status, "Page status retrieved successfully")
		return
	}

	h.errorResponse(c, http.StatusNotImplemented, fmt.Errorf("status not available for this plugin"), "Status not available for this plugin")
}

// handleExecuteAdminPageAction executes an action on a specific admin page
func (h *PluginAPIHandlers) handleExecuteAdminPageAction(c *gin.Context) {
	pluginID := c.Param("id")
	pageID := c.Param("pageId")
	actionID := c.Param("actionId")

	h.logger.Debug("executing admin page action", "plugin_id", pluginID, "page_id", pageID, "action_id", actionID)

	// Parse request payload
	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		// Allow empty payload
		payload = make(map[string]interface{})
	}

	// Find the plugin
	plugin := h.findPluginByID(pluginID)
	if plugin == nil {
		h.errorResponse(c, http.StatusNotFound, fmt.Errorf("plugin not found"), "Plugin not found")
		return
	}

	// For now, provide mock responses for FFmpeg transcoder - this should be replaced with real plugin communication
	if pluginID == "ffmpeg_transcoder" {
		result := h.executeMockFFmpegAction(pageID, actionID, payload)
		h.successResponse(c, result, "Page action executed successfully")
		return
	}

	h.errorResponse(c, http.StatusNotImplemented, fmt.Errorf("actions not available for this plugin"), "Actions not available for this plugin")
}

// getMockFFmpegStatus provides mock status data for FFmpeg transcoder pages
func (h *PluginAPIHandlers) getMockFFmpegStatus(pageID string) map[string]interface{} {
	switch pageID {
	case "ffmpeg_config":
		return map[string]interface{}{
			"status":  "green",
			"color":   "green",
			"message": "Configuration active",
			"indicators": []map[string]interface{}{
				{"key": "Hardware Acceleration", "value": "Enabled", "color": "green"},
				{"key": "Quality Preset", "value": "Fast", "color": "blue"},
			},
		}
	case "ffmpeg_monitoring":
		return map[string]interface{}{
			"status":  "yellow",
			"color":   "yellow",
			"message": "Jobs in queue",
			"indicators": []map[string]interface{}{
				{"key": "Queue", "value": "3 jobs", "color": "yellow"},
				{"key": "Processing", "value": "sample_video.mkv", "color": "green"},
			},
		}
	case "ffmpeg_sessions":
		return map[string]interface{}{
			"status":  "blue",
			"color":   "blue",
			"message": "Active transcoding",
			"indicators": []map[string]interface{}{
				{"key": "Active Sessions", "value": "2 transcoding", "color": "yellow"},
				{"key": "CPU Usage", "value": "78%", "color": "orange"},
			},
			"progress": map[string]interface{}{
				"current":    67,
				"total":      100,
				"percentage": 67,
				"label":      "Progress",
			},
		}
	case "ffmpeg_health":
		return map[string]interface{}{
			"status":  "green",
			"color":   "green",
			"message": "System healthy",
			"indicators": []map[string]interface{}{
				{"key": "CPU Usage", "value": "45%", "color": "green"},
				{"key": "Memory", "value": "2.1 GB", "color": "blue"},
			},
		}
	default:
		return map[string]interface{}{
			"status":     "gray",
			"color":      "gray",
			"message":    "Status unknown",
			"indicators": []map[string]interface{}{},
		}
	}
}

// executeMockFFmpegAction provides mock action execution for FFmpeg transcoder
func (h *PluginAPIHandlers) executeMockFFmpegAction(pageID, actionID string, payload map[string]interface{}) map[string]interface{} {
	h.logger.Info("executing mock FFmpeg action", "page_id", pageID, "action_id", actionID)

	switch actionID {
	case "quick_edit":
		return map[string]interface{}{
			"success": true,
			"message": "Configuration editor opened",
			"data":    map[string]interface{}{"redirect": "/admin/plugins/ffmpeg_transcoder/config"},
		}
	case "clear_queue":
		return map[string]interface{}{
			"success": true,
			"message": "Transcoding queue cleared successfully",
			"data":    map[string]interface{}{"cleared_jobs": 3},
		}
	case "stop_sessions":
		return map[string]interface{}{
			"success": true,
			"message": "All transcoding sessions stopped",
			"data":    map[string]interface{}{"stopped_sessions": 2},
		}
	case "restart":
		return map[string]interface{}{
			"success": true,
			"message": "FFmpeg transcoder restarted successfully",
			"data":    map[string]interface{}{"restart_time": time.Now().Format(time.RFC3339)},
		}
	default:
		return map[string]interface{}{
			"success": false,
			"message": "Unknown action",
			"error":   fmt.Sprintf("Action '%s' not supported", actionID),
		}
	}
}

// handleSessionAction handles session actions like stop, restart, prioritize
func (h *PluginAPIHandlers) handleSessionAction(c *gin.Context) {
	sessionID := c.Param("sessionId")
	action := c.Param("action")

	h.logger.Info("handling session action", "session_id", sessionID, "action", action)

	switch action {
	case "stop":
		err := h.stopTranscodingSession(sessionID)
		if err != nil {
			h.logger.Error("failed to stop session", "session_id", sessionID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Session stopped successfully",
		})

	case "restart":
		// For restart, we need to stop and start again with same parameters
		err := h.restartTranscodingSession(sessionID)
		if err != nil {
			h.logger.Error("failed to restart session", "session_id", sessionID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Session restarted successfully",
		})

	case "prioritize":
		// For prioritize, we update the session priority
		err := h.prioritizeTranscodingSession(sessionID)
		if err != nil {
			h.logger.Error("failed to prioritize session", "session_id", sessionID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Session prioritized successfully",
		})

	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Unknown action: %s", action),
		})
	}
}

// handleEngineAction handles engine-wide actions like clear_cache, restart_service
func (h *PluginAPIHandlers) handleEngineAction(c *gin.Context) {
	action := c.Param("action")

	h.logger.Info("handling engine action", "action", action)

	switch action {
	case "clear_cache":
		err := h.clearTranscodingCache()
		if err != nil {
			h.logger.Error("failed to clear cache", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Cache cleared successfully",
		})

	case "restart_service":
		err := h.restartTranscodingService()
		if err != nil {
			h.logger.Error("failed to restart service", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Service restart initiated",
		})

	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   fmt.Sprintf("Unknown action: %s", action),
		})
	}
}

// stopTranscodingSession stops a specific transcoding session
func (h *PluginAPIHandlers) stopTranscodingSession(sessionID string) error {
	// TODO: Update to use TranscodingProvider once gRPC support is added
	// For now, return not implemented
	return fmt.Errorf("transcoding session management not implemented - awaiting TranscodingProvider gRPC support")
}

// restartTranscodingSession restarts a transcoding session
func (h *PluginAPIHandlers) restartTranscodingSession(sessionID string) error {
	// TODO: Update to use TranscodingProvider once gRPC support is added
	// For now, return not implemented
	return fmt.Errorf("transcoding session management not implemented - awaiting TranscodingProvider gRPC support")
}

// prioritizeTranscodingSession moves a session to higher priority
func (h *PluginAPIHandlers) prioritizeTranscodingSession(sessionID string) error {
	// This is a placeholder implementation since priority handling
	// would require extending the transcoding interface
	h.logger.Info("prioritizing session (placeholder implementation)", "session_id", sessionID)
	return nil
}

// clearTranscodingCache clears transcoding cache
func (h *PluginAPIHandlers) clearTranscodingCache() error {
	// TODO: Update to use TranscodingProvider once gRPC support is added
	// For now, return not implemented
	return fmt.Errorf("transcoding cache management not implemented - awaiting TranscodingProvider gRPC support")
}

// restartTranscodingService restarts transcoding services
func (h *PluginAPIHandlers) restartTranscodingService() error {
	// TODO: Update to use TranscodingProvider once gRPC support is added
	// For now, return not implemented
	return fmt.Errorf("transcoding service management not implemented - awaiting TranscodingProvider gRPC support")
}
