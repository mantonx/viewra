package services

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mantonx/viewra/plugins/tmdb_enricher_v2/internal/config"
	"github.com/mantonx/viewra/plugins/tmdb_enricher_v2/internal/models"
	"github.com/mantonx/viewra/plugins/tmdb_enricher_v2/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
	"gorm.io/gorm"
)

// EnrichmentService handles the core enrichment logic
type EnrichmentService struct {
	db            *gorm.DB
	config        *config.Config
	unifiedClient *plugins.UnifiedServiceClient
	logger        plugins.Logger
	lastAPICall   *time.Time
}

// NewEnrichmentService creates a new enrichment service
func NewEnrichmentService(db *gorm.DB, cfg *config.Config, client *plugins.UnifiedServiceClient, logger plugins.Logger) (*EnrichmentService, error) {
	return &EnrichmentService{
		db:            db,
		config:        cfg,
		unifiedClient: client,
		logger:        logger,
	}, nil
}

// ProcessMediaFile processes a media file for enrichment
func (s *EnrichmentService) ProcessMediaFile(mediaFileID string, filePath string, metadata map[string]string) error {
	s.logger.Info("processing media file for enrichment", "media_file_id", mediaFileID, "path", filePath)

	// Check if already enriched
	if !s.config.Features.OverwriteExisting {
		var existing models.TMDbEnrichment
		if err := s.db.Where("media_file_id = ?", mediaFileID).First(&existing).Error; err == nil {
			s.logger.Debug("media file already enriched, skipping", "media_file_id", mediaFileID)
			return nil
		}
	}

	// Extract title and year from filename or metadata
	title := s.extractTitle(filePath, metadata)
	year := s.extractYear(filePath, metadata)

	if title == "" {
		s.logger.Debug("no title extracted, skipping enrichment", "media_file_id", mediaFileID)
		return nil
	}

	s.logger.Debug("searching TMDb", "title", title, "year", year, "file_path", filePath)

	// Search for content
	results, err := s.searchContent(title, year)
	if err != nil {
		s.logger.Warn("failed to search for content", "error", err, "title", title)
		return nil
	}

	// Find best match
	bestMatch := s.findBestMatch(results, title, year, filePath)
	if bestMatch == nil {
		s.logger.Debug("no suitable match found", "title", title, "threshold", s.config.Matching.MatchThreshold)
		return nil
	}

	s.logger.Info("Found TMDb match", "media_file_id", mediaFileID, "title", title, "tmdb_id", bestMatch.ID, "match_title", s.getResultTitle(*bestMatch))

	// Save enrichment
	if err := s.saveEnrichment(mediaFileID, bestMatch); err != nil {
		s.logger.Warn("Failed to save enrichment", "error", err, "media_file_id", mediaFileID)
		return nil
	}

	s.logger.Info("Successfully enriched media file", "media_file_id", mediaFileID, "tmdb_id", bestMatch.ID)
	return nil
}

// searchContent searches TMDb for content
func (s *EnrichmentService) searchContent(title string, year int) ([]types.Result, error) {
	// Check cache first
	queryHash := s.generateQueryHash(fmt.Sprintf("search:%s:%d", title, year))
	if cached, err := s.getCachedResponse("search", queryHash); err == nil {
		s.logger.Debug("using cached search results", "title", title, "year", year, "results", len(cached))
		return cached, nil
	}

	// Build search URL
	baseURL := "https://api.themoviedb.org/3/search/multi"
	params := url.Values{}
	params.Set("query", title)
	params.Set("language", s.config.API.Language)
	params.Set("region", s.config.API.Region)
	if year > 0 && s.config.Matching.MatchYear {
		params.Set("year", strconv.Itoa(year))
		params.Set("first_air_date_year", strconv.Itoa(year))
	}

	searchURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	s.logger.Debug("searching TMDb", "title", title, "year", year, "url", searchURL)

	// Make API request with retries
	var searchResp types.SearchResponse
	err := s.makeAPIRequestWithRetries(searchURL, &searchResp, fmt.Sprintf("search for '%s'", title))
	if err != nil {
		return nil, err
	}

	// Cache the results
	s.cacheResults("search", queryHash, searchResp.Results)

	s.logger.Debug("search completed", "title", title, "results", len(searchResp.Results))
	return searchResp.Results, nil
}

