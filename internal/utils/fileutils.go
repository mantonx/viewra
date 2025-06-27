// Package utils provides file system utilities and media file handling functions.
// This package contains optimized file operations, hashing utilities, and media type detection
// designed for high-performance media scanning and processing.
package utils

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"hash/fnv"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MediaExtensions contains supported media file extensions.
// This map is used by IsMediaFile to quickly determine if a file should be processed.
// The map includes common video and audio formats that Viewra can handle.
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

// SkippedExtensions contains file extensions that should never be processed.
// These are typically system files, previews, thumbnails, or other non-media files
// that media servers generate but should not be scanned as actual media content.
// This helps avoid duplicate processing and improves scanning performance.
var SkippedExtensions = map[string]bool{
	// Trickplay and preview files (Plex, Jellyfin, Emby, etc.)
	".bif":        true, // Roku/Plex trickplay files
	".vtt":        true, // WebVTT subtitle files (often trickplay metadata)
	".storyboard": true, // Storyboard preview files
	".chapter":    true, // Chapter thumbnail files
	".thumbnail":  true, // Thumbnail files
	".preview":    true, // Preview image files
	".sprite":     true, // Sprite sheet files for trickplay
	".keyframe":   true, // Keyframe extraction files
	".scene":      true, // Scene detection files
	".timeline":   true, // Timeline preview files

	// Media server metadata and cache files
	".nfo":        true, // Media info files (Kodi, Plex, etc.)
	".xml":        true, // Metadata files
	".plist":      true, // Property list files (macOS media metadata)
	".meta":       true, // Generic metadata files
	".info":       true, // Info files
	".dat":        true, // Data files (often cache)
	".cache":      true, // Cache files
	".index":      true, // Index files
	".temp":       true, // Temporary files
	".tmp":        true, // Temporary files
	".part":       true, // Partial download files
	".crdownload": true, // Chrome partial downloads
	".download":   true, // Generic partial downloads

	// Subtitle files (not media content)
	".srt": true, // SubRip subtitle files
	".sub": true, // MicroDVD subtitle files
	".idx": true, // VobSub subtitle index files
	".ass": true, // Advanced SubStation Alpha subtitle files
	".ssa": true, // SubStation Alpha subtitle files
	".sup": true, // Blu-ray PGS subtitle files
	".usf": true, // Universal Subtitle Format
	".smi": true, // SAMI subtitle files
	".rt":  true, // RealText subtitle files
	".sbv": true, // SubViewer subtitle files

	// Thumbnail and preview files
	".jpg":  true, // Often thumbnails or cover art
	".jpeg": true, // Often thumbnails or cover art
	".png":  true, // Often thumbnails or cover art
	".gif":  true, // Often thumbnails or animated previews
	".bmp":  true, // Bitmap images
	".webp": true, // Web images
	".tiff": true, // Image files
	".tif":  true, // Image files
	".svg":  true, // Vector graphics
	".ico":  true, // Icon files
	".psd":  true, // Photoshop files
	".ai":   true, // Adobe Illustrator files
	".eps":  true, // Encapsulated PostScript files

	// System and metadata files
	".txt":        true, // Text files, often logs or metadata
	".log":        true, // Log files
	".db":         true, // Database files
	".db-journal": true, // SQLite journal files
	".db-wal":     true, // SQLite WAL files
	".db-shm":     true, // SQLite shared memory files
	".json":       true, // JSON metadata files
	".yml":        true, // YAML configuration files
	".yaml":       true, // YAML configuration files
	".ini":        true, // Configuration files
	".cfg":        true, // Configuration files
	".conf":       true, // Configuration files
	".config":     true, // Configuration files
	".properties": true, // Properties files

	// Lock and temporary files
	".lock":   true, // Lock files
	".lck":    true, // Lock files
	".backup": true, // Backup files
	".bak":    true, // Backup files
	".old":    true, // Old backup files
	".orig":   true, // Original backup files
	".swp":    true, // Vim swap files
	".swo":    true, // Vim swap files
	".~":      true, // Temporary files

	// Archive files (not media)
	".zip":  true, // Archive files
	".rar":  true, // Archive files
	".7z":   true, // Archive files
	".tar":  true, // Archive files
	".gz":   true, // Compressed files
	".bz2":  true, // Compressed files
	".xz":   true, // Compressed files
	".z":    true, // Compressed files
	".lz":   true, // Compressed files
	".lzma": true, // Compressed files
	".cab":  true, // Cabinet files
	".dmg":  true, // macOS disk images
	".iso":  true, // Disk images (unless specifically for media)
}

