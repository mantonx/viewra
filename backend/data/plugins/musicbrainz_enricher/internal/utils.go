// Package internal provides utility functions for the MusicBrainz enricher plugin.
package internal

import (
	"crypto/md5"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// IsAudioFile checks if a file is an audio file based on its extension
func IsAudioFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	audioExts := []string{
		".mp3", ".flac", ".m4a", ".aac", ".ogg", 
		".wav", ".wma", ".opus", ".ape", ".wv",
	}

	for _, audioExt := range audioExts {
		if ext == audioExt {
			return true
		}
	}
	return false
}

// GenerateQueryHash generates a hash for caching API queries
func GenerateQueryHash(query string) string {
	hash := md5.Sum([]byte(query))
	return fmt.Sprintf("%x", hash)
}

// CleanString removes extra whitespace and normalizes a string
func CleanString(s string) string {
	// Trim whitespace
	s = strings.TrimSpace(s)
	
	// Replace multiple spaces with single space
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// FormatDuration formats a duration in milliseconds to a human-readable string
func FormatDuration(milliseconds int) string {
	if milliseconds <= 0 {
		return ""
	}
	
	duration := time.Duration(milliseconds) * time.Millisecond
	
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60
	
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// ExtractMetadataString safely extracts a string value from metadata map
func ExtractMetadataString(metadata map[string]interface{}, key string) string {
	if value, exists := metadata[key]; exists {
		if str, ok := value.(string); ok {
			return CleanString(str)
		}
	}
	return ""
}

// ExtractMetadataInt safely extracts an integer value from metadata map
func ExtractMetadataInt(metadata map[string]interface{}, key string) int {
	if value, exists := metadata[key]; exists {
		switch v := value.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		case string:
			// Try to parse string as int
			if len(v) > 0 {
				var result int
				if _, err := fmt.Sscanf(v, "%d", &result); err == nil {
					return result
				}
			}
		}
	}
	return 0
}

// SanitizeFilename removes or replaces characters that are invalid in filenames
func SanitizeFilename(filename string) string {
	// Replace invalid characters with underscores
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	result := filename
	
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}
	
	// Remove leading/trailing dots and spaces
	result = strings.Trim(result, ". ")
	
	// Limit length
	if len(result) > 200 {
		result = result[:200]
	}
	
	return result
}

// ValidateStringLength checks if a string is within acceptable length limits
func ValidateStringLength(s string, maxLength int) bool {
	return len(s) <= maxLength
}

// TruncateString truncates a string to the specified length with ellipsis
func TruncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	
	if maxLength <= 3 {
		return s[:maxLength]
	}
	
	return s[:maxLength-3] + "..."
}

// ParseYear extracts a year from various date formats
func ParseYear(dateStr string) int {
	if len(dateStr) < 4 {
		return 0
	}
	
	// Try to extract first 4 digits
	yearStr := dateStr[:4]
	var year int
	if _, err := fmt.Sscanf(yearStr, "%d", &year); err == nil {
		// Validate year range
		if year >= 1900 && year <= time.Now().Year()+1 {
			return year
		}
	}
	
	return 0
}

// MergeStringSlices merges multiple string slices and removes duplicates
func MergeStringSlices(slices ...[]string) []string {
	seen := make(map[string]bool)
	var result []string
	
	for _, slice := range slices {
		for _, item := range slice {
			if item != "" && !seen[item] {
				seen[item] = true
				result = append(result, item)
			}
		}
	}
	
	return result
}

// CalculatePercentage calculates percentage with proper rounding
func CalculatePercentage(part, total int) float64 {
	if total == 0 {
		return 0.0
	}
	return float64(part) / float64(total) * 100.0
} 