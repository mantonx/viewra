package common

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// NormalizeSortTitle creates a normalized version of a title for sorting purposes.
// It:
// 1. Removes accents and diacritics (à → a, é → e, etc.)
// 2. Removes leading special characters ('Round Midnight → Round Midnight)
// 3. Removes leading articles ("The", "A", "An", "Le", "La", "Les", "El", "Los", etc.)
// 4. Converts to lowercase
// 5. Trims whitespace
//
// This function should be used when setting sort_title for movies, TV shows, and music.
func NormalizeSortTitle(title string) string {
	if title == "" {
		return ""
	}

	// Step 1: Remove accents/diacritics first (so "À" becomes "A" before article detection)
	normalized := removeAccents(title)

	// Step 2: Remove leading special characters (apostrophes, parentheses, ellipsis, etc.)
	normalized = removeLeadingSpecialChars(normalized)

	// Step 3: Remove leading articles
	normalized = removeLeadingArticles(normalized)

	// Step 4: Lowercase and trim
	normalized = strings.ToLower(strings.TrimSpace(normalized))

	return normalized
}

// removeLeadingArticles removes common articles from the beginning of titles
func removeLeadingArticles(title string) string {
	// Common articles in multiple languages
	articles := []string{
		"The ", "A ", "An ",     // English
		"Le ", "La ", "Les ",    // French
		"El ", "La ", "Los ",    // Spanish
		"Der ", "Die ", "Das ",  // German
		"Il ", "Lo ", "La ",     // Italian
		"De ", "Het ",           // Dutch
		"O ", "A ", "Os ", "As ", // Portuguese
	}

	for _, article := range articles {
		if strings.HasPrefix(title, article) {
			return strings.TrimPrefix(title, article)
		}
		// Also check lowercase version
		if strings.HasPrefix(strings.ToLower(title), strings.ToLower(article)) {
			return title[len(article):]
		}
	}

	return title
}

// removeLeadingSpecialChars removes leading non-alphanumeric characters
// This ensures titles like "'Round Midnight" sort under "R" instead of before "A"
// We strip punctuation/symbols but keep numbers, so "(500) Days" becomes "500) Days" → handled by next step
// But "'Round Midnight" becomes "Round Midnight"
func removeLeadingSpecialChars(s string) string {
	// First, check if there are any letters in the string at all
	hasLetters := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetters = true
			break
		}
	}

	// If there are letters, strip until we hit a letter (removing numbers and symbols)
	// This makes "(500) Days" → "Days" which is what we want for sorting
	if hasLetters {
		return strings.TrimLeftFunc(s, func(r rune) bool {
			return !unicode.IsLetter(r)
		})
	}

	// If there are no letters (pure numeric titles like "10", "300", "1917")
	// just strip non-alphanumeric characters (symbols/punctuation only)
	// This makes "300" → "300" (keeps it) and "50/50" → "50/50" (keeps numbers)
	return strings.TrimLeftFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

// removeAccents removes diacritical marks from text
// Uses Unicode normalization to decompose characters, then removes combining marks
func removeAccents(s string) string {
	// Normalize to NFD (decomposed form) where accents are separate combining characters
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)
	return result
}

// NormalizeTitle normalizes a title for deduplication purposes.
// Handles punctuation variants like:
// - "Star Trek: Voyager" vs "Star Trek Voyager"
// - "Are You Afraid of the Dark?" vs "Are You Afraid of the Dark!"
// - "Giri/Haji" vs "Giri+Haji"
// This is used to ensure consistent title matching regardless of punctuation differences.
func NormalizeTitle(title string) string {
	if title == "" {
		return ""
	}

	// Remove common punctuation that varies between sources
	replacer := strings.NewReplacer(
		":", "",  // Colons (Star Trek: Voyager → Star Trek Voyager)
		"-", " ", // Hyphens to space (Spider-Man → Spider Man)
		"–", " ", // En-dash
		"—", " ", // Em-dash
		"/", " ", // Slashes to space (Giri/Haji → Giri Haji)
		"+", " ", // Plus to space (Giri+Haji → Giri Haji)
		"&", "and", // Ampersand
		"'", "",  // Apostrophes
		"'", "",  // Smart apostrophe
		".", "",  // Periods
		",", "",  // Commas
		"?", "",  // Question marks (Are You Afraid of the Dark? → Are You Afraid of the Dark)
		"!", "",  // Exclamation marks
		"\"", "", // Double quotes
		"(", "",  // Opening parentheses
		")", "",  // Closing parentheses
	)
	normalized := replacer.Replace(title)

	// Collapse multiple spaces
	for strings.Contains(normalized, "  ") {
		normalized = strings.ReplaceAll(normalized, "  ", " ")
	}

	return strings.TrimSpace(normalized)
}
