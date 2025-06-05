package enrichmentmodule

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

// DuplicationManager handles detection and resolution of duplicate TV show entries
type DuplicationManager struct {
	db        *gorm.DB
	validator *TVShowValidator
}

// NewDuplicationManager creates a new duplication manager
func NewDuplicationManager(db *gorm.DB) *DuplicationManager {
	return &DuplicationManager{
		db:        db,
		validator: NewTVShowValidator(db),
	}
}

// DuplicateGroup represents a group of potentially duplicate TV shows
type DuplicateGroup struct {
	Shows           []database.TVShow `json:"shows"`
	SimilarityScore float64           `json:"similarity_score"`
	MergeStrategy   string            `json:"merge_strategy"`
	Recommendations []string          `json:"recommendations"`
}

// MergeCandidate represents a candidate for merging
type MergeCandidate struct {
	PrimaryShow   database.TVShow `json:"primary_show"`
	DuplicateShow database.TVShow `json:"duplicate_show"`
	Confidence    float64         `json:"confidence"`
	Reasons       []string        `json:"reasons"`
}

// DetectDuplicates finds potential duplicate TV show entries
func (dm *DuplicationManager) DetectDuplicates() ([]DuplicateGroup, error) {
	var allShows []database.TVShow
	if err := dm.db.Find(&allShows).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch TV shows: %w", err)
	}

	var duplicateGroups []DuplicateGroup

	// Group by TMDB ID first
	tmdbGroups := dm.groupByTMDBID(allShows)
	for tmdbID, shows := range tmdbGroups {
		if len(shows) > 1 && tmdbID != "" {
			group := DuplicateGroup{
				Shows:           shows,
				SimilarityScore: 1.0, // Same TMDB ID = perfect match
				MergeStrategy:   "merge_by_tmdb_id",
				Recommendations: []string{
					fmt.Sprintf("Found %d shows with TMDB ID %s", len(shows), tmdbID),
					"These should be merged as they represent the same entity",
				},
			}
			duplicateGroups = append(duplicateGroups, group)
		}
	}

	// Group by similar titles
	titleGroups := dm.groupBySimilarTitles(allShows)
	for _, shows := range titleGroups {
		if len(shows) > 1 {
			// Calculate similarity score based on title closeness
			score := dm.calculateTitleSimilarity(shows)
			if score > 0.8 { // High similarity threshold
				group := DuplicateGroup{
					Shows:           shows,
					SimilarityScore: score,
					MergeStrategy:   "merge_by_title_similarity",
					Recommendations: dm.generateMergeRecommendations(shows),
				}
				duplicateGroups = append(duplicateGroups, group)
			}
		}
	}

	return duplicateGroups, nil
}

// groupByTMDBID groups shows by their TMDB ID
func (dm *DuplicationManager) groupByTMDBID(shows []database.TVShow) map[string][]database.TVShow {
	groups := make(map[string][]database.TVShow)
	
	for _, show := range shows {
		if show.TmdbID != "" {
			groups[show.TmdbID] = append(groups[show.TmdbID], show)
		}
	}
	
	return groups
}

// groupBySimilarTitles groups shows by similar titles
func (dm *DuplicationManager) groupBySimilarTitles(shows []database.TVShow) [][]database.TVShow {
	var groups [][]database.TVShow
	processed := make(map[string]bool)

	for i, show1 := range shows {
		if processed[show1.ID] {
			continue
		}

		group := []database.TVShow{show1}
		processed[show1.ID] = true

		for j := i + 1; j < len(shows); j++ {
			show2 := shows[j]
			if processed[show2.ID] {
				continue
			}

			similarity := dm.calculateStringSimilarity(show1.Title, show2.Title)
			if similarity > 0.8 { // 80% similarity threshold
				group = append(group, show2)
				processed[show2.ID] = true
			}
		}

		if len(group) > 1 {
			groups = append(groups, group)
		}
	}

	return groups
}

// calculateTitleSimilarity calculates similarity score for a group of shows
func (dm *DuplicationManager) calculateTitleSimilarity(shows []database.TVShow) float64 {
	if len(shows) < 2 {
		return 0.0
	}

	totalSimilarity := 0.0
	comparisons := 0

	for i := 0; i < len(shows); i++ {
		for j := i + 1; j < len(shows); j++ {
			similarity := dm.calculateStringSimilarity(shows[i].Title, shows[j].Title)
			totalSimilarity += similarity
			comparisons++
		}
	}

	return totalSimilarity / float64(comparisons)
}

