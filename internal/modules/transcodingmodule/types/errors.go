// Package types provides types and interfaces for the transcoding module.
package types

// ValidationError represents a validation error with field context
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}