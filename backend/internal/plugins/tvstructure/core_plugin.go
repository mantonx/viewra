package tvstructure

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

// Register TV Structure core plugin with the global registry
func init() {
	pluginmodule.RegisterCorePluginFactory("tv_structure", func() pluginmodule.CorePlugin {
		return NewTVStructureCorePlugin()
	})
}

// TVStructureCorePlugin implements the CorePlugin interface for TV show files
type TVStructureCorePlugin struct {
	name          string
	supportedExts []string
	enabled       bool
	initialized   bool
}

// TVShowInfo holds parsed TV show information
type TVShowInfo struct {
	ShowName      string
	SeasonNumber  int
	EpisodeNumber int
	EpisodeTitle  string
	Year          int
	Resolution    string
	Source        string
	IsDateBased   bool
	AirDate       *time.Time
}

// NewTVStructureCorePlugin creates a new TV structure parser core plugin instance
func NewTVStructureCorePlugin() pluginmodule.CorePlugin {
	return &TVStructureCorePlugin{
		name:    "tv_structure_parser_core_plugin",
		enabled: true,
		supportedExts: []string{
			// Video formats commonly used for TV shows
			".mkv", ".mp4", ".avi", ".mov", ".wmv",
			".flv", ".webm", ".m4v", ".ts", ".mts", ".m2ts",
			".mpg", ".mpeg", ".ogv",
		},
	}
}

// GetName returns the plugin name (implements FileHandlerPlugin)
func (p *TVStructureCorePlugin) GetName() string {
	return p.name
}

// GetPluginType returns the plugin type (implements FileHandlerPlugin)
func (p *TVStructureCorePlugin) GetPluginType() string {
	return "tv_structure_parser"
}

// GetSupportedExtensions returns the file extensions this plugin supports (implements FileHandlerPlugin)
func (p *TVStructureCorePlugin) GetSupportedExtensions() []string {
	return p.supportedExts
}

// GetDisplayName returns a human-readable display name for the plugin (implements CorePlugin)
func (p *TVStructureCorePlugin) GetDisplayName() string {
	return "TV Structure Core Plugin"
}

// IsEnabled returns whether the plugin is enabled (implements CorePlugin)
func (p *TVStructureCorePlugin) IsEnabled() bool {
	return p.enabled
}

// Enable enables the plugin (implements CorePlugin)
func (p *TVStructureCorePlugin) Enable() error {
	p.enabled = true
	return p.Initialize()
}

// Disable disables the plugin (implements CorePlugin)
func (p *TVStructureCorePlugin) Disable() error {
	p.enabled = false
	return p.Shutdown()
}

// GetType returns the plugin type (implements BasePlugin)
func (p *TVStructureCorePlugin) GetType() string {
	return "tv"
}

// Initialize performs any setup needed for the plugin (implements CorePlugin)
func (p *TVStructureCorePlugin) Initialize() error {
	if p.initialized {
		return nil
	}

	fmt.Printf("DEBUG: Initializing TV Structure Parser Core Plugin\n")
	fmt.Printf("DEBUG: TV Structure Parser plugin supports %d file types: %v\n", len(p.supportedExts), p.supportedExts)

	p.initialized = true
	fmt.Printf("✅ TV Structure Parser initialized - TV show metadata parsing available\n")
	return nil
}

// Shutdown performs any cleanup needed when the plugin is disabled (implements CorePlugin)
func (p *TVStructureCorePlugin) Shutdown() error {
	fmt.Printf("DEBUG: Shutting down TV Structure Parser Core Plugin\n")
	p.initialized = false
	return nil
}

