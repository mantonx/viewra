package user

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// User represents a user in the system.
// This is the aggregate root for user-related operations.
type User struct {
	ID           int64  // Internal database ID (for joins/FKs)
	PublicID     string // External opaque ID (e.g., "usr_abc123") for API responses
	Username     string
	DisplayName  string
	PasswordHash string
	IsAdmin      bool
	IsDisabled   bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Validation constants
const (
	MinUsernameLength = 3
	MaxUsernameLength = 32
	MinPasswordLength = 8
	MaxDisplayNameLen = 64
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// NewUser creates a new user with the given details.
// The password should already be hashed before passing to this function.
// ID is set to 0 and will be assigned by the database on insert.
func NewUser(publicID, username, displayName, passwordHash string, isAdmin bool) *User {
	now := time.Now()
	return &User{
		ID:           0, // Assigned by database
		PublicID:     publicID,
		Username:     strings.ToLower(username),
		DisplayName:  displayName,
		PasswordHash: passwordHash,
		IsAdmin:      isAdmin,
		IsDisabled:   false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// ValidateUsername checks if a username meets the requirements.
func ValidateUsername(username string) error {
	if len(username) < MinUsernameLength {
		return ErrUsernameTooShort
	}
	if len(username) > MaxUsernameLength {
		return ErrUsernameTooLong
	}
	if !usernameRegex.MatchString(username) {
		return ErrUsernameInvalidChars
	}
	return nil
}

// ValidatePassword checks if a password meets the minimum requirements.
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}

// ValidateDisplayName checks if a display name meets the requirements.
func ValidateDisplayName(displayName string) error {
	if utf8.RuneCountInString(displayName) == 0 {
		return ErrDisplayNameEmpty
	}
	if utf8.RuneCountInString(displayName) > MaxDisplayNameLen {
		return ErrDisplayNameTooLong
	}
	return nil
}

// Validate validates the user entity.
func (u *User) Validate() error {
	if u.PublicID == "" {
		return ErrUserIDEmpty
	}
	if err := ValidateUsername(u.Username); err != nil {
		return err
	}
	if err := ValidateDisplayName(u.DisplayName); err != nil {
		return err
	}
	if u.PasswordHash == "" {
		return ErrPasswordHashEmpty
	}
	return nil
}

// UpdateDisplayName updates the user's display name.
func (u *User) UpdateDisplayName(displayName string) error {
	if err := ValidateDisplayName(displayName); err != nil {
		return err
	}
	u.DisplayName = displayName
	u.UpdatedAt = time.Now()
	return nil
}

// UpdatePassword updates the user's password hash.
func (u *User) UpdatePassword(passwordHash string) {
	u.PasswordHash = passwordHash
	u.UpdatedAt = time.Now()
}

// SetAdmin sets the user's admin status.
func (u *User) SetAdmin(isAdmin bool) {
	u.IsAdmin = isAdmin
	u.UpdatedAt = time.Now()
}

// Disable disables the user account.
func (u *User) Disable() {
	u.IsDisabled = true
	u.UpdatedAt = time.Now()
}

// Enable enables the user account.
func (u *User) Enable() {
	u.IsDisabled = false
	u.UpdatedAt = time.Now()
}

// CanAuthenticate returns true if the user can log in.
func (u *User) CanAuthenticate() bool {
	return !u.IsDisabled
}