// extractTitle extracts title from file path and metadata
func (s *EnrichmentService) extractTitle(filePath string, metadata map[string]string) string {
	// For TV shows, prioritize show/series name over episode title
	if seriesName, exists := metadata["series_name"]; exists && seriesName != "" {
		return s.cleanupTitle(seriesName)
	}
	if showName, exists := metadata["show_name"]; exists && showName != "" {
		return s.cleanupTitle(showName)
	}
	if artist, exists := metadata["artist"]; exists && artist != "" {
		cleaned := s.cleanupTitle(artist)
		if cleaned != "" {
			s.logger.Info("extracted show title from artist metadata", "artist", artist, "cleaned", cleaned)
			return cleaned
		}
	}

	// Try to get title from metadata
	if title, exists := metadata["title"]; exists && title != "" {
		if !s.looksLikeEpisodeTitle(title) {
			cleaned := s.cleanupTitle(title)
			if cleaned != "" {
				return cleaned
			}
		}
	}

	// Extract from filename
	filename := filepath.Base(filePath)
	filename = strings.TrimSuffix(filename, filepath.Ext(filename))

	s.logger.Info("extracting title from filename", "original", filename)

	// Try TV show patterns
	if title := s.extractTVShowTitle(filename, filePath); title != "" {
		s.logger.Debug("extracted TV show title", "title", title, "filename", filename)
		return title
	}

	// Try movie patterns
	if title := s.extractMovieTitle(filename); title != "" {
		s.logger.Debug("extracted movie title", "title", title, "filename", filename)
		return title
	}

	// Fallback: clean up the filename
	title := s.cleanupTitle(filename)
	s.logger.Debug("fallback title extraction", "title", title, "filename", filename)
	return title
}

// looksLikeEpisodeTitle checks if a title looks like an episode title
func (s *EnrichmentService) looksLikeEpisodeTitle(title string) bool {
	qualityPatterns := []string{
		"[", "]", "720p", "1080p", "2160p", "4K", "WEBDL", "WEB-DL", "BluRay", "BDRip",
		"DVDRip", "HDTV", "x264", "x265", "h264", "h265", "HEVC", "AAC", "AC3", "DTS",
		"5.1", "7.1", "2.0", "-", "Remux",
	}

	titleLower := strings.ToLower(title)
	qualityCount := 0
	for _, pattern := range qualityPatterns {
		if strings.Contains(titleLower, strings.ToLower(pattern)) {
			qualityCount++
		}
	}

	return qualityCount >= 2
}

