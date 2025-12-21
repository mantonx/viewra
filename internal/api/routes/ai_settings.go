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

		// Provider information
		ai.GET("/providers", h.GetProviders)
		ai.GET("/providers/:provider/models", h.ListModels)
		ai.POST("/providers/:provider/test", h.TestConnection)

		// Ollama-specific model management
		ai.GET("/models", h.ListOllamaModels)                          // Legacy, same as /providers/ollama/models
		ai.GET("/models/recommended", h.GetRecommendedModels)          // System-aware embedding recommendations
		ai.GET("/models/recommended/chat", h.GetRecommendedChatModels) // System-aware chat recommendations
		ai.POST("/models/pull", h.PullOllamaModel)
		ai.DELETE("/models", h.DeleteOllamaModel)

		// Legacy connection tests (redirect to new endpoints)
		ai.POST("/test/ollama", h.TestOllamaConnection)
		ai.POST("/test/openai", h.TestOpenAIConnection)
	}
}
