package filesystem

import (
	"os"
	"path/filepath"
	"strings"
)

// normalizeExtension returns the lowercase extension with leading dot
func normalizeExtension(path string) string {
	return strings.ToLower(filepath.Ext(path))
}

// hasExtension checks if a file path has any of the given extensions
func hasExtension(path string, extensions []string) bool {
	ext := normalizeExtension(path)
	for _, validExt := range extensions {
		if ext == validExt {
			return true
		}
	}
	return false
}

// containsAny checks if a string contains any of the given substrings
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// openFile opens a file and returns both the file and a cleanup function
// This helps ensure files are always closed properly
func openFile(path string) (*os.File, func(), error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		file.Close()
	}
	return file, cleanup, nil
}