// Match determines if this plugin can handle the given file (implements FileHandlerPlugin)
func (p *TVStructureCorePlugin) Match(path string, info fs.FileInfo) bool {
	if !p.enabled || !p.initialized {
		return false
	}

	// Skip directories
	if info.IsDir() {
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

// HandleFile processes a TV show file and extracts structure metadata (implements FileHandlerPlugin)
func (p *TVStructureCorePlugin) HandleFile(path string, ctx *pluginmodule.MetadataContext) error {
	if !p.enabled || !p.initialized {
		return fmt.Errorf("TV Structure Parser plugin is disabled or not initialized")
	}

	// Check if we support this file extension
	ext := strings.ToLower(filepath.Ext(path))
	if !p.isExtensionSupported(ext) {
		return fmt.Errorf("unsupported file extension: %s", ext)
	}

	// Get database connection from context
	db := ctx.DB

	// Parse TV show information from file path
	showInfo, err := p.parseTVShowFromPath(path)
	if err != nil {
		return fmt.Errorf("failed to parse TV show info: %w", err)
	}

	if showInfo == nil {
		// Not a recognizable TV show file, skip without error
		fmt.Printf("DEBUG: File %s doesn't match TV show patterns, skipping\n", path)
		return nil
	}

	fmt.Printf("DEBUG: Parsed TV show info: %+v\n", showInfo)

	// Create TV show structure in database
	err = p.createTVShowStructure(db, ctx.MediaFile, showInfo, ctx.PluginID)
	if err != nil {
		return fmt.Errorf("failed to create TV show structure: %w", err)
	}

	fmt.Printf("✅ Successfully processed TV show file: %s -> %s S%02dE%02d\n",
		filepath.Base(path), showInfo.ShowName, showInfo.SeasonNumber, showInfo.EpisodeNumber)

	return nil
}

// isExtensionSupported checks if the file extension is supported
func (p *TVStructureCorePlugin) isExtensionSupported(ext string) bool {
	for _, supportedExt := range p.supportedExts {
		if ext == supportedExt {
			return true
		}
	}
	return false
}

// parseTVShowFromPath extracts TV show information from file path
func (p *TVStructureCorePlugin) parseTVShowFromPath(filePath string) (*TVShowInfo, error) {
	// Extract filename from path
	filename := filepath.Base(filePath)

	// Remove file extension
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	var showInfo *TVShowInfo

	// Pattern 1: Standard SxxExx format (highest priority)
	if info := p.parseSxxExx(nameWithoutExt, filepath.Dir(filePath)); info != nil {
		showInfo = info
	}

	// Pattern 2: Date-based episodes (e.g., "Show - 2013-02-08 - Episode Title")
	if showInfo == nil {
		if info := p.parseDateBasedEpisode(nameWithoutExt); info != nil {
			showInfo = info
		}
	}

	// Pattern 3: Folder structure analysis
	if showInfo == nil {
		if info := p.parseFromFolderStructure(filePath); info != nil {
			showInfo = info
		}
	}

	// Pattern 4: Episode in numbered season folder
	if showInfo == nil {
		if info := p.parseEpisodeInSeasonFolder(filePath); info != nil {
			showInfo = info
		}
	}

	if showInfo != nil {
		// Extract additional metadata from filename
		p.extractAdditionalMetadata(showInfo, nameWithoutExt)
	}

	return showInfo, nil
}

// parseDateBasedEpisode parses date-based episodes like "Show - 2013-02-08 - Episode Title"
func (p *TVStructureCorePlugin) parseDateBasedEpisode(filename string) *TVShowInfo {
	// Pattern: Show Name - YYYY-MM-DD - Episode Title
	// Also handles: Show Name - YYYY-MM-DD without episode title
	datePatterns := []string{
		`(.+?)\s*-\s*(\d{4})-(\d{1,2})-(\d{1,2})\s*-\s*(.+)`, // With episode title
		`(.+?)\s*-\s*(\d{4})-(\d{1,2})-(\d{1,2})`,            // Without episode title
		`(.+?)\s*(\d{4})-(\d{1,2})-(\d{1,2})\s*-\s*(.+)`,     // Alternative format
		`(.+?)\s*(\d{4})\.(\d{1,2})\.(\d{1,2})\s*-\s*(.+)`,   // Dot-separated date
	}

	for _, pattern := range datePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(filename)

		if len(matches) >= 4 {
			showName := strings.TrimSpace(matches[1])
			year, _ := strconv.Atoi(matches[2])
			month, _ := strconv.Atoi(matches[3])
			day, _ := strconv.Atoi(matches[4])

			// Validate date ranges
			if year < 1950 || year > 2030 || month < 1 || month > 12 || day < 1 || day > 31 {
				continue
			}

			episodeTitle := ""
			if len(matches) > 5 && matches[5] != "" {
				episodeTitle = strings.TrimSpace(matches[5])
			}

			// Clean up names
			showName = p.cleanShowName(showName)
			episodeTitle = p.cleanEpisodeTitle(episodeTitle)

			// For date-based episodes, we'll use year as season and a calculated episode number
			// This is a common pattern for talk shows, news shows, etc.
			seasonNumber := year

			// Calculate episode number as day of year for consistency
			date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
			episodeNumber := date.YearDay()

			return &TVShowInfo{
				ShowName:      showName,
				SeasonNumber:  seasonNumber,
				EpisodeNumber: episodeNumber,
				EpisodeTitle:  episodeTitle,
				Year:          year,
				IsDateBased:   true,
				AirDate:       &date,
			}
		}
	}

	return nil
}

// parseSxxExx parses the SxxExx format from filename
func (p *TVStructureCorePlugin) parseSxxExx(filename, dirPath string) *TVShowInfo {
	fmt.Printf("DEBUG: Attempting to parse SxxExx from filename: %s\n", filename)
	
	// Enhanced regex patterns for SxxExx format with better debugging
	patterns := []string{
		// Pattern 1: Most common S##E## format with optional show name and episode title
		`(?i)^(.+?)\s*[.\-\s]*s(\d{1,2})e(\d{1,2})(?:[.\-\s]*(.+?))?(?:\s*\[.*?\]|\s*$)`,
		
		// Pattern 2: Season/episode with words
		`(?i)^(.+?)\s*[.\-\s]*season\s*(\d{1,2})\s*episode\s*(\d{1,2})(?:[.\-\s]*(.+?))?(?:\s*\[.*?\]|\s*$)`,
		
		// Pattern 3: XxYY format (e.g., 1x01)
		`(?i)^(.+?)\s*[.\-\s]*(\d{1,2})x(\d{1,2})(?:[.\-\s]*(.+?))?(?:\s*\[.*?\]|\s*$)`,
		
		// Pattern 4: With year in parentheses
		`(?i)^(.+?)\s*\(\d{4}\)\s*[.\-\s]*s(\d{1,2})e(\d{1,2})(?:[.\-\s]*(.+?))?(?:\s*\[.*?\]|\s*$)`,
		
		// Pattern 5: Spaced S## E## format  
		`(?i)^(.+?)\s*[.\-\s]*s(\d{1,2})\s*e(\d{1,2})(?:[.\-\s]*(.+?))?(?:\s*\[.*?\]|\s*$)`,
		
		// Pattern 6: Dotted format (Show.Name.S##E##.Episode.Title)
		`(?i)^(.+?)\.s(\d{1,2})e(\d{1,2})(?:\.(.+?))?(?:\.[^.]*$|\s*$)`,
	}

	for patternIndex, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(filename)

		fmt.Printf("DEBUG: Testing pattern %d: %s\n", patternIndex+1, pattern)
		if len(matches) >= 4 {
			showName := strings.TrimSpace(matches[1])
			seasonStr := matches[2]
			episodeStr := matches[3]
			
			fmt.Printf("DEBUG: Pattern %d matched - raw values: show='%s', season='%s', episode='%s'\n", 
				patternIndex+1, showName, seasonStr, episodeStr)
				
			seasonNum, err1 := strconv.Atoi(seasonStr)
			episodeNum, err2 := strconv.Atoi(episodeStr)

			// Validate extraction was successful and numbers are reasonable
			if err1 != nil || err2 != nil {
				fmt.Printf("DEBUG: Pattern %d failed int conversion: season error=%v, episode error=%v\n", 
					patternIndex+1, err1, err2)
				continue
			}
			
			if seasonNum < 1 || seasonNum > 50 || episodeNum < 1 || episodeNum > 999 {
				fmt.Printf("DEBUG: Pattern %d failed range validation: season=%d, episode=%d\n", 
					patternIndex+1, seasonNum, episodeNum)
				continue
			}

			episodeTitle := ""
			if len(matches) > 4 && matches[4] != "" {
				episodeTitle = strings.TrimSpace(matches[4])
			}

			// Clean up names
			showName = p.cleanShowName(showName)
			episodeTitle = p.cleanEpisodeTitle(episodeTitle)

			// Extract year from show name if present
			year := p.extractYearFromName(showName)

			fmt.Printf("DEBUG: Successfully parsed with pattern %d: show='%s', S%02dE%02d, title='%s'\n",
				patternIndex+1, showName, seasonNum, episodeNum, episodeTitle)

			return &TVShowInfo{
				ShowName:      showName,
				SeasonNumber:  seasonNum,
				EpisodeNumber: episodeNum,
				EpisodeTitle:  episodeTitle,
				Year:          year,
			}
		} else {
			fmt.Printf("DEBUG: Pattern %d didn't match (matches count: %d)\n", patternIndex+1, len(matches))
		}
	}

	// If no pattern matched, try a more aggressive fallback approach
	fmt.Printf("DEBUG: No regex patterns matched, trying fallback approach for: %s\n", filename)
	
	// Look for any S##E## pattern anywhere in the filename
	fallbackRegex := regexp.MustCompile(`(?i)s(\d{1,2})e(\d{1,2})`)
	if matches := fallbackRegex.FindStringSubmatch(filename); len(matches) >= 3 {
		seasonNum, err1 := strconv.Atoi(matches[1])
		episodeNum, err2 := strconv.Atoi(matches[2])
		
		fmt.Printf("DEBUG: Fallback regex found S%dE%d, errors: %v, %v\n", seasonNum, episodeNum, err1, err2)
		
		if err1 == nil && err2 == nil && seasonNum >= 1 && seasonNum <= 50 && episodeNum >= 1 && episodeNum <= 999 {
			// Extract show name as everything before the S##E## pattern
			seIndex := strings.Index(strings.ToLower(filename), strings.ToLower(matches[0]))
			if seIndex > 0 {
				showPart := filename[:seIndex]
				showName := p.cleanShowName(strings.TrimSpace(showPart))
				if showName != "" {
					fmt.Printf("DEBUG: Fallback extraction successful: show='%s', S%02dE%02d\n",
						showName, seasonNum, episodeNum)
					
					return &TVShowInfo{
						ShowName:      showName,
						SeasonNumber:  seasonNum,
						EpisodeNumber: episodeNum,
						EpisodeTitle:  "",
						Year:          0,
					}
				}
			}
		}
	}

	fmt.Printf("DEBUG: All parsing attempts failed for: %s\n", filename)
	return nil
}

