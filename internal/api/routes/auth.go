package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/handlers"
	"github.com/mantonx/viewra/internal/api/middleware"
)

// RegisterAuthRoutes registers all authentication-related routes.
func RegisterAuthRoutes(
	rg *gin.RouterGroup,
	handler *handlers.AuthHandler,
	usersHandler *handlers.UsersHandler,
	authValidator middleware.AuthValidator,
	rateLimiters *middleware.AuthRateLimiters,
) {
	auth := rg.Group("/auth")
	{
		// Public endpoints (no auth required, rate limited)
		auth.POST("/login", middleware.RateLimitByIP(rateLimiters.Login), handler.Login)
		auth.POST("/refresh", middleware.RateLimitByIP(rateLimiters.Refresh), handler.Refresh)
		auth.POST("/logout", handler.Logout)
		auth.GET("/setup", handler.SetupStatus)
		auth.POST("/setup", middleware.RateLimitByIP(rateLimiters.Setup), handler.Setup)

		// Protected endpoints (require auth)
		protected := auth.Group("")
		protected.Use(middleware.RequireAuth(authValidator))
		{
			protected.GET("/me", handler.GetMe)
			protected.PUT("/password", handler.ChangePassword)
			protected.POST("/logout-all", handler.LogoutAll)
			protected.GET("/sessions", handler.ListSessions)
			protected.DELETE("/sessions/:id", handler.RevokeSession)
		}
	}

	// User management (admin only)
	users := rg.Group("/users")
	users.Use(middleware.RequireAdmin(authValidator))
	{
		users.GET("", usersHandler.List)
		users.POST("", usersHandler.Create)
		users.GET("/:id", usersHandler.Get)
		users.PUT("/:id", usersHandler.Update)
		users.DELETE("/:id", usersHandler.Delete)
		users.POST("/:id/reset-password", usersHandler.ResetPassword)
	}
}
