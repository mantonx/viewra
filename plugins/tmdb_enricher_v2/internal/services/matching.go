package services

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mantonx/viewra/plugins/tmdb_enricher_v2/internal/config"
	"github.com/mantonx/viewra/plugins/tmdb_enricher_v2/internal/types"
	plugins "github.com/mantonx/viewra/sdk"
)

// MatchingService handles content matching and identification
type MatchingService struct {
	config *config.Config
	logger plugins.Logger
}

// NewMatchingService creates a new matching service
func NewMatchingService(cfg *config.Config, logger plugins.Logger) *MatchingService {
	return &MatchingService{
		config: cfg,
		logger: logger,
	}
}

// MatchResult represents a match result with confidence score
type MatchResult struct {
	Result *types.Result
	Score  float64
	Reason string
}

// FindBestMatch finds the best match from search results
func (m *MatchingService) FindBestMatch(results []types.Result, title string, year int, filePath string) *MatchResult {
	if len(results) == 0 {
		return nil
	}

	var bestMatch *MatchResult
	bestScore := 0.0

	for _, result := range results {
		score := m.calculateMatchScore(result, title, year)
		reason := m.getMatchReason(result, title, year, score)

		m.logger.Debug("match candidate",
			"tmdb_id", result.ID,
			"title", m.getResultTitle(result),
			"year", m.getResultYear(result),
			"score", fmt.Sprintf("%.3f", score),
			"reason", reason)

		if score > bestScore && score >= m.config.Matching.MatchThreshold {
			bestMatch = &MatchResult{
				Result: &result,
				Score:  score,
				Reason: reason,
			}
			bestScore = score
		}
	}

	if bestMatch != nil {
		m.logger.Info("best match found",
			"tmdb_id", bestMatch.Result.ID,
			"title", m.getResultTitle(*bestMatch.Result),
			"score", fmt.Sprintf("%.3f", bestMatch.Score),
			"reason", bestMatch.Reason)
	} else {
		m.logger.Debug("no match above threshold",
			"threshold", m.config.Matching.MatchThreshold,
			"best_score", fmt.Sprintf("%.3f", bestScore))
	}

	return bestMatch
}

