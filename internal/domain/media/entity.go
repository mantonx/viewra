package media

import (
	"path/filepath"
	"strings"
	"time"
)

// Media represents the base media entity with common fields shared across all media types.
type Media struct {
	ID        int64
	LibraryID int64
	Title     string
	Type      string // Type of media: 'movie', 'tv_episode', 'music_track'
	FilePath  string // Relative path from library root
	FileSize  int64  // In bytes
	FileHash  string // SHA-256 hash of file for integrity verification and duplicate detection
	Duration  int    // In seconds
	IsExtra   bool   // True for extras (trailers, deleted scenes, featurettes)

	// Technical video metadata
	Width           int     // Video width in pixels
	Height          int     // Video height in pixels
	VideoCodec      string  // Video codec (h264, hevc, etc.)
	AudioCodec      string  // Audio codec (aac, mp3, etc.)
	Bitrate         int64   // Overall bitrate in bits per second
	FrameRate       float64 // Frames per second
	ContainerFormat string  // File container format (mkv, mp4, etc.)

	// Advanced video quality metadata
	CodecProfile   string // Codec profile (High, Main, Baseline, etc.)
	ScanType       string // progressive or interlaced
	HDRFormat      string // HDR format (HDR10, Dolby Vision, HLG, etc.)
	ColorSpace     string // Color space (bt709, bt2020nc, etc.)
	ColorPrimaries string // Color primaries (bt709, bt2020, etc.)

	CreatedAt time.Time
	UpdatedAt time.Time
}

// IsValid validates the media entity fields.
// Returns domain-specific errors for invalid states.
func (m *Media) IsValid() error {
	if m.LibraryID <= 0 {
		return ErrInvalidLibraryID
	}

	if err := m.validateTitle(); err != nil {
		return err
	}

	if err := m.validateFilePath(); err != nil {
		return err
	}

	if m.FileSize < 0 {
		return ErrInvalidFileSize
	}

	if m.Duration < 0 {
		return ErrInvalidDuration
	}

	return nil
}

// validateTitle validates the media title field.
func (m *Media) validateTitle() error {
	title := strings.TrimSpace(m.Title)

	if title == "" {
		return ErrEmptyTitle
	}

	if len(title) > 500 {
		return ErrTitleTooLong
	}

	// Normalize title by removing extra whitespace
	m.Title = strings.Join(strings.Fields(title), " ")
	return nil
}

// validateFilePath validates the media file path.
// Media file paths must be relative to the library root.
func (m *Media) validateFilePath() error {
	if strings.TrimSpace(m.FilePath) == "" {
		return ErrEmptyFilePath
	}

	// Path must be relative, not absolute
	if filepath.IsAbs(m.FilePath) {
		return ErrAbsoluteFilePath
	}

	// Clean path to normalize
	cleanPath := filepath.Clean(m.FilePath)

	// Check for path traversal attempts (.. after cleaning)
	if strings.Contains(cleanPath, "..") {
		return ErrPathTraversal
	}

	m.FilePath = cleanPath

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(m.FilePath))
	if ext == "" {
		return ErrMissingFileExtension
	}

	return nil
}

// GetFileExtension returns the lowercase file extension without the dot.
func (m *Media) GetFileExtension() string {
	ext := filepath.Ext(m.FilePath)
	return strings.ToLower(strings.TrimPrefix(ext, "."))
}

// GetFileName returns the base filename without the extension.
func (m *Media) GetFileName() string {
	base := filepath.Base(m.FilePath)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}