// calculateStringSimilarity calculates similarity between two strings using Levenshtein distance
func (dm *DuplicationManager) calculateStringSimilarity(s1, s2 string) float64 {
	s1 = strings.ToLower(strings.TrimSpace(s1))
	s2 = strings.ToLower(strings.TrimSpace(s2))

	if s1 == s2 {
		return 1.0
	}

	// Simple similarity based on common words
	words1 := strings.Fields(s1)
	words2 := strings.Fields(s2)

	commonWords := 0
	totalWords := len(words1) + len(words2)

	for _, word1 := range words1 {
		for _, word2 := range words2 {
			if word1 == word2 {
				commonWords++
				break
			}
		}
	}

	if totalWords == 0 {
		return 0.0
	}

	return float64(commonWords*2) / float64(totalWords)
}

// generateMergeRecommendations generates recommendations for merging shows
func (dm *DuplicationManager) generateMergeRecommendations(shows []database.TVShow) []string {
	var recommendations []string

	// Analyze the shows to provide specific recommendations
	hasDescription := 0
	hasAirDate := 0
	hasTMDBID := 0
	
	for _, show := range shows {
		if show.Description != "" {
			hasDescription++
		}
		if show.FirstAirDate != nil {
			hasAirDate++
		}
		if show.TmdbID != "" {
			hasTMDBID++
		}
	}

	recommendations = append(recommendations, fmt.Sprintf("Found %d similar shows", len(shows)))

	if hasTMDBID > 0 {
		recommendations = append(recommendations, "Consider using the entry with TMDB ID as primary")
	}

	if hasDescription > 0 && hasDescription < len(shows) {
		recommendations = append(recommendations, "Some entries have descriptions, others don't - merge data")
	}

	if hasAirDate > 0 && hasAirDate < len(shows) {
		recommendations = append(recommendations, "Some entries have air dates, others don't - merge data")
	}

	return recommendations
}

// GetMergeCandidates identifies specific merge candidates with high confidence
func (dm *DuplicationManager) GetMergeCandidates(threshold float64) ([]MergeCandidate, error) {
	duplicateGroups, err := dm.DetectDuplicates()
	if err != nil {
		return nil, fmt.Errorf("failed to detect duplicates: %w", err)
	}

	var candidates []MergeCandidate

	for _, group := range duplicateGroups {
		if group.SimilarityScore >= threshold && len(group.Shows) == 2 {
			// Determine which show should be primary
			primary, duplicate := dm.selectPrimaryShow(group.Shows[0], group.Shows[1])
			
			candidate := MergeCandidate{
				PrimaryShow:   primary,
				DuplicateShow: duplicate,
				Confidence:    group.SimilarityScore,
				Reasons:       group.Recommendations,
			}
			
			candidates = append(candidates, candidate)
		}
	}

	// Sort by confidence (highest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Confidence > candidates[j].Confidence
	})

	return candidates, nil
}

// selectPrimaryShow determines which show should be the primary one in a merge
func (dm *DuplicationManager) selectPrimaryShow(show1, show2 database.TVShow) (primary, duplicate database.TVShow) {
	score1 := dm.calculateShowQualityScore(show1)
	score2 := dm.calculateShowQualityScore(show2)

	if score1 >= score2 {
		return show1, show2
	}
	return show2, show1
}

// calculateShowQualityScore calculates a quality score for a show entry
func (dm *DuplicationManager) calculateShowQualityScore(show database.TVShow) float64 {
	score := 0.0

	// TMDB ID adds significant value
	if show.TmdbID != "" {
		score += 3.0
	}

	// Description adds value
	if show.Description != "" && len(show.Description) > 10 {
		score += 2.0
	}

	// Air date adds value
	if show.FirstAirDate != nil {
		score += 1.5
	}

	// Poster adds value
	if show.Poster != "" {
		score += 1.0
	}

	// Status adds value
	if show.Status != "" && show.Status != "Unknown" {
		score += 0.5
	}

	// Backdrop adds value
	if show.Backdrop != "" {
		score += 0.5
	}

	// More recent creation suggests better data
	now := time.Now()
	daysSinceCreation := now.Sub(show.CreatedAt).Hours() / 24
	if daysSinceCreation < 30 { // Recent entries get bonus
		score += 0.5
	}

	return score
}

