package format

import "fmt"

// Bytes formats a byte count as a human-readable string (e.g., "10.5 GB").
func Bytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB",
		float64(bytes)/float64(div),
		"KMGTPE"[exp])
}