// extractYearFromName extracts year from show name (e.g., "Show Name (2024)")
func (p *TVStructureCorePlugin) extractYearFromName(name string) int {
	yearRegex := regexp.MustCompile(`\((\d{4})\)`)
	matches := yearRegex.FindStringSubmatch(name)
	if len(matches) >= 2 {
		if year, err := strconv.Atoi(matches[1]); err == nil {
			if year >= 1950 && year <= 2030 {
				return year
			}
		}
	}
	return 0
}

// parseFromFolderStructure parses TV show info from folder structure
func (p *TVStructureCorePlugin) parseFromFolderStructure(filePath string) *TVShowInfo {
	parts := strings.Split(filePath, string(filepath.Separator))
	if len(parts) < 3 {
		return nil
	}

	filename := filepath.Base(filePath)
	parentDir := filepath.Base(filepath.Dir(filePath))
	grandParentDir := ""

	if len(parts) >= 3 {
		grandParentDir = parts[len(parts)-3]
	}

	// Check if parent directory is a season folder
	seasonRegex := regexp.MustCompile(`(?i)season\s*(\d+)`)
	seasonMatches := seasonRegex.FindStringSubmatch(parentDir)

	if len(seasonMatches) >= 2 {
		seasonNum, _ := strconv.Atoi(seasonMatches[1])

		// Extract episode number from filename
		episodeRegex := regexp.MustCompile(`(?i)(?:episode\s*|ep\s*|e)(\d+)`)
		episodeMatches := episodeRegex.FindStringSubmatch(filename)

		if len(episodeMatches) >= 2 {
			episodeNum, _ := strconv.Atoi(episodeMatches[1])

			// Show name is likely the grandparent directory
			showName := p.cleanShowName(grandParentDir)

			// Extract episode title from filename
			episodeTitle := p.extractEpisodeTitleFromFilename(filename)

			return &TVShowInfo{
				ShowName:      showName,
				SeasonNumber:  seasonNum,
				EpisodeNumber: episodeNum,
				EpisodeTitle:  episodeTitle,
			}
		}
	}

	return nil
}

