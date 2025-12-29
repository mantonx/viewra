package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mantonx/viewra/internal/infrastructure/auth"
)

// devModeEnabled returns true if VIEWRA_DEV_MODE=1 is set
func devModeEnabled() bool {
	return os.Getenv("VIEWRA_DEV_MODE") == "1"
}

// DevModePublicID is the public_id placeholder used in dev mode.
// This is used in dev mode to provide a valid user ID for database operations.
// In production, the actual JWT token contains the real user's public_id.
// Exported so handlers can check for this placeholder and look up the real user.
const DevModePublicID = "usr_dev_mode_placeholder"

// devModeClaims returns mock claims for dev mode
func devModeClaims() *auth.AccessClaims {
	claims := &auth.AccessClaims{
		UserID:  1,
		IsAdmin: true,
	}
	// Use the dev mode placeholder - handlers should check for this
	// and either skip user-specific operations or look up the real user
	claims.Subject = DevModePublicID
	return claims
}

// Context keys for user information
const (
	ContextKeyUserID  = "user_id"
	ContextKeyIsAdmin = "is_admin"
	ContextKeyClaims  = "claims"
)

// AuthValidator is the interface for validating access tokens.
type AuthValidator interface {
	ValidateAccessToken(token string) (*auth.AccessClaims, error)
}

// RequireAuth returns middleware that requires a valid access token.
// It extracts the token from the Authorization header (Bearer scheme)
// and validates it. If valid, user info is added to the context.
// In dev mode (VIEWRA_DEV_MODE=1), auth is bypassed with admin access.
func RequireAuth(validator AuthValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Dev mode bypass - grant admin access without token
		if devModeEnabled() {
			c.Set(ContextKeyUserID, int64(1))
			c.Set(ContextKeyIsAdmin, true)
			c.Set(ContextKeyClaims, devModeClaims())
			c.Next()
			return
		}

		token := extractBearerToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "missing authorization token",
			})
			c.Abort()
			return
		}

		claims, err := validator.ValidateAccessToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
			})
			c.Abort()
			return
		}

		// Add user info to context
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyIsAdmin, claims.IsAdmin)
		c.Set(ContextKeyClaims, claims)

		c.Next()
	}
}

// RequireAdmin returns middleware that requires a valid admin token.
// It first validates the token, then checks if the user is an admin.
// In dev mode (VIEWRA_DEV_MODE=1), auth is bypassed with admin access.
func RequireAdmin(validator AuthValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Dev mode bypass - grant admin access without token
		if devModeEnabled() {
			c.Set(ContextKeyUserID, int64(1))
			c.Set(ContextKeyIsAdmin, true)
			c.Set(ContextKeyClaims, devModeClaims())
			c.Next()
			return
		}

		token := extractBearerToken(c)
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "missing authorization token",
			})
			c.Abort()
			return
		}

		claims, err := validator.ValidateAccessToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
			})
			c.Abort()
			return
		}

		if !claims.IsAdmin {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "admin access required",
			})
			c.Abort()
			return
		}

		// Add user info to context
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyIsAdmin, claims.IsAdmin)
		c.Set(ContextKeyClaims, claims)

		c.Next()
	}
}

// OptionalAuth returns middleware that validates tokens if present,
// but allows requests without tokens to proceed.
// Useful for endpoints that have different behavior for authenticated users.
func OptionalAuth(validator AuthValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearerToken(c)
		if token == "" {
			c.Next()
			return
		}

		claims, err := validator.ValidateAccessToken(token)
		if err != nil {
			// Invalid token - proceed without auth context
			c.Next()
			return
		}

		// Add user info to context
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyIsAdmin, claims.IsAdmin)
		c.Set(ContextKeyClaims, claims)

		c.Next()
	}
}

// extractBearerToken extracts the token from the Authorization header.
// Expects format: "Bearer <token>"
func extractBearerToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}

	return parts[1]
}

// GetUserID returns the internal user ID from the context.
// Returns 0 if not authenticated.
func GetUserID(c *gin.Context) int64 {
	if userID, exists := c.Get(ContextKeyUserID); exists {
		return userID.(int64)
	}
	return 0
}

// IsAuthenticated returns true if a user is authenticated.
func IsAuthenticated(c *gin.Context) bool {
	_, exists := c.Get(ContextKeyUserID)
	return exists
}

// IsAdmin returns true if the current user is an admin.
func IsAdmin(c *gin.Context) bool {
	if isAdmin, exists := c.Get(ContextKeyIsAdmin); exists {
		return isAdmin.(bool)
	}
	return false
}

// GetClaims returns the access claims from the context.
// Returns nil if not authenticated.
func GetClaims(c *gin.Context) *auth.AccessClaims {
	if claims, exists := c.Get(ContextKeyClaims); exists {
		return claims.(*auth.AccessClaims)
	}
	return nil
}

// GetUserPublicID returns the public user ID (UUID) from the context.
// This is the external-facing user ID stored in the JWT Subject claim.
// Returns empty string if not authenticated.
func GetUserPublicID(c *gin.Context) string {
	claims := GetClaims(c)
	if claims == nil {
		return ""
	}
	return claims.Subject
}
