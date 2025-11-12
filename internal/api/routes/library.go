package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/viewra/viewra/internal/api/handlers"
)

// RegisterLibraryRoutes registers all library-related routes
func RegisterLibraryRoutes(rg *gin.RouterGroup, handler *handlers.LibraryHandler) {
	libraries := rg.Group("/libraries")
	libraries.POST("", handler.Create)
	libraries.GET("", handler.List)
	libraries.GET("/:id", handler.Get)
	libraries.PUT("/:id", handler.Update)
	libraries.DELETE("/:id", handler.Delete)
	libraries.POST("/:id/scan", handler.Scan)
}
