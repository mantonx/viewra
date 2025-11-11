package library

import "errors"

// Domain errors for library operations
var (
	// ErrLibraryNotFound is returned when a library cannot be found
	ErrLibraryNotFound = errors.New("library not found")

	// ErrInvalidName is returned when the library name is empty
	ErrInvalidName = errors.New("library name cannot be empty")

	// ErrNameTooLong is returned when the library name exceeds the maximum length
	ErrNameTooLong = errors.New("library name cannot exceed 100 characters")

	// ErrInvalidPath is returned when the library path is invalid
	ErrInvalidPath = errors.New("library path is invalid")

	// ErrEmptyPath is returned when the library path is empty
	ErrEmptyPath = errors.New("library path cannot be empty")

	// ErrPathNotAbsolute is returned when the library path is not absolute
	ErrPathNotAbsolute = errors.New("library path must be absolute")

	// ErrPathTraversal is returned when the path contains traversal attempts
	ErrPathTraversal = errors.New("library path contains invalid traversal")

	// ErrInvalidType is returned when the library type is not recognized
	ErrInvalidType = errors.New("invalid library type")

	// ErrDuplicatePath is returned when a library with the same path already exists
	ErrDuplicatePath = errors.New("library path already exists")

	// ErrPathNotAccessible is returned when the library path cannot be accessed
	ErrPathNotAccessible = errors.New("library path is not accessible")

	// ErrPathNotReadable is returned when the library path cannot be read
	ErrPathNotReadable = errors.New("library path is not readable")

	// ErrPathNotFound is returned when the library path does not exist
	ErrPathNotFound = errors.New("library path does not exist")

	// ErrPathNotDirectory is returned when the library path is not a directory
	ErrPathNotDirectory = errors.New("library path is not a directory")
)