// extractTVShowTitle handles various TV show filename patterns
func (s *EnrichmentService) extractTVShowTitle(filename, filePath string) string {
	// Pattern 1: "Show Name (Year) - S##E## - Episode Title [Quality]"
	seasonPatterns := []string{" - S", "- S", " -S", "-S"}
	for _, pattern := range seasonPatterns {
		if strings.Contains(filename, pattern) && strings.Contains(filename, "E") {
			seasonPos := strings.Index(filename, pattern)
			if seasonPos > 0 {
				showPart := filename[:seasonPos]
				cleaned := s.cleanupTitle(showPart)
				if cleaned != "" {
					s.logger.Debug("extracted show title via season pattern", "pattern", pattern, "show", cleaned, "filename", filename)
					return cleaned
				}
			}
		}
	}

	// Pattern 2: "Show Name S##E## Episode Title"
	seasonRegex := regexp.MustCompile(`^(.+?)\s+S\d+E\d+`)
	if matches := seasonRegex.FindStringSubmatch(filename); len(matches) > 1 {
		cleaned := s.cleanupTitle(matches[1])
		if cleaned != "" {
			s.logger.Debug("extracted show title via regex pattern", "show", cleaned, "filename", filename)
			return cleaned
		}
	}

	// Pattern 3: "Show Name.S##E##.Episode.Title"
	dotSeasonRegex := regexp.MustCompile(`^(.+?)\.S\d+E\d+`)
	if matches := dotSeasonRegex.FindStringSubmatch(filename); len(matches) > 1 {
		showName := strings.ReplaceAll(matches[1], ".", " ")
		cleaned := s.cleanupTitle(showName)
		if cleaned != "" {
			s.logger.Debug("extracted show title via dot pattern", "show", cleaned, "filename", filename)
			return cleaned
		}
	}

	// Pattern 4: Directory-based detection
	parentDir := filepath.Base(filepath.Dir(filePath))
	if parentDir != "" && parentDir != "." && !strings.Contains(parentDir, "Season") {
		if cleanDir := s.cleanupTitle(parentDir); cleanDir != "" {
			if len(cleanDir) > 2 && !strings.Contains(strings.ToLower(cleanDir), "season") {
				s.logger.Debug("extracted show title from directory", "show", cleanDir, "directory", parentDir)
				return cleanDir
			}
		}
	}

	s.logger.Debug("no TV show pattern matched", "filename", filename)
	return ""
}

// extractMovieTitle handles various movie filename patterns
func (s *EnrichmentService) extractMovieTitle(filename string) string {
	title := filename

	// Remove release group patterns
	releaseGroupRegex := regexp.MustCompile(`-[A-Z][A-Za-z0-9]*$`)
	title = releaseGroupRegex.ReplaceAllString(title, "")

	// Remove quality tags in brackets
	qualityRegex := regexp.MustCompile(`\[[^\]]*\]`)
	title = qualityRegex.ReplaceAllString(title, "")

	// Remove quality indicators
	qualityPatterns := []string{
		`\bBluRay\b`, `\bBDRip\b`, `\bBRRip\b`, `\bDVDRip\b`, `\bWEBRip\b`, `\bWEB-DL\b`,
		`\bHDTV\b`, `\bSDTV\b`, `\b720p\b`, `\b1080p\b`, `\b4K\b`, `\bUHD\b`,
		`\bx264\b`, `\bx265\b`, `\bH\.?264\b`, `\bH\.?265\b`, `\bHEVC\b`,
		`\bAAC\b`, `\bAC3\b`, `\bDTS\b`, `\bFLAC\b`, `\bMP3\b`,
		`\b2\.0\b`, `\b5\.1\b`, `\b7\.1\b`,
	}

	for _, pattern := range qualityPatterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		title = re.ReplaceAllString(title, "")
	}

	// Clean up spaces
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
	title = strings.TrimSpace(title)

	// Handle year extraction - "Movie Title (2023)"
	yearRegex := regexp.MustCompile(`^(.+?)\s*\((\d{4})\)$`)
	if matches := yearRegex.FindStringSubmatch(title); len(matches) > 2 {
		movieTitle := strings.TrimSpace(matches[1])
		if movieTitle != "" {
			return movieTitle
		}
	}

	// Look for year without parentheses - "Movie Title 2023"
	yearEndRegex := regexp.MustCompile(`^(.+?)\s+(\d{4})$`)
	if matches := yearEndRegex.FindStringSubmatch(title); len(matches) > 2 {
		year, _ := strconv.Atoi(matches[2])
		if year >= 1900 && year <= time.Now().Year()+5 {
			movieTitle := strings.TrimSpace(matches[1])
			if movieTitle != "" {
				return movieTitle
			}
		}
	}

	return s.cleanupTitle(title)
}

