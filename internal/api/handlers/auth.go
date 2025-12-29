package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/api/middleware"
	"github.com/mantonx/viewra/internal/application/auth"
	domainUser "github.com/mantonx/viewra/internal/domain/user"
)

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	service *auth.Service
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(service *auth.Service) *AuthHandler {
	return &AuthHandler{service: service}
}

// LoginRequest is the request body for login.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login handles POST /api/auth/login
// @Summary User login
// @Description Authenticates a user and returns access and refresh tokens
// @Tags auth
// @Accept json
// @Produce json
// @Param credentials body LoginRequest true "Login credentials"
// @Success 200 {object} auth.LoginResponse
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Router /api/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST_BODY", err.Error())
		return
	}

	resp, err := h.service.Login(c.Request.Context(), &auth.LoginRequest{
		Username:  req.Username,
		Password:  req.Password,
		UserAgent: c.Request.UserAgent(),
		IPAddress: c.ClientIP(),
	})
	if err != nil {
		handleAuthError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// RefreshRequest is the request body for token refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Refresh handles POST /api/auth/refresh
// @Summary Refresh access token
// @Description Exchanges a refresh token for a new access token
// @Tags auth
// @Accept json
// @Produce json
// @Param token body RefreshRequest true "Refresh token"
// @Success 200 {object} auth.LoginResponse
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Router /api/auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST_BODY", err.Error())
		return
	}

	resp, err := h.service.Refresh(c.Request.Context(), &auth.RefreshRequest{
		RefreshToken: req.RefreshToken,
		UserAgent:    c.Request.UserAgent(),
		IPAddress:    c.ClientIP(),
	})
	if err != nil {
		handleAuthError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// LogoutRequest is the request body for logout.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// Logout handles POST /api/auth/logout
// @Summary Logout
// @Description Invalidates the current session
// @Tags auth
// @Accept json
// @Produce json
// @Param token body LogoutRequest true "Refresh token to invalidate"
// @Success 204 "No Content"
// @Failure 400 {object} APIError
// @Router /api/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST_BODY", err.Error())
		return
	}

	_ = h.service.Logout(c.Request.Context(), req.RefreshToken)
	c.Status(http.StatusNoContent)
}

