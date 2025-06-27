package enrichmentmodule

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// TVShowValidator provides validation for TV show metadata to prevent data corruption
type TVShowValidator struct {
	db *gorm.DB
}

// NewTVShowValidator creates a new TV show validator
func NewTVShowValidator(db *gorm.DB) *TVShowValidator {
	return &TVShowValidator{db: db}
}

// ValidationResult represents the result of a validation check
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Warnings []string `json:"warnings,omitempty"`
	Errors   []string `json:"errors,omitempty"`
	Score    float64  `json:"score"` // Confidence score 0-1
}

// ValidateTVShowMetadata validates TV show metadata before assignment
func (v *TVShowValidator) ValidateTVShowMetadata(title string, tmdbID string, description string, firstAirDate string) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Warnings: []string{},
		Errors:   []string{},
		Score:    1.0,
	}

	// Basic validation checks
	if err := v.validateTitle(title, result); err != nil {
		log.Printf("ERROR: Title validation failed: %v", err)
	}

	if tmdbID != "" {
		if err := v.validateTMDBID(tmdbID, result); err != nil {
			log.Printf("ERROR: TMDB ID validation failed: %v", err)
		}
	}

	if description != "" {
		if err := v.validateDescription(description, result); err != nil {
			log.Printf("ERROR: Description validation failed: %v", err)
		}
	}

	if firstAirDate != "" {
		if err := v.validateAirDate(firstAirDate, result); err != nil {
			log.Printf("ERROR: Air date validation failed: %v", err)
		}
	}

	// Cross-validation checks
	if err := v.validateConsistency(title, tmdbID, description, firstAirDate, result); err != nil {
		log.Printf("ERROR: Consistency validation failed: %v", err)
	}

	// Check for duplicates
	if err := v.checkForDuplicates(title, tmdbID, result); err != nil {
		log.Printf("ERROR: Duplicate check failed: %v", err)
	}

	// Calculate final validity
	result.Valid = len(result.Errors) == 0
	result.Score = v.calculateConfidenceScore(result)

	return result
}

// validateTitle validates the TV show title
func (v *TVShowValidator) validateTitle(title string, result *ValidationResult) error {
	if strings.TrimSpace(title) == "" {
		result.Errors = append(result.Errors, "Title cannot be empty")
		return nil
	}

	title = strings.TrimSpace(title)

	// Check for suspicious patterns that suggest this isn't a TV show
	suspiciousPatterns := []struct {
		pattern string
		message string
	}{
		{`(?i)(commentary|behind.?the.?scenes|making.?of|interview|documentary)`, "Title suggests commentary/documentary content"},
		{`(?i)(album|track|song|music|artist|band)`, "Title suggests music content"},
		{`(?i)(actor|actress|celebrity|person|people)$`, "Title suggests person rather than show"},
		{`(?i)^(the|a|an)\s+(actor|actress|person|people)`, "Title pattern suggests person"},
		{`(?i)(live\s+concert|tour|performance)`, "Title suggests live performance"},
		{`(?i)(tutorial|how.?to|guide|instruction)`, "Title suggests instructional content"},
		{`(?i)(news|update|announcement|press)`, "Title suggests news content"},
		{`(?i)(trailer|teaser|preview|promo)`, "Title suggests promotional content"},
	}

	for _, pattern := range suspiciousPatterns {
		if matched, _ := regexp.MatchString(pattern.pattern, title); matched {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Suspicious title pattern: %s", pattern.message))
			result.Score -= 0.2 // Reduce confidence
		}
	}

	// Check length - TV show titles are typically reasonable length
	if len(title) > 100 {
		result.Warnings = append(result.Warnings, "Unusually long title for TV show")
		result.Score -= 0.1
	}

	if len(title) < 2 {
		result.Errors = append(result.Errors, "Title too short")
	}

	// Check for special characters that might indicate parsing errors
	if strings.Contains(title, "#") && !strings.HasPrefix(title, "#") {
		result.Warnings = append(result.Warnings, "Title contains hashtag - might be social media content")
		result.Score -= 0.15
	}

	return nil
}

// validateTMDBID validates the TMDB ID
func (v *TVShowValidator) validateTMDBID(tmdbID string, result *ValidationResult) error {
	if tmdbID == "" {
		return nil // Optional field
	}

	// TMDB IDs should be numeric
	if _, err := strconv.Atoi(tmdbID); err != nil {
		result.Errors = append(result.Errors, "TMDB ID must be numeric")
		return nil
	}

	// Check if TMDB ID is already used by another TV show
	var existingShow database.TVShow
	err := v.db.Where("tmdb_id = ? AND tmdb_id != ''", tmdbID).First(&existingShow).Error
	if err == nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("TMDB ID %s already used by show: %s", tmdbID, existingShow.Title))
		result.Score -= 0.3
	}

	return nil
}

// validateDescription validates the description
func (v *TVShowValidator) validateDescription(description string, result *ValidationResult) error {
	if description == "" {
		return nil // Optional field
	}

	description = strings.TrimSpace(description)

	// Check for patterns that suggest this isn't a TV show
	suspiciousDescPatterns := []struct {
		pattern string
		message string
	}{
		{`(?i)(singer|musician|artist|band|album|recording)`, "Description suggests music artist"},
		{`(?i)(actor|actress|born|biography|life story)`, "Description suggests person biography"},
		{`(?i)(commentary|behind.?scenes|making.?of)`, "Description suggests commentary content"},
		{`(?i)(tutorial|how.?to|instruction|guide)`, "Description suggests instructional content"},
		{`(?i)(live\s+(concert|performance|show))`, "Description suggests live performance"},
		{`(?i)(news|journalist|reporter|anchor)`, "Description suggests news content"},
	}

	for _, pattern := range suspiciousDescPatterns {
		if matched, _ := regexp.MatchString(pattern.pattern, description); matched {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Suspicious description pattern: %s", pattern.message))
			result.Score -= 0.15
		}
	}

	return nil
}