// cleanupTitle performs common cleanup operations on titles
func (s *EnrichmentService) cleanupTitle(title string) string {
	if title == "" {
		return ""
	}

	// Remove file extensions
	title = strings.TrimSuffix(title, ".mkv")
	title = strings.TrimSuffix(title, ".mp4")
	title = strings.TrimSuffix(title, ".avi")
	title = strings.TrimSuffix(title, ".mov")

	// Remove quality tags in brackets
	qualityRegex := regexp.MustCompile(`\[[^\]]*\]`)
	title = qualityRegex.ReplaceAllString(title, "")

	// Remove common release group suffixes
	suffixes := []string{"-Pahe", "-RARBG", "-YTS", "-EZTV", "-TGx", "-BORDURE", "-OFT", "-DUSKLiGHT", "-MaG"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(title, suffix) {
			title = strings.TrimSuffix(title, suffix)
			break
		}
	}

	// Remove year in parentheses if present
	if strings.Contains(title, "(") && strings.Contains(title, ")") {
		yearStart := strings.LastIndex(title, "(")
		yearEnd := strings.LastIndex(title, ")")
		if yearEnd > yearStart && yearEnd == len(title)-1 {
			yearStr := title[yearStart+1 : yearEnd]
			if len(yearStr) == 4 {
				if year, err := strconv.Atoi(yearStr); err == nil && year >= 1900 && year <= 2030 {
					title = strings.TrimSpace(title[:yearStart])
				}
			}
		}
	}

	// Replace dots and underscores with spaces
	title = strings.ReplaceAll(title, ".", " ")
	title = strings.ReplaceAll(title, "_", " ")

	// Clean up multiple spaces
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")

	return strings.TrimSpace(title)
}

// extractYear extracts year from file path and metadata
func (s *EnrichmentService) extractYear(filePath string, metadata map[string]string) int {
	// Try to get year from metadata first
	if yearStr, exists := metadata["year"]; exists && yearStr != "" {
		if year, err := strconv.Atoi(yearStr); err == nil {
			return year
		}
	}

	// Extract from filename
	filename := filepath.Base(filePath)

	// Look for 4-digit year patterns
	for i := len(filename) - 4; i >= 0; i-- {
		if i+4 <= len(filename) {
			substr := filename[i : i+4]
			if year, err := strconv.Atoi(substr); err == nil {
				if year >= 1900 && year <= time.Now().Year()+5 {
					return year
				}
			}
		}
	}

	return 0
}

// findBestMatch finds the best matching result
func (s *EnrichmentService) findBestMatch(results []types.Result, title string, year int, filePath string) *types.Result {
	var bestMatch *types.Result
	bestScore := 0.0

	// Determine content type hints from file path
	isLikelyTVShow := s.looksLikeEpisodeTitle(title) ||
		strings.Contains(strings.ToLower(filePath), "/tv/") ||
		strings.Contains(strings.ToLower(filePath), "/shows/") ||
		strings.Contains(strings.ToLower(filePath), "/series/") ||
		strings.Contains(strings.ToLower(filePath), "season") ||
		strings.Contains(strings.ToLower(filePath), "episode") ||
		strings.Contains(strings.ToLower(filePath), " s0") ||
		strings.Contains(strings.ToLower(filePath), " s1") ||
		strings.Contains(strings.ToLower(filePath), " s2")

	isLikelyMovie := strings.Contains(strings.ToLower(filePath), "/movies/") ||
		strings.Contains(strings.ToLower(filePath), "/films/") ||
		strings.Contains(strings.ToLower(filePath), "/cinema/")

	for _, result := range results {
		score := s.calculateMatchScore(result, title, year)

		// Add context bonus for media type matching
		if isLikelyTVShow && (result.MediaType == "tv" || result.FirstAirDate != "" || (result.Name != "" && result.Title == "")) {
			score += 0.15
		} else if isLikelyMovie && (result.MediaType == "movie" || result.ReleaseDate != "" || (result.Title != "" && result.Name == "")) {
			score += 0.15
		}

		if score > bestScore && score >= s.config.Matching.MatchThreshold {
			bestScore = score
			bestMatch = &result
		}
	}

	if bestMatch != nil {
		s.logger.Debug("found best match", "title", s.getResultTitle(*bestMatch), "type", bestMatch.MediaType, "score", bestScore)
	}

	return bestMatch
}

