package media

import "errors"

// Media domain errors
var (
	// ErrMediaNotFound indicates the requested media was not found
	ErrMediaNotFound = errors.New("media not found")

	// ErrInvalidLibraryID indicates an invalid or missing library ID
	ErrInvalidLibraryID = errors.New("invalid library ID")

	// ErrEmptyTitle indicates the media title is empty
	ErrEmptyTitle = errors.New("media title cannot be empty")

	// ErrTitleTooLong indicates the media title exceeds maximum length
	ErrTitleTooLong = errors.New("media title too long (max 500 characters)")

	// ErrEmptyFilePath indicates the file path is empty
	ErrEmptyFilePath = errors.New("file path cannot be empty")

	// ErrAbsoluteFilePath indicates the file path is absolute (should be relative)
	ErrAbsoluteFilePath = errors.New("file path must be relative to library root")

	// ErrPathTraversal indicates the file path attempts directory traversal
	ErrPathTraversal = errors.New("file path cannot contain path traversal")

	// ErrMissingFileExtension indicates the file has no extension
	ErrMissingFileExtension = errors.New("file must have an extension")

	// ErrInvalidFileSize indicates an invalid file size (negative)
	ErrInvalidFileSize = errors.New("file size cannot be negative")

	// ErrInvalidDuration indicates an invalid duration (negative)
	ErrInvalidDuration = errors.New("duration cannot be negative")

	// ErrDuplicateFilePath indicates a media with the same file path already exists
	ErrDuplicateFilePath = errors.New("media with this file path already exists in library")

	// ErrInvalidMediaType indicates an unsupported media type
	ErrInvalidMediaType = errors.New("invalid media type")

	// Movie-specific errors

	// ErrInvalidYear indicates an invalid movie year
	ErrInvalidYear = errors.New("invalid year (must be between 1800 and 2100)")

	// ErrInvalidRating indicates an invalid rating value
	ErrInvalidRating = errors.New("invalid rating (must be between 0 and 10)")

	// TV-specific errors

	// ErrInvalidSeason indicates an invalid season number
	ErrInvalidSeason = errors.New("invalid season number (must be positive)")

	// ErrInvalidEpisode indicates an invalid episode number
	ErrInvalidEpisode = errors.New("invalid episode number (must be positive)")

	// Music-specific errors

	// ErrInvalidTrackNumber indicates an invalid track number
	ErrInvalidTrackNumber = errors.New("invalid track number (must be positive)")
)
