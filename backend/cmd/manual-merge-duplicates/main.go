package main

import (
	"fmt"
	"log"

	"github.com/mantonx/viewra/internal/config"
	"github.com/mantonx/viewra/internal/database"
	"gorm.io/gorm"
)

func main() {
	fmt.Println("🧹 Manual TV Show Duplicate Cleanup Tool")
	fmt.Println()

	// Initialize configuration and database
	if err := config.Load(""); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	database.Initialize()
	db := database.GetDB()
	if db == nil {
		log.Fatalf("Failed to get database connection")
	}

	// Find all TV shows grouped by TMDB ID
	fmt.Println("🔍 Analyzing TV show duplicates...")
	duplicateCount, err := cleanupTMDBDuplicates(db)
	if err != nil {
		log.Fatalf("Failed to cleanup TMDB duplicates: %v", err)
	}

	// Find all TV shows grouped by exact title match
	titleDuplicateCount, err := cleanupTitleDuplicates(db)
	if err != nil {
		log.Fatalf("Failed to cleanup title duplicates: %v", err)
	}

	fmt.Printf("\n✅ Cleanup completed!\n")
	fmt.Printf("   - Removed %d TMDB ID duplicates\n", duplicateCount)
	fmt.Printf("   - Removed %d exact title duplicates\n", titleDuplicateCount)
	
	// Show final stats
	var totalShows int64
	if err := db.Model(&database.TVShow{}).Count(&totalShows).Error; err != nil {
		log.Printf("Warning: Failed to count remaining shows: %v", err)
	} else {
		fmt.Printf("   - Remaining TV shows: %d\n", totalShows)
	}
}

func cleanupTMDBDuplicates(db *gorm.DB) (int, error) {
	// Find all shows with TMDB IDs that appear more than once
	var tmdbGroups []struct {
		TmdbID string
		Count  int64
	}

	if err := db.Model(&database.TVShow{}).
		Select("tmdb_id, COUNT(*) as count").
		Where("tmdb_id != '' AND tmdb_id IS NOT NULL").
		Group("tmdb_id").
		Having("COUNT(*) > 1").
		Find(&tmdbGroups).Error; err != nil {
		return 0, fmt.Errorf("failed to find TMDB duplicates: %w", err)
	}

	fmt.Printf("Found %d TMDB ID groups with duplicates\n", len(tmdbGroups))

	deletedCount := 0
	for _, group := range tmdbGroups {
		fmt.Printf("Processing TMDB ID %s (%d duplicates)...\n", group.TmdbID, group.Count)

		// Get all shows with this TMDB ID
		var shows []database.TVShow
		if err := db.Where("tmdb_id = ?", group.TmdbID).Order("created_at ASC").Find(&shows).Error; err != nil {
			log.Printf("Warning: Failed to get shows for TMDB ID %s: %v", group.TmdbID, err)
			continue
		}

		if len(shows) <= 1 {
			continue
		}

		// Keep the first (oldest) show, delete the rest
		primary := shows[0]
		duplicates := shows[1:]

		fmt.Printf("  Keeping: %s (ID: %s, Created: %s)\n", 
			primary.Title, primary.ID, primary.CreatedAt.Format("2006-01-02 15:04:05"))

		for _, duplicate := range duplicates {
			fmt.Printf("  Deleting: %s (ID: %s, Created: %s)\n", 
				duplicate.Title, duplicate.ID, duplicate.CreatedAt.Format("2006-01-02 15:04:05"))

			// Move any seasons to the primary show
			if err := db.Model(&database.Season{}).
				Where("tv_show_id = ?", duplicate.ID).
				Update("tv_show_id", primary.ID).Error; err != nil {
				log.Printf("Warning: Failed to move seasons from %s: %v", duplicate.ID, err)
			}

			// Delete the duplicate
			if err := db.Delete(&database.TVShow{}, "id = ?", duplicate.ID).Error; err != nil {
				log.Printf("Warning: Failed to delete duplicate %s: %v", duplicate.ID, err)
			} else {
				deletedCount++
			}
		}
	}

	return deletedCount, nil
}

func cleanupTitleDuplicates(db *gorm.DB) (int, error) {
	// Find all shows with exact titles that appear more than once (excluding those with TMDB IDs)
	var titleGroups []struct {
		Title string
		Count int64
	}

	if err := db.Model(&database.TVShow{}).
		Select("title, COUNT(*) as count").
		Where("(tmdb_id = '' OR tmdb_id IS NULL)").
		Group("title").
		Having("COUNT(*) > 1").
		Find(&titleGroups).Error; err != nil {
		return 0, fmt.Errorf("failed to find title duplicates: %w", err)
	}

	fmt.Printf("\nFound %d title groups with duplicates (no TMDB ID)\n", len(titleGroups))

	deletedCount := 0
	for _, group := range titleGroups {
		fmt.Printf("Processing title '%s' (%d duplicates)...\n", group.Title, group.Count)

		// Get all shows with this title
		var shows []database.TVShow
		if err := db.Where("title = ? AND (tmdb_id = '' OR tmdb_id IS NULL)", group.Title).
			Order("created_at ASC").Find(&shows).Error; err != nil {
			log.Printf("Warning: Failed to get shows for title '%s': %v", group.Title, err)
			continue
		}

		if len(shows) <= 1 {
			continue
		}

		// Keep the show with the most data, or the first one if they're equal
		primary := shows[0]
		for _, show := range shows[1:] {
			if calculateDataCompleteness(show) > calculateDataCompleteness(primary) {
				primary = show
			}
		}

		// Remove primary from duplicates list
		var duplicates []database.TVShow
		for _, show := range shows {
			if show.ID != primary.ID {
				duplicates = append(duplicates, show)
			}
		}

		fmt.Printf("  Keeping: %s (ID: %s, Data score: %.1f)\n", 
			primary.Title, primary.ID, calculateDataCompleteness(primary))

		for _, duplicate := range duplicates {
			fmt.Printf("  Deleting: %s (ID: %s, Data score: %.1f)\n", 
				duplicate.Title, duplicate.ID, calculateDataCompleteness(duplicate))

			// Move any seasons to the primary show
			if err := db.Model(&database.Season{}).
				Where("tv_show_id = ?", duplicate.ID).
				Update("tv_show_id", primary.ID).Error; err != nil {
				log.Printf("Warning: Failed to move seasons from %s: %v", duplicate.ID, err)
			}

			// Delete the duplicate
			if err := db.Delete(&database.TVShow{}, "id = ?", duplicate.ID).Error; err != nil {
				log.Printf("Warning: Failed to delete duplicate %s: %v", duplicate.ID, err)
			} else {
				deletedCount++
			}
		}
	}

	return deletedCount, nil
}

func calculateDataCompleteness(show database.TVShow) float64 {
	score := 0.0

	if show.TmdbID != "" {
		score += 3.0
	}
	if show.Description != "" && len(show.Description) > 10 {
		score += 2.0
	}
	if show.FirstAirDate != nil {
		score += 1.5
	}
	if show.Poster != "" {
		score += 1.0
	}
	if show.Status != "" && show.Status != "Unknown" {
		score += 0.5
	}
	if show.Backdrop != "" {
		score += 0.5
	}

	return score
} 