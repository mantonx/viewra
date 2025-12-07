package probe

import "errors"

// ErrInvalidFile is returned when the provided file path is invalid or inaccessible.
var ErrInvalidFile = errors.New("invalid or inaccessible file path")

// ErrMetadataExtraction is returned when metadata extraction fails.
var ErrMetadataExtraction = errors.New("failed to extract metadata from file")
