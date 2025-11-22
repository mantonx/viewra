package images

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dhowden/tag"
	"github.com/mantonx/viewra/internal/domain/images"
	"github.com/zeebo/xxh3"
)

// EmbeddedExtractor handles extraction of artwork embedded in audio files (ID3/APIC tags)
type EmbeddedExtractor struct{}

// NewEmbeddedExtractor creates a new embedded artwork extractor
func NewEmbeddedExtractor() *EmbeddedExtractor {
	return &EmbeddedExtractor{}
}

// ExtractFromAudioFile extracts embedded artwork from an audio file's ID3 tags
// Returns nil if no embedded artwork is found
func (e *EmbeddedExtractor) ExtractFromAudioFile(audioFilePath string, imageType images.ImageType) (*ImageInfo, error) {
	// Open the audio file
	file, err := os.Open(audioFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close()

	// Read ID3/Vorbis/APE tags
	m, err := tag.ReadFrom(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read tags: %w", err)
	}

	// Extract picture (APIC frame for ID3, METADATA_BLOCK_PICTURE for FLAC, etc.)
	picture := m.Picture()
	if picture == nil {
		return nil, nil // No embedded artwork found
	}

	// We need to save the embedded image to a temporary location
	// so it can be processed by the normal image pipeline (metadata extraction, caching, etc.)
	tempDir := filepath.Join(os.TempDir(), "viewra-embedded-artwork")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Generate a unique filename based on the image content hash (XXH3-128: 50x faster than SHA256, zero collision risk)
	hasher := xxh3.New()
	hasher.Write(picture.Data)
	hash128 := hasher.Sum128()
	hashBytes := hash128.Bytes()
	hashStr := hex.EncodeToString(hashBytes[:])

	// Determine file extension from MIME type or Picture.Ext
	ext := picture.Ext
	if ext == "" {
		// Fallback to common extensions based on MIME type
		switch picture.MIMEType {
		case "image/jpeg":
			ext = ".jpg"
		case "image/png":
			ext = ".png"
		case "image/gif":
			ext = ".gif"
		case "image/webp":
			ext = ".webp"
		default:
			ext = ".jpg" // Default fallback
		}
	}
	if ext[0] != '.' {
		ext = "." + ext
	}

	// Create temporary file path
	tempFilePath := filepath.Join(tempDir, hashStr+ext)

	// Write the image data to the temporary file (only if it doesn't exist)
	if _, err := os.Stat(tempFilePath); os.IsNotExist(err) {
		if err := os.WriteFile(tempFilePath, picture.Data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write embedded image: %w", err)
		}
	}

	// Return ImageInfo pointing to the temporary file
	return &ImageInfo{
		Path:     tempFilePath,
		Type:     imageType,
		Priority: 1, // Lower priority than filesystem images (which use 0)
	}, nil
}

// ExtractAlbumArtFromFirstTrack extracts album artwork from the first audio file in a directory
// This is used as a fallback when no folder.jpg/cover.jpg is found
func (e *EmbeddedExtractor) ExtractAlbumArtFromFirstTrack(albumDir string) (*ImageInfo, error) {
	// Find the first audio file in the album directory
	audioFile, err := e.findFirstAudioFile(albumDir)
	if err != nil {
		return nil, err
	}
	if audioFile == "" {
		return nil, nil // No audio files found
	}

	return e.ExtractFromAudioFile(audioFile, images.ImageTypeCover)
}

// ExtractArtistArtFromFirstTrack extracts artist artwork from the first audio file in any album
// This is used as a fallback when no artist-level folder.jpg is found
func (e *EmbeddedExtractor) ExtractArtistArtFromFirstTrack(artistDir string) (*ImageInfo, error) {
	// Find the first audio file in any subdirectory
	audioFile, err := e.findFirstAudioFileRecursive(artistDir)
	if err != nil {
		return nil, err
	}
	if audioFile == "" {
		return nil, nil // No audio files found
	}

	return e.ExtractFromAudioFile(audioFile, images.ImageTypeFolder)
}

// findFirstAudioFile finds the first audio file in a directory (non-recursive)
func (e *EmbeddedExtractor) findFirstAudioFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	audioExts := map[string]bool{
		".mp3":  true,
		".flac": true,
		".m4a":  true,
		".aac":  true,
		".ogg":  true,
		".opus": true,
		".wma":  true,
		".wav":  true,
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if audioExts[ext] {
			return filepath.Join(dir, entry.Name()), nil
		}
	}

	return "", nil // No audio files found
}

// findFirstAudioFileRecursive finds the first audio file in a directory tree
func (e *EmbeddedExtractor) findFirstAudioFileRecursive(dir string) (string, error) {
	var foundFile string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if foundFile != "" {
			return filepath.SkipDir // Already found, skip rest
		}

		if info.IsDir() {
			return nil
		}

		audioExts := map[string]bool{
			".mp3":  true,
			".flac": true,
			".m4a":  true,
			".aac":  true,
			".ogg":  true,
			".opus": true,
			".wma":  true,
			".wav":  true,
		}

		ext := filepath.Ext(path)
		if audioExts[ext] {
			foundFile = path
			return filepath.SkipDir // Stop walking
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return foundFile, nil
}
