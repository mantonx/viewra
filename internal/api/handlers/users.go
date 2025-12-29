package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/application/auth"
	domainUser "github.com/mantonx/viewra/internal/domain/user"
)

// UsersHandler handles user management HTTP requests (admin only).
type UsersHandler struct {
	adminService *auth.AdminService
}

// NewUsersHandler creates a new users handler.
func NewUsersHandler(adminService *auth.AdminService) *UsersHandler {
	return &UsersHandler{adminService: adminService}
}

// List handles GET /api/users
// @Summary List users
// @Description Returns a paginated list of all users (admin only)
// @Tags users
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Limit" default(50)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} auth.ListUsersResponse
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Router /api/users [get]
func (h *UsersHandler) List(c *gin.Context) {
	limit := getQueryInt(c, "limit", 50)
	offset := getQueryInt(c, "offset", 0)

	resp, err := h.adminService.ListUsers(c.Request.Context(), limit, offset)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Get handles GET /api/users/:id
// @Summary Get user
// @Description Returns a user by ID (admin only)
// @Tags users
// @Security BearerAuth
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} auth.UserResponse
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Router /api/users/{id} [get]
func (h *UsersHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "User ID required",
		})
		return
	}

	resp, err := h.adminService.GetUser(c.Request.Context(), id)
	if err != nil {
		handleUserError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// CreateUserRequest is the request body for creating a user.
type CreateUserRequest struct {
	Username    string `json:"username" binding:"required"`
	DisplayName string `json:"display_name" binding:"required"`
	Password    string `json:"password" binding:"required"`
	IsAdmin     bool   `json:"is_admin"`
}

// Create handles POST /api/users
// @Summary Create user
// @Description Creates a new user (admin only)
// @Tags users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param user body CreateUserRequest true "User details"
// @Success 201 {object} auth.UserResponse
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 409 {object} APIError "Username exists"
// @Router /api/users [post]
func (h *UsersHandler) Create(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.adminService.CreateUser(c.Request.Context(), &auth.CreateUserRequest{
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Password:    req.Password,
		IsAdmin:     req.IsAdmin,
	})
	if err != nil {
		handleUserError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// UpdateUserRequest is the request body for updating a user.
type UpdateUserRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	IsAdmin     *bool   `json:"is_admin,omitempty"`
	IsDisabled  *bool   `json:"is_disabled,omitempty"`
}

// Update handles PUT /api/users/:id
// @Summary Update user
// @Description Updates a user (admin only)
// @Tags users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param user body UpdateUserRequest true "User updates"
// @Success 200 {object} auth.UserResponse
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Router /api/users/{id} [put]
func (h *UsersHandler) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "User ID required",
		})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.adminService.UpdateUser(c.Request.Context(), id, &auth.UpdateUserRequest{
		DisplayName: req.DisplayName,
		IsAdmin:     req.IsAdmin,
		IsDisabled:  req.IsDisabled,
	})
	if err != nil {
		handleUserError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Delete handles DELETE /api/users/:id
// @Summary Delete user
// @Description Deletes a user (admin only)
// @Tags users
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 204 "No Content"
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Router /api/users/{id} [delete]
func (h *UsersHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "User ID required",
		})
		return
	}

	err := h.adminService.DeleteUser(c.Request.Context(), id)
	if err != nil {
		handleUserError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ResetPasswordRequest is the request body for admin password reset.
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required"`
}

// ResetPassword handles POST /api/users/:id/reset-password
// @Summary Reset user password
// @Description Resets a user's password (admin only)
// @Tags users
// @Security BearerAuth
// @Accept json
// @Param id path string true "User ID"
// @Param password body ResetPasswordRequest true "New password"
// @Success 204 "No Content"
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Failure 403 {object} APIError
// @Failure 404 {object} APIError
// @Router /api/users/{id}/reset-password [post]
func (h *UsersHandler) ResetPassword(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "User ID required",
		})
		return
	}

	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request body",
			Message: err.Error(),
		})
		return
	}

	err := h.adminService.ResetPassword(c.Request.Context(), &auth.ResetPasswordRequest{
		UserID:      id,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		handleUserError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// handleUserError handles user-specific errors with appropriate status codes.
func handleUserError(c *gin.Context, err error) {
	switch err {
	case domainUser.ErrUserNotFound:
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "User not found",
		})
	case domainUser.ErrUsernameExists:
		c.JSON(http.StatusConflict, ErrorResponse{
			Error: "Username already exists",
		})
	case domainUser.ErrUsernameTooShort, domainUser.ErrUsernameTooLong,
		domainUser.ErrUsernameInvalidChars, domainUser.ErrPasswordTooShort,
		domainUser.ErrDisplayNameEmpty, domainUser.ErrDisplayNameTooLong:
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Validation error",
			Message: err.Error(),
		})
	default:
		handleError(c, err)
	}
}
