package moviestructure

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/pluginmodule"
	"github.com/mantonx/viewra/internal/utils"
	"gorm.io/gorm"
)

// Register Movie Structure core plugin with the global registry
func init() {
	pluginmodule.RegisterCorePluginFactory("movie_structure", func() pluginmodule.CorePlugin {
		return NewMovieStructureCorePlugin()
	})
}

// MovieStructureCorePlugin implements the CorePlugin interface for movie files
type MovieStructureCorePlugin struct {
	name          string
	supportedExts []string
	enabled       bool
	initialized   bool
}

// MovieInfo holds parsed movie information
type MovieInfo struct {
	Title        string
	Year         int
	ImdbID       string
	Resolution   string
	Source       string // Remux, WEBDL, Bluray, etc.
	Quality      string // 2160p, 1080p, etc.
	AudioCodec   string
	VideoCodec   string
	ReleaseGroup string
}

// NewMovieStructureCorePlugin creates a new movie structure parser core plugin instance
func NewMovieStructureCorePlugin() pluginmodule.CorePlugin {
	return &MovieStructureCorePlugin{
		name:    "movie_structure_parser_core_plugin",
		enabled: true,
		supportedExts: []string{
			// Video formats commonly used for movies
			".mkv", ".mp4", ".avi", ".mov", ".wmv",
			".flv", ".webm", ".m4v", ".ts", ".mts", ".m2ts",
			".mpg", ".mpeg", ".ogv",
		},
	}
}

// GetName returns the plugin name (implements FileHandlerPlugin)
func (p *MovieStructureCorePlugin) GetName() string {
	return p.name
}

// GetPluginType returns the plugin type (implements FileHandlerPlugin)
func (p *MovieStructureCorePlugin) GetPluginType() string {
	return "movie_structure_parser"
}

// GetType returns the plugin type (implements BasePlugin)
func (p *MovieStructureCorePlugin) GetType() string {
	return "movies"
}

// GetDisplayName returns a human-readable display name for the plugin (implements CorePlugin)
func (p *MovieStructureCorePlugin) GetDisplayName() string {
	return "Movie Structure Core Plugin"
}

// GetSupportedExtensions returns the file extensions this plugin supports (implements FileHandlerPlugin)
func (p *MovieStructureCorePlugin) GetSupportedExtensions() []string {
	return p.supportedExts
}

// IsEnabled returns whether the plugin is enabled (implements CorePlugin)
func (p *MovieStructureCorePlugin) IsEnabled() bool {
	return p.enabled
}

// Enable enables the plugin (implements CorePlugin)
func (p *MovieStructureCorePlugin) Enable() error {
	p.enabled = true
	return p.Initialize()
}

// Disable disables the plugin (implements CorePlugin)
func (p *MovieStructureCorePlugin) Disable() error {
	p.enabled = false
	return p.Shutdown()
}

// Initialize performs any setup needed for the plugin (implements CorePlugin)
func (p *MovieStructureCorePlugin) Initialize() error {
	if p.initialized {
		return nil
	}

	fmt.Printf("DEBUG: Initializing Movie Structure Parser Core Plugin\n")
	fmt.Printf("DEBUG: Movie Structure Parser plugin supports %d file types: %v\n", len(p.supportedExts), p.supportedExts)

	p.initialized = true
	fmt.Printf("✅ Movie Structure Parser initialized - Movie metadata parsing available\n")
	return nil
}

// Shutdown performs any cleanup needed when the plugin is disabled (implements CorePlugin)
func (p *MovieStructureCorePlugin) Shutdown() error {
	fmt.Printf("DEBUG: Shutting down Movie Structure Parser Core Plugin\n")
	p.initialized = false
	return nil
}

