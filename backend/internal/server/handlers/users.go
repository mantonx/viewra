package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/viewra/internal/database"
)

// GetUsers retrieves all users from the database
func GetUsers(c *gin.Context) {
	var users []database.User
	db := database.GetDB()
	
	result := db.Find(&users)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve users",
			"details": result.Error.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"count": len(users),
	})
}

// CreateUser creates a new user account
func CreateUser(c *gin.Context) {
	var user database.User
	
	// Bind and validate JSON input
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}
	
	// Create user in database
	db := database.GetDB()
	result := db.Create(&user)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create user",
			"details": result.Error.Error(),
		})
		return
	}
	
	c.JSON(http.StatusCreated, gin.H{
		"user":    user,
		"message": "User created successfully",
	})
}
