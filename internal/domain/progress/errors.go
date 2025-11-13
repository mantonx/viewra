package progress

import "errors"

// Domain-specific errors for watch progress.
// These errors represent business rule violations.
var (
	ErrInvalidMediaID           = errors.New("invalid media ID")
	ErrInvalidProgress          = errors.New("progress seconds cannot be negative")
	ErrInvalidDuration          = errors.New("duration seconds cannot be negative")
	ErrProgressExceedsDuration  = errors.New("progress cannot exceed duration")
	ErrProgressNotFound         = errors.New("watch progress not found")
	ErrProgressAlreadyExists    = errors.New("watch progress already exists for this media")
)
