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

// ScanPhase represents the current phase of an active scan
type ScanPhase string

const (
	ScanPhaseDiscovering ScanPhase = "discovering" // Walking directory tree to find files
	ScanPhaseProcessing  ScanPhase = "processing"  // Processing discovered files with FFprobe
	ScanPhaseCompleted   ScanPhase = "completed"   // Scan finished
)

// ScanJob represents a library scanning operation
type ScanJob struct {
	ID              int64
	LibraryID       int64
	Status          ScanStatus
	Progress        float64
	FilesFound      int64
	FilesProcessed  int64
	BytesProcessed  int64
	ErrorCount      int64
	WarningCount    int64  // Number of files processed with warnings
	StartedAt       time.Time
	CompletedAt     *time.Time
	ErrorMessage    string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Phase           ScanPhase // Current scan phase
	EstimatedTotal  int64     // Estimated total files from previous scan
	DiscoveryDone   bool      // Whether file discovery is complete
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

	// Warning tracking for non-fatal issues (e.g., metadata extraction failures)
	Warning         error  // Non-fatal warning message
	WarningCategory string // Category of warning (e.g., "ffmpeg", "metadata")

	// Technical video metadata (from FFmpeg)
	FileSize        int64
	Width           int
	Height          int
	VideoCodec      string
	AudioCodec      string
	Bitrate         int64
	FrameRate       float64
	ContainerFormat string

	// Advanced video quality metadata
	CodecProfile   string
	ScanType       string
	HDRFormat      string
	ColorSpace     string
	ColorPrimaries string
}

// Progress tracks scanning progress with thread-safe counters
type Progress struct {
	FilesFound      int64
	FilesProcessed  int64
	BytesProcessed  int64
	ErrorCount      int64
	StartTime       time.Time
	LastUpdate      time.Time
	Phase           ScanPhase // Current phase of the scan
	EstimatedTotal  int64     // Estimated total files from previous scan (0 if unknown)
	DiscoveryDone   bool      // True when file discovery is complete
}

// GetPercentage calculates the completion percentage
// Uses estimated total during discovery phase for more accurate progress
func (p *Progress) GetPercentage() float64 {
	// Determine the denominator (total files to process)
	total := p.FilesFound

	// If discovery is not done and we have an estimate, use it for better accuracy
	if !p.DiscoveryDone && p.EstimatedTotal > 0 && p.EstimatedTotal > p.FilesFound {
		total = p.EstimatedTotal
	}

	if total == 0 {
		return 0
	}

	percentage := float64(p.FilesProcessed) / float64(total) * 100
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
	// Basic metadata
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	TrackNumber int
	DiscNumber  int
	Year        int
	Genre       string
	Duration    int // In seconds
	Composer    string

	// Extended metadata
	TotalTracks   int    // Total tracks on disc/album
	TotalDiscs    int    // Total discs in album
	ReleaseDate   string // ISO 8601 date string (YYYY-MM-DD)
	Lyricist      string // Lyric writer
	ISRC          string // International Standard Recording Code
	ReleaseType   string // album, single, ep, compilation, live, remix, soundtrack
	Compilation   bool   // Compilation album flag
	OriginalTitle string // Title in original language
	Publisher     string // Record label

	// MusicBrainz IDs (for future plugin use)
	MusicBrainzTrackID  string
	MusicBrainzAlbumID  string
	MusicBrainzArtistID string
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
