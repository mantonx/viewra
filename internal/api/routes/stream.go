package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
)

// RegisterStreamRoutes registers all streaming-related routes
func RegisterStreamRoutes(rg *gin.RouterGroup, handler *handlers.StreamHandler) {
	rg.GET("/stream/:id", handler.Stream)
}