// CalculateFileHash calculates SHA1 hash of a file.
// This function reads the entire file to compute the hash, which can be slow for large files.
// For better performance with large media files, consider using CalculateFileHashFast or CalculateFileHashSampled.
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

// CalculateFileHashIfNeeded calculates SHA1 hash only if the file has changed.
// Returns the existing hash if file hasn't changed based on size and modification time.
// This optimization avoids rehashing files that haven't been modified, significantly
// improving scan performance for large media libraries.
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

// CalculateFileHashFast calculates SHA1 hash using a larger buffer for better performance.
// Uses a 64KB buffer instead of the default 32KB, which improves I/O efficiency
// for sequential reads on modern storage systems.
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

// CalculateFileHashUltraFast calculates a hash optimized for very large files (10GB+).
// Uses minimal sampling for maximum speed on large media files.
//
// The function samples strategic positions (start, 25%, 75%) to create a unique
// fingerprint while minimizing I/O. This is suitable when speed is critical
// and perfect hash accuracy is not required.
//
// Includes NFS retry logic for network storage reliability.
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

// CalculateFileHashSampled calculates a hash by sampling parts of large files.
// This is much faster for large files while still providing good uniqueness.
//
// Samples the beginning, middle, and end of the file (1MB each) to create
// a fingerprint that balances speed and uniqueness. The file size is also
// included in the hash to differentiate files of different sizes.
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

// IsMediaFile checks if a file has a supported media extension.
// Returns true if the file extension is in the MediaExtensions map.
// This function includes debug output for troubleshooting scanning issues.
func IsMediaFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	result := MediaExtensions[ext]
	fmt.Printf("[DEBUG] IsMediaFile: path=%s, ext=%s, result=%t\n", filePath, ext, result)
	return result
}

// IsSkippedFile returns true if a file should be skipped during scanning
// based on its extension being in the SkippedExtensions list.
// This helps avoid scanning trickplay files, thumbnails, metadata, and other
// non-media files generated by media servers.
func IsSkippedFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return SkippedExtensions[ext]
}

// IsMediaFileOptimized returns true if a file is a supported media file and should be processed.
// This also checks that the file is not in the skipped extensions list.
//
// Performance optimized version that avoids string allocations and uses
// inline lowercase conversion. Falls back to a switch statement for
// extensions not in the main maps.
func IsMediaFileOptimized(path string) bool {
	// Get extension without allocating new string
	lastDot := strings.LastIndexByte(path, '.')
	if lastDot == -1 || lastDot == len(path)-1 {
		return false
	}

	// Convert to lowercase inline for comparison
	ext := strings.ToLower(path[lastDot:])

	// First check if it's explicitly skipped
	if SkippedExtensions[ext] {
		return false
	}

	// Check if it's a supported media extension
	if MediaExtensions[ext] {
		return true
	}

	// Detailed extension check for audio/video formats
	switch ext {
	// Audio formats
	case ".mp3", ".m4a", ".flac", ".wav", ".ogg", ".opus", ".aac", ".wma", ".alac", ".ape", ".dsd", ".dsf":
		return true
	// Video formats
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".mpg", ".mpeg", ".3gp", ".ogv":
		return true
	default:
		return false
	}
}