// parseEpisodeInSeasonFolder parses episode within a season-specific folder
func (p *TVStructureCorePlugin) parseEpisodeInSeasonFolder(filePath string) *TVShowInfo {
	parts := strings.Split(filePath, string(filepath.Separator))
	if len(parts) < 2 {
		return nil
	}

	filename := filepath.Base(filePath)
	parentDir := filepath.Base(filepath.Dir(filePath))

	// Check if we're in a numbered folder that could be a season
	if seasonNum, err := strconv.Atoi(parentDir); err == nil && seasonNum > 0 && seasonNum <= 50 {
		// Extract episode number from filename
		episodeRegex := regexp.MustCompile(`(?i)(\d+)`)
		matches := episodeRegex.FindAllString(filename, -1)

		if len(matches) > 0 {
			if episodeNum, err := strconv.Atoi(matches[0]); err == nil && episodeNum > 0 && episodeNum <= 999 {
				// Show name might be in a parent directory
				showName := "Unknown Show"
				if len(parts) >= 3 {
					showName = p.cleanShowName(parts[len(parts)-3])
				}

				episodeTitle := p.extractEpisodeTitleFromFilename(filename)

				return &TVShowInfo{
					ShowName:      showName,
					SeasonNumber:  seasonNum,
					EpisodeNumber: episodeNum,
					EpisodeTitle:  episodeTitle,
				}
			}
		}
	}

	return nil
}

