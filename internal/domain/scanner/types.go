package scanner

import (
	"time"
)

// ScanStatus represents the state of a scan job
type ScanStatus string

const (
	ScanStatusPending   ScanStatus = "pending"
	ScanStatusRunning   ScanStatus = "running"
	ScanStatusPaused    ScanStatus = "paused"
	ScanStatusCompleted ScanStatus = "completed"
	ScanStatusFailed    ScanStatus = "failed"
)

// ScanJob represents a library scanning operation
type ScanJob struct {
	ID             int64
	LibraryID      int64
	Status         ScanStatus
	Progress       float64
	FilesFound     int64
	FilesProcessed int64
	BytesProcessed int64
	ErrorCount     int64
	StartedAt      time.Time
	CompletedAt    *time.Time
	ErrorMessage   string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// FileInfo represents a discovered file during scanning
type FileInfo struct {
	Path      string
	Size      int64
	ModTime   time.Time
	Extension string
	IsDir     bool
}

// MediaType represents the type of media content
type MediaType string

const (
	MediaTypeMovie   MediaType = "movie"
	MediaTypeEpisode MediaType = "episode"
	MediaTypeTrack   MediaType = "track"
	MediaTypeUnknown MediaType = "unknown"
)

// ScanResult represents the outcome of processing a file
type ScanResult struct {
	FilePath       string
	MediaType      MediaType
	Title          string
	Year           *int
	SeasonNumber   *int // TV episodes only
	EpisodeNumber  *int // TV episodes only
	Artist         string
	Album          string
	TrackNumber    *int
	Duration       int64 // Seconds
	Hash           string
	Error          error
	BytesProcessed int64

	// Technical video metadata (from FFmpeg)
	FileSize        int64
	Width           int
	Height          int
	VideoCodec      string
	AudioCodec      string
	Bitrate         int64
	FrameRate       float64
	ContainerFormat string
}

// Progress tracks scanning progress with thread-safe counters
type Progress struct {
	FilesFound     int64
	FilesProcessed int64
	BytesProcessed int64
	ErrorCount     int64
	StartTime      time.Time
	LastUpdate     time.Time
}

// GetPercentage calculates the completion percentage
func (p *Progress) GetPercentage() float64 {
	if p.FilesFound == 0 {
		return 0
	}
	percentage := float64(p.FilesProcessed) / float64(p.FilesFound) * 100
	if percentage > 100 {
		return 100
	}
	return percentage
}

// HashingStrategy defines how file hashing is performed
type HashingStrategy string

const (
	// HashingStrategyAlways always computes hash for every file
	HashingStrategyAlways HashingStrategy = "always"
	// HashingStrategyOnConflict only hashes files with duplicate sizes
	HashingStrategyOnConflict HashingStrategy = "on_conflict"
	// HashingStrategyDisabled disables hashing (no duplicate detection)
	HashingStrategyDisabled HashingStrategy = "disabled"
)

// FileCacheEntry represents cached metadata for a file
type FileCacheEntry struct {
	Path      string
	Size      int64
	ModTime   time.Time
	Hash      string
	MediaType MediaType

	// Parsed metadata (avoid reparsing)
	Title         string
	Artist        string
	Album         string
	Year          *int
	SeasonNumber  *int
	EpisodeNumber *int
	TrackNumber   *int
}

// IsUnchanged returns true if the file has not been modified
func (e *FileCacheEntry) IsUnchanged(modTime time.Time, size int64) bool {
	return e.ModTime.Equal(modTime) && e.Size == size
}

// MovieInfo contains metadata parsed from a movie filename
type MovieInfo struct {
	// Title is the movie name
	Title string

	// Year is the release year (optional, 0 if not found)
	Year int

	// Resolution is the video resolution (e.g., "720p", "1080p", "4K")
	Resolution string

	// Quality is the source quality (e.g., "BluRay", "WEB-DL", "HDTV")
	Quality string

	// ImdbID is the IMDb identifier (e.g., "tt1234567", empty if not found)
	ImdbID string

	// Edition is the movie edition (e.g., "Extended Cut", "Director's Cut", "Remastered")
	Edition string
}

// TVEpisodeInfo contains metadata parsed from a TV show filename
type TVEpisodeInfo struct {
	// ShowName is the TV show title
	ShowName string

	// Season is the season number (1-99)
	Season int

	// Episode is the episode number (1-999)
	Episode int

	// EpisodeEnd is the last episode number for multi-episode files (0 if single episode)
	EpisodeEnd int

	// EpisodeTitle is the episode name (optional)
	EpisodeTitle string

	// Year is the release year (optional, 0 if not found)
	Year int
}

// MusicInfo contains metadata for a music track
type MusicInfo struct {
	// Title is the track title
	Title string

	// Artist is the track artist
	Artist string

	// Album is the album name
	Album string

	// AlbumArtist is the album artist (may differ from track artist)
	AlbumArtist string

	// TrackNumber is the track number on the album (0 if not found)
	TrackNumber int

	// DiscNumber is the disc number for multi-disc albums (0 if not found)
	DiscNumber int

	// Year is the release year (0 if not found)
	Year int

	// Genre is the music genre
	Genre string

	// Duration is the track length in seconds (0 if not found)
	Duration int
}

// FilenameParser parses metadata from media filenames
type FilenameParser interface {
	// ParseMovie extracts movie metadata from a filename
	ParseMovie(filename string) (*MovieInfo, error)

	// ParseTVEpisode extracts TV show metadata from a filename
	ParseTVEpisode(filename string) (*TVEpisodeInfo, error)

	// ParseMusic extracts music metadata from a file (ID3 tags + filename fallback)
	ParseMusic(path string) (*MusicInfo, error)
}