// LogoutAll handles POST /api/auth/logout-all
// @Summary Logout from all sessions
// @Description Invalidates all sessions for the current user
// @Tags auth
// @Security BearerAuth
// @Success 204 "No Content"
// @Failure 401 {object} APIError
// @Router /api/auth/logout-all [post]
func (h *AuthHandler) LogoutAll(c *gin.Context) {
	if !middleware.IsAuthenticated(c) {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	userID := middleware.GetUserID(c)

	_ = h.service.LogoutAll(c.Request.Context(), userID)
	c.Status(http.StatusNoContent)
}

// GetMe handles GET /api/auth/me
// @Summary Get current user
// @Description Returns the currently authenticated user
// @Tags auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} auth.UserResponse
// @Failure 401 {object} APIError
// @Router /api/auth/me [get]
func (h *AuthHandler) GetMe(c *gin.Context) {
	if !middleware.IsAuthenticated(c) {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	userID := middleware.GetUserID(c)

	resp, err := h.service.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ChangePasswordRequest is the request body for password change.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}

// ChangePassword handles PUT /api/auth/password
// @Summary Change password
// @Description Changes the current user's password
// @Tags auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param passwords body ChangePasswordRequest true "Password change request"
// @Success 204 "No Content"
// @Failure 400 {object} APIError
// @Failure 401 {object} APIError
// @Router /api/auth/password [put]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	if !middleware.IsAuthenticated(c) {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	userID := middleware.GetUserID(c)

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST_BODY", err.Error())
		return
	}

	err := h.service.ChangePassword(c.Request.Context(), &auth.ChangePasswordRequest{
		UserID:          userID,
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	})
	if err != nil {
		handleAuthError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ListSessions handles GET /api/auth/sessions
// @Summary List sessions
// @Description Returns all active sessions for the current user
// @Tags auth
// @Security BearerAuth
// @Produce json
// @Success 200 {array} auth.SessionResponse
// @Failure 401 {object} APIError
// @Router /api/auth/sessions [get]
func (h *AuthHandler) ListSessions(c *gin.Context) {
	if !middleware.IsAuthenticated(c) {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	userID := middleware.GetUserID(c)

	// We don't have access to the current refresh token hash here,
	// so we can't mark the current session
	sessions, err := h.service.ListSessions(c.Request.Context(), userID, "")
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, sessions)
}

// RevokeSession handles DELETE /api/auth/sessions/:id
// @Summary Revoke session
// @Description Revokes a specific session
// @Tags auth
// @Security BearerAuth
// @Param id path string true "Session ID"
// @Success 204 "No Content"
// @Failure 401 {object} APIError
// @Failure 404 {object} APIError
// @Router /api/auth/sessions/{id} [delete]
func (h *AuthHandler) RevokeSession(c *gin.Context) {
	if !middleware.IsAuthenticated(c) {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Not authenticated")
		return
	}
	userID := middleware.GetUserID(c)

	sessionID := c.Param("id")
	if sessionID == "" {
		respondError(c, http.StatusBadRequest, "SESSION_ID_REQUIRED", "Session ID required")
		return
	}

	err := h.service.RevokeSession(c.Request.Context(), userID, sessionID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// SetupStatus handles GET /api/auth/setup
// @Summary Check setup status
// @Description Returns whether initial setup is required (no users exist)
// @Tags auth
// @Produce json
// @Success 200 {object} SetupStatusResponse
// @Router /api/auth/setup [get]
func (h *AuthHandler) SetupStatus(c *gin.Context) {
	needsSetup, err := h.service.NeedsSetup(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, SetupStatusResponse{
		NeedsSetup: needsSetup,
	})
}

// SetupStatusResponse is the response for setup status check.
type SetupStatusResponse struct {
	NeedsSetup bool `json:"needs_setup"`
}

// SetupRequest is the request body for initial setup.
type SetupRequest struct {
	Username    string `json:"username" binding:"required"`
	DisplayName string `json:"display_name" binding:"required"`
	Password    string `json:"password" binding:"required"`
}

// Setup handles POST /api/auth/setup
// @Summary Initial setup
// @Description Creates the initial admin user (only works if no users exist)
// @Tags auth
// @Accept json
// @Produce json
// @Param admin body SetupRequest true "Admin user details"
// @Success 201 {object} auth.UserResponse
// @Failure 400 {object} APIError
// @Failure 409 {object} APIError "Setup already completed"
// @Router /api/auth/setup [post]
func (h *AuthHandler) Setup(c *gin.Context) {
	// Check if setup is needed
	needsSetup, err := h.service.NeedsSetup(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}

	if !needsSetup {
		respondError(c, http.StatusConflict, "CONFLICT", "Setup already completed")
		return
	}

	var req SetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REQUEST_BODY", err.Error())
		return
	}

	// Create admin user
	resp, err := h.service.Register(c.Request.Context(), &auth.RegisterRequest{
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Password:    req.Password,
		IsAdmin:     true, // First user is always admin
	})
	if err != nil {
		handleAuthError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// handleAuthError handles auth-specific errors with appropriate status codes.
func handleAuthError(c *gin.Context, err error) {
	switch err {
	case domainUser.ErrInvalidCredentials:
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid username or password")
	case domainUser.ErrUserDisabled:
		respondError(c, http.StatusForbidden, "FORBIDDEN", "Account is disabled")
	case domainUser.ErrRefreshTokenInvalid:
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired refresh token")
	case domainUser.ErrUsernameExists:
		respondError(c, http.StatusConflict, "CONFLICT", "Username already exists")
	case domainUser.ErrUsernameTooShort, domainUser.ErrUsernameTooLong,
		domainUser.ErrUsernameInvalidChars, domainUser.ErrPasswordTooShort,
		domainUser.ErrDisplayNameEmpty, domainUser.ErrDisplayNameTooLong,
		domainUser.ErrPasswordSameAsOld:
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
	default:
		handleError(c, err)
	}
}
