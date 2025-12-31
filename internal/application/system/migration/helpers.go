package migration

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// generateMigrationID creates a unique migration identifier.
func generateMigrationID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "mig_" + hex.EncodeToString(b)
}

// formatDuration formats seconds into a human-readable duration string.
func formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("~%d seconds", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("~%d minutes", minutes)
	}
	hours := minutes / 60
	mins := minutes % 60
	if mins == 0 {
		return fmt.Sprintf("~%d hours", hours)
	}
	return fmt.Sprintf("~%d hours %d minutes", hours, mins)
}