// validateAirDate validates the first air date
func (v *TVShowValidator) validateAirDate(firstAirDate string, result *ValidationResult) error {
	if firstAirDate == "" {
		return nil // Optional field
	}

	// Try to parse the date
	layouts := []string{
		"2006-01-02",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		"2006",
	}

	var parsedDate time.Time
	var err error
	for _, layout := range layouts {
		parsedDate, err = time.Parse(layout, firstAirDate)
		if err == nil {
			break
		}
	}

	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Invalid air date format: %s", firstAirDate))
		return nil
	}

	// Check if date is reasonable
	now := time.Now()
	if parsedDate.After(now.AddDate(2, 0, 0)) {
		result.Warnings = append(result.Warnings, "Air date is more than 2 years in the future")
		result.Score -= 0.1
	}

	if parsedDate.Before(time.Date(1920, 1, 1, 0, 0, 0, 0, time.UTC)) {
		result.Warnings = append(result.Warnings, "Air date is before 1920 - unusually old for TV")
		result.Score -= 0.1
	}

	return nil
}

// validateConsistency performs cross-field validation
func (v *TVShowValidator) validateConsistency(title, tmdbID, description, firstAirDate string, result *ValidationResult) error {
	// Check if title and description are consistent
	if title != "" && description != "" {
		titleLower := strings.ToLower(title)
		descLower := strings.ToLower(description)

		// If title suggests it's a person but description talks about TV show, that's suspicious
		personTitlePattern := `(?i)(actor|actress|singer|musician|artist|celebrity)`
		showDescPattern := `(?i)(series|show|episode|season|drama|comedy|sitcom)`

		if matched, _ := regexp.MatchString(personTitlePattern, titleLower); matched {
			if matched2, _ := regexp.MatchString(showDescPattern, descLower); matched2 {
				result.Warnings = append(result.Warnings, "Title suggests person but description suggests TV show")
				result.Score -= 0.2
			}
		}
	}

	return nil
}

// checkForDuplicates checks for potential duplicate entries
func (v *TVShowValidator) checkForDuplicates(title, tmdbID string, result *ValidationResult) error {
	if title == "" {
		return nil
	}

	// Check for exact title matches
	var exactMatches []database.TVShow
	err := v.db.Where("LOWER(title) = ?", strings.ToLower(strings.TrimSpace(title))).Find(&exactMatches).Error
	if err != nil {
		return fmt.Errorf("failed to check for duplicate titles: %w", err)
	}

	if len(exactMatches) > 0 {
		if tmdbID != "" {
			// Check if any of the matches have different TMDB IDs
			for _, match := range exactMatches {
				if match.TmdbID != tmdbID && match.TmdbID != "" {
					result.Warnings = append(result.Warnings, fmt.Sprintf("Title '%s' already exists with different TMDB ID: %s vs %s", title, match.TmdbID, tmdbID))
					result.Score -= 0.4
				}
			}
		} else {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Title '%s' already exists %d time(s)", title, len(exactMatches)))
			result.Score -= 0.3
		}
	}

	// Check for similar titles (fuzzy matching)
	var similarMatches []database.TVShow
	err = v.db.Where("LOWER(title) LIKE ?", "%"+strings.ToLower(strings.TrimSpace(title))+"%").
		Where("LOWER(title) != ?", strings.ToLower(strings.TrimSpace(title))).
		Limit(5).Find(&similarMatches).Error
	if err != nil {
		return fmt.Errorf("failed to check for similar titles: %w", err)
	}

	if len(similarMatches) > 0 {
		var similarTitles []string
		for _, match := range similarMatches {
			similarTitles = append(similarTitles, match.Title)
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf("Found similar titles: %s", strings.Join(similarTitles, ", ")))
		result.Score -= 0.1
	}

	return nil
}

// calculateConfidenceScore calculates the final confidence score
func (v *TVShowValidator) calculateConfidenceScore(result *ValidationResult) float64 {
	if len(result.Errors) > 0 {
		return 0.0 // Any errors = zero confidence
	}

	// Start with the score adjusted by warnings
	score := result.Score

	// Additional adjustments based on warning count
	warningPenalty := float64(len(result.Warnings)) * 0.05
	score -= warningPenalty

	// Ensure score stays in valid range
	if score < 0.0 {
		score = 0.0
	}
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// SuggestCorrections suggests possible corrections for invalid metadata
func (v *TVShowValidator) SuggestCorrections(title, tmdbID, description string) map[string]string {
	suggestions := make(map[string]string)

	// Title corrections
	if title != "" {
		cleaned := strings.TrimSpace(title)
		cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ") // Normalize whitespace
		if cleaned != title {
			suggestions["title"] = cleaned
		}
	}

	// TMDB ID corrections
	if tmdbID != "" {
		if strings.Contains(tmdbID, ".") {
			cleaned := strings.Split(tmdbID, ".")[0] // Remove decimal part if present
			suggestions["tmdb_id"] = cleaned
		}
	}

	return suggestions
}