// MergeShows merges a duplicate show into a primary show
func (dm *DuplicationManager) MergeShows(primaryID, duplicateID string, dryRun bool) (*MergeResult, error) {
	var primary, duplicate database.TVShow
	
	if err := dm.db.Where("id = ?", primaryID).First(&primary).Error; err != nil {
		return nil, fmt.Errorf("primary show not found: %w", err)
	}
	
	if err := dm.db.Where("id = ?", duplicateID).First(&duplicate).Error; err != nil {
		return nil, fmt.Errorf("duplicate show not found: %w", err)
	}

	result := &MergeResult{
		PrimaryID:   primaryID,
		DuplicateID: duplicateID,
		Changes:     []string{},
		Success:     false,
		DryRun:      dryRun,
	}

	// Merge logic - fill in missing fields from duplicate
	updated := primary

	if updated.Description == "" && duplicate.Description != "" {
		updated.Description = duplicate.Description
		result.Changes = append(result.Changes, "Added description from duplicate")
	}

	if updated.FirstAirDate == nil && duplicate.FirstAirDate != nil {
		updated.FirstAirDate = duplicate.FirstAirDate
		result.Changes = append(result.Changes, "Added air date from duplicate")
	}

	if updated.Poster == "" && duplicate.Poster != "" {
		updated.Poster = duplicate.Poster
		result.Changes = append(result.Changes, "Added poster from duplicate")
	}

	if updated.Backdrop == "" && duplicate.Backdrop != "" {
		updated.Backdrop = duplicate.Backdrop
		result.Changes = append(result.Changes, "Added backdrop from duplicate")
	}

	if (updated.Status == "" || updated.Status == "Unknown") && duplicate.Status != "" && duplicate.Status != "Unknown" {
		updated.Status = duplicate.Status
		result.Changes = append(result.Changes, "Added status from duplicate")
	}

	if updated.TmdbID == "" && duplicate.TmdbID != "" {
		updated.TmdbID = duplicate.TmdbID
		result.Changes = append(result.Changes, "Added TMDB ID from duplicate")
	}

	if !dryRun && len(result.Changes) > 0 {
		// Start transaction
		tx := dm.db.Begin()
		
		// Update primary show
		if err := tx.Save(&updated).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to update primary show: %w", err)
		}

		// Reassign seasons from duplicate to primary
		if err := tx.Model(&database.Season{}).Where("tv_show_id = ?", duplicateID).Update("tv_show_id", primaryID).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to reassign seasons: %w", err)
		}

		// Delete the duplicate show
		if err := tx.Delete(&database.TVShow{}, "id = ?", duplicateID).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to delete duplicate show: %w", err)
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			return nil, fmt.Errorf("failed to commit merge: %w", err)
		}

		result.Success = true
		result.Changes = append(result.Changes, "Reassigned all seasons to primary show")
		result.Changes = append(result.Changes, "Deleted duplicate show")

		log.Printf("INFO: Successfully merged TV show '%s' into '%s'", duplicate.Title, primary.Title)
	} else if dryRun {
		result.Success = true
		result.Changes = append(result.Changes, "DRY RUN: No changes made")
	}

	return result, nil
}

// MergeResult represents the result of a merge operation
type MergeResult struct {
	PrimaryID   string   `json:"primary_id"`
	DuplicateID string   `json:"duplicate_id"`
	Changes     []string `json:"changes"`
	Success     bool     `json:"success"`
	DryRun      bool     `json:"dry_run"`
	Error       string   `json:"error,omitempty"`
}

// AutoMergeSafeCandidates automatically merges candidates with very high confidence
func (dm *DuplicationManager) AutoMergeSafeCandidates(confidenceThreshold float64) ([]MergeResult, error) {
	candidates, err := dm.GetMergeCandidates(confidenceThreshold)
	if err != nil {
		return nil, fmt.Errorf("failed to get merge candidates: %w", err)
	}

	var results []MergeResult

	for _, candidate := range candidates {
		// Only auto-merge if confidence is very high and conditions are met
		if candidate.Confidence >= confidenceThreshold {
			// Additional safety checks
			if dm.isSafeToMerge(candidate) {
				result, err := dm.MergeShows(candidate.PrimaryShow.ID, candidate.DuplicateShow.ID, false)
				if err != nil {
					result = &MergeResult{
						PrimaryID:   candidate.PrimaryShow.ID,
						DuplicateID: candidate.DuplicateShow.ID,
						Success:     false,
						Error:       err.Error(),
					}
				}
				results = append(results, *result)
			} else {
				log.Printf("INFO: Skipping auto-merge for shows '%s' and '%s' - not safe", 
					candidate.PrimaryShow.Title, candidate.DuplicateShow.Title)
			}
		}
	}

	return results, nil
}

// isSafeToMerge performs additional safety checks before auto-merging
func (dm *DuplicationManager) isSafeToMerge(candidate MergeCandidate) bool {
	// Don't auto-merge if titles are too different
	similarity := dm.calculateStringSimilarity(candidate.PrimaryShow.Title, candidate.DuplicateShow.Title)
	if similarity < 0.9 {
		return false
	}

	// Don't auto-merge if one has TMDB ID and the other has a different one
	if candidate.PrimaryShow.TmdbID != "" && candidate.DuplicateShow.TmdbID != "" {
		if candidate.PrimaryShow.TmdbID != candidate.DuplicateShow.TmdbID {
			return false
		}
	}

	// Don't auto-merge if air dates are significantly different
	if candidate.PrimaryShow.FirstAirDate != nil && candidate.DuplicateShow.FirstAirDate != nil {
		timeDiff := candidate.PrimaryShow.FirstAirDate.Sub(*candidate.DuplicateShow.FirstAirDate)
		if timeDiff.Abs() > 365*24*time.Hour { // More than 1 year difference
			return false
		}
	}

	return true
} 