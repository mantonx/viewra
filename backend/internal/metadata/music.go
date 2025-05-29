package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhowden/tag"
	"github.com/mantonx/viewra/internal/database"
	"github.com/mantonx/viewra/internal/modules/mediaassetmodule"
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
		
		// Store artwork data temporarily - will be saved after MediaFile gets its ID
		musicMeta.ArtworkData = picture.Data
		musicMeta.ArtworkExt = picture.Ext
		
		// If we have the MediaFile ID, save the artwork immediately
		if mediaFile.ID != 0 {
			if err := SaveArtworkForMediaFile(mediaFile.ID, picture.Data, picture.MIMEType); err != nil {
				fmt.Printf("WARNING: Failed to save artwork for file %s: %v\n", filePath, err)
			} else {
				fmt.Printf("Successfully saved artwork for file %s\n", filePath)
			}
		} else {
			fmt.Printf("Stored artwork data for file %s (will save after MediaFile gets ID)\n", filePath)
		}
	} else {
		fmt.Printf("No artwork found in file %s\n", filePath)
	}

	return musicMeta, nil
}

// SaveArtworkForMediaFile saves artwork for a media file using the new asset system
func SaveArtworkForMediaFile(mediaFileID uint, data []byte, mimeType string) error {
	if len(data) == 0 {
		return fmt.Errorf("artwork data cannot be empty")
	}
	
	if mimeType == "" {
		mimeType = "image/jpeg" // Default MIME type
	}
	
	// Create asset request
	request := &mediaassetmodule.AssetRequest{
		MediaFileID: mediaFileID,
		Type:        mediaassetmodule.AssetTypeMusic,
		Category:    mediaassetmodule.CategoryAlbum,
		Subtype:     mediaassetmodule.SubtypeArtwork,
		Data:        data,
		MimeType:    mimeType,
	}

	// Save using the new asset manager
	_, err := mediaassetmodule.SaveMediaAsset(request)
	if err != nil {
		return fmt.Errorf("failed to save artwork with new asset system: %w", err)
	}

	fmt.Printf("INFO: Successfully saved artwork for media file ID %d\n", mediaFileID)
	return nil
}