// cleanShowName cleans up the show name
func (p *TVStructureCorePlugin) cleanShowName(name string) string {
	// Remove year pattern like "(2024)"
	yearRegex := regexp.MustCompile(`\s*\(\d{4}\)\s*`)
	name = yearRegex.ReplaceAllString(name, "")

	// Replace dots and underscores with spaces
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Remove common quality markers
	qualityRegex := regexp.MustCompile(`(?i)\s*(720p|1080p|4k|2160p|x264|x265|hevc|bluray|webrip|hdtv|dvdrip)\s*`)
	name = qualityRegex.ReplaceAllString(name, "")

	// Clean up extra whitespace
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)

	return name
}

// cleanEpisodeTitle cleans up the episode title
func (p *TVStructureCorePlugin) cleanEpisodeTitle(title string) string {
	if title == "" {
		return ""
	}

	// First, remove everything from the first bracket onwards (quality markers)
	// This handles cases like "Episode Title [WEBDL-2160p][EAC3 Atmos 5.1][h265]-GROUP"
	if idx := strings.Index(title, "["); idx > 0 {
		title = title[:idx]
	}

	// Remove everything from the first hyphen at the end (release groups)
	// This handles cases like "Episode Title - GROUP" or "Episode Title-GROUP"
	if idx := strings.LastIndex(title, " - "); idx > 0 {
		// Only remove if it looks like a release group (short, at the end)
		potential := strings.TrimSpace(title[idx+3:])
		if len(potential) <= 20 && !strings.Contains(potential, " ") {
			title = title[:idx]
		}
	}

	// Replace dots and underscores with spaces
	title = strings.ReplaceAll(title, ".", " ")
	title = strings.ReplaceAll(title, "_", " ")

	// Remove specific quality markers that might be inline
	qualityRegex := regexp.MustCompile(`(?i)\s*(720p|1080p|4k|2160p|x264|x265|hevc|h264|h265|bluray|webrip|hdtv|dvdrip|webdl|web-dl)\s*`)
	title = qualityRegex.ReplaceAllString(title, " ")

	// Clean up extra whitespace
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
	title = strings.TrimSpace(title)

	return title
}

