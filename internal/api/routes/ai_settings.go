package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
	"github.com/mantonx/viewra/internal/api/middleware"
)

// RegisterAISettingsRoutes registers AI settings routes.
// These routes require admin authentication.
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
		// Settings CRUD
		ai.GET("", h.GetAISettings)
		ai.PUT("", h.UpdateAISettings)

		// Model management
		ai.GET("/models", h.ListOllamaModels)
		ai.POST("/models/pull", h.PullOllamaModel)
		ai.DELETE("/models", h.DeleteOllamaModel)

		// Connection tests
		ai.POST("/test/ollama", h.TestOllamaConnection)
		ai.POST("/test/openai", h.TestOpenAIConnection)
	}
}
