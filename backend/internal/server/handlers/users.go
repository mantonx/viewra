// filepath: /home/fictional/Projects/viewra/backend/internal/server/handlers/users.go
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/events"
)

// UsersHandler handles user-related API endpoints
type UsersHandler struct {
	eventBus events.EventBus
}

// NewUsersHandler creates a new users handler with event bus
func NewUsersHandler(eventBus events.EventBus) *UsersHandler {
	return &UsersHandler{
		eventBus: eventBus,
	}
}

// GetUsers retrieves all users from the database
func (h *UsersHandler) GetUsers(c *gin.Context) {
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
func (h *UsersHandler) CreateUser(c *gin.Context) {
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
	
	// Publish user created event
	if h.eventBus != nil {
		userEvent := events.NewSystemEvent(
			events.EventUserCreated,
			"User Created",
			"A new user account has been created",
		)
		userEvent.Data = map[string]interface{}{
			"userId":   user.ID,
			"username": user.Username,
		}
		h.eventBus.PublishAsync(userEvent)
	}
	
	c.JSON(http.StatusCreated, gin.H{
		"user":    user,
		"message": "User created successfully",
	})
}

// LoginUser handles user login and session creation
func (h *UsersHandler) LoginUser(c *gin.Context) {
	// User login logic would go here
	
	// Example for handling login event
	var loginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	
	if err := c.ShouldBindJSON(&loginRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid login format",
			"details": err.Error(),
		})
		return
	}

	// Authentication logic would go here
	// ...
	
	// For example purposes, user with ID 1
	userID := uint(1)
	
	// Publish login event
	if h.eventBus != nil {
		loginEvent := events.NewSystemEvent(
			events.EventUserLoggedIn,
			"User Login",
			"User logged in successfully",
		)
		loginEvent.Data = map[string]interface{}{
			"userId":   userID,
			"username": loginRequest.Username,
			"ipAddress": c.ClientIP(),
			"userAgent": c.GetHeader("User-Agent"),
		}
		h.eventBus.PublishAsync(loginEvent)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"token":   "sample-token-would-go-here",
	})
}

// LogoutUser handles user logout
func (h *UsersHandler) LogoutUser(c *gin.Context) {
	// User logout logic would go here
	
	// For example purposes
	userID := uint(1)
	username := "example_user"
	
	// Publish logout event
	if h.eventBus != nil {
		logoutEvent := events.NewSystemEvent(
			events.EventInfo, // Could create a dedicated EventUserLoggedOut type
			"User Logout",
			"User logged out successfully",
		)
		logoutEvent.Data = map[string]interface{}{
			"userId":   userID,
			"username": username,
		}
		h.eventBus.PublishAsync(logoutEvent)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "Logout successful",
	})
}

// Keep original function-based handlers for backward compatibility
// These will delegate to the struct-based handlers

// GetUsers function-based handler for backward compatibility
func GetUsers(c *gin.Context) {
	// Create a temporary handler without event bus for backward compatibility
	handler := &UsersHandler{}
	handler.GetUsers(c)
}

// CreateUser function-based handler for backward compatibility
func CreateUser(c *gin.Context) {
	// Create a temporary handler without event bus for backward compatibility
	handler := &UsersHandler{}
	handler.CreateUser(c)
}