// calculateMatchScore calculates similarity score between search result and target
func (m *MatchingService) calculateMatchScore(result types.Result, title string, year int) float64 {
	resultTitle := m.getResultTitle(result)
	titleScore := m.calculateTitleSimilarity(strings.ToLower(title), strings.ToLower(resultTitle))

	score := titleScore * 0.8 // Title gets 80% weight

	// Year matching gets 20% weight if enabled
	if m.config.Matching.MatchYear && year > 0 {
		resultYear := m.getResultYear(result)
		if resultYear > 0 {
			yearDiff := abs(year - resultYear)
			if yearDiff <= m.config.Matching.YearTolerance {
				yearScore := 1.0 - (float64(yearDiff) / float64(m.config.Matching.YearTolerance+1))
				score += yearScore * 0.2
			}
		} else {
			// No year available, slight penalty
			score += 0.1
		}
	} else {
		// Year matching disabled, full score from title
		score = titleScore
	}

	// Bonus for exact title matches
	if strings.EqualFold(title, resultTitle) {
		score += 0.1
	}

	// Ensure score doesn't exceed 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// calculateTitleSimilarity calculates similarity between two titles
func (m *MatchingService) calculateTitleSimilarity(title1, title2 string) float64 {
	// Simple similarity based on common words and character similarity
	words1 := strings.Fields(title1)
	words2 := strings.Fields(title2)

	// Exact match
	if title1 == title2 {
		return 1.0
	}

	// Levenshtein distance for character-level similarity
	distance := m.levenshteinDistance(title1, title2)
	maxLen := max(len(title1), len(title2))
	if maxLen == 0 {
		return 0.0
	}
	charSimilarity := 1.0 - (float64(distance) / float64(maxLen))

	// Word overlap similarity
	commonWords := 0
	totalWords := len(words1) + len(words2)

	if totalWords > 0 {
		wordMap := make(map[string]bool)
		for _, word := range words1 {
			wordMap[word] = true
		}

		for _, word := range words2 {
			if wordMap[word] {
				commonWords++
			}
		}

		wordSimilarity := float64(commonWords*2) / float64(totalWords)

		// Combine character and word similarity
		return (charSimilarity * 0.6) + (wordSimilarity * 0.4)
	}

	return charSimilarity
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func (m *MatchingService) levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			matrix[i][j] = min(
				min(matrix[i-1][j]+1, matrix[i][j-1]+1), // deletion, insertion
				matrix[i-1][j-1]+cost,                   // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

// getMatchReason provides a human-readable explanation for the match
func (m *MatchingService) getMatchReason(result types.Result, title string, year int, score float64) string {
	resultTitle := m.getResultTitle(result)
	resultYear := m.getResultYear(result)

	reasons := []string{}

	if strings.EqualFold(title, resultTitle) {
		reasons = append(reasons, "exact title match")
	} else if score >= 0.9 {
		reasons = append(reasons, "very high title similarity")
	} else if score >= 0.7 {
		reasons = append(reasons, "good title similarity")
	}

	if m.config.Matching.MatchYear && year > 0 && resultYear > 0 {
		yearDiff := abs(year - resultYear)
		if yearDiff == 0 {
			reasons = append(reasons, "exact year match")
		} else if yearDiff <= m.config.Matching.YearTolerance {
			reasons = append(reasons, fmt.Sprintf("year within tolerance (%d)", yearDiff))
		}
	}

	if len(reasons) == 0 {
		return "partial match"
	}

	return strings.Join(reasons, ", ")
}

// ExtractContentInfo extracts content information from file path and metadata
func (m *MatchingService) ExtractContentInfo(filePath string, metadata map[string]string) *ContentInfo {
	info := &ContentInfo{
		FilePath: filePath,
		Metadata: metadata,
	}

	// Extract title
	info.Title = m.extractTitle(filePath, metadata)

	// Extract year
	info.Year = m.extractYear(filePath, metadata)

	// Determine content type
	info.ContentType = m.determineContentType(filePath, metadata)

	// Extract additional TV show info if applicable
	if info.ContentType == "tv" {
		info.TVInfo = m.extractTVInfo(filePath, metadata)
	}

	return info
}

// ContentInfo represents extracted content information
type ContentInfo struct {
	FilePath    string
	Metadata    map[string]string
	Title       string
	Year        int
	ContentType string // "movie", "tv", "episode"
	TVInfo      *TVInfo
}

// TVInfo represents TV show specific information
type TVInfo struct {
	ShowName      string
	SeasonNumber  int
	EpisodeNumber int
	EpisodeTitle  string
}

// extractTitle extracts title from file path and metadata
func (m *MatchingService) extractTitle(filePath string, metadata map[string]string) string {
	// Priority: series_name > show_name > artist > title (if not episode-like)

	if seriesName, exists := metadata["series_name"]; exists && seriesName != "" {
		return m.cleanupTitle(seriesName)
	}

	if showName, exists := metadata["show_name"]; exists && showName != "" {
		return m.cleanupTitle(showName)
	}

	if artist, exists := metadata["artist"]; exists && artist != "" {
		cleaned := m.cleanupTitle(artist)
		if cleaned != "" {
			return cleaned
		}
	}

	if title, exists := metadata["title"]; exists && title != "" {
		if !m.looksLikeEpisodeTitle(title) {
			cleaned := m.cleanupTitle(title)
			if cleaned != "" {
				return cleaned
			}
		}
	}

	// Extract from filename
	filename := filepath.Base(filePath)
	filename = strings.TrimSuffix(filename, filepath.Ext(filename))

	// Try TV show patterns first
	if title := m.extractTVShowTitle(filename, filePath); title != "" {
		return title
	}

	// Try movie patterns
	if title := m.extractMovieTitle(filename); title != "" {
		return title
	}

	return ""
}

// extractYear extracts year from file path and metadata
func (m *MatchingService) extractYear(filePath string, metadata map[string]string) int {
	// Try metadata first
	if yearStr, exists := metadata["year"]; exists && yearStr != "" {
		if year, err := strconv.Atoi(yearStr); err == nil && year > 1900 && year <= 2030 {
			return year
		}
	}

	if dateStr, exists := metadata["date"]; exists && dateStr != "" {
		if year, err := strconv.Atoi(dateStr[:4]); err == nil && year > 1900 && year <= 2030 {
			return year
		}
	}

	// Extract from filename
	yearRegex := regexp.MustCompile(`\b(19|20)\d{2}\b`)
	if matches := yearRegex.FindStringSubmatch(filePath); len(matches) > 0 {
		if year, err := strconv.Atoi(matches[0]); err == nil {
			return year
		}
	}

	return 0
}

// determineContentType determines if content is movie, TV show, or episode
func (m *MatchingService) determineContentType(filePath string, metadata map[string]string) string {
	filename := strings.ToLower(filepath.Base(filePath))

	// TV show indicators
	tvPatterns := []string{
		`s\d+e\d+`, // S01E01
		`\d+x\d+`,  // 1x01
		`season`,   // season
		`episode`,  // episode
	}

	for _, pattern := range tvPatterns {
		if matched, _ := regexp.MatchString(pattern, filename); matched {
			return "tv"
		}
	}

	// Check metadata for TV indicators
	if _, exists := metadata["series_name"]; exists {
		return "tv"
	}
	if _, exists := metadata["show_name"]; exists {
		return "tv"
	}

	// Default to movie
	return "movie"
}

// Helper methods
func (m *MatchingService) getResultTitle(result types.Result) string {
	if result.Title != "" {
		return result.Title
	}
	return result.Name
}

func (m *MatchingService) getResultYear(result types.Result) int {
	if result.ReleaseDate != "" {
		if year, err := strconv.Atoi(result.ReleaseDate[:4]); err == nil {
			return year
		}
	}
	if result.FirstAirDate != "" {
		if year, err := strconv.Atoi(result.FirstAirDate[:4]); err == nil {
			return year
		}
	}
	return 0
}

func (m *MatchingService) looksLikeEpisodeTitle(title string) bool {
	lower := strings.ToLower(title)
	episodeIndicators := []string{
		"episode", "ep.", "part", "chapter", "act",
	}

	for _, indicator := range episodeIndicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}

	// Check for patterns like "1.01", "S01E01", etc.
	episodePatterns := []regexp.Regexp{
		*regexp.MustCompile(`\d+\.\d+`),
		*regexp.MustCompile(`s\d+e\d+`),
		*regexp.MustCompile(`\d+x\d+`),
	}

	for _, pattern := range episodePatterns {
		if pattern.MatchString(lower) {
			return true
		}
	}

	return false
}

func (m *MatchingService) extractTVShowTitle(filename, filePath string) string {
	// Implementation similar to original but more organized
	// This is a simplified version - can be expanded

	patterns := []string{
		`^(.+?)\s*[Ss]\d+[Ee]\d+`,   // ShowName S01E01
		`^(.+?)\s*\d+x\d+`,          // ShowName 1x01
		`^(.+?)\s*-\s*Season\s*\d+`, // ShowName - Season 1
		`^(.+?)\s*Season\s*\d+`,     // ShowName Season 1
	}

	for _, pattern := range patterns {
		regex := regexp.MustCompile(pattern)
		if matches := regex.FindStringSubmatch(filename); len(matches) > 1 {
			title := strings.TrimSpace(matches[1])
			return m.cleanupTitle(title)
		}
	}

	return ""
}

func (m *MatchingService) extractMovieTitle(filename string) string {
	// Remove year patterns
	yearRegex := regexp.MustCompile(`\s*\(?\b(19|20)\d{2}\b\)?`)
	title := yearRegex.ReplaceAllString(filename, "")

	return m.cleanupTitle(title)
}

func (m *MatchingService) extractTVInfo(filePath string, metadata map[string]string) *TVInfo {
	// Extract TV show specific information
	// This is a simplified implementation
	return &TVInfo{
		ShowName: m.extractTitle(filePath, metadata),
		// Additional parsing logic would go here
	}
}

func (m *MatchingService) cleanupTitle(title string) string {
	if title == "" {
		return ""
	}

	// Remove common artifacts
	cleanupPatterns := []string{
		`\[.*?\]`, // [tags]
		`\(.*?\)`, // (tags)
		`\{.*?\}`, // {tags}
		`\b(HDTV|720p|1080p|x264|x265|HEVC|AAC|AC3|DTS|BluRay|WEB-DL|WEBRip)\b`,
	}

	cleaned := title
	for _, pattern := range cleanupPatterns {
		regex := regexp.MustCompile(`(?i)` + pattern)
		cleaned = regex.ReplaceAllString(cleaned, "")
	}

	// Clean up whitespace
	cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(cleaned), " ")

	return cleaned
}

// Utility functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// UpdateConfiguration updates the matching service configuration at runtime
func (s *MatchingService) UpdateConfiguration(newConfig *config.Config) {
	s.config = newConfig
	s.logger.Debug("matching service configuration updated",
		"match_threshold", newConfig.Matching.MatchThreshold,
		"match_year", newConfig.Matching.MatchYear,
		"year_tolerance", newConfig.Matching.YearTolerance)
}
