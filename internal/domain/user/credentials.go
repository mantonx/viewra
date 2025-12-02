package user

// Credentials represents login credentials.
type Credentials struct {
	Username string
	Password string
}

// Validate validates the credentials.
func (c *Credentials) Validate() error {
	if c.Username == "" {
		return ErrUsernameEmpty
	}
	if c.Password == "" {
		return ErrPasswordEmpty
	}
	return nil
}

// RegistrationRequest represents a user registration request.
type RegistrationRequest struct {
	Username    string
	DisplayName string
	Password    string
}

// Validate validates the registration request.
func (r *RegistrationRequest) Validate() error {
	if err := ValidateUsername(r.Username); err != nil {
		return err
	}
	if err := ValidateDisplayName(r.DisplayName); err != nil {
		return err
	}
	if err := ValidatePassword(r.Password); err != nil {
		return err
	}
	return nil
}

// PasswordChangeRequest represents a password change request.
type PasswordChangeRequest struct {
	CurrentPassword string
	NewPassword     string
}

// Validate validates the password change request.
func (r *PasswordChangeRequest) Validate() error {
	if r.CurrentPassword == "" {
		return ErrPasswordEmpty
	}
	if err := ValidatePassword(r.NewPassword); err != nil {
		return err
	}
	if r.CurrentPassword == r.NewPassword {
		return ErrPasswordSameAsOld
	}
	return nil
}
