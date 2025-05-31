package utils

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"hash/fnv"
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

// CalculateFileHashUltraFast calculates a hash optimized for very large files (10GB+)
// Uses minimal sampling for maximum speed on large media files
func CalculateFileHashUltraFast(filePath string, fileSize int64) (string, error) {
	// OPTIMIZATION 13: NFS retry logic for network storage reliability
	var file *os.File
	var err error
	
	// Retry file opening for NFS resilience
	for attempts := 0; attempts < 3; attempts++ {
		file, err = os.Open(filePath)
		if err == nil {
			break
		}
		if attempts < 2 {
			time.Sleep(time.Duration(attempts+1) * 50 * time.Millisecond)
		}
	}
	
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()

	// ULTRA-FAST SAMPLING for very large files:
	// Use smaller samples (256KB vs 1MB) for files over 10GB
	// This dramatically reduces I/O while maintaining reasonable uniqueness
	sampleSize := int64(256 * 1024) // 256KB chunks for maximum speed

	// Hash the file size and path for uniqueness
	fmt.Fprintf(hasher, "size:%d:path:%s", fileSize, filepath.Base(filePath))

	// OPTIMIZATION 14: Pre-allocate buffer for better performance
	buffer := make([]byte, sampleSize)

	// Read first chunk (beginning of file)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}
	hasher.Write(buffer[:n])

	// For very large files, sample strategically placed chunks for speed
	if fileSize > sampleSize*4 {
		// Sample at 25% position for content variation
		quarterOffset := fileSize / 4
		_, err = file.Seek(quarterOffset, 0)
		if err != nil {
			return "", err
		}
		
		n, err = file.Read(buffer)
		if err != nil && err != io.EOF {
			return "", err
		}
		hasher.Write(buffer[:n])

		// Sample at 75% position for additional uniqueness
		threeFourthOffset := (fileSize * 3) / 4
		_, err = file.Seek(threeFourthOffset, 0)
		if err != nil {
			return "", err
		}

		n, err = file.Read(buffer)
		if err != nil && err != io.EOF {
			return "", err
		}
		hasher.Write(buffer[:n])
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// CalculateFileHashSampled calculates a hash by sampling parts of large files
// This is much faster for large files while still providing good uniqueness
func CalculateFileHashSampled(filePath string, fileSize int64) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()

	// Sample size: 1MB chunks
	sampleSize := int64(1024 * 1024)

	// Hash the file size first (helps differentiate files of different sizes)
	fmt.Fprintf(hasher, "size:%d", fileSize)

	// Read first chunk
	buffer := make([]byte, sampleSize)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}
	hasher.Write(buffer[:n])

	// Read middle chunk if file is large enough
	if fileSize > sampleSize*3 {
		middleOffset := (fileSize / 2) - (sampleSize / 2)
		_, err = file.Seek(middleOffset, 0)
		if err != nil {
			return "", err
		}

		n, err = file.Read(buffer)
		if err != nil && err != io.EOF {
			return "", err
		}
		hasher.Write(buffer[:n])
	}

	// Read last chunk if file is large enough
	if fileSize > sampleSize*2 {
		lastOffset := fileSize - sampleSize
		if lastOffset < 0 {
			lastOffset = 0
		}

		_, err = file.Seek(lastOffset, 0)
		if err != nil {
			return "", err
		}

		n, err = file.Read(buffer)
		if err != nil && err != io.EOF {
			return "", err
		}
		hasher.Write(buffer[:n])
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// IsMediaFile checks if a file has a supported media extension
func IsMediaFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	result := MediaExtensions[ext]
	fmt.Printf("[DEBUG] IsMediaFile: path=%s, ext=%s, result=%t\n", filePath, ext, result)
	return result
}

// IsMediaFileOptimized checks if a file is a media file using optimized string operations
func IsMediaFileOptimized(path string) bool {
	// Get extension without allocating new string
	lastDot := strings.LastIndexByte(path, '.')
	if lastDot == -1 || lastDot == len(path)-1 {
		return false
	}

	// Convert to lowercase inline for comparison
	ext := path[lastDot+1:]

	// Check against known extensions using a switch for better performance
	switch strings.ToLower(ext) {
	// Audio formats
	case "mp3", "m4a", "flac", "wav", "ogg", "opus", "aac", "wma", "alac", "ape", "dsd", "dsf":
		return true
	// Video formats
	case "mp4", "mkv", "avi", "mov", "wmv", "flv", "webm", "m4v", "mpg", "mpeg", "3gp", "ogv":
		return true
	// Image formats (album art, etc.)
	case "jpg", "jpeg", "png", "gif", "bmp", "webp", "tiff", "svg":
		return true
	default:
		return false
	}
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

// NewFastHash returns a new FNV-1a 64-bit hash
func NewFastHash() hash.Hash64 {
	return fnv.New64a()
}

// SumString returns the hex string of the hash
func SumString(h hash.Hash64) string {
	return fmt.Sprintf("%x", h.Sum64())
}