// Match determines if this plugin can handle the given file (implements FileHandlerPlugin)
func (p *MovieStructureCorePlugin) Match(path string, info fs.FileInfo) bool {
	if !p.enabled || !p.initialized {
		return false
	}

	// Skip directories
	if info.IsDir() {
		return false
	}

	// Skip trailer files
	if strings.Contains(strings.ToLower(path), "trailer") {
		return false
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	for _, supportedExt := range p.supportedExts {
		if ext == supportedExt {
			return true
		}
	}

	return false
}

// HandleFile processes a movie file and extracts structure metadata (implements FileHandlerPlugin)
func (p *MovieStructureCorePlugin) HandleFile(path string, ctx *pluginmodule.MetadataContext) error {
	if !p.enabled || !p.initialized {
		return fmt.Errorf("Movie Structure Parser plugin is disabled or not initialized")
	}

	// Check if we support this file extension
	ext := strings.ToLower(filepath.Ext(path))
	if !p.isExtensionSupported(ext) {
		return fmt.Errorf("unsupported file extension: %s", ext)
	}

	// Get database connection from context
	db := ctx.DB

	// IMPORTANT: Only process files from movie libraries, not TV libraries
	// Check if the media file belongs to a movie library
	if ctx.MediaFile != nil && ctx.MediaFile.LibraryID != 0 {
		var library database.MediaLibrary
		if err := db.First(&library, ctx.MediaFile.LibraryID).Error; err != nil {
			return fmt.Errorf("failed to get library info: %w", err)
		}

		// Only process if this is a movie library
		if library.Type != "movie" {
			fmt.Printf("DEBUG: Skipping file %s - not from movie library (library type: %s)\n", path, library.Type)
			return nil
		}

		fmt.Printf("DEBUG: Processing movie file from movie library: %s\n", path)
	}

	// Parse movie information from file path
	movieInfo, err := p.parseMovieFromPath(path)
	if err != nil {
		return fmt.Errorf("failed to parse movie info: %w", err)
	}

	if movieInfo == nil {
		// Not a recognizable movie file, skip without error
		fmt.Printf("DEBUG: File %s doesn't match movie patterns, skipping\n", path)
		return nil
	}

	fmt.Printf("DEBUG: Parsed movie info: %+v\n", movieInfo)

	// Create movie structure in database
	err = p.createMovieStructure(db, ctx.MediaFile, movieInfo, ctx.PluginID)
	if err != nil {
		return fmt.Errorf("failed to create movie structure: %w", err)
	}

	fmt.Printf("✅ Successfully processed movie file: %s -> %s (%d)\n",
		filepath.Base(path), movieInfo.Title, movieInfo.Year)

	return nil
}

// isExtensionSupported checks if the file extension is supported
func (p *MovieStructureCorePlugin) isExtensionSupported(ext string) bool {
	for _, supportedExt := range p.supportedExts {
		if ext == supportedExt {
			return true
		}
	}
	return false
}

// parseMovieFromPath extracts movie information from file path
func (p *MovieStructureCorePlugin) parseMovieFromPath(filePath string) (*MovieInfo, error) {
	// Extract filename from path
	filename := filepath.Base(filePath)

	// Remove file extension
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	fmt.Printf("DEBUG: Attempting to parse movie from filename: %s\n", nameWithoutExt)

	// Common movie patterns to match:
	// "Movie Title (Year) [imdbid-ttXXXXXX] - [Quality][Audio][Video]-Group"
	// "Movie Title (Year) - [Quality][Audio][Video]-Group"
	// "Movie Title (Year)"
	// "Movie Title.Year.Quality.Source-Group"

	var movieInfo *MovieInfo

	// Pattern 1: Standard format with IMDb ID
	if info := p.parseStandardFormat(nameWithoutExt); info != nil {
		fmt.Printf("DEBUG: Successfully parsed movie with standard format: %s (%d)\n", info.Title, info.Year)
		movieInfo = info
	}

	// Pattern 2: Simple format (Title (Year))
	if movieInfo == nil {
		if info := p.parseSimpleFormat(nameWithoutExt); info != nil {
			fmt.Printf("DEBUG: Successfully parsed movie with simple format: %s (%d)\n", info.Title, info.Year)
			movieInfo = info
		}
	}

	// Pattern 3: Dot-separated format
	if movieInfo == nil {
		if info := p.parseDotFormat(nameWithoutExt); info != nil {
			fmt.Printf("DEBUG: Successfully parsed movie with dot format: %s (%d)\n", info.Title, info.Year)
			movieInfo = info
		}
	}

	if movieInfo != nil {
		// Extract additional metadata from filename
		p.extractAdditionalMetadata(movieInfo, nameWithoutExt)

		// Clean up title
		movieInfo.Title = p.cleanMovieTitle(movieInfo.Title)
		
		fmt.Printf("DEBUG: Final movie info: title='%s', year=%d, quality='%s', source='%s'\n",
			movieInfo.Title, movieInfo.Year, movieInfo.Quality, movieInfo.Source)
	} else {
		fmt.Printf("DEBUG: Failed to parse movie info from: %s\n", nameWithoutExt)
	}

	return movieInfo, nil
}

// parseStandardFormat parses the standard format: "Movie Title (Year) [imdbid-ttXXXXXX] - [Quality][Audio][Video]-Group"
func (p *MovieStructureCorePlugin) parseStandardFormat(filename string) *MovieInfo {
	// Pattern: Movie Title (Year) [imdbid-ttXXXXXX] - [additional info]
	re := regexp.MustCompile(`^(.+?)\s*\((\d{4})\)\s*(?:\[imdbid-(tt\d+)\])?\s*-?\s*(.*)$`)
	matches := re.FindStringSubmatch(filename)

	if len(matches) >= 3 {
		yearStr := matches[2]
		year, err := strconv.Atoi(yearStr)
		if err != nil {
			return nil
		}

		info := &MovieInfo{
			Title: strings.TrimSpace(matches[1]),
			Year:  year,
		}

		if len(matches) > 3 && matches[3] != "" {
			info.ImdbID = matches[3]
		}

		return info
	}

	return nil
}

// parseSimpleFormat parses simple format: "Movie Title (Year)"
func (p *MovieStructureCorePlugin) parseSimpleFormat(filename string) *MovieInfo {
	// Pattern: Movie Title (Year)
	re := regexp.MustCompile(`^(.+?)\s*\((\d{4})\)\s*$`)
	matches := re.FindStringSubmatch(filename)

	if len(matches) >= 3 {
		yearStr := matches[2]
		year, err := strconv.Atoi(yearStr)
		if err != nil {
			return nil
		}

		return &MovieInfo{
			Title: strings.TrimSpace(matches[1]),
			Year:  year,
		}
	}

	return nil
}

// parseDotFormat parses dot-separated format: "Movie.Title.Year.Quality.Source-Group"
func (p *MovieStructureCorePlugin) parseDotFormat(filename string) *MovieInfo {
	// Pattern: Movie.Title.Year.Quality.Source-Group
	parts := strings.Split(filename, ".")

	if len(parts) >= 3 {
		// Look for year in the parts
		var yearIndex = -1
		var year int

		for i, part := range parts {
			if len(part) == 4 {
				if y, err := strconv.Atoi(part); err == nil && y >= 1900 && y <= 2030 {
					year = y
					yearIndex = i
					break
				}
			}
		}

		if yearIndex > 0 {
			// Title is everything before the year
			titleParts := parts[:yearIndex]
			title := strings.Join(titleParts, " ")

			return &MovieInfo{
				Title: title,
				Year:  year,
			}
		}
	}

	return nil
}

// cleanMovieTitle cleans up movie title by removing common artifacts
func (p *MovieStructureCorePlugin) cleanMovieTitle(title string) string {
	// Remove extra whitespace
	title = strings.TrimSpace(title)

	// Remove leading/trailing special characters
	title = strings.Trim(title, ".-_")

	// Replace dots with spaces if it looks like a dot-separated title
	if strings.Count(title, ".") > strings.Count(title, " ") {
		title = strings.ReplaceAll(title, ".", " ")
	}

	// Replace underscores with spaces
	title = strings.ReplaceAll(title, "_", " ")

	// Clean up multiple spaces
	re := regexp.MustCompile(`\s+`)
	title = re.ReplaceAllString(title, " ")

	return strings.TrimSpace(title)
}

// extractAdditionalMetadata extracts quality, source, codecs, etc. from filename
func (p *MovieStructureCorePlugin) extractAdditionalMetadata(info *MovieInfo, filename string) {
	// Extract resolution/quality
	qualityRegex := regexp.MustCompile(`\b(2160p|1080p|720p|480p|4K|UHD)\b`)
	if match := qualityRegex.FindString(filename); match != "" {
		info.Quality = match
		info.Resolution = match
	}

	// Extract source
	sourceRegex := regexp.MustCompile(`\b(Remux|WEBDL|Bluray|BluRay|DVD|HDTV|WEB-DL|WEBRip|BDRip|DVDRip|CAM|TS)\b`)
	if match := sourceRegex.FindString(filename); match != "" {
		info.Source = match
	}

	// Extract video codec
	videoCodecRegex := regexp.MustCompile(`\b(HEVC|h265|x265|h264|x264|AVC|VC1|XviD|DivX)\b`)
	if match := videoCodecRegex.FindString(filename); match != "" {
		info.VideoCodec = match
	}

	// Extract audio codec
	audioCodecRegex := regexp.MustCompile(`\b(DTS-HD\s*MA|DTS-HD|TrueHD|Atmos|DTS-X|DTS|AC3|EAC3|AAC|MP3|FLAC|PCM)\b`)
	if match := audioCodecRegex.FindString(filename); match != "" {
		info.AudioCodec = match
	}

	// Extract release group (usually after the last dash)
	groupRegex := regexp.MustCompile(`-([A-Za-z0-9]+)(?:\.[a-z]+)?$`)
	if matches := groupRegex.FindStringSubmatch(filename); len(matches) > 1 {
		info.ReleaseGroup = matches[1]
	}
}

// createMovieStructure creates or updates movie records in database
func (p *MovieStructureCorePlugin) createMovieStructure(db *gorm.DB, mediaFile *database.MediaFile, movieInfo *MovieInfo, pluginID string) error {
	// Create or get the movie record
	movie, err := p.createOrGetMovie(db, movieInfo)
	if err != nil {
		return fmt.Errorf("failed to create/get movie: %w", err)
	}

	// Update the media file to link to the movie
	mediaFile.MediaID = movie.ID
	mediaFile.MediaType = database.MediaTypeMovie

	if err := db.Save(mediaFile).Error; err != nil {
		return fmt.Errorf("failed to update media file: %w", err)
	}

	// Create MediaEnrichment record to track that this plugin processed the media
	enrichment := database.MediaEnrichment{
		MediaID:   movie.ID,
		MediaType: database.MediaTypeMovie,
		Plugin:    pluginID,
		Payload:   fmt.Sprintf("{\"title\":\"%s\",\"year\":%d,\"source\":\"filename\"}", movieInfo.Title, movieInfo.Year),
		UpdatedAt: time.Now(),
	}

	// Use raw SQL INSERT OR REPLACE since the table doesn't have proper primary key constraints
	result := db.Exec(`
		INSERT OR REPLACE INTO media_enrichments (media_id, media_type, plugin, payload, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, enrichment.MediaID, enrichment.MediaType, enrichment.Plugin, enrichment.Payload, enrichment.UpdatedAt)

	if result.Error != nil {
		return fmt.Errorf("failed to create enrichment record: %w", result.Error)
	}

	fmt.Printf("✅ Created/linked movie: %s (%d) -> Media File: %s (Plugin: %s)\n",
		movie.Title, movieInfo.Year, mediaFile.ID, pluginID)

	return nil
}

// createOrGetMovie creates a new movie or returns existing one
func (p *MovieStructureCorePlugin) createOrGetMovie(db *gorm.DB, movieInfo *MovieInfo) (*database.Movie, error) {
	var movie database.Movie

	// Try to find existing movie by title and year (SQLite compatible)
	result := db.Where("title = ? AND strftime('%Y', release_date) = ?", movieInfo.Title, fmt.Sprintf("%d", movieInfo.Year)).First(&movie)

	if result.Error == nil {
		// Movie exists, return it
		return &movie, nil
	}

	if result.Error != gorm.ErrRecordNotFound {
		// Some other database error
		return nil, result.Error
	}

	// Create new movie
	var releaseDate *time.Time
	if movieInfo.Year > 0 {
		date := time.Date(movieInfo.Year, 1, 1, 0, 0, 0, 0, time.UTC)
		releaseDate = &date
	}

	movie = database.Movie{
		ID:          utils.GenerateUUID(),
		Title:       movieInfo.Title,
		ReleaseDate: releaseDate,
		ImdbID:      movieInfo.ImdbID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := db.Create(&movie).Error; err != nil {
		return nil, fmt.Errorf("failed to create movie: %w", err)
	}

	fmt.Printf("✅ Created new movie: %s (%d)\n", movie.Title, movieInfo.Year)
	return &movie, nil
}
