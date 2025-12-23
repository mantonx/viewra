package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
	"github.com/mantonx/viewra/internal/api/middleware"
)

// RegisterAISettingsRoutes registers AI settings routes.
// These routes require admin authentication.
//
// Provider-specific routes (schema, configure, test, models) are handled by the
// unified plugin API at /api/plugins/:id/... The frontend should use:
//   - GET /api/plugins/:id/settings - Get schema + values
//   - PUT /api/plugins/:id/settings - Configure plugin
//   - GET /api/plugins/:id/health - Health check
//   - GET /api/plugins/provider-ollama/models - List installed models
//   - GET /api/plugins/provider-ollama/models/recommended - Model recommendations
func RegisterAISettingsRoutes(protected *gin.RouterGroup, h *handlers.AISettingsHandler, authValidator middleware.AuthValidator) {
	if h == nil {
		return
	}

	// AI settings group (requires admin)
	ai := protected.Group("/settings/ai")
	if authValidator != nil {
		ai.Use(middleware.RequireAdmin(authValidator))
	}
	{
		// AI feature settings (provider selection, search params)
		ai.GET("", h.GetAISettings)
		ai.PUT("", h.UpdateAISettings)

		// Provider listing (with capabilities)
		ai.GET("/providers", h.GetProviders)
	}
}