// extractEpisodeTitleFromFilename extracts episode title from filename
func (p *TVStructureCorePlugin) extractEpisodeTitleFromFilename(filename string) string {
	// Remove file extension
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Try to extract title after episode number
	episodeRegex := regexp.MustCompile(`(?i)(?:episode\s*|ep\s*|e)(\d+)\s*[-.\s]*(.+)`)
	matches := episodeRegex.FindStringSubmatch(name)

	if len(matches) >= 3 {
		return p.cleanEpisodeTitle(matches[2])
	}

	// If no specific pattern, clean the whole filename
	return p.cleanEpisodeTitle(name)
}

// extractAdditionalMetadata extracts additional metadata from filename
func (p *TVStructureCorePlugin) extractAdditionalMetadata(info *TVShowInfo, filename string) {
	// Extract year
	yearRegex := regexp.MustCompile(`\((\d{4})\)`)
	if matches := yearRegex.FindStringSubmatch(filename); len(matches) >= 2 {
		if year, err := strconv.Atoi(matches[1]); err == nil {
			info.Year = year
		}
	}

	// Extract resolution
	resolutionRegex := regexp.MustCompile(`(?i)(720p|1080p|4k|2160p)`)
	if matches := resolutionRegex.FindStringSubmatch(filename); len(matches) >= 2 {
		info.Resolution = strings.ToUpper(matches[1])
	}

	// Extract source
	sourceRegex := regexp.MustCompile(`(?i)(bluray|webrip|hdtv|dvdrip|web-dl)`)
	if matches := sourceRegex.FindStringSubmatch(filename); len(matches) >= 2 {
		info.Source = strings.ToUpper(matches[1])
	}
}