// calculateMatchScore calculates match score between result and search terms
func (s *EnrichmentService) calculateMatchScore(result types.Result, title string, year int) float64 {
	score := 0.0

	// Title matching
	resultTitle := s.getResultTitle(result)
	if strings.EqualFold(resultTitle, title) {
		score += 0.8
	} else if strings.Contains(strings.ToLower(resultTitle), strings.ToLower(title)) {
		score += 0.6
	} else if strings.Contains(strings.ToLower(title), strings.ToLower(resultTitle)) {
		score += 0.4
	}

	// Year matching
	if year > 0 && s.config.Matching.MatchYear {
		resultYear := s.getResultYear(result)
		if resultYear > 0 {
			yearDiff := abs(year - resultYear)
			if yearDiff == 0 {
				score += 0.2
			} else if yearDiff <= s.config.Matching.YearTolerance {
				score += 0.1
			}
		}
	}

	return score
}

// getResultTitle gets the appropriate title from a result
func (s *EnrichmentService) getResultTitle(result types.Result) string {
	if result.Title != "" {
		return result.Title
	}
	return result.Name
}

// getResultYear extracts year from a result
func (s *EnrichmentService) getResultYear(result types.Result) int {
	var dateStr string
	if result.ReleaseDate != "" {
		dateStr = result.ReleaseDate
	} else if result.FirstAirDate != "" {
		dateStr = result.FirstAirDate
	}

	if dateStr != "" && len(dateStr) >= 4 {
		if year, err := strconv.Atoi(dateStr[:4]); err == nil {
			return year
		}
	}

	return 0
}

// saveEnrichment saves enrichment data to database
func (s *EnrichmentService) saveEnrichment(mediaFileID string, result *types.Result) error {
	// Determine media type
	mediaType := "movie"
	if result.MediaType == "tv" || result.Name != "" || result.FirstAirDate != "" {
		mediaType = "tv"
	}

	var releaseDate *time.Time
	dateStr := result.ReleaseDate
	if dateStr == "" {
		dateStr = result.FirstAirDate
	}
	if dateStr != "" {
		if date, err := time.Parse("2006-01-02", dateStr); err == nil {
			releaseDate = &date
		}
	}

	enrichment := &models.TMDbEnrichment{
		MediaFileID:     mediaFileID,
		TMDbID:          result.ID,
		TMDbType:        mediaType,
		Title:           s.getResultTitle(*result),
		OriginalTitle:   result.OriginalTitle,
		Overview:        result.Overview,
		ReleaseDate:     releaseDate,
		ConfidenceScore: s.calculateMatchScore(*result, s.getResultTitle(*result), s.getResultYear(*result)),
		SourcePlugin:    "tmdb_enricher_v2",
	}

	// Store additional metadata as JSON
	if len(result.GenreIDs) > 0 {
		if genresJSON, err := json.Marshal(result.GenreIDs); err == nil {
			enrichment.Genres = string(genresJSON)
		}
	}

	if err := s.db.Save(enrichment).Error; err != nil {
		return fmt.Errorf("failed to save enrichment: %w", err)
	}

	// Register with centralized enrichment system
	if s.unifiedClient != nil {
		if err := s.registerWithCentralizedSystem(mediaFileID, result, mediaType); err != nil {
			s.logger.Warn("Failed to register with centralized system", "error", err)
		}
	}

	s.logger.Info("saved enrichment", "media_file_id", mediaFileID, "tmdb_id", result.ID, "title", enrichment.Title, "type", mediaType)
	return nil
}

