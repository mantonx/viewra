package thumbnail

import "errors"

// ErrInvalidFile is returned when the provided file path is invalid or inaccessible.
var ErrInvalidFile = errors.New("invalid or inaccessible file path")

// ErrThumbnailGeneration is returned when thumbnail generation fails.
var ErrThumbnailGeneration = errors.New("failed to generate thumbnail")