// createTVShowStructure creates TV show, season, and episode records in the database
func (p *TVStructureCorePlugin) createTVShowStructure(db *gorm.DB, mediaFile *database.MediaFile, showInfo *TVShowInfo, pluginID string) error {
	// Create or get TV show
	tvShow, err := p.createOrGetTVShow(db, showInfo)
	if err != nil {
		return fmt.Errorf("failed to create TV show: %w", err)
	}

	// Create or get season
	season, err := p.createOrGetSeason(db, tvShow.ID, showInfo.SeasonNumber)
	if err != nil {
		return fmt.Errorf("failed to create season: %w", err)
	}

	// Create or get episode
	episode, err := p.createOrGetEpisode(db, season.ID, showInfo.EpisodeNumber, showInfo.EpisodeTitle)
	if err != nil {
		return fmt.Errorf("failed to create episode: %w", err)
	}

	// Update media file to link to the episode
	if err := db.Model(mediaFile).Updates(map[string]interface{}{
		"media_id":   episode.ID,
		"media_type": "episode",
	}).Error; err != nil {
		return fmt.Errorf("failed to link media file to episode: %w", err)
	}

	// Create MediaEnrichment record to track that this plugin processed the media
	enrichment := database.MediaEnrichment{
		MediaID:   episode.ID,
		MediaType: database.MediaTypeEpisode,
		Plugin:    pluginID,
		Payload: fmt.Sprintf("{\"show\":\"%s\",\"season\":%d,\"episode\":%d,\"title\":\"%s\",\"source\":\"filename\"}",
			showInfo.ShowName, showInfo.SeasonNumber, showInfo.EpisodeNumber, showInfo.EpisodeTitle),
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

	fmt.Printf("✅ Created TV show structure: %s -> Season %d -> Episode %d (%s) (Plugin: %s)\n",
		tvShow.Title, showInfo.SeasonNumber, showInfo.EpisodeNumber, episode.Title, pluginID)

	return nil
}

// createOrGetTVShow creates or retrieves a TV show record
func (p *TVStructureCorePlugin) createOrGetTVShow(db *gorm.DB, showInfo *TVShowInfo) (*database.TVShow, error) {
	// ENHANCED DUPLICATE PREVENTION: Check for existing TV show by multiple criteria
	var existingShow database.TVShow
	
	// First priority: Try to find existing TV show by exact title match (case insensitive)
	if err := db.Where("LOWER(title) = LOWER(?)", showInfo.ShowName).First(&existingShow).Error; err == nil {
		// Update the existing show with any new information we have
		updated := false
		
		// Update first air date if we have year info and existing record doesn't have it
		if showInfo.Year > 0 && existingShow.FirstAirDate == nil {
			firstAirDate := time.Date(showInfo.Year, 1, 1, 0, 0, 0, 0, time.UTC)
			existingShow.FirstAirDate = &firstAirDate
			updated = true
		}
		
		if updated {
			existingShow.UpdatedAt = time.Now()
			if err := db.Save(&existingShow).Error; err != nil {
				return nil, fmt.Errorf("failed to update existing TV show: %w", err)
			}
		}
		
		return &existingShow, nil
	}
	
	// Second priority: Check for similar titles that might be the same show
	// Look for titles that are very similar (helps with slight variations in naming)
	var similarShows []database.TVShow
	if err := db.Where("LOWER(title) LIKE ?", "%"+strings.ToLower(showInfo.ShowName)+"%").
		Or("LOWER(title) LIKE ?", "%"+strings.ToLower(strings.ReplaceAll(showInfo.ShowName, " ", ""))+"%").
		Limit(5).Find(&similarShows).Error; err == nil {
		
		// Check for exact matches after normalization
		normalizedTitle := strings.ToLower(strings.ReplaceAll(showInfo.ShowName, " ", ""))
		for _, show := range similarShows {
			existingNormalized := strings.ToLower(strings.ReplaceAll(show.Title, " ", ""))
			if existingNormalized == normalizedTitle {
				// Found a match after normalization
				return &show, nil
			}
		}
	}
	
	// Third priority: Create new TV show only if no existing record found
	tvShow := &database.TVShow{
		ID:        utils.GenerateUUID(),
		Title:     showInfo.ShowName,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Set first air date if year is available
	if showInfo.Year > 0 {
		firstAirDate := time.Date(showInfo.Year, 1, 1, 0, 0, 0, 0, time.UTC)
		tvShow.FirstAirDate = &firstAirDate
	}

	if err := db.Create(tvShow).Error; err != nil {
		return nil, fmt.Errorf("failed to create TV show: %w", err)
	}

	return tvShow, nil
}

// createOrGetSeason creates or retrieves a season record
func (p *TVStructureCorePlugin) createOrGetSeason(db *gorm.DB, tvShowID string, seasonNumber int) (*database.Season, error) {
	// First try to find existing season
	var existingSeason database.Season
	if err := db.Where("tv_show_id = ? AND season_number = ?", tvShowID, seasonNumber).First(&existingSeason).Error; err == nil {
		return &existingSeason, nil
	}

	// Create new season
	season := &database.Season{
		ID:           utils.GenerateUUID(),
		TVShowID:     tvShowID,
		SeasonNumber: seasonNumber,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := db.Create(season).Error; err != nil {
		return nil, fmt.Errorf("failed to create season: %w", err)
	}

	return season, nil
}

// createOrGetEpisode creates or retrieves an episode record
func (p *TVStructureCorePlugin) createOrGetEpisode(db *gorm.DB, seasonID string, episodeNumber int, episodeTitle string) (*database.Episode, error) {
	// First try to find existing episode
	var existingEpisode database.Episode
	if err := db.Where("season_id = ? AND episode_number = ?", seasonID, episodeNumber).First(&existingEpisode).Error; err == nil {
		// Update title if we have a better one
		if episodeTitle != "" && episodeTitle != existingEpisode.Title {
			existingEpisode.Title = episodeTitle
			existingEpisode.UpdatedAt = time.Now()
			db.Save(&existingEpisode)
		}
		return &existingEpisode, nil
	}

	// Generate episode title if not provided
	if episodeTitle == "" {
		episodeTitle = fmt.Sprintf("Episode %d", episodeNumber)
	}

	// Create new episode
	episode := &database.Episode{
		ID:            utils.GenerateUUID(),
		SeasonID:      seasonID,
		Title:         episodeTitle,
		EpisodeNumber: episodeNumber,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := db.Create(episode).Error; err != nil {
		return nil, fmt.Errorf("failed to create episode: %w", err)
	}

	return episode, nil
}
