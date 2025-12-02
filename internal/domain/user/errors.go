package user

import "errors"

// User validation errors
var (
	ErrUserIDEmpty         = errors.New("user ID cannot be empty")
	ErrUsernameEmpty       = errors.New("username cannot be empty")
	ErrUsernameTooShort    = errors.New("username must be at least 3 characters")
	ErrUsernameTooLong     = errors.New("username must be at most 32 characters")
	ErrUsernameInvalidChars = errors.New("username can only contain letters, numbers, and underscores")
	ErrDisplayNameEmpty    = errors.New("display name cannot be empty")
	ErrDisplayNameTooLong  = errors.New("display name must be at most 64 characters")
	ErrPasswordEmpty       = errors.New("password cannot be empty")
	ErrPasswordTooShort    = errors.New("password must be at least 8 characters")
	ErrPasswordHashEmpty   = errors.New("password hash cannot be empty")
	ErrPasswordSameAsOld   = errors.New("new password must be different from current password")
)

// Session validation errors
var (
	ErrSessionIDEmpty        = errors.New("session ID cannot be empty")
	ErrSessionUserIDEmpty    = errors.New("session user ID cannot be empty")
	ErrRefreshTokenHashEmpty = errors.New("refresh token hash cannot be empty")
	ErrSessionExpiresAtEmpty = errors.New("session expires_at cannot be empty")
)

// Repository errors
var (
	ErrUserNotFound     = errors.New("user not found")
	ErrUsernameExists   = errors.New("username already exists")
	ErrSessionNotFound  = errors.New("session not found")
	ErrSessionExpired   = errors.New("session has expired")
)

// Authentication errors
var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserDisabled       = errors.New("user account is disabled")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrTokenExpired       = errors.New("token has expired")
	ErrRefreshTokenInvalid = errors.New("refresh token is invalid or expired")
	ErrMaxSessionsReached = errors.New("maximum number of sessions reached")
)
