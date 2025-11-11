package ffmpeg

import "errors"

// ErrFFmpegNotFound is returned when the FFmpeg executable is not found in the system PATH.
var ErrFFmpegNotFound = errors.New("ffmpeg executable not found in system PATH")

// ErrFFprobeNotFound is returned when the FFprobe executable is not found in the system PATH.
var ErrFFprobeNotFound = errors.New("ffprobe executable not found in system PATH")

// ErrInvalidFile is returned when the provided file path is invalid or inaccessible.
var ErrInvalidFile = errors.New("invalid or inaccessible file path")

// ErrMetadataExtraction is returned when metadata extraction fails.
var ErrMetadataExtraction = errors.New("failed to extract metadata from file")

// ErrThumbnailGeneration is returned when thumbnail generation fails.
var ErrThumbnailGeneration = errors.New("failed to generate thumbnail")
