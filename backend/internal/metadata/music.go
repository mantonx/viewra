package metadata

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhowden/tag"
	"github.com/mantonx/viewra/internal/database"
)

// MusicFileExtensions defines supported music file formats (audio only)
var MusicFileExtensions = map[string]bool{
	".mp3":  true,
	".flac": true,
	".m4a":  true,
	".mp4":  true,
	".aac":  true,
	".ogg":  true,
	".wav":  true,
	".wma":  true,
	".ape":  true,
	".mpc":  true,
	".wv":   true,
	".opus": true,
	".aiff": true,
}

// MediaLibraryExtensions defines supported media file formats for music libraries (audio + video)
var MediaLibraryExtensions = map[string]bool{
	// Audio formats
	".mp3":  true,
	".flac": true,
	".m4a":  true,
	".aac":  true,
	".ogg":  true,
	".wav":  true,
	".wma":  true,
	".ape":  true,
	".mpc":  true,
	".wv":   true,
	".opus": true,
	".aiff": true,
	
	// Video formats (for music videos, concerts, etc.)
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
}

// IsMusicFile checks if a file is a supported music format (audio only)
func IsMusicFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return MusicFileExtensions[ext]
}

// IsMediaLibraryFile checks if a file is supported in a media library (audio + video)
func IsMediaLibraryFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return MediaLibraryExtensions[ext]
}

// ExtractMusicMetadata extracts metadata from a music file and saves artwork
func ExtractMusicMetadata(filePath string, mediaFile *database.MediaFile) (*database.MusicMetadata, error) {
	fmt.Printf("Extracting metadata from: %s\n", filePath)
	
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening file %s: %v\n", filePath, err)
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Extract metadata using the tag library
	metadata, err := tag.ReadFrom(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	// Create MusicMetadata instance
	musicMeta := &database.MusicMetadata{
		MediaFileID: mediaFile.ID,
		Title:       metadata.Title(),
		Album:       metadata.Album(),
		Artist:      metadata.Artist(),
		AlbumArtist: metadata.AlbumArtist(),
		Genre:       metadata.Genre(),
		Format:      strings.ToLower(filepath.Ext(filePath)[1:]), // Remove the dot
	}

	// Handle track and disc numbers which return multiple values
	trackNum, trackTotal := metadata.Track()
	musicMeta.Track = trackNum
	musicMeta.TrackTotal = trackTotal

	discNum, discTotal := metadata.Disc()
	musicMeta.Disc = discNum
	musicMeta.DiscTotal = discTotal

	// Handle year
	if metadata.Year() != 0 {
		musicMeta.Year = metadata.Year()
	}

	// Handle duration (if available)
	// Note: The tag library doesn't provide duration, we might need to use a different library
	// or skip this for now
	
	// Handle artwork
	picture := metadata.Picture()
	if picture != nil && len(picture.Data) > 0 {
		musicMeta.HasArtwork = true
		fmt.Printf("Found artwork in file %s with MIME type: %s\n", filePath, picture.MIMEType)
		
		// Determine extension if not provided
		ext := picture.Ext
		if ext == "" {
			// Try to determine from MIME type
			switch picture.MIMEType {
			case "image/jpeg":
				ext = "jpg"
			case "image/png":
				ext = "png"
			case "image/gif":
				ext = "gif"
			default:
				ext = "jpg" // Default to jpg
			}
		}
		
		// Store artwork data temporarily - will be saved after MediaFile gets its ID
		musicMeta.ArtworkData = picture.Data
		musicMeta.ArtworkExt = ext
		fmt.Printf("Stored artwork data for file %s (will save after MediaFile gets ID)\n", filePath)
	} else {
		fmt.Printf("No artwork found in file %s\n", filePath)
	}

	return musicMeta, nil
}

// SaveArtwork saves album artwork to the cache directory
func SaveArtwork(mediaFileID uint, data []byte, ext string) error {
	// Create cache directory if it doesn't exist
	// Support both Docker path and local development path
	cacheDir := "./data/artwork"
	if _, err := os.Stat("/app/data"); err == nil {
		cacheDir = "/app/data/artwork"
	}
	err := os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Generate filename based on media file ID and data hash
	hash := md5.Sum(data)
	filename := fmt.Sprintf("%d_%x.%s", mediaFileID, hash, ext)
	artworkPath := filepath.Join(cacheDir, filename)

	// Save the artwork file
	file, err := os.Create(artworkPath)
	if err != nil {
		return fmt.Errorf("failed to create artwork file: %w", err)
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write artwork data: %w", err)
	}

	return nil
}

// SaveArtworkWithID saves album artwork with a specific media file ID
// This is used when the MediaFile has already been saved to the database
func SaveArtworkWithID(mediaFileID uint, data []byte, ext string) error {
	return SaveArtwork(mediaFileID, data, ext)
}

// GetArtworkPath returns the path to the artwork file for a given media file ID
func GetArtworkPath(mediaFileID uint) (string, error) {
	// Support both Docker path and local development path
	cacheDir := "./data/artwork"
	if _, err := os.Stat("/app/data"); err == nil {
		cacheDir = "/app/data/artwork"
	}
	
	// Find artwork file by media file ID prefix
	files, err := filepath.Glob(filepath.Join(cacheDir, fmt.Sprintf("%d_*", mediaFileID)))
	if err != nil {
		return "", fmt.Errorf("failed to search for artwork: %w", err)
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no artwork found for media file ID %d", mediaFileID)
	}

	// Return the first match (there should only be one)
	return files[0], nil
}

// CleanupArtwork removes artwork files for a given media file ID
func CleanupArtwork(mediaFileID uint) error {
	// Support both Docker path and local development path
	cacheDir := "./data/artwork"
	if _, err := os.Stat("/app/data"); err == nil {
		cacheDir = "/app/data/artwork"
	}
	
	// Find artwork files by media file ID prefix
	files, err := filepath.Glob(filepath.Join(cacheDir, fmt.Sprintf("%d_*", mediaFileID)))
	if err != nil {
		return fmt.Errorf("failed to search for artwork: %w", err)
	}

	// Remove all matching files
	for _, file := range files {
		err := os.Remove(file)
		if err != nil {
			return fmt.Errorf("failed to remove artwork file %s: %w", file, err)
		}
	}

	return nil
}
