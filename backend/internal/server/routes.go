package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/viewra/internal/database"
)

// setupRoutes configures all API routes
func setupRoutes(r *gin.Engine) {
	// API v1 routes
	api := r.Group("/api")
	{
		// Health check endpoint
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "ok",
				"service": "viewra",
			})
		})
		
		// Hello world endpoint
		api.GET("/hello", func(c *gin.Context) {
			c.String(http.StatusOK, "Hello from Viewra backend! (Docker Compose development environment)")
		})
		
		// Database status endpoint
		api.GET("/db-status", func(c *gin.Context) {
			db := database.GetDB()
			sqlDB, err := db.DB()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"status": "error",
					"error":  err.Error(),
				})
				return
			}
			
			err = sqlDB.Ping()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"status": "error",
					"error":  err.Error(),
				})
				return
			}
			
			c.JSON(http.StatusOK, gin.H{
				"status": "connected",
				"database": "ready",
			})
		})
		
		// Media routes
		media := api.Group("/media")
		{
			media.GET("/", getMedia)
			media.POST("/", uploadMedia) // Will implement file upload later
		}
		
		// User routes
		users := api.Group("/users")
		{
			users.GET("/", getUsers)
			users.POST("/", createUser)
		}
	}
}

// Media handlers
func getMedia(c *gin.Context) {
	var media []database.Media
	db := database.GetDB()
	
	result := db.Preload("User").Find(&media)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": result.Error.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"media": media,
		"count": len(media),
	})
}

func uploadMedia(c *gin.Context) {
	// TODO: Implement file upload
	c.JSON(http.StatusNotImplemented, gin.H{
		"message": "File upload coming soon",
	})
}

// User handlers
func getUsers(c *gin.Context) {
	var users []database.User
	db := database.GetDB()
	
	result := db.Find(&users)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": result.Error.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"count": len(users),
	})
}

func createUser(c *gin.Context) {
	var user database.User
	
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	
	db := database.GetDB()
	result := db.Create(&user)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": result.Error.Error(),
		})
		return
	}
	
	c.JSON(http.StatusCreated, gin.H{
		"user": user,
	})
}