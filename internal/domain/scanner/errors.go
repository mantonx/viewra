package scanner

import "errors"

// Domain errors for scanner operations
var (
	// ErrNotFound is returned when a scan job is not found
	ErrNotFound = errors.New("scan job not found")

	// ErrInvalidPath is returned when a library path is invalid
	ErrInvalidPath = errors.New("invalid library path")

	// ErrPathNotExist is returned when a library path does not exist
	ErrPathNotExist = errors.New("library path does not exist")

	// ErrPathNotDirectory is returned when a library path is not a directory
	ErrPathNotDirectory = errors.New("library path is not a directory")

	// ErrAlreadyRunning is returned when trying to start an already running scan
	ErrAlreadyRunning = errors.New("scan already running for this library")

	// ErrNotRunning is returned when trying to pause/cancel a non-running scan
	ErrNotRunning = errors.New("scan is not running")

	// ErrInvalidStatus is returned when a status transition is invalid
	ErrInvalidStatus = errors.New("invalid scan status")
)
