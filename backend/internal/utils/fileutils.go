package utils

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MediaExtensions contains supported media file extensions
var MediaExtensions = map[string]bool{
	// Video formats
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".wmv":  true,
	".flv":  true,
	".webm": true,
	".m4v":  true,
	".3gp":  true,
	".ogv":  true,

	// Audio formats
	".mp3":  true,
	".wav":  true,
	".flac": true,
	".aac":  true,
	".ogg":  true,
	".wma":  true,
	".m4a":  true,
	".opus": true,
	".aiff": true,
}

// CalculateFileHash calculates SHA1 hash of a file
func CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha1.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// CalculateFileHashIfNeeded calculates SHA1 hash only if the file has changed
// Returns the existing hash if file hasn't changed based on size and modification time
func CalculateFileHashIfNeeded(filePath string, existingHash string, existingSize int64, existingModTime time.Time) (string, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	// If size and modification time haven't changed, return existing hash
	if existingHash != "" && fileInfo.Size() == existingSize && !fileInfo.ModTime().After(existingModTime) {
		return existingHash, nil
	}

	// File has changed, calculate new hash
	return CalculateFileHash(filePath)
}

// CalculateFileHashFast calculates SHA1 hash using a larger buffer for better performance
func CalculateFileHashFast(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha1.New()
	// Use a larger buffer (64KB) for better I/O performance
	buffer := make([]byte, 65536)

	for {
		n, err := file.Read(buffer)
		if n > 0 {
			hasher.Write(buffer[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// IsMediaFile checks if a file has a supported media extension
func IsMediaFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return MediaExtensions[ext]
}

// GetContentType returns the appropriate content type for a file extension
func GetContentType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	// Try MIME type detection first
	contentType := ""
	if ct := getBasicContentType(ext); ct != "" {
		contentType = ct
	}

	// Fallback to specific mappings for audio formats
	if contentType == "" {
		switch ext {
		case ".mp3":
			contentType = "audio/mpeg"
		case ".wav":
			contentType = "audio/wav"
		case ".flac":
			contentType = "audio/flac"
		case ".ogg":
			contentType = "audio/ogg"
		case ".m4a":
			contentType = "audio/mp4"
		case ".aac":
			contentType = "audio/aac"
		default:
			contentType = "application/octet-stream"
		}
	}

	return contentType
}

// getBasicContentType is a placeholder for mime.TypeByExtension
func getBasicContentType(ext string) string {
	// This would use mime.TypeByExtension in the actual implementation
	return ""
}