// GetContentType returns the appropriate MIME content type for a file extension.
// Used for HTTP responses when serving media files. Returns "application/octet-stream"
// for unknown file types.
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

// NewFastHash returns a new FNV-1a 64-bit hash.
// FNV (Fowler-Noll-Vo) is a fast, non-cryptographic hash function
// suitable for hash tables and checksums where speed is important.
func NewFastHash() hash.Hash64 {
	return fnv.New64a()
}

// SumString returns the hex string representation of the hash.
// Converts the 64-bit hash value to a hexadecimal string.
func SumString(h hash.Hash64) string {
	return fmt.Sprintf("%x", h.Sum64())
}

// IsTrickplayFile returns true if a file appears to be a trickplay, preview, or thumbnail file.
// This includes both extension-based and filename pattern-based detection.
//
// Trickplay files are generated by media servers (Plex, Jellyfin, Emby) for
// video scrubbing previews. These should be excluded from media scanning
// to avoid duplicate entries and improve performance.
func IsTrickplayFile(filePath string) bool {
	// First check extension
	if IsSkippedFile(filePath) {
		return true
	}

	// Check filename patterns commonly used by media servers
	filename := strings.ToLower(filepath.Base(filePath))
	trickplayPatterns := []string{
		"trickplay", "preview", "thumbnail", "sprite", "chapter",
		"keyframe", "timeline", "storyboard", "scene", "frame",
		"bif", "-thumb", "_thumb", ".thumb", "poster", "fanart",
		"banner", "logo", "clearart", "landscape", "disc",
	}

	for _, pattern := range trickplayPatterns {
		if strings.Contains(filename, pattern) {
			return true
		}
	}

	return false
}

// IsTrickplayDirectory returns true if a directory appears to contain trickplay files.
// Checks common directory naming patterns used by media servers for storing
// generated content like thumbnails, previews, and metadata.
func IsTrickplayDirectory(dirPath string) bool {
	dirName := strings.ToLower(filepath.Base(dirPath))
	trickplayDirPatterns := []string{
		"trickplay", "previews", "thumbnails", "sprites", "chapters",
		"keyframes", "timeline", "storyboard", "scenes", "frames",
		"metadata", "cache", "temp", ".plex", ".emby", ".jellyfin",
		"artwork", "fanart", "posters", "banners",
	}

	for _, pattern := range trickplayDirPatterns {
		if strings.Contains(dirName, pattern) {
			return true
		}
	}

	return false
}

// TrickplayStats contains statistics about trickplay files found during directory analysis.
// Used for debugging and optimizing scan performance by identifying directories
// with high concentrations of non-media files.
type TrickplayStats struct {
	TrickplayFiles       int   `json:"trickplay_files"`
	TrickplayDirectories int   `json:"trickplay_directories"`
	TotalFilesScanned    int   `json:"total_files_scanned"`
	TotalDirsScanned     int   `json:"total_dirs_scanned"`
	SkippedBytes         int64 `json:"skipped_bytes"`
}

// AnalyzeTrickplayInDirectory scans a directory and returns statistics about trickplay content.
// This function is useful for analyzing media libraries to understand the volume
// of generated content and optimize scanning strategies.
//
// The function continues on errors to provide best-effort statistics even
// when some files or directories cannot be accessed.
func AnalyzeTrickplayInDirectory(dirPath string) (*TrickplayStats, error) {
	stats := &TrickplayStats{}

	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		if d.IsDir() {
			stats.TotalDirsScanned++
			if IsTrickplayDirectory(path) {
				stats.TrickplayDirectories++
			}
			return nil
		}

		stats.TotalFilesScanned++
		if IsTrickplayFile(path) {
			stats.TrickplayFiles++
			if info, err := d.Info(); err == nil {
				stats.SkippedBytes += info.Size()
			}
		}

		return nil
	})

	return stats, err
}