// registerWithCentralizedSystem registers enrichment with the centralized system
func (s *EnrichmentService) registerWithCentralizedSystem(mediaFileID string, result *types.Result, mediaType string) error {
	enrichments := make(map[string]string)

	enrichments["tmdb_id"] = fmt.Sprintf("%d", result.ID)
	enrichments["title"] = s.getResultTitle(*result)
	enrichments["overview"] = result.Overview
	enrichments["year"] = fmt.Sprintf("%d", s.getResultYear(*result))
	enrichments["rating"] = fmt.Sprintf("%.2f", result.VoteAverage)
	enrichments["popularity"] = fmt.Sprintf("%.2f", result.Popularity)
	enrichments["media_type"] = mediaType
	enrichments["original_language"] = result.OriginalLanguage

	if mediaType == "movie" {
		enrichments["release_date"] = result.ReleaseDate
		enrichments["original_title"] = result.OriginalTitle
	} else if mediaType == "tv" {
		enrichments["first_air_date"] = result.FirstAirDate
		enrichments["original_name"] = result.OriginalName
		if len(result.OriginCountry) > 0 {
			enrichments["origin_country"] = strings.Join(result.OriginCountry, ",")
		}
	}

	if result.PosterPath != "" {
		enrichments["poster_path"] = result.PosterPath
		enrichments["poster_url"] = fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", s.config.Artwork.PosterSize, result.PosterPath)
	}
	if result.BackdropPath != "" {
		enrichments["backdrop_path"] = result.BackdropPath
		enrichments["backdrop_url"] = fmt.Sprintf("https://image.tmdb.org/t/p/%s%s", s.config.Artwork.BackdropSize, result.BackdropPath)
	}

	matchMetadata := make(map[string]string)
	matchMetadata["source"] = "tmdb"
	matchMetadata["vote_count"] = fmt.Sprintf("%d", result.VoteCount)
	matchMetadata["adult"] = fmt.Sprintf("%t", result.Adult)

	request := &plugins.RegisterEnrichmentRequest{
		MediaFileID:     mediaFileID,
		SourceName:      "tmdb",
		Enrichments:     enrichments,
		ConfidenceScore: s.calculateMatchScore(*result, s.getResultTitle(*result), s.getResultYear(*result)),
		MatchMetadata:   matchMetadata,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := s.unifiedClient.EnrichmentService().RegisterEnrichment(ctx, request)
	if err != nil {
		return fmt.Errorf("failed to register enrichment: %w", err)
	}

	if !response.Success {
		return fmt.Errorf("enrichment registration failed: %s", response.Message)
	}

	return nil
}

// makeAPIRequestWithRetries makes API request with retry logic
func (s *EnrichmentService) makeAPIRequestWithRetries(url string, result interface{}, operation string) error {
	var lastErr error

	for attempt := 0; attempt <= s.config.Reliability.MaxRetries; attempt++ {
		// Rate limiting
		if err := s.ensureRateLimit(); err != nil {
			s.logger.Warn("rate limit delay error", "error", err, "operation", operation)
		}

		if attempt > 0 {
			delay := s.calculateRetryDelay(attempt)
			s.logger.Debug("retrying TMDb request", "attempt", attempt, "delay_sec", delay, "operation", operation)
			time.Sleep(time.Duration(delay) * time.Second)
		}

		err := s.makeAPIRequest(url, result)
		if err == nil {
			if attempt > 0 {
				s.logger.Info("TMDb request succeeded after retries", "attempt", attempt, "operation", operation)
			}
			return nil
		}

		lastErr = err

		if !s.shouldRetryError(err) {
			s.logger.Debug("not retrying TMDb request", "error", err, "operation", operation)
			break
		}
	}

	s.logger.Warn("TMDb request failed after all retries", "operation", operation, "final_error", lastErr)
	return fmt.Errorf("failed after %d retries: %w", s.config.Reliability.MaxRetries, lastErr)
}

// makeAPIRequest performs a single API request
func (s *EnrichmentService) makeAPIRequest(url string, result interface{}) error {
	client := &http.Client{
		Timeout: s.config.API.GetRequestTimeout(),
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.API.Key)
	req.Header.Set("User-Agent", s.config.API.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// Success
	case http.StatusTooManyRequests:
		return fmt.Errorf("rate limited by TMDb API (429)")
	case http.StatusUnauthorized:
		return fmt.Errorf("unauthorized TMDb API request (401) - check API key")
	case http.StatusNotFound:
		return fmt.Errorf("TMDb API endpoint not found (404)")
	default:
		return fmt.Errorf("TMDb API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	return nil
}

// ensureRateLimit ensures proper rate limiting
func (s *EnrichmentService) ensureRateLimit() error {
	if s.lastAPICall != nil {
		elapsed := time.Since(*s.lastAPICall)
		minInterval := time.Duration(1.0/s.config.API.RateLimit) * time.Second

		if elapsed < minInterval {
			sleepTime := minInterval - elapsed
			s.logger.Debug("rate limiting TMDb requests", "sleep_ms", sleepTime.Milliseconds())
			time.Sleep(sleepTime)
		}
	}

	if s.config.API.DelayMs > 0 {
		time.Sleep(s.config.API.GetRequestDelay())
	}

	now := time.Now()
	s.lastAPICall = &now
	return nil
}

// calculateRetryDelay calculates exponential backoff delay
func (s *EnrichmentService) calculateRetryDelay(attempt int) int {
	if attempt <= 0 {
		return 0
	}

	delay := float64(s.config.Reliability.InitialDelaySeconds)
	for i := 1; i < attempt; i++ {
		delay *= s.config.Reliability.BackoffMultiplier
	}

	if int(delay) > s.config.Reliability.MaxDelaySeconds {
		delay = float64(s.config.Reliability.MaxDelaySeconds)
	}

	return int(delay)
}

// shouldRetryError determines if error should trigger retry
func (s *EnrichmentService) shouldRetryError(err error) bool {
	errStr := err.Error()

	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "rate limited") ||
		strings.Contains(errStr, "server error") ||
		strings.Contains(errStr, "bad gateway") ||
		strings.Contains(errStr, "service unavailable") ||
		strings.Contains(errStr, "gateway timeout") {
		return true
	}

	return false
}

// generateQueryHash generates MD5 hash for query
func (s *EnrichmentService) generateQueryHash(query string) string {
	hash := md5.Sum([]byte(query))
	return fmt.Sprintf("%x", hash)
}

// getCachedResponse retrieves cached API response
func (s *EnrichmentService) getCachedResponse(queryType, queryHash string) ([]types.Result, error) {
	var cache models.TMDbCache
	if err := s.db.Where("query_type = ? AND query_hash = ? AND expires_at > ?",
		queryType, queryHash, time.Now()).First(&cache).Error; err != nil {
		return nil, err
	}

	var results []types.Result
	if err := json.Unmarshal([]byte(cache.Response), &results); err != nil {
		return nil, err
	}

	return results, nil
}

// cacheResults caches API response
func (s *EnrichmentService) cacheResults(queryType, queryHash string, results []types.Result) {
	data, err := json.Marshal(results)
	if err != nil {
		s.logger.Error("failed to marshal cache data", "error", err)
		return
	}

	cache := &models.TMDbCache{
		QueryHash: queryHash,
		QueryType: queryType,
		Response:  string(data),
		ExpiresAt: time.Now().Add(s.config.Cache.GetCacheDuration()),
	}

	s.db.Save(cache)
}

// Helper function for absolute value
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// UpdateConfiguration updates the enrichment service configuration at runtime
func (s *EnrichmentService) UpdateConfiguration(newConfig *config.Config) {
	s.config = newConfig
	s.logger.Debug("enrichment service configuration updated",
		"api_rate_limit", newConfig.API.RateLimit,
		"auto_enrich", newConfig.Features.AutoEnrich,
		"enable_movies", newConfig.Features.EnableMovies,
		"enable_tv_shows", newConfig.Features.EnableTVShows)
}
